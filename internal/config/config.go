package config

import "os"

type Config struct {
	Port           string
	DatabaseURL    string
	AuthServiceURL string
	OwnerID        string
	// TelegramBotToken/TelegramChatID enable best-effort notifications
	// (feedback inbox). Either empty → notifications disabled.
	TelegramBotToken string
	TelegramChatID   string
	// R2 credentials for uploading achievement certificates.
	// All three must be set for file uploads to work.
	R2APIToken  string
	R2AccountID string
	R2Bucket    string
}

func Load() *Config {
	return &Config{
		Port:             getenv("PORT", "8084"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		AuthServiceURL:   getenv("AUTH_SERVICE_URL", "http://localhost:8080"),
		OwnerID:          os.Getenv("OWNER_USER_ID"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		R2APIToken:       os.Getenv("R2_API_TOKEN"),
		R2AccountID:      os.Getenv("R2_ACCOUNT_ID"),
		R2Bucket:         os.Getenv("R2_BUCKET"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
