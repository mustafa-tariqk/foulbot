package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"foulbot/data"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/inconshreveable/go-update"
)

var (
	VERSION     string
	CONFIG_JSON = "config.json"
	POLL_LENGTH = 16 * time.Hour
	NUMBERS     = []string{":one:", ":two:", ":three:", ":four:", ":five:",
		":six:", ":seven:", ":eight:", ":nine:", ":keycap_ten:"}
)

type Config struct {
	DiscordToken   string `json:"discord_token"`
	DiscordGuildID string `json:"discord_guild_id"`
	DiscordAppID   string `json:"discord_application_id"`
}

var AppId string

func main() {
	bot, guildId, appId := loadEnv()
	AppId = appId

	handleInputs(bot)

	err := bot.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer bot.Close()

	handleExpiredPolls(bot)

	establishCommands(bot, guildId, appId)
	fmt.Println("Bot is running...")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	fmt.Println("Bot is shutting down...")
}

func loadEnv() (*discordgo.Session, string, string) {
	configData, err := os.ReadFile(CONFIG_JSON)
	if err != nil {
		log.Fatalf("could not read config file: %s", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatalf("could not parse config file: %s", err)
	}

	bot, err := discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		log.Fatal(err)
	}

	return bot, config.DiscordGuildID, config.DiscordAppID
}

func handleExpiredPolls(bot *discordgo.Session) {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			evaluatedPolls := data.EvaluatePolls()
			for _, poll := range evaluatedPolls {
				fields := []*discordgo.MessageEmbedField{
					{
						Name:   "Creator",
						Value:  fmt.Sprintf("<@%s>", poll.CreatorId),
						Inline: true,
					},
					{
						Name:   "Gainers",
						Value:  fmt.Sprintf("<@%s>", strings.Join(poll.GainerIds, ">\n<@")),
						Inline: true,
					},
					{
						Name:   "Points",
						Value:  fmt.Sprintf("%+d", poll.Points),
						Inline: true,
					},
					{
						Name:   "Reason",
						Value:  fmt.Sprintf("[%s](https://discord.com/channels/%s/%s/%s)", poll.Reason, bot.State.Guilds[0].ID, poll.ChannelId, poll.MessageId),
						Inline: false,
					},
				}

				if shouldShowVotes() {
					fields = append(fields,
						&discordgo.MessageEmbedField{
							Name: "Votes For",
							Value: func() string {
								if len(poll.VotesFor) == 0 {
									return "none"
								}
								return fmt.Sprintf("<@%s>", strings.Join(poll.VotesFor, ">\n<@"))
							}(),
							Inline: true,
						},
						&discordgo.MessageEmbedField{
							Name: "Votes Against",
							Value: func() string {
								if len(poll.VotesAgainst) == 0 {
									return "none"
								}
								return fmt.Sprintf("<@%s>", strings.Join(poll.VotesAgainst, ">\n<@"))
							}(),
							Inline: true,
						},
					)
				}

				embed := &discordgo.MessageEmbed{
					Title:  map[bool]string{true: "Passed", false: "Failed"}[poll.Passed],
					Color:  0x417e4b, // Green for passed
					Fields: fields,
				}
				if !poll.Passed {
					embed.Color = 0xc94543 // Red for failed
				}

				message, err := bot.ChannelMessageSendEmbed(poll.ChannelId, embed)
				if err != nil {
					log.Printf("Failed to send poll result: %v", err)
				}

				bot.MessageThreadStartComplex(message.ChannelID, message.ID, &discordgo.ThreadStart{
					Name:                "Result",
					AutoArchiveDuration: 60,
				})

				bot.ChannelMessageEditComplex(&discordgo.MessageEdit{
					ID:         poll.MessageId,
					Channel:    poll.ChannelId,
					Components: &[]discordgo.MessageComponent{},
				})

			}
		}
	}()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

var lastPollTime = make(map[string]time.Time)

