package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/abuamar142/portfolio-service/internal/config"
	"github.com/abuamar142/portfolio-service/internal/db"
	"github.com/abuamar142/portfolio-service/internal/game"
	"github.com/abuamar142/portfolio-service/internal/handlers"
	"github.com/abuamar142/portfolio-service/internal/middleware"
	"github.com/abuamar142/portfolio-service/internal/notify"
	"github.com/abuamar142/portfolio-service/internal/services"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}
	log.Println("migrations applied successfully")

	quoteSvc := services.NewQuoteService(pool)
	linkSvc := services.NewLinkService(pool)
	snippetSvc := services.NewSnippetService(pool)
	feedbackSvc := services.NewFeedbackService(pool)
	// Certificate uploads go to media-service, which owns the R2 credentials
	// and the portfolio-assets bucket. Not fatal when unset: every other
	// endpoint keeps working, and the file routes answer 503 with a clear
	// reason rather than the service refusing to start.
	mediaClient := services.NewMediaClient(cfg.MediaServiceURL)
	if !mediaClient.Configured() {
		log.Println("warning: MEDIA_SERVICE_URL not set — certificate uploads disabled")
	}
	achievementSvc := services.NewAchievementService(pool, mediaClient)
	profileSvc := services.NewProfileService(pool)

	// Party game: rooms live in memory (minutes-long, worthless after the last
	// player leaves) and the word bank is compiled in, so this needs no table
	// and no migration.
	gameBank := game.NewBank(game.DefaultEntries())
	gameManager := game.NewManager(gameBank)
	// Sweep with generous slack: a room waiting in a lobby for a late friend is
	// not idle debris, but one untouched for six hours certainly is.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if removed := gameManager.Sweep(6 * time.Hour); removed > 0 {
					log.Printf("game: swept %d idle room(s)", removed)
				}
			}
		}
	}()

	healthH := handlers.NewHealthHandler()
	quoteH := handlers.NewQuoteHandler(quoteSvc)
	linkH := handlers.NewLinkHandler(linkSvc)
	snippetH := handlers.NewSnippetHandler(snippetSvc)
	feedbackH := handlers.NewFeedbackHandler(feedbackSvc, cfg.OwnerID, notify.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID))
	achievementH := handlers.NewAchievementHandler(achievementSvc, cfg.OwnerID)
	profileH := handlers.NewProfileHandler(profileSvc)
	gameH := handlers.NewGameHandler(gameManager)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(newCORSMiddleware(cfg.AllowedOrigins))

	r.Get("/api/health", healthH.ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/quotes", quoteH.List)
		r.Get("/quotes/tags", quoteH.ListTags)
		r.Get("/quotes/{id}", quoteH.GetByID)
		r.Get("/links", linkH.List)
		r.Get("/links/tags", linkH.ListTags)
		r.Get("/links/{id}", linkH.GetByID)
		r.Get("/snippets", snippetH.List)
		r.Get("/snippets/tags", snippetH.ListTags)
		r.Get("/snippets/languages", snippetH.ListLanguages)
		r.Get("/snippets/{id}", snippetH.GetByID)
		// Public: visitors submit without an account (honeypot-guarded).
		// Rate-limited per IP:3 messages/minute blunts floods before they
		// reach the database or the Telegram notifier.
		feedbackLimiter := middleware.RateLimit(middleware.NewRateLimiter(3, time.Minute))
		r.With(feedbackLimiter).Post("/feedback", feedbackH.Create)

		// Party game. Public by design — the room code is the only access
		// control, and a code is handed out precisely so friends can join.
		// Writes are behind no auth: a player is identified by the id issued at
		// create/join, which the handler validates on every action.
		r.Route("/game", func(r chi.Router) {
			r.Post("/rooms", gameH.Create)
			r.Get("/rooms/{code}/events", gameH.Stream)
			r.Post("/rooms/{code}/join", gameH.Join)
			r.Post("/rooms/{code}/start", gameH.Start)
			r.Post("/rooms/{code}/next", gameH.Next)
			r.Post("/rooms/{code}/restart", gameH.Restart)
			r.Post("/rooms/{code}/settings", gameH.UpdateSettings)
			r.Post("/rooms/{code}/guess", gameH.Guess)
			r.Post("/rooms/{code}/score", gameH.AdjustScore)
			r.Delete("/rooms/{code}/players/{playerID}", gameH.Leave)
		})

		r.Get("/achievements", achievementH.List)
		// Profile: the identity block and the four ordered lists that replaced
		// the Payload CMS. Public — the homepage renders it for every visitor.
		r.Get("/profile", profileH.Get)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(cfg.AuthServiceURL))
			r.Post("/quotes", quoteH.Create)
			r.Put("/quotes/{id}", quoteH.Update)
			r.Delete("/quotes/{id}", quoteH.Delete)
			r.Post("/links", linkH.Create)
			r.Put("/links/{id}", linkH.Update)
			r.Delete("/links/{id}", linkH.Delete)
			r.Post("/snippets", snippetH.Create)
			r.Put("/snippets/{id}", snippetH.Update)
			r.Delete("/snippets/{id}", snippetH.Delete)
			r.Get("/feedback", feedbackH.List)
			r.Patch("/feedback/{id}", feedbackH.Update)
			r.Delete("/feedback/{id}", feedbackH.Delete)

			r.Post("/achievements", achievementH.Create)
			r.Put("/achievements/{id}", achievementH.Update)
			r.Delete("/achievements/{id}", achievementH.Delete)
			r.Post("/achievements/{id}/file", achievementH.UploadFile)
			r.Delete("/achievements/{id}/file", achievementH.DeleteFile)
		})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("portfolio service starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func runMigrations(databaseURL string) error {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", d, databaseURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

// newCORSMiddleware builds the CORS layer from the configured allow-list.
// Origins are matched exactly — the header echoes the request origin only when
// it is in the list, never a wildcard.
//
// Vary: Origin is mandatory here, not decorative. The response body/headers
// differ per Origin, so without it a shared cache can store the un-allow-listed
// variant (no Access-Control-Allow-Origin) and later serve it to a request
// that would have been allowed, breaking the browser fetch. Cache-Control on
// these endpoints does not prevent that on every layer.
func newCORSMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			if allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-File-Name")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
