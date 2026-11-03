package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// CORS contract, exercised through the real middleware:
//
//   - an allow-listed origin is echoed back in Access-Control-Allow-Origin
//   - an unknown origin gets no ACAO at all (no wildcard fallback)
//   - every response carries Vary: Origin, because the headers differ per
//     origin and a shared cache must not reuse the un-allowed variant
func TestCORSMiddlewareEchoesOnlyAllowListedOrigins(t *testing.T) {
	handler := newCORSMiddleware([]string{"https://abuamar142.github.io"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	tests := []struct {
		name       string
		origin     string
		wantACAO   string
		wantStatus int
	}{
		{"allow-listed", "https://abuamar142.github.io", "https://abuamar142.github.io", http.StatusOK},
		{"unknown origin", "https://evil.example", "", http.StatusOK},
		{"no origin header", "", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantACAO {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantACAO)
			}
			if got := rec.Code; got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
			// Without this a shared cache can serve the no-ACAO variant to an
			// allow-listed origin, which the browser then blocks.
			if got := rec.Header().Values("Vary"); len(got) == 0 || got[0] != "Origin" {
				t.Errorf("Vary = %v, want [Origin]", got)
			}
		})
	}
}

// Preflight must answer 204 with the allow-listed origin, and must not call
// the wrapped handler.
func TestCORSMiddlewarePreflight(t *testing.T) {
	called := false
	handler := newCORSMiddleware([]string{"https://abuamar142.github.io"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}),
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/profile", nil)
	req.Header.Set("Origin", "https://abuamar142.github.io")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("handler ran for an OPTIONS preflight")
	}
	if got := rec.Code; got != http.StatusNoContent {
		t.Errorf("status = %d, want %d", got, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://abuamar142.github.io" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the request origin", got)
	}
}
