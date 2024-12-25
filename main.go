package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"foulbot/dao"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/inconshreveable/go-update"
)

var (
	VERSION     string
	POINTS_JSON = "points.json"
	POLLS_JSON  = "polls.json"
	CONFIG_JSON = "config.json"
	POLL_LENGTH = 24 * time.Hour
	pollsMutex  sync.RWMutex
	activePolls = make(map[string]*VotePoll)
	NUMBERS     = []string{":one:", ":two:", ":three:", ":four:", ":five:",
		":six:", ":seven:", ":eight:", ":nine:", ":keycap_ten:"}
)

type userPoints struct {
	userID string
	points int64
}

type VotePoll struct {
	MessageID string
	ChannelID string
	UserID    string
	CreatorID string
	Points    int64
	Reason    string
	ExpiresAt time.Time
}

type StoredPolls struct {
	Polls map[string]*VotePoll `json:"polls"`
}

type Config struct {
	DiscordToken   string `json:"discord_token"`
	DiscordGuildID string `json:"discord_guild_id"`
	DiscordAppID   string `json:"discord_application_id"`
}

func main() {
	bot, points, guildId, appId := loadEnv()

	handleInputs(bot, points)
	handlePollsMutex(bot)

	err := bot.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer bot.Close()

	handleExpiredPolls(bot, points)

	establishCommands(bot, guildId, appId)
	fmt.Println("Bot is running...")

	// Runs forever, shut down with ctrl+c
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	fmt.Println("Bot is shutting down...")
}

func loadEnv() (*discordgo.Session, map[string]int64, string, string) {
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
	points := loadPoints()
	dao.Clear()

	return bot, points, config.DiscordGuildID, config.DiscordAppID
}

func loadPoints() map[string]int64 {
	points := make(map[string]int64)
	if _, err := os.Stat(POINTS_JSON); os.IsNotExist(err) {
		data, err := json.Marshal(points)
		if err != nil {
			log.Fatalf("could not marshal empty points: %s", err)
		}
		if err := os.WriteFile(POINTS_JSON, data, 0644); err != nil {
			log.Fatalf("could not create points file: %s", err)
		}
	} else {
		data, err := os.ReadFile(POINTS_JSON)
		if err != nil {
			log.Fatalf("could not read points file: %s", err)
		}
		if err := json.Unmarshal(data, &points); err != nil {
			log.Fatalf("could not unmarshal points: %s", err)
		}
	}

	return points
}

func loadPolls() map[string]*VotePoll {
	polls := make(map[string]*VotePoll)
	if _, err := os.Stat(POLLS_JSON); os.IsNotExist(err) {
		savePolls(polls)
		return polls
	}

	data, err := os.ReadFile(POLLS_JSON)
	if err != nil {
		log.Fatalf("could not read polls file: %s", err)
	}

	var storedPolls StoredPolls
	if err := json.Unmarshal(data, &storedPolls); err != nil {
		log.Fatalf("could not unmarshal polls: %s", err)
	}

	// Set default values for missing parameters
	for id, poll := range storedPolls.Polls {
		if poll.CreatorID == "" {
			poll.CreatorID = "" // Set a default creator ID
		}
		// Add more default value checks as needed
		storedPolls.Polls[id] = poll
	}

	return storedPolls.Polls
}

func savePolls(polls map[string]*VotePoll) {
	storedPolls := StoredPolls{Polls: polls}
	data, err := json.MarshalIndent(storedPolls, "", "    ")
	if err != nil {
		log.Fatalf("could not marshal polls: %s", err)
	}
	if err := os.WriteFile(POLLS_JSON, data, 0644); err != nil {
		log.Fatalf("could not write polls file: %s", err)
	}
}

func handleExpiredPolls(bot *discordgo.Session, points map[string]int64) {
	polls := loadPolls()
	now := time.Now()

	for _, poll := range polls {
		if now.After(poll.ExpiresAt) {
			concludePoll(bot, poll, points)
		} else {
			pollsMutex.Lock()
			activePolls[poll.MessageID] = poll
			pollsMutex.Unlock()

			timeUntilExpiry := time.Until(poll.ExpiresAt)
			time.AfterFunc(timeUntilExpiry, func() {
				concludePoll(bot, poll, points)
			})
		}
	}
}

func handleInputs(bot *discordgo.Session, points map[string]int64) {
	bot.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			options := i.ApplicationCommandData().Options
			switch i.ApplicationCommandData().Name {
			case "own":
				user := options[0].UserValue(s)
				number := options[1].IntValue()
				reason := options[2].StringValue()

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

				pollMsg, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title: "Own",
							Fields: []*discordgo.MessageEmbedField{
								{
									Name:   "User",
									Value:  fmt.Sprintf("<@%s>", user.ID),
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
							Timestamp: time.Now().Format(time.RFC3339),
						},
					},
				})
				if err != nil {
					return
				}

				_, err = s.MessageThreadStartComplex(pollMsg.ChannelID, pollMsg.ID, &discordgo.ThreadStart{
					Name:                reason,
					AutoArchiveDuration: 60,
				})

				if err != nil {
					log.Printf("Failed to create thread: %v", err)
				}

				s.MessageReactionAdd(i.ChannelID, pollMsg.ID, "%F0%9F%91%8D")
				s.MessageReactionAdd(i.ChannelID, pollMsg.ID, "%F0%9F%91%8E")

				poll := &VotePoll{
					MessageID: pollMsg.ID,
					ChannelID: i.ChannelID,
					UserID:    user.ID,
					CreatorID: i.Member.User.ID,
					Points:    number,
					Reason:    reason,
					ExpiresAt: time.Now().Add(POLL_LENGTH),
				}

				pollsMutex.Lock()
				activePolls[pollMsg.ID] = poll
				savePolls(activePolls)
				pollsMutex.Unlock()

				time.AfterFunc(POLL_LENGTH, func() {
					concludePoll(s, poll, points)
				})
			case "leaderboard":
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds: []*discordgo.MessageEmbed{create_leaderboard(points, s)},
					},
				})
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
					if info.IsDir() || strings.HasPrefix(path, "foulbot-") {
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
}

