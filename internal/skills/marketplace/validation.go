package marketplace

import (
	"path/filepath"
	"strings"
)

// NormalizeSkillSlug validates marketplace slugs accepted by configured registries.
func NormalizeSkillSlug(slug string) (string, error) {
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" {
		return "", classifiedf(ErrValidation, "skill slug is required")
	}
	if validSkillSlugPattern.MatchString(trimmed) {
		return trimmed, nil
	}
	switch {
	case trimmed == ".", trimmed == "..":
		return "", classifiedf(ErrValidation, "skill slug must not be a relative path segment")
	case filepath.IsAbs(trimmed):
		return "", classifiedf(ErrValidation, "skill slug must be relative")
	case strings.Contains(trimmed, "/"), strings.Contains(trimmed, `\`):
		return "", classifiedf(ErrValidation, "skill slug must not include path separators")
	case !validSkillNamePattern.MatchString(trimmed):
		return "", classifiedf(
			ErrValidation,
			`skill slug must match "@author/name" or contain only letters, numbers, dots, underscores, and hyphens`,
		)
	default:
		return trimmed, nil
	}
}

// NormalizeSkillName validates one local skill name.
func NormalizeSkillName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return "", classifiedf(ErrValidation, "skill name is required")
	case trimmed == ".", trimmed == "..":
		return "", classifiedf(ErrValidation, "skill name must not be a relative path segment")
	case filepath.IsAbs(trimmed):
		return "", classifiedf(ErrValidation, "skill name must be relative")
	case strings.Contains(trimmed, "/"), strings.Contains(trimmed, `\`):
		return "", classifiedf(ErrValidation, "skill name must not include path separators")
	case !validSkillNamePattern.MatchString(trimmed):
		return "", classifiedf(
			ErrValidation,
			"skill name must contain only letters, numbers, dots, underscores, and hyphens",
		)
	default:
		return trimmed, nil
	}
}
