package tasks

import (
	"fmt"
	"strings"
)

func ParseCommaSeparatedSlugs(input string) ([]string, error) {
	parts := strings.Split(input, ",")
	slugs := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for idx, raw := range parts {
		slug := strings.TrimSpace(raw)
		if slug == "" {
			return nil, fmt.Errorf("task slug at position %d cannot be empty", idx+1)
		}
		if _, exists := seen[slug]; exists {
			return nil, fmt.Errorf("duplicate task slug %q", slug)
		}
		seen[slug] = struct{}{}
		slugs = append(slugs, slug)
	}
	return slugs, nil
}
