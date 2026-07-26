package acp

import (
	"encoding/json"
	"errors"

	"strconv"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func tokenUsageFromPromptResponse(turnID string, usage *wireUsage) TokenUsage {
	if usage == nil {
		return TokenUsage{TurnID: turnID}
	}
	return TokenUsage{
		TurnID:           turnID,
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		ThoughtTokens:    usage.ThoughtTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		Timestamp:        timeNowUTC(),
	}
}

func tokenUsageFromUsageUpdate(turnID string, update wireUsageUpdate) TokenUsage {
	var amount *float64
	var currency *string
	if update.Cost != nil {
		amount = update.Cost.Amount
		currency = update.Cost.Currency
	}
	return TokenUsage{
		TurnID:       turnID,
		ContextUsed:  update.Used,
		ContextSize:  update.Size,
		CostAmount:   amount,
		CostCurrency: currency,
		Timestamp:    timeNowUTC(),
	}
}

func requestError(err error) *acpsdk.RequestError {
	if err == nil {
		return nil
	}
	if requestErr, ok := errors.AsType[*acpsdk.RequestError](err); ok {
		return requestErr
	}
	if data, ok := toolErrorRequestData(err); ok {
		return acpsdk.NewInternalError(data)
	}
	if errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrInvalidPath) ||
		errors.Is(err, ErrPathOutsideWorkspace) ||
		errors.Is(err, ErrToolBlockedForNetworkTurn) {
		return acpsdk.NewInvalidParams(map[string]any{EventTypeError: err.Error()})
	}
	return acpsdk.NewInternalError(map[string]any{EventTypeError: err.Error()})
}

func toolErrorRequestData(err error) (map[string]any, bool) {
	var toolErr *toolspkg.ToolError
	if errors.As(err, &toolErr) && toolErr != nil {
		data := map[string]any{
			EventTypeError: err.Error(),
			"tool_code":    string(toolErr.Code),
		}
		if string(toolErr.ToolID) != "" {
			data["tool_id"] = string(toolErr.ToolID)
		}
		if len(toolErr.ReasonCodes) > 0 {
			data["reason_codes"] = reasonCodesAsStrings(toolErr.ReasonCodes)
		}
		return data, true
	}
	if validation, ok := errors.AsType[*toolspkg.ValidationError](err); ok {
		data := map[string]any{
			EventTypeError: err.Error(),
			"reason":       string(validation.Reason),
		}
		if strings.TrimSpace(validation.Field) != "" {
			data["field"] = strings.TrimSpace(validation.Field)
		}
		return data, true
	}
	return nil, false
}

func reasonCodesAsStrings(reasons []toolspkg.ReasonCode) []string {
	values := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if trimmed := strings.TrimSpace(string(reason)); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func sliceLines(content string, line, limit *int) string {
	if line == nil && limit == nil {
		return content
	}

	lines := strings.Split(content, "\n")
	start := 0
	if line != nil && *line > 1 {
		start = min(*line-1, len(lines))
	}
	end := len(lines)
	if limit != nil && *limit >= 0 && start+*limit < end {
		end = start + *limit
	}
	return strings.Join(lines[start:end], "\n")
}

func extractContentText(block acpsdk.ContentBlock) string {
	switch {
	case block.Text != nil:
		return block.Text.Text
	case block.ResourceLink != nil:
		return block.ResourceLink.Uri
	default:
		return ""
	}
}

func fallbackPermissionEventRaw(requestID string, decision permissionDecision) json.RawMessage {
	var builder strings.Builder
	builder.WriteString(`{"request_id":`)
	builder.WriteString(strconv.Quote(requestID))
	if decision != "" && decision != decisionPending {
		builder.WriteString(`,"decision":`)
		builder.WriteString(strconv.Quote(string(decision)))
	}
	builder.WriteByte('}')
	return json.RawMessage(builder.String())
}

func mustMarshalJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func (p *AgentProcess) activeTurnID() string {
	p.promptMu.RLock()
	defer p.promptMu.RUnlock()
	active := p.activePrompt
	if active == nil {
		return ""
	}
	return active.turnID
}
