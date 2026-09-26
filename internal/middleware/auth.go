package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/abuamar142/portfolio-service/internal/response"
)

type contextKey string

const UserKey contextKey = "user"

func Auth(authServiceURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				response.Error(w, http.StatusUnauthorized, "MISSING_TOKEN", "missing or invalid Authorization header", "expected: Bearer <token>")
				return
			}
			token := strings.TrimPrefix(header, "Bearer ")

			user, err := validateToken(authServiceURL, token)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired token", "")
				return
			}

			ctx := context.WithValue(r.Context(), UserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUser(ctx context.Context) *models.AuthUser {
	user, _ := ctx.Value(UserKey).(*models.AuthUser)
	return user
}

func validateToken(authServiceURL, token string) (*models.AuthUser, error) {
	url := fmt.Sprintf("%s/api/v1/auth/me", authServiceURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Bounded: a hung auth service must fail this request, not pin the
	// goroutine and the caller's connection indefinitely.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling auth service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth service returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool            `json:"success"`
		Data    models.AuthUser `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("auth validation failed")
	}

	return &result.Data, nil
}
