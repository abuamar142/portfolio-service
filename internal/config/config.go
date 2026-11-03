package config

import (
	"os"
	"strings"
)

type Config struct {
	Port           string
	DatabaseURL    string
	AuthServiceURL string
	OwnerID        string
	// TelegramBotToken/TelegramChatID enable best-effort notifications
	// (feedback inbox). Either empty → notifications disabled.
	TelegramBotToken string
	TelegramChatID   string
	// MediaServiceURL is where certificate uploads and deletes go.
	// media-service owns the R2 credentials and the portfolio-assets bucket,
	// so this process holds no storage credential at all. Empty disables
	// uploads — the routes answer 503 while every other endpoint keeps
	// working.
	MediaServiceURL string
	// AllowedOrigins is the CORS allow-list, exactly matched (no wildcard).
	// Built-in defaults cover the sites that live on abuamar.online; the
	// ALLOWED_ORIGINS env var (comma-separated) appends more so a new frontend
	// origin does not require a code change and a rebuild.
	AllowedOrigins []string
}

// defaultAllowedOrigins are the origins the service ships knowing about:
// the production portfolio, its dev twin, the standalone tools, and the local
// Vite dev server. `abuamar142.github.io` is the GitHub Pages gateway hub —
// it reads /profile and /achievements in the browser, so it must be here or
// every fetch from that origin fails CORS.
var defaultAllowedOrigins = []string{
	"https://abuamar.online",
	"https://dev.abuamar.online",
	"https://quote.abuamar.online",
	"https://portfolio.abuamar.online",
	"https://portfolio-service-dev.abuamar.online",
	"https://abuamar142.github.io",
	"http://localhost:5173",
}

func Load() *Config {
	return &Config{
		Port:             getenv("PORT", "8084"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		AuthServiceURL:   getenv("AUTH_SERVICE_URL", "http://localhost:8080"),
		OwnerID:          os.Getenv("OWNER_USER_ID"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		MediaServiceURL:  getenv("MEDIA_SERVICE_URL", "http://media-service:8080"),
		AllowedOrigins:   allowedOrigins(os.Getenv("ALLOWED_ORIGINS")),
	}
}

// allowedOrigins merges the built-in list with the comma-separated env value.
// Empty entries and duplicates are dropped so a stray trailing comma in the
// env var cannot add an empty origin to the allow-list.
func allowedOrigins(extra string) []string {
	out := make([]string, 0, len(defaultAllowedOrigins)+4)
	seen := make(map[string]bool, len(defaultAllowedOrigins))
	add := func(origin string) {
		origin = strings.TrimSpace(origin)
		if origin == "" || seen[origin] {
			return
		}
		seen[origin] = true
		out = append(out, origin)
	}
	for _, origin := range defaultAllowedOrigins {
		add(origin)
	}
	for _, origin := range strings.Split(extra, ",") {
		add(origin)
	}
	return out
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
