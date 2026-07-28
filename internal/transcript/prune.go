package transcript

import (
	"crypto/sha256"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	defaultMaxToolResultLines = 1
	defaultMaxArgBytes        = 500
	protectedTailMessages     = 8
	minPrunableResultBytes    = 200
	maxSummaryPreviewBytes    = 160
)

// PruneOptions bounds deterministic transcript pruning.
type PruneOptions struct {
	MaxToolResultLines int
	MaxArgBytes        int
	Dedup              bool
}

// Prune returns an isolated transcript projection with bounded tool noise.
func Prune(messages []Message, opts PruneOptions) []Message {
	if messages == nil {
		return nil
	}
	opts = opts.normalized()
	pruned := cloneMessages(messages)

	if opts.Dedup {
		deduplicateToolResults(pruned)
	}
	summarizeOldToolResults(pruned, opts.MaxToolResultLines)
	truncateToolInputs(pruned, opts.MaxArgBytes)
	return pruned
}

func (o PruneOptions) normalized() PruneOptions {
	if o.MaxToolResultLines <= 0 {
		o.MaxToolResultLines = defaultMaxToolResultLines
	}
	if o.MaxArgBytes <= 0 {
		o.MaxArgBytes = defaultMaxArgBytes
	}
	return o
}

func cloneMessages(messages []Message) []Message {
	cloned := make([]Message, len(messages))
	for i := range messages {
		cloned[i] = messages[i]
		cloned[i].ToolInput = cloneRawMessage(messages[i].ToolInput)
		cloned[i].ToolResult = cloneToolResult(messages[i].ToolResult)
	}
	return cloned
}

func deduplicateToolResults(messages []Message) {
	groups := make(map[[sha256.Size]byte][]int)
	for i := range messages {
		if messages[i].Role != RoleToolResult || toolResultSize(messages[i].ToolResult) < minPrunableResultBytes {
			continue
		}
		digest := sha256.Sum256([]byte(toolResultIdentity(messages[i])))
		groups[digest] = append(groups[digest], i)
	}

	for _, indices := range groups {
		if len(indices) < 2 {
			continue
		}
		newest := indices[len(indices)-1]
		newestID := messages[newest].ID
		if newestID == "" {
			newestID = "the newest matching result"
		}
		marker := "[Duplicate tool output — " + strconv.Itoa(len(indices)-1) +
			" older copies replaced; full content kept in " + newestID + "]"
		for _, index := range indices[:len(indices)-1] {
			messages[index].ToolResult = &ToolResult{Content: marker}
		}
	}
}

func toolResultIdentity(message Message) string {
	result := message.ToolResult
	if result == nil {
		return ""
	}
	return strings.Join([]string{
		strconv.Quote(message.ToolName),
		strconv.Quote(strconv.FormatBool(message.ToolError)),
		strconv.Quote(result.Stdout),
		strconv.Quote(result.Stderr),
		strconv.Quote(result.FilePath),
		strconv.Quote(result.Content),
		strconv.Quote(string(result.StructuredPatch)),
		strconv.Quote(result.Error),
		strconv.Quote(string(result.RawOutput)),
	}, "\x00")
}

func toolResultSize(result *ToolResult) int {
	if result == nil {
		return 0
	}
	return len(result.Stdout) + len(result.Stderr) + len(result.FilePath) + len(result.Content) +
		len(result.StructuredPatch) + len(result.Error) + len(result.RawOutput)
}

func summarizeOldToolResults(messages []Message, maxLines int) {
	boundary := len(messages) - protectedTailMessages
	if boundary <= 0 {
		return
	}
	for i := range boundary {
		message := &messages[i]
		if message.Role != RoleToolResult || toolResultSize(message.ToolResult) < minPrunableResultBytes {
			continue
		}
		message.ToolResult = &ToolResult{Content: summarizeToolResult(*message, maxLines)}
	}
}

func summarizeToolResult(message Message, maxLines int) string {
	result := message.ToolResult
	body := firstNonEmpty(
		result.Error,
		result.Stderr,
		result.Content,
		result.Stdout,
		string(result.RawOutput),
		string(result.StructuredPatch),
	)
	trimmed := strings.TrimSpace(body)
	lines := strings.Split(trimmed, "\n")
	if trimmed == "" {
		lines = nil
	}
	previewLines := lines
	if len(previewLines) > maxLines {
		previewLines = previewLines[:maxLines]
	}
	preview := strings.Join(previewLines, " | ")
	preview = truncateUTF8Bytes(preview, maxSummaryPreviewBytes)
	if preview == "" {
		preview = "output omitted"
	}
	toolName := strings.TrimSpace(message.ToolName)
	if toolName == "" {
		toolName = "tool"
	}
	return "[" + toolName + "] " + preview + " (" + strconv.Itoa(len(lines)) +
		" lines, " + strconv.Itoa(len(body)) + " bytes)"
}

func truncateToolInputs(messages []Message, maxBytes int) {
	for i := range messages {
		if messages[i].Role != RoleToolCall || len(messages[i].ToolInput) <= maxBytes {
			continue
		}
		messages[i].ToolInput = truncateToolInput(messages[i].ToolInput, maxBytes)
	}
}

func truncateToolInput(raw json.RawMessage, maxBytes int) json.RawMessage {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return cloneRawMessage(raw)
	}
	value, changed := truncateJSONStrings(value, maxBytes)
	if !changed {
		return cloneRawMessage(raw)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return cloneRawMessage(raw)
	}
	return json.RawMessage(encoded)
}

func truncateJSONStrings(value any, maxBytes int) (any, bool) {
	switch typed := value.(type) {
	case string:
		if len(typed) <= maxBytes {
			return typed, false
		}
		return truncateUTF8Bytes(typed, maxBytes) + "...[truncated]", true
	case []any:
		changed := false
		for i := range typed {
			var childChanged bool
			typed[i], childChanged = truncateJSONStrings(typed[i], maxBytes)
			changed = changed || childChanged
		}
		return typed, changed
	case map[string]any:
		changed := false
		for key, child := range typed {
			var childChanged bool
			typed[key], childChanged = truncateJSONStrings(child, maxBytes)
			changed = changed || childChanged
		}
		return typed, changed
	default:
		return value, false
	}
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	truncated := value[:maxBytes]
	for !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}
