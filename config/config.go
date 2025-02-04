package config

import (
	"encoding/json"
	"os"
	"time"
)

var (
	VERSION     string
	CONFIG_JSON = "config.json"
	POLL_LENGTH = 10 * time.Second
	NUMBERS     = []string{":one:", ":two:", ":three:", ":four:", ":five:",
		":six:", ":seven:", ":eight:", ":nine:", ":keycap_ten:"}
)

type Config struct {
	DiscordToken   string `json:"discord_token"`
	DiscordGuildID string `json:"discord_guild_id"`
	DiscordAppID   string `json:"discord_application_id"`
}

func LoadConfig() (*Config, error) {
	configData, err := os.ReadFile(CONFIG_JSON)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
