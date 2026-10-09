package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment           string
	Addr                  string
	DatabaseURL           string
	WebOrigin             string
	OllamaURL             string
	OllamaModel           string
	OIDCIssuer            string
	OIDCClientID          string
	AuthDevBypass         bool
	TelegramToken         string
	TelegramMode          string
	TelegramWebhookURL    string
	TelegramWebhookSecret string
	TelegramAllowlist     map[int64]bool
	ShutdownTimeout       time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:           env("APP_ENV", "development"),
		Addr:                  env("HTTP_ADDR", ":8091"),
		DatabaseURL:           env("DATABASE_URL", "postgres://suppervisor:suppervisor@localhost:5432/suppervisor?sslmode=disable"),
		WebOrigin:             env("WEB_ORIGIN", "http://localhost:4179"),
		OllamaURL:             env("OLLAMA_URL", ""),
		OllamaModel:           env("OLLAMA_MODEL", "qwen3:8b"),
		OIDCIssuer:            os.Getenv("OIDC_ISSUER"),
		OIDCClientID:          env("OIDC_CLIENT_ID", "suppervisor"),
		AuthDevBypass:         boolEnv("AUTH_DEV_BYPASS", true),
		TelegramToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramMode:          env("TELEGRAM_MODE", "polling"),
		TelegramWebhookURL:    os.Getenv("TELEGRAM_WEBHOOK_URL"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		TelegramAllowlist:     parseIDs(os.Getenv("TELEGRAM_ALLOWED_USER_IDS")),
		ShutdownTimeout:       15 * time.Second,
	}
	if cfg.Environment == "production" && cfg.AuthDevBypass {
		return Config{}, errors.New("AUTH_DEV_BYPASS cannot be enabled in production")
	}
	if cfg.Environment == "production" && cfg.OIDCIssuer == "" {
		return Config{}, errors.New("OIDC_ISSUER is required in production")
	}
	if cfg.TelegramToken != "" && len(cfg.TelegramAllowlist) == 0 {
		return Config{}, errors.New("TELEGRAM_ALLOWED_USER_IDS is required when Telegram is enabled")
	}
	if cfg.TelegramMode != "polling" && cfg.TelegramMode != "webhook" {
		return Config{}, errors.New("TELEGRAM_MODE must be polling or webhook")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func parseIDs(value string) map[int64]bool {
	out := map[int64]bool{}
	for _, raw := range strings.Split(value, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err == nil && id != 0 {
			out[id] = true
		}
	}
	return out
}
