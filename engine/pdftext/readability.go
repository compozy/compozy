package pdftext

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Stats captures readability metrics for extracted PDF text.
type Stats struct {
	RuneCount         int
	WordCount         int
	SpaceCount        int
	LineCount         int
	AverageWordLength float64
	SpaceRatio        float64
	FallbackUsed      bool
}

// Issues returns human-readable reasons when the extracted text appears unreadable.
func (s Stats) Issues() []string {
	issues := make([]string, 0, 3)
	if s.RuneCount == 0 {
		issues = append(issues, "no text extracted")
		return issues
	}
	if s.SpaceRatio < 0.08 {
		issues = append(issues, formatMetric("space ratio", s.SpaceRatio))
	}
	if s.AverageWordLength > 14 {
		issues = append(issues, formatMetric("average word length", s.AverageWordLength))
	}
	if s.WordCount == 0 {
		issues = append(issues, "no words detected")
	}
	return issues
}

// IsReadable reports whether the extracted text looks human-readable.
func (s Stats) IsReadable() bool {
	return len(s.Issues()) == 0
}

func computeStats(text string, fallbackUsed bool) Stats {
	stats := Stats{FallbackUsed: fallbackUsed}
	if text == "" {
		return stats
	}
	stats.RuneCount = utf8.RuneCountInString(text)
	stats.SpaceCount = countSpaces(text)
	stats.SpaceRatio = safeRatio(stats.SpaceCount, stats.RuneCount)
	stats.WordCount = len(strings.Fields(text))
	if stats.WordCount > 0 {
		stats.AverageWordLength = averageWordLength(text, stats.WordCount)
	}
	stats.LineCount = lineEstimate(text)
	return stats
}

func countSpaces(text string) int {
	total := 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			total++
		}
	}
	return total
}

func averageWordLength(text string, words int) float64 {
	if words <= 0 {
		return 0
	}
	nonSpace := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			nonSpace++
		}
	}
	if nonSpace == 0 {
		return 0
	}
	return float64(nonSpace) / float64(words)
}

func lineEstimate(text string) int {
	if text == "" {
		return 0
	}
	lines := strings.Count(text, "\n") + 1
	if lines < 1 {
		lines = 1
	}
	return lines
}

func safeRatio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func formatMetric(name string, value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 4, 64)
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	if formatted == "" {
		formatted = "0"
	}
	return name + "=" + formatted
}
