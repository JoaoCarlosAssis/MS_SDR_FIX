package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              string
	DatabaseURL       string
	CacheTTL          time.Duration
	AdminIngestURL    string
	AdminServiceToken string
	SlackWebhookURL   string
	SlackBotToken     string
	SlackChannelID    string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")

	ttlStr := os.Getenv("CACHE_TTL_SECONDS")
	ttlSeconds := 60
	if ttlStr != "" {
		if v, err := strconv.Atoi(ttlStr); err == nil && v > 0 {
			ttlSeconds = v
		}
	}

	return Config{
		Port:              port,
		DatabaseURL:       dbURL,
		CacheTTL:          time.Duration(ttlSeconds) * time.Second,
		AdminIngestURL:    os.Getenv("ADMIN_INGEST_URL"),
		AdminServiceToken: os.Getenv("ADMIN_SERVICE_TOKEN"),
		SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
		SlackBotToken:     os.Getenv("SLACK_BOT_TOKEN"),
		SlackChannelID:    os.Getenv("SLACK_CHANNEL_ID"),
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
