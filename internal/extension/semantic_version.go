package extensionpkg

import (
	"strconv"
	"strings"
)

type semanticVersion struct {
	core       [3]int
	prerelease []prereleaseIdentifier
}

type prereleaseIdentifier struct {
	raw     string
	number  int
	numeric bool
}

func parseSemanticVersion(value string) (semanticVersion, bool) {
	trimmed := normalizeSemanticVersion(value)
	if trimmed == "" {
		return semanticVersion{}, false
	}

	coreAndPrerelease, buildMetadata, hasBuild := strings.Cut(trimmed, "+")
	if hasBuild && !validIdentifierList(buildMetadata, true) {
		return semanticVersion{}, false
	}

	corePart, prereleasePart, hasPrerelease := strings.Cut(coreAndPrerelease, "-")
	core, ok := parseSemanticCore(corePart)
	if !ok {
		return semanticVersion{}, false
	}

	parsed := semanticVersion{core: core}
	if !hasPrerelease {
		return parsed, true
	}

	identifiers, ok := parsePrereleaseIdentifiers(prereleasePart)
	if !ok {
		return semanticVersion{}, false
	}
	parsed.prerelease = identifiers
	return parsed, true
}

func normalizeSemanticVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	return trimmed
}

func parseSemanticCore(value string) ([3]int, bool) {
	segments := strings.Split(value, ".")
	if len(segments) != 3 {
		return [3]int{}, false
	}

	var core [3]int
	for idx, segment := range segments {
		number, ok := parseNumericVersionPart(segment)
		if !ok {
			return [3]int{}, false
		}
		core[idx] = number
	}
	return core, true
}

func parseNumericVersionPart(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	if len(value) > 1 && strings.HasPrefix(value, "0") {
		return 0, false
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return number, true
}

func parsePrereleaseIdentifiers(value string) ([]prereleaseIdentifier, bool) {
	if !validIdentifierList(value, false) {
		return nil, false
	}

	segments := strings.Split(value, ".")
	identifiers := make([]prereleaseIdentifier, 0, len(segments))
	for _, segment := range segments {
		number, numeric := parseNumericVersionPart(segment)
		if !numeric && !validPrereleasePart(segment) {
			return nil, false
		}
		identifiers = append(identifiers, prereleaseIdentifier{
			raw:     segment,
			number:  number,
			numeric: numeric,
		})
	}
	return identifiers, true
}

func validIdentifierList(value string, allowLeadingZeroNumeric bool) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}

	for segment := range strings.SplitSeq(value, ".") {
		if segment == "" {
			return false
		}
		if number, err := strconv.Atoi(segment); err == nil {
			if !allowLeadingZeroNumeric && len(segment) > 1 && strings.HasPrefix(segment, "0") {
				return false
			}
			if number < 0 {
				return false
			}
			continue
		}
		if !validPrereleasePart(segment) {
			return false
		}
	}
	return true
}

func validPrereleasePart(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return value != ""
}

func compareSemanticVersions(current, required semanticVersion) int {
	for idx := range current.core {
		switch {
		case current.core[idx] < required.core[idx]:
			return -1
		case current.core[idx] > required.core[idx]:
			return 1
		}
	}

	switch {
	case len(current.prerelease) == 0 && len(required.prerelease) == 0:
		return 0
	case len(current.prerelease) == 0:
		return 1
	case len(required.prerelease) == 0:
		return -1
	default:
		return comparePrerelease(current.prerelease, required.prerelease)
	}
}

func comparePrerelease(current, required []prereleaseIdentifier) int {
	limit := max(len(required), len(current))

	for idx := range limit {
		switch {
		case idx >= len(current):
			return -1
		case idx >= len(required):
			return 1
		}

		left := current[idx]
		right := required[idx]
		switch {
		case left.numeric && right.numeric:
			switch {
			case left.number < right.number:
				return -1
			case left.number > right.number:
				return 1
			}
		case left.numeric:
			return -1
		case right.numeric:
			return 1
		default:
			switch {
			case left.raw < right.raw:
				return -1
			case left.raw > right.raw:
				return 1
			}
		}
	}

	return 0
}