func handleInputs(bot *discordgo.Session) {
	bot.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			options := i.ApplicationCommandData().Options
			switch i.ApplicationCommandData().Name {
			case "own":
				userID := i.Member.User.ID
				now := time.Now()

				if lastTime, exists := lastPollTime[userID]; exists && now.Sub(lastTime) < 5*time.Minute {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "You can only create one poll every 5 minutes.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}

				lastPollTime[userID] = now

				user := options[0].UserValue(s)
				number := options[1].IntValue()
				reason := options[2].StringValue()

				// create a list of users
				var users []*discordgo.User
				if user != nil {
					users = append(users, user)
				}
				for _, option := range options[3:] {
					if option.Type == discordgo.ApplicationCommandOptionUser {
						if userValue := option.UserValue(s); userValue != nil {
							users = append(users, userValue)
						}
					}
				}

				seen := make(map[string]bool)
				unique := make([]*discordgo.User, 0, len(users))
				for _, user := range users {
					if !seen[user.ID] {
						seen[user.ID] = true
						unique = append(unique, user)
					}
				}
				users = unique

				if number == 0 {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Can't give out 0 points",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}

				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Creating poll...",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})

				expiry := time.Now().Add(POLL_LENGTH).Format(time.RFC3339)

				pollMsg, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title: "Own",
							Fields: []*discordgo.MessageEmbedField{
								{
									Name:   "Creator",
									Value:  fmt.Sprintf("<@%s>", i.Member.User.ID),
									Inline: true,
								},
								{
									Name:   "Gainers",
									Value:  formatUserMentions(users),
									Inline: true,
								},
								{
									Name:   "Points",
									Value:  fmt.Sprintf("%+d", number),
									Inline: true,
								},
								{
									Name:   "Reason",
									Value:  reason,
									Inline: false,
								},
							},
							Timestamp: expiry,
						},
					},
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Style:    discordgo.SuccessButton,
									CustomID: "vote_yes",
									Emoji: &discordgo.ComponentEmoji{
										Name: "\U0001F44D",
									},
								},
								discordgo.Button{
									Style:    discordgo.DangerButton,
									CustomID: "vote_no",
									Emoji: &discordgo.ComponentEmoji{
										Name: "\U0001F44E",
									},
								},
							},
						},
					},
				})
				if err != nil {
					return
				}

				err = createThreadWithTags(s, pollMsg.ChannelID, pollMsg.ID, reason, users)
				if err != nil {
					log.Printf("Thread creation failed: %v", err)
				}

				poll := &data.Poll{
					MessageId: pollMsg.ID,
					ChannelId: i.ChannelID,
					CreatorId: i.Member.User.ID,
					Points:    number,
					Reason:    reason,
					GainerIds: func() []string {
						ids := make([]string, len(users))
						for i, user := range users {
							ids[i] = user.ID
						}
						return ids
					}(),
					Expiry: expiry,
				}

				data.CreatePoll(*poll)
			case "leaderboard":
				var year string
				if len(options) > 0 {
					year = options[0].StringValue()
				} else {
					year = strconv.Itoa(time.Now().Year())
				}
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Making leaderboard...",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				msg, err := s.ChannelMessageSendEmbed(i.ChannelID, create_leaderboard(year))
				if err != nil {
					log.Printf("Failed to send leaderboard: %v", err)
				}
				s.MessageThreadStart(i.ChannelID, msg.ID, "Leaderboard", 60)
			case "version":
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Current version: %s", VERSION),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			case "update":
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Attempting to update...",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})

				extension := map[string]string{"windows": ".exe"}[runtime.GOOS]
				binaryName := fmt.Sprintf("foulbot-%s-%s%s", runtime.GOOS, runtime.GOARCH, extension)
				downloadURL := fmt.Sprintf("https://github.com/mustafa-tariqk/foulbot/releases/latest/download/%s", binaryName)

				resp, err := http.Get(downloadURL)
				if err != nil {
					s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
						Content: "Failed to download update: " + err.Error(),
						Flags:   discordgo.MessageFlagsEphemeral,
					})
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
						Content: fmt.Sprintf("Failed to download update: HTTP %d", resp.StatusCode),
						Flags:   discordgo.MessageFlagsEphemeral,
					})
					return
				}

				err = update.Apply(resp.Body, update.Options{})
				if err != nil {
					s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
						Content: "Failed to apply update: " + err.Error(),
						Flags:   discordgo.MessageFlagsEphemeral,
					})
					return
				}

				s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
					Content: "Update successful! Restarting bot...",
					Flags:   discordgo.MessageFlagsEphemeral,
				})

				run_migrations()

				// Restart the application
				cmd := exec.Command(os.Args[0], os.Args[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				err = cmd.Start()
				if err != nil {
					log.Printf("Failed to restart: %v", err)
					return
				}
				// Exit current process only after ensuring new one started
				os.Exit(0)
			case "logs":
				// Create a temporary zip file
				zipFile, err := os.CreateTemp("", "foulbot-logs-*.zip")
				if err != nil {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Failed to create temp zip: %s", err),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}
				defer os.Remove(zipFile.Name())
				defer zipFile.Close()

				// Create zip writer
				zipWriter := zip.NewWriter(zipFile)
				defer zipWriter.Close()

				// Walk through current directory
				err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					// Skip directories and binary files
					if info.IsDir() || strings.HasPrefix(path, "foulbot-") || strings.HasPrefix(path, ".foulbot-") {
						return nil
					}

					// Create zip entry
					f, err := zipWriter.Create(path)
					if err != nil {
						return err
					}

					// Copy file contents to zip
					content, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					_, err = f.Write(content)
					return err
				})

				if err != nil {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Failed to create zip: %s", err),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}
				zipWriter.Close()

				// Reopen zip file for reading
				zipReader, err := os.Open(zipFile.Name())
				if err != nil {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Failed to read zip: %s", err),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}
				defer zipReader.Close()

				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Uploading logs...",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})

				_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
					Content: "Here are the log files:",
					Flags:   discordgo.MessageFlagsEphemeral,
					Files: []*discordgo.File{
						{
							Name:   "foulbot-logs.zip",
							Reader: zipReader,
						},
					},
				})
				if err != nil {
					log.Printf("Failed to upload logs zip: %v", err)
				}
			}
		}
	})

	// Add button handler
	bot.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionMessageComponent {
			// Handle button interactions
			switch i.MessageComponentData().CustomID {
			case "vote_yes":
				data.Vote(i.ChannelID, i.Message.ID, i.Member.User.ID, true)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Vote recorded: 👍",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			case "vote_no":
				data.Vote(i.ChannelID, i.Message.ID, i.Member.User.ID, false)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Vote recorded: 👎",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
		}
	})
}

