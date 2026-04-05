package tasks

import "strings"

var legacyTypeRemap = map[string]string{
	"bug fix":                "bugfix",
	"bugfix":                 "bugfix",
	"fix":                    "bugfix",
	"refactor":               "refactor",
	"refactoring":            "refactor",
	"documentation":          "docs",
	"docs":                   "docs",
	"test":                   "test",
	"testing":                "test",
	"chore":                  "chore",
	"cleanup":                "chore",
	"configuration":          "infra",
	"config":                 "infra",
	"infrastructure":         "infra",
	"feature":                "",
	"feature implementation": "",
}

// RemapLegacyTaskType canonicalizes a legacy v1 task type into an allowed v2
// registry value when possible. It returns the empty string when the legacy
// type requires manual selection or has no known mapping.
func RemapLegacyTaskType(raw string, registry *TypeRegistry) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}

	if mapped, ok := legacyTypeRemap[normalized]; ok {
		if mapped == "" {
			return ""
		}
		if registry != nil && registry.IsAllowed(mapped) {
			return mapped
		}
		return ""
	}

	for _, candidate := range registry.Values() {
		if strings.EqualFold(candidate, normalized) {
			return candidate
		}
	}

	return ""
}
