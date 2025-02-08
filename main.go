package main

import (
	"fmt"
	"foulbot/config"
	"foulbot/data"
	"foulbot/inputs"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

func main() {
	bot, guildId, appId := loadEnv()

	inputs.HandleInputs(bot)

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
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("could not load config: %s", err)
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
					{
						Name: "Votes For",
						Value: func() string {
							if len(poll.VotesFor) == 0 {
								return "none"
							}
							return fmt.Sprintf("<@%s>", strings.Join(poll.VotesFor, ">\n<@"))
						}(),
						Inline: true,
					},
					{
						Name: "Votes Against",
						Value: func() string {
							if len(poll.VotesAgainst) == 0 {
								return "none"
							}
							return fmt.Sprintf("<@%s>", strings.Join(poll.VotesAgainst, ">\n<@"))
						}(),
						Inline: true,
					},
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
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user2",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user3",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user4",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user5",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user6",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user7",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user8",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user9",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user10",
					Description: "Additional user to mention (optional)",
					Required:    false,
				},
			},
		},
		{
			Name:        "leaderboard",
			Description: fmt.Sprintf("Displays a top %d leaderboard", len(config.NUMBERS)),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "year",
					Description: "Year to show leaderboard for (defaults to current year)",
					Required:    false,
				},
			},
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
