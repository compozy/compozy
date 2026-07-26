package toolmeta

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	redactpkg "github.com/compozy/agh/internal/redact"
)

var previewArgumentPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)

const (
	previewHintDelegate = "delegate"
	previewHintNone     = "none"
	previewHintPath     = "path"
	previewHintQuery    = "query"
)

// Render builds one redacted presentation payload without exposing raw arguments.
func Render(toolID string, rawInput []byte, opts RenderOptions) Presentation {
	toolID = oneLine(toolID)
	entry, native := NativeEntry(toolID)
	fallback := !native && strings.TrimSpace(opts.Descriptor.FriendlyVerb) == ""
	if !native {
		entry = Entry{Verb: toolID, Connector: " ", Emoji: "⚙️", Preview: "auto"}
	}

	if friendlyVerb := oneLine(opts.Descriptor.FriendlyVerb); friendlyVerb != "" {
		entry.Verb = friendlyVerb
		fallback = false
	}
	if hint := normalizePreviewHint(opts.Descriptor.Preview); hint != "" {
		entry.Preview = hint
	}

	preview := ""
	if !entry.DropsPreview {
		preview = buildPreview(toolID, entry.Preview, decodeInput(rawInput))
	}
	label := redactpkg.String(oneLine(entry.Verb))
	preview = redactpkg.String(oneLine(preview))
	if !opts.Verbose {
		if fallback {
			preview = capQuotedPreview(preview, opts.PreviewLimit)
		} else {
			preview = capRunes(preview, opts.PreviewLimit)
		}
	}

	return Presentation{
		ToolID:    toolID,
		Label:     label,
		Preview:   preview,
		Emoji:     entry.Emoji,
		Connector: entry.Connector,
		Fallback:  fallback,
	}
}

func capQuotedPreview(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(strconv.Quote(value)) <= limit {
		return value
	}
	runes := []rune(value)
	low, high, best := 0, len(runes), 0
	for low <= high {
		middle := low + (high-low)/2
		if utf8.RuneCountInString(strconv.Quote(string(runes[:middle]))) <= limit {
			best = middle
			low = middle + 1
			continue
		}
		high = middle - 1
	}
	if best == 0 {
		return ""
	}
	return strings.TrimSpace(string(runes[:best]))
}

func buildPreview(toolID string, hint string, input map[string]any) string {
	switch {
	case hint == previewHintNone:
		return ""
	case hint == "command":
		return terminalPreview(input)
	case hint == previewHintPath:
		return filePreview(input)
	case hint == previewHintDelegate:
		return delegatePreview(input)
	case hint == previewHintQuery:
		return searchPreview(input)
	case strings.HasPrefix(hint, "arg:"):
		argument := strings.TrimPrefix(hint, "arg:")
		if redactpkg.IsSensitiveKey(argument) {
			return ""
		}
		return scalarString(input[argument])
	default:
		switch {
		case strings.Contains(toolID, "terminal") || strings.Contains(toolID, "shell"):
			return terminalPreview(input)
		case strings.Contains(toolID, "read_file") || strings.Contains(toolID, "write_file"):
			return filePreview(input)
		case strings.Contains(toolID, previewHintDelegate):
			return delegatePreview(input)
		case strings.Contains(toolID, "search") || strings.Contains(toolID, "grep"):
			return searchPreview(input)
		default:
			return autoPreview(input)
		}
	}
}

func normalizePreviewHint(value string) string {
	hint := oneLine(value)
	normalized := strings.ToLower(hint)
	switch normalized {
	case "", "auto", previewHintNone, "command", previewHintPath, previewHintDelegate, previewHintQuery:
		return normalized
	}
	if strings.HasPrefix(normalized, "arg:") {
		argument := hint[len("arg:"):]
		if previewArgumentPattern.MatchString(argument) && !redactpkg.IsSensitiveKey(argument) {
			return "arg:" + argument
		}
	}
	return ""
}

func autoPreview(input map[string]any) string {
	for _, key := range []string{previewHintQuery, "needle", "pattern", "name", "id", previewHintPath, "file_path"} {
		if value := scalarString(input[key]); value != "" {
			return value
		}
	}
	return ""
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%g", typed)
	default:
		return ""
	}
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func capRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit]))
}
