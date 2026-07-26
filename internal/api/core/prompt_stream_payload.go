package core

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
)

func promptAgentEventPayloadFromEvent(event acp.AgentEvent) promptAgentEventPayload {
	base := AgentEventPayloadFromEvent(event)
	payload := promptAgentEventPayload{
		Type:       base.Type,
		SessionID:  base.SessionID,
		TurnID:     base.TurnID,
		RequestID:  base.RequestID,
		Text:       promptRedactString(base.Text),
		Title:      promptRedactString(base.Title),
		ToolCallID: base.ToolCallID,
		StopReason: promptRedactString(base.StopReason),
		Action:     promptRedactString(base.Action),
		Resource:   promptRedactString(base.Resource),
		Decision:   promptRedactString(base.Decision),
		Error:      promptRedactString(base.Error),
		Usage:      promptTokenUsagePayloadFromUsage(event.Usage),
		Runtime:    base.Runtime,
		Raw:        promptRedactRaw(base.Raw),
	}
	if !event.Timestamp.IsZero() {
		payload.Timestamp = event.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	return payload
}

func promptTokenUsagePayloadFromUsage(usage *acp.TokenUsage) *promptTokenUsagePayload {
	base := TokenUsagePayloadFromUsage(usage)
	if base == nil {
		return nil
	}

	payload := &promptTokenUsagePayload{
		TurnID:           base.TurnID,
		InputTokens:      base.InputTokens,
		OutputTokens:     base.OutputTokens,
		TotalTokens:      base.TotalTokens,
		ThoughtTokens:    base.ThoughtTokens,
		CacheReadTokens:  base.CacheReadTokens,
		CacheWriteTokens: base.CacheWriteTokens,
		ContextUsed:      base.ContextUsed,
		ContextSize:      base.ContextSize,
		CostAmount:       base.CostAmount,
		CostCurrency:     base.CostCurrency,
	}
	if !base.Timestamp.IsZero() {
		payload.Timestamp = base.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	return payload
}

func promptToolInputReady(input any) bool {
	switch value := input.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(value) != ""
	case []any:
		return len(value) > 0
	case map[string]any:
		return len(value) > 0
	default:
		return true
	}
}

func promptFirstNonNil(values ...any) (any, bool) {
	for _, value := range values {
		if value != nil {
			return value, true
		}
	}
	return nil, false
}
