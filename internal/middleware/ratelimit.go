package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/abuamar142/portfolio-service/internal/response"
)

// RateLimiter is a small fixed-window counter keyed by client IP. In-process
// is enough here: one service instance owns the public feedback POST, and the
// limiter only needs to blunt floods (Cloudflare and nginx sit in front).
type RateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*rateEntry
}

type rateEntry struct {
	count int
	reset time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string]*rateEntry),
	}
}

// Allow reports whether key is still within the current window. The window
// resets lazily per key; a size-triggered sweep keeps the map bounded.
func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	e := l.hits[key]
	if e == nil || now.After(e.reset) {
		l.hits[key] = &rateEntry{count: 1, reset: now.Add(l.window)}
		if len(l.hits) > 4096 {
			for k, v := range l.hits {
				if now.After(v.reset) {
					delete(l.hits, k)
				}
			}
		}
		return l.limit >= 1
	}
	e.count++
	return e.count <= l.limit
}

// RateLimit rejects requests over limit per window with429. The client IP is
// taken from the edge headers first (port8084 is bound to localhost, so a
// direct caller cannot spoof them past nginx/Cloudflare), then the socket.
func RateLimit(l *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(clientIP(r)) {
				response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many messages, try again in a minute", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
