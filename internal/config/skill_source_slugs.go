package config

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const customSourceHashLength = 6

// CustomSourceSlugs allocates deterministic display slugs for a set of paths.
func CustomSourceSlugs(paths []string) map[string]string {
	canonicalPaths := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		canonical := canonicalSkillSourcePath(path)
		if canonical == "" {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		canonicalPaths = append(canonicalPaths, canonical)
	}
	sort.Strings(canonicalPaths)

	groups := make(map[string][]string, len(canonicalPaths))
	for _, canonical := range canonicalPaths {
		base := sanitizeSkillSourceSlug(filepath.Base(canonical))
		groups[base] = append(groups[base], canonical)
	}

	allocated := make(map[string]string, len(canonicalPaths))
	for base, group := range groups {
		if len(group) == 1 {
			allocated[group[0]] = base
			continue
		}
		for _, canonical := range group {
			digest := sha256.Sum256([]byte(canonical))
			allocated[canonical] = base + "-" + hex.EncodeToString(digest[:])[:customSourceHashLength]
		}
	}
	return allocated
}

func sanitizeSkillSourceSlug(value string) string {
	var builder strings.Builder
	lastHyphen := false
	for _, current := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			if current <= unicode.MaxASCII {
				builder.WriteRune(current)
				lastHyphen = false
				continue
			}
		}
		if builder.Len() > 0 && !lastHyphen {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "custom"
	}
	return result
}
