package modelcatalog

import (
	"fmt"
	"regexp"
	"strings"
)

var sourceSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidateSourceID checks a stable catalog source identity.
func ValidateSourceID(sourceID string) error {
	trimmed := strings.TrimSpace(sourceID)
	if trimmed == "" {
		return fmt.Errorf("model catalog source id is required")
	}
	if trimmed != sourceID {
		return fmt.Errorf("model catalog source id %q must not include surrounding whitespace", sourceID)
	}
	if staticSourceKind(trimmed) != "" {
		return nil
	}
	kind, slug, ok := strings.Cut(trimmed, ":")
	if !ok || kind == "" || slug == "" {
		return fmt.Errorf("model catalog source id %q must be static or <kind>:<slug>", sourceID)
	}
	switch SourceKind(kind) {
	case SourceKindProviderLive, SourceKindExtension, SourceKindACPSession:
	default:
		return fmt.Errorf("model catalog source id %q uses unsupported dynamic kind %q", sourceID, kind)
	}
	if !sourceSlugPattern.MatchString(slug) {
		return fmt.Errorf("model catalog source id %q slug must match ^[a-z0-9][a-z0-9_-]*$", sourceID)
	}
	return nil
}

// ValidateSourceIdentity checks that a source id and kind describe the same source family.
func ValidateSourceIdentity(sourceID string, kind SourceKind) error {
	if err := ValidateSourceID(sourceID); err != nil {
		return err
	}
	trimmedID := sourceID
	trimmedKind := SourceKind(strings.TrimSpace(string(kind)))
	if trimmedKind == "" {
		return fmt.Errorf("model catalog source kind is required")
	}
	if staticKind := staticSourceKind(trimmedID); staticKind != "" {
		if staticKind != trimmedKind {
			return fmt.Errorf("model catalog source id %q requires kind %q, got %q", trimmedID, staticKind, trimmedKind)
		}
		return nil
	}
	prefix, _, _ := strings.Cut(trimmedID, ":")
	if SourceKind(prefix) != trimmedKind {
		return fmt.Errorf("model catalog source id %q requires kind %q, got %q", trimmedID, prefix, trimmedKind)
	}
	return nil
}

// SourceKindExtensionID returns the stable source id for an extension model source.
func SourceKindExtensionID(extensionName string) (string, error) {
	slug, err := NormalizeExtensionSourceSlug(extensionName)
	if err != nil {
		return "", err
	}
	return string(SourceKindExtension) + ":" + slug, nil
}

// NormalizeExtensionSourceSlug converts an extension name into the dynamic source-id slug.
func NormalizeExtensionSourceSlug(extensionName string) (string, error) {
	trimmed := strings.TrimSpace(extensionName)
	if trimmed == "" {
		return "", fmt.Errorf("model catalog extension source name is required")
	}
	var builder strings.Builder
	lastSeparator := false
	for _, r := range trimmed {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
			lastSeparator = false
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastSeparator = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastSeparator = false
		case r == '-' || r == '_':
			builder.WriteRune(r)
			lastSeparator = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if !lastSeparator {
				builder.WriteRune('-')
				lastSeparator = true
			}
		default:
			return "", fmt.Errorf("model catalog extension source slug cannot include %q", string(r))
		}
	}
	slug := builder.String()
	if !sourceSlugPattern.MatchString(slug) {
		return "", fmt.Errorf("model catalog extension source slug %q must match ^[a-z0-9][a-z0-9_-]*$", slug)
	}
	return slug, nil
}

func staticSourceKind(sourceID string) SourceKind {
	switch sourceID {
	case SourceIDBuiltin:
		return SourceKindBuiltin
	case SourceIDConfig:
		return SourceKindConfig
	case SourceIDModelsDev:
		return SourceKindModelsDev
	default:
		return ""
	}
}
