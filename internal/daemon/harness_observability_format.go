package daemon

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func joinHarnessPromptSections(sections []HarnessPromptSection) string {
	if len(sections) == 0 {
		return "-"
	}
	names := make([]string, 0, len(sections))
	for _, section := range sections {
		if name := strings.TrimSpace(string(section)); name != "" {
			names = append(names, name)
		}
	}
	return joinHarnessNames(names)
}

func joinHarnessAugmenters(augmenters []HarnessAugmenter) string {
	if len(augmenters) == 0 {
		return "-"
	}
	names := make([]string, 0, len(augmenters))
	for _, augmenter := range augmenters {
		if name := strings.TrimSpace(string(augmenter)); name != "" {
			names = append(names, name)
		}
	}
	return joinHarnessNames(names)
}

func joinHarnessNames(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			filtered = append(filtered, truncateHarnessToken(trimmed, 80))
		}
	}
	if len(filtered) == 0 {
		return "-"
	}
	return strings.Join(filtered, "|")
}

func truncateHarnessQuoted(value string, maxRunes int) string {
	return fmt.Sprintf("%q", truncateHarnessText(value, maxRunes))
}

func truncateHarnessToken(value string, maxRunes int) string {
	return strings.ReplaceAll(truncateHarnessText(value, maxRunes), " ", "_")
}

func truncateHarnessText(value string, maxRunes int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || maxRunes <= 0 || utf8.RuneCountInString(trimmed) <= maxRunes {
		return trimmed
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))

	count := 0
	for _, r := range trimmed {
		if count >= maxRunes {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return strings.TrimSpace(builder.String()) + "..."
}
