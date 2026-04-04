package run

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

const toolNameGeneric = "Tool Call"

func toolUseDisplayTitle(toolUse model.ToolUseBlock) string {
	base := strings.TrimSpace(toolUse.Name)
	if preferred := preferredToolUseTitle(toolUse); preferred != "" {
		base = preferred
	}
	if base == "" {
		base = toolNameGeneric
	}
	summary := toolUseSummary(toolUse)
	if summary == "" {
		return base
	}
	if strings.Contains(strings.ToLower(base), strings.ToLower(summary)) {
		return base
	}
	return fmt.Sprintf("%s %s", base, summary)
}

func toolUseSummary(toolUse model.ToolUseBlock) string {
	input := decodeToolUseInput(toolUse.Input)
	if len(input) == 0 {
		return ""
	}
	if summary := summarizeCommandInput(input); summary != "" {
		return summary
	}
	if summary := summarizeFileInput(input); summary != "" {
		return summary
	}
	if summary := summarizeQueryInput(input); summary != "" {
		return summary
	}
	if summary := summarizeURLInput(input); summary != "" {
		return summary
	}
	if summary := summarizePatternInput(input); summary != "" {
		return summary
	}
	if summary := summarizeRefInput(input); summary != "" {
		return summary
	}
	if summary := summarizeSimpleFields(input); summary != "" {
		return summary
	}
	return summarizeTeamInput(input)
}

func summarizeCommandInput(input map[string]any) string {
	if command := extractToolSummaryString(input, "command"); command != "" {
		return truncateSingleLine(command)
	}
	return ""
}

func summarizeFileInput(input map[string]any) string {
	path := firstNonEmptySummary(input, "file_path", "path")
	if path == "" {
		return ""
	}
	rangeSummary := ""
	if startLine, ok := extractToolSummaryNumberAny(input, "start_line", "startLine"); ok {
		rangeSummary = fmt.Sprintf(":%d", startLine)
		if endLine, ok := extractToolSummaryNumberAny(input, "end_line", "endLine"); ok && endLine >= startLine {
			rangeSummary = fmt.Sprintf(":%d-%d", startLine, endLine)
		}
	}
	return truncateSingleLine(path + rangeSummary)
}

func summarizeQueryInput(input map[string]any) string {
	if query := firstNonEmptySummary(input, "action_query", "actionQuery"); query != "" {
		return quoteSummary(query)
	}
	if query := firstNonEmptySummary(input, "query", "q"); query != "" {
		return quoteSummary(query)
	}
	if queries := extractToolSummaryList(input, "queries"); len(queries) > 0 {
		return quoteSummary(joinListPreview(queries))
	}
	return ""
}

func summarizeURLInput(input map[string]any) string {
	url := extractToolSummaryString(input, "url")
	if url == "" {
		return ""
	}
	if pattern := extractToolSummaryString(input, "pattern"); pattern != "" {
		return truncateSingleLine(fmt.Sprintf("%s [%s]", url, pattern))
	}
	return truncateSingleLine(url)
}

func summarizePatternInput(input map[string]any) string {
	pattern := extractToolSummaryString(input, "pattern")
	if pattern == "" {
		return ""
	}
	if path := extractToolSummaryString(input, "path"); path != "" {
		return quoteSummary(pattern) + " in " + truncateSingleLine(path)
	}
	return quoteSummary(pattern)
}

func summarizeRefInput(input map[string]any) string {
	refID := firstNonEmptySummary(input, "ref_id", "refId")
	if refID == "" {
		return ""
	}
	if clickID, ok := extractToolSummaryNumberAny(input, "id"); ok {
		return truncateSingleLine(fmt.Sprintf("%s #%d", refID, clickID))
	}
	return truncateSingleLine(refID)
}

func summarizeSimpleFields(input map[string]any) string {
	for _, key := range []string{"ticker", "location", "utc_offset", "slug", "prompt"} {
		if value := extractToolSummaryString(input, key); value != "" {
			return truncateSingleLine(value)
		}
	}
	return ""
}

func summarizeTeamInput(input map[string]any) string {
	team := extractToolSummaryString(input, "team")
	if team == "" {
		return ""
	}
	if opponent := extractToolSummaryString(input, "opponent"); opponent != "" {
		return truncateSingleLine(team + " vs " + opponent)
	}
	return truncateSingleLine(team)
}

func quoteSummary(value string) string {
	value = truncateSingleLine(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%q", value)
}

func joinListPreview(values []string) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return values[0]
	}
	return fmt.Sprintf("%s (+%d more)", values[0], len(values)-1)
}

func decodeToolUseInput(payload json.RawMessage) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	var record map[string]any
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil
	}
	return record
}

func preferredToolUseTitle(toolUse model.ToolUseBlock) string {
	title := strings.TrimSpace(toolUse.Title)
	if title == "" || strings.EqualFold(title, toolNameGeneric) {
		return ""
	}

	name := strings.TrimSpace(toolUse.Name)
	if name == "" || strings.EqualFold(name, toolNameGeneric) {
		return title
	}
	if canonicalToolSummaryToken(title) == canonicalToolSummaryToken(name) {
		return ""
	}
	if looksLikeToolIdentifier(title) {
		return ""
	}
	return title
}

func looksLikeToolIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "mcp__") {
		return true
	}
	return strings.ContainsAny(value, "_.")
}

func canonicalToolSummaryToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("_", "", "-", "", " ", "", ".", "")
	return replacer.Replace(value)
}

func extractToolSummaryString(record map[string]any, key string) string {
	if record == nil {
		return ""
	}
	value, ok := record[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func firstNonEmptySummary(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := extractToolSummaryString(record, key); value != "" {
			return value
		}
	}
	return ""
}

func extractToolSummaryNumberAny(record map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := extractToolSummaryNumber(record, key); ok {
			return value, true
		}
	}
	return 0, false
}

func extractToolSummaryNumber(record map[string]any, key string) (int, bool) {
	if record == nil {
		return 0, false
	}
	value, ok := record[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	}
	return 0, false
}

func extractToolSummaryList(record map[string]any, key string) []string {
	if record == nil {
		return nil
	}
	value, ok := record[key]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
