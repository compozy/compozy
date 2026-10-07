package extensionpkg

import (
	"errors"
	"fmt"
	"slices"

	"strings"

	"github.com/compozy/compozy/internal/version"
)

func validateDaemonCompatibility(minVersion string) error {
	current := version.Current().Version
	return validateCompozyVersionCompatibility(current, minVersion)
}

func validateCompozyVersionCompatibility(current, minVersion string) error {
	currentVersion, ok := parseSemanticVersion(normalizeDaemonVersionForCompatibility(current))
	if !ok {
		return nil
	}

	requiredVersion, ok := parseSemanticVersion(minVersion)
	if !ok {
		return &ManifestValidationError{
			Field:   manifestMinCompozyVersionKey,
			Value:   minVersion,
			Message: manifestMustBeASemanticVersionValue,
		}
	}

	if compareSemanticVersions(currentVersion, requiredVersion) >= 0 {
		return nil
	}

	return &ManifestCompatibilityError{
		CurrentVersion: current,
		MinVersion:     strings.TrimSpace(minVersion),
	}
}

// ValidateManifestForCompozyVersion checks a manifest against an explicitly stamped daemon version.
func ValidateManifestForCompozyVersion(manifest *Manifest, currentVersion string) error {
	if manifest == nil {
		return errors.New("extension: manifest is required")
	}
	if _, ok := parseSemanticVersion(normalizeDaemonVersionForCompatibility(currentVersion)); !ok {
		return &ManifestValidationError{
			Field:   "compozy_version",
			Value:   currentVersion,
			Message: manifestMustBeASemanticVersionValue,
		}
	}
	return validateCompozyVersionCompatibility(currentVersion, manifest.MinCompozyVersion)
}

func normalizeDaemonVersionForCompatibility(value string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "-dirty")
	beforeCommit, commit, found := strings.CutLast(trimmed, "-g")
	if !found || commit == "" {
		return trimmed
	}
	if !isGitDescribeShortSHA(commit) {
		return trimmed
	}
	version, count, found := strings.CutLast(beforeCommit, "-")
	if !found || count == "" {
		return trimmed
	}
	for _, char := range count {
		if char < '0' || char > '9' {
			return trimmed
		}
	}
	return version
}

func isGitDescribeShortSHA(value string) bool {
	if len(value) < 7 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func validateDottedIdentifiers(field string, values []string, allowWildcards bool) error {
	for idx, value := range values {
		if err := validateSeparatedIdentifier(value, ".", allowWildcards); err != nil {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("%s[%d]", field, idx),
				Value:   value,
				Message: err.Error(),
			}
		}
	}
	return nil
}

func validateSlashIdentifiers(field string, values []string) error {
	for idx, value := range values {
		if err := validateSeparatedIdentifier(value, "/", false); err != nil {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("%s[%d]", field, idx),
				Value:   value,
				Message: err.Error(),
			}
		}
	}
	return nil
}

func validateSeparatedIdentifier(value, separator string, allowWildcards bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("value is required")
	}
	if allowWildcards && trimmed == "*" {
		return nil
	}

	parts := strings.Split(trimmed, separator)
	if len(parts) < 2 {
		return fmt.Errorf("must use %q-separated identifiers", separator)
	}

	for _, part := range parts {
		if allowWildcards && part == "*" {
			continue
		}
		if !validIdentifierPart(part) {
			return fmt.Errorf("contains invalid identifier segment %q", part)
		}
	}
	return nil
}

func validIdentifierPart(part string) bool {
	if part == "" {
		return false
	}

	for idx, r := range part {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if idx == 0 {
				return false
			}
		case r == '_' || r == '-':
			if idx == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func providesCapability(values []string, want string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		return strings.TrimSpace(value) == strings.TrimSpace(want)
	})
}
