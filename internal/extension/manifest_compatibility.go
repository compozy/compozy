package extensionpkg

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/version"
)

func validateDaemonCompatibility(minVersion string) error {
	current := version.Current().Version
	currentVersion, ok := parseSemanticVersion(normalizeDaemonVersionForCompatibility(current))
	if !ok {
		return nil
	}

	requiredVersion, ok := parseSemanticVersion(minVersion)
	if !ok {
		return &ManifestValidationError{
			Field:   "min_agh_version",
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

func normalizeDaemonVersionForCompatibility(value string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "-dirty")
	commitSep := strings.LastIndex(trimmed, "-g")
	if commitSep < 0 || commitSep+2 >= len(trimmed) {
		return trimmed
	}
	commit := trimmed[commitSep+2:]
	if !isGitDescribeShortSHA(commit) {
		return trimmed
	}
	beforeCommit := trimmed[:commitSep]
	countSep := strings.LastIndex(beforeCommit, "-")
	if countSep < 0 || countSep+1 >= len(beforeCommit) {
		return trimmed
	}
	for _, char := range beforeCommit[countSep+1:] {
		if char < '0' || char > '9' {
			return trimmed
		}
	}
	return beforeCommit[:countSep]
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
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}
