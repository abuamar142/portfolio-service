package config

import (
	"strings"
	"testing"
)

// The CORS allow-list is literal: a frontend origin that is missing from it
// has every browser fetch blocked. These tests pin the two properties that
// matter for that contract — the GitHub Pages hub is present by default, and
// the env var can extend (never shrink) the list without duplicates.
func TestLoadIncludesGitHubPagesOriginByDefault(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")

	origins := Load().AllowedOrigins
	if !contains(origins, "https://abuamar142.github.io") {
		t.Fatalf("default allow-list is missing https://abuamar142.github.io: %v", origins)
	}
	if !contains(origins, "https://abuamar.online") {
		t.Fatalf("default allow-list dropped https://abuamar.online: %v", origins)
	}
}

func TestLoadAppendsEnvOriginsWithoutDuplicates(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", " https://example.test , https://abuamar.online,,")

	origins := Load().AllowedOrigins
	if !contains(origins, "https://example.test") {
		t.Fatalf("env origin missing from allow-list: %v", origins)
	}

	var count int
	for _, origin := range origins {
		if origin == "https://abuamar.online" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("https://abuamar.online appears %d times, want 1: %v", count, origins)
	}

	for _, origin := range origins {
		if strings.TrimSpace(origin) == "" {
			t.Fatalf("allow-list contains an empty origin: %v", origins)
		}
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
