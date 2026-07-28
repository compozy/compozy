package redact

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

const (
	minimumEntropyTokenLength = 28
	minimumEntropyBitsPerRune = 3.6
)

var (
	entropyCandidatePattern = regexp.MustCompile(`[A-Za-z0-9_+/=]{28,}`)
	uuidPattern             = regexp.MustCompile(
		`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
	)
	hexPattern = regexp.MustCompile(`(?i)^[0-9a-f]+$`)
)

func redactEntropyCandidates(value string) string {
	indices := entropyCandidatePattern.FindAllStringIndex(value, -1)
	if len(indices) == 0 {
		return value
	}

	var redacted strings.Builder
	redacted.Grow(len(value))
	last := 0
	changed := false
	for _, bounds := range indices {
		start, end := bounds[0], bounds[1]
		candidate := value[start:end]
		if looksLikePathCandidate(value, start, candidate) || !looksLikeHighEntropySecret(candidate) {
			continue
		}
		redacted.WriteString(value[last:start])
		redacted.WriteString(Marker)
		last = end
		changed = true
	}
	if !changed {
		return value
	}
	redacted.WriteString(value[last:])
	return redacted.String()
}

func looksLikePathCandidate(value string, start int, candidate string) bool {
	if strings.HasPrefix(candidate, "/") || strings.Count(candidate, "/") > 1 {
		return true
	}
	return start > 0 && (value[start-1] == '/' || value[start-1] == '\\')
}

func looksLikeHighEntropySecret(candidate string) bool {
	if len(candidate) < minimumEntropyTokenLength || alreadyRedacted(candidate) ||
		uuidPattern.MatchString(candidate) || hexPattern.MatchString(candidate) {
		return false
	}

	var hasLower, hasUpper, hasDigit bool
	for _, r := range candidate {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLower || !hasUpper || !hasDigit {
		return false
	}

	counts := make(map[rune]int)
	var total float64
	for _, r := range strings.TrimRight(candidate, "=") {
		counts[r]++
		total++
	}
	if total == 0 {
		return false
	}
	var entropy float64
	for _, count := range counts {
		probability := float64(count) / total
		entropy -= probability * math.Log2(probability)
	}
	return entropy >= minimumEntropyBitsPerRune
}
