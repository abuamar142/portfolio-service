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

	healthH := handlers.NewHealthHandler()
	quoteH := handlers.NewQuoteHandler(quoteSvc)
	linkH := handlers.NewLinkHandler(linkSvc)
	snippetH := handlers.NewSnippetHandler(snippetSvc)
	feedbackH := handlers.NewFeedbackHandler(feedbackSvc, cfg.OwnerID, notify.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID))

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(corsMiddleware)

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

var allowedOrigins = map[string]bool{
	"https://abuamar.online":                       true,
	"https://dev.abuamar.online":                   true,
	"https://quote.abuamar.online":                 true,
	"https://portfolio.abuamar.online":             true,
	"https://portfolio-service-dev.abuamar.online": true,
	"http://localhost:5173":                        true,
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
