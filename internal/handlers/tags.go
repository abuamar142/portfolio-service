package handlers

import "strings"

// splitTags parses the comma-separated ?tag= query parameter into individual
// tag names for multi-select (OR) filtering. Empty entries and duplicates are
// dropped; an empty result means "no filter", which keeps the old single-tag
// behaviour intact.
func splitTags(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := make(map[string]struct{}, 4)
	var tags []string
	for _, part := range strings.Split(raw, ",") {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}
	return tags
}