func formatUserMentions(users []*discordgo.User) string {
	mentions := make([]string, len(users))
	for i, user := range users {
		mentions[i] = fmt.Sprintf("<@%s>", user.ID)
	}
	return strings.Join(mentions, "\n")
}

func create_leaderboard(year string) *discordgo.MessageEmbed {
	leaderboard := data.Leaderboard(year)
	description := ""
	for i, position := range leaderboard {
		if i >= len(NUMBERS) {
			break
		}
		description += fmt.Sprintf("%s <@%s>: %d\n", NUMBERS[i], position.UserId, position.Points)
	}
	return &discordgo.MessageEmbed{
		Title:       "Leaderboard",
		Description: description,
	}
}

func establishCommands(bot *discordgo.Session, guildId string, appId string) {
	commands := []*discordgo.ApplicationCommand{
		// {
		// 	Name:        "own",
		// 	Description: "Accuse someone of gaining",
		// 	Options: []*discordgo.ApplicationCommandOption{
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionUser,
		// 			Name:        "user",
		// 			Description: "The user to mention",
		// 			Required:    true,
		// 		},
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionInteger,
		// 			Name:        "number",
		// 			Description: "An integer value",
		// 			Required:    true,
		// 		},
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionString,
		// 			Name:        "reason",
		// 			Description: "The reason for gaining",
		// 			Required:    true,
		// 		},
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionUser,
		// 			Name:        "user2",
		// 			Description: "Additional user to mention (optional)",
		// 			Required:    false,
		// 		},
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionUser,
		// 			Name:        "user3",
		// 			Description: "Additional user to mention (optional)",
		// 			Required:    false,
		// 		},
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionUser,
		// 			Name:        "user4",
		// 			Description: "Additional user to mention (optional)",
		// 			Required:    false,
		// 		},
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionUser,
		// 			Name:        "user5",
		// 			Description: "Additional user to mention (optional)",
		// 			Required:    false,
		// 		},
		// 	},
		// },
		// {
		// 	Name:        "leaderboard",
		// 	Description: fmt.Sprintf("Displays a top %d leaderboard", len(NUMBERS)),
		// 	Options: []*discordgo.ApplicationCommandOption{
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionString,
		// 			Name:        "year",
		// 			Description: "Year to show leaderboard for (defaults to current year)",
		// 			Required:    false,
		// 		},
		// 	},
		// },
		// {
		// 	Name:        "version",
		// 	Description: "Displays the current version",
		// 	Options:     []*discordgo.ApplicationCommandOption{},
		// },
		{
			Name:        "update",
			Description: "Update the bot to a new version",
			Options:     []*discordgo.ApplicationCommandOption{},
		},
		// {
		// 	Name:        "logs",
		// 	Description: "Uploads files importing for debugging",
		// 	Options:     []*discordgo.ApplicationCommandOption{},
		// },
	}
	_, err := bot.ApplicationCommandBulkOverwrite(appId, guildId, commands)
	if err != nil {
		log.Fatalf("could not register commands: %s", err)
	}
	bot.Identify.Intents = discordgo.IntentsAllWithoutPrivileged
}

func run_migrations() {
}

func shouldShowVotes() bool {
	return rand.Float32() < 0.5
}

// Add new helper function
func createThreadWithTags(s *discordgo.Session, channelID string, messageID string, reason string, users []*discordgo.User) error {
	thread, err := s.MessageThreadStartComplex(channelID, messageID, &discordgo.ThreadStart{
		Name:                truncateString(reason, 100),
		AutoArchiveDuration: 60,
	})
	if err != nil {
		return fmt.Errorf("failed to create thread: %v", err)
	}

	// Create initial message tagging users
	mentions := formatUserMentions(users)
	_, err = s.ChannelMessageSend(thread.ID, mentions)
	if err != nil {
		return fmt.Errorf("failed to send initial thread message: %v", err)
	}

	return nil
}
