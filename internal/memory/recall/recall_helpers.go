package recall

import (
	"fmt"

	"strings"
	"time"
	"unicode"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func isTrivialQuery(query string) bool {
	tokens := meaningfulTokens(query)
	if len(tokens) >= trivialTokenFloor {
		return false
	}
	for _, token := range tokens {
		if containsNonASCII(token) && len([]rune(token)) >= nonASCIITokenFloor {
			return false
		}
	}
	return true
}

func meaningfulTokens(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(query)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	tokens := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		token := strings.TrimSpace(field)
		if len([]rune(token)) < 2 {
			continue
		}
		if _, stop := stopWords[token]; stop {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}

func normalizeOptions(opts memcontract.RecallOptions) memcontract.RecallOptions {
	if opts.TopK <= 0 {
		opts.TopK = defaultTopK
	}
	opts.TopK = min(opts.TopK, maxTopK)
	if opts.RawCandidates <= 0 {
		opts.RawCandidates = defaultRawCandidates
	}
	opts.RawCandidates = min(opts.RawCandidates, maxRawCandidates)
	return opts
}

func normalizeWeights(weights Weights) Weights {
	total := weights.Unicode + weights.Trigram + weights.Recency + weights.Signal
	if total <= 0 {
		return defaultWeights
	}
	return Weights{
		Unicode: weights.Unicode / total,
		Trigram: weights.Trigram / total,
		Recency: weights.Recency / total,
		Signal:  weights.Signal / total,
	}
}

func surfacedSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}

func ageDays(modTime time.Time, now time.Time) int {
	if modTime.IsZero() {
		return 0
	}
	days := calendarDayNumber(now) - calendarDayNumber(modTime)
	if days < 0 {
		return 0
	}
	return days
}

func stalenessBanner(age int) string {
	if age <= 1 {
		return ""
	}
	return fmt.Sprintf("This memory is %d days old. Verify against current state before asserting as fact.", age)
}

func calendarDayNumber(value time.Time) int {
	year, month, day := value.UTC().Date()
	return int(time.Date(year, month, day, 12, 0, 0, 0, time.UTC).Unix() / int64(24*time.Hour/time.Second))
}

func packagedEntryCount(packaged memcontract.Packaged) int {
	return packagedBlockEntryCount(packaged.Blocks)
}

func packagedBlockEntryCount(blocks []memcontract.Block) int {
	count := 0
	for _, block := range blocks {
		count += len(block.Entries)
	}
	return count
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func containsNonASCII(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}
