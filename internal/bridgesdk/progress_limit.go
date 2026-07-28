package bridgesdk

import (
	"errors"
	"fmt"
	"strings"
)

func splitProgressText(
	text string,
	limit int,
	length func(string) int,
	protectedTail string,
) ([]string, error) {
	if text == "" {
		return nil, errors.New("bridgesdk: progress text is required")
	}
	if limit <= 0 {
		return nil, errors.New("bridgesdk: progress message limit must be positive")
	}
	if length(text) <= limit {
		return []string{text}, nil
	}
	if protectedTail == "" {
		return splitProgressRunes([]rune(text), limit, length)
	}
	if !strings.HasSuffix(text, protectedTail) {
		return nil, errors.New("bridgesdk: protected progress tail is not a text suffix")
	}
	if length(protectedTail) > limit {
		return nil, fmt.Errorf(
			"bridgesdk: progress repeat suffix length %d exceeds provider limit %d",
			length(protectedTail),
			limit,
		)
	}

	baseRunes := []rune(strings.TrimSuffix(text, protectedTail))
	tailStart := longestProgressSuffixStart(baseRunes, protectedTail, limit, length)
	chunks, err := splitProgressRunes(baseRunes[:tailStart], limit, length)
	if err != nil {
		return nil, err
	}
	tail := string(baseRunes[tailStart:]) + protectedTail
	return append(chunks, tail), nil
}

func splitProgressRunes(runes []rune, limit int, length func(string) int) ([]string, error) {
	if len(runes) == 0 {
		return nil, nil
	}

	chunks := make([]string, 0, 1)
	for len(runes) > 0 {
		count := longestProgressPrefix(runes, limit, length)
		if count == 0 {
			return nil, fmt.Errorf(
				"bridgesdk: progress rune %q exceeds provider limit %d",
				string(runes[0]),
				limit,
			)
		}
		chunks = append(chunks, string(runes[:count]))
		runes = runes[count:]
	}
	return chunks, nil
}

func longestProgressPrefix(runes []rune, limit int, length func(string) int) int {
	low := 1
	high := len(runes)
	best := 0
	for low <= high {
		middle := low + (high-low)/2
		if length(string(runes[:middle])) <= limit {
			best = middle
			low = middle + 1
			continue
		}
		high = middle - 1
	}
	return best
}

func longestProgressSuffixStart(
	base []rune,
	protectedTail string,
	limit int,
	length func(string) int,
) int {
	low := 0
	high := len(base)
	bestCount := 0
	for low <= high {
		middle := low + (high-low)/2
		candidate := string(base[len(base)-middle:]) + protectedTail
		if length(candidate) <= limit {
			bestCount = middle
			low = middle + 1
			continue
		}
		high = middle - 1
	}
	return len(base) - bestCount
}