func concludePoll(s *discordgo.Session, poll *VotePoll, points map[string]int64) {
	pollsMutex.Lock()
	delete(activePolls, poll.MessageID)
	savePolls(activePolls)
	pollsMutex.Unlock()

	upVotes, _ := s.MessageReactions(poll.ChannelID, poll.MessageID, "%F0%9F%91%8D", 100, "", "")
	downVotes, _ := s.MessageReactions(poll.ChannelID, poll.MessageID, "%F0%9F%91%8E", 100, "", "")

	result := "Failed"
	if len(upVotes) > len(downVotes) {
		points[poll.UserID] += poll.Points
		savePoints(points)
		result = "Passed"
	}

	messageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", s.State.Guilds[0].ID, poll.ChannelID, poll.MessageID)

	embed := &discordgo.MessageEmbed{
		Title: "Poll Result",
		Color: 0x57F287,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Creator",
				Value:  fmt.Sprintf("<@%s>", poll.CreatorID),
				Inline: true,
			},
			{
				Name:   "Gainer",
				Value:  fmt.Sprintf("<@%s>", poll.UserID),
				Inline: true,
			},
			{
				Name:   "Points",
				Value:  fmt.Sprintf("%+d", poll.Points),
				Inline: true,
			},
			{
				Name:   "Reason",
				Value:  fmt.Sprintf("[%s](%s)", poll.Reason, messageLink),
				Inline: false,
			},
			{
				Name:   "Result",
				Value:  result,
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if result == "Failed" {
		embed.Color = 0xED4245 // Red color for failed
	}

	s.ChannelMessageSendEmbed(poll.ChannelID, embed)
}

func savePoints(points map[string]int64) {
	data, err := json.MarshalIndent(points, "", "    ")
	if err != nil {
		log.Fatalf("could not marshal points: %s", err)
	}
	if err := os.WriteFile(POINTS_JSON, data, 0644); err != nil {
		log.Fatalf("could not write points file: %s", err)
	}
}

func create_leaderboard(points map[string]int64, s *discordgo.Session) *discordgo.MessageEmbed {
	pairs := make([]userPoints, 0, len(points))
	for id, score := range points {
		pairs = append(pairs, userPoints{id, score})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].points > pairs[j].points
	})
	if len(pairs) > len(NUMBERS) {
		pairs = pairs[:len(NUMBERS)]
	}
	return create_leaderboard_string(pairs, s)
}

func create_leaderboard_string(pairs []userPoints, s *discordgo.Session) *discordgo.MessageEmbed {
	description := ""
	for i, pair := range pairs {
		user, err := s.User(pair.userID)
		if err != nil {
			log.Fatal(err)
			continue
		}
		description += fmt.Sprintf("%s %s: %d\n", NUMBERS[i], user.Username, pair.points)
	}
	return &discordgo.MessageEmbed{
		Title:       "Leaderboard",
		Description: description,
	}
}

func handlePollsMutex(bot *discordgo.Session) {
	bot.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		pollsMutex.RLock()
		_, exists := activePolls[r.MessageID]
		pollsMutex.RUnlock()
		if !exists || r.UserID == s.State.User.ID {
			return
		}
	})
}

func establishCommands(bot *discordgo.Session, guildId string, appId string) {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "own",
			Description: "Accuse someone of gaining",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to mention",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "number",
					Description: "An integer value",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "The reason for gaining",
					Required:    true,
				},
			},
		},
		{
			Name:        "leaderboard",
			Description: fmt.Sprintf("Displays a top %d leaderboard", len(NUMBERS)),
			Options:     []*discordgo.ApplicationCommandOption{},
		},
		{
			Name:        "version",
			Description: "Displays the current version",
			Options:     []*discordgo.ApplicationCommandOption{},
		},
		{
			Name:        "update",
			Description: "Update the bot to a new version",
			Options:     []*discordgo.ApplicationCommandOption{},
		},
		{
			Name:        "logs",
			Description: "Uploads files importing for debugging",
			Options:     []*discordgo.ApplicationCommandOption{},
		},
	}
	_, err := bot.ApplicationCommandBulkOverwrite(appId, guildId, commands)
	if err != nil {
		log.Fatalf("could not register commands: %s", err)
	}
	bot.Identify.Intents = discordgo.IntentsAllWithoutPrivileged
}

func run_migrations() {
	// No migrations yet
}
