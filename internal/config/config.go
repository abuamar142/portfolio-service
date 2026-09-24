package config

import "os"

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AuthServiceURL string
	OwnerID        string
}

func Load() *Config {
	return &Config{
		Port:           getenv("PORT", "8084"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AuthServiceURL: getenv("AUTH_SERVICE_URL", "http://localhost:8080"),
		OwnerID:        os.Getenv("OWNER_USER_ID"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
