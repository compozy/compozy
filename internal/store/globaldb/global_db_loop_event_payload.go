package globaldb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

func normalizeLoopRunEventPayload(kind string, payload any) (json.RawMessage, error) {
	var raw json.RawMessage
	switch typed := payload.(type) {
	case nil:
		raw = json.RawMessage(`{}`)
	case json.RawMessage:
		raw = cloneLoopEventRawJSON(typed)
	case []byte:
		raw = cloneLoopEventRawJSON(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil, fmt.Errorf("store: marshal loop run event payload: %w", err)
		}
		raw = data
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%w: loop run event payload must be valid JSON", looppkg.ErrValidation)
	}
	if kind == loopRunEventTokenTick {
		return normalizeLoopTokenTickEventPayload(raw)
	}
	redacted, err := diagnostics.RedactJSON(raw)
	if err != nil {
		return boundedLoopRunEventTextPayload(string(raw))
	}
	if len(redacted) > maxLoopRunEventPayloadBytes {
		return boundedLoopRunEventTextPayload(string(redacted))
	}
	return cloneLoopEventRawJSON(redacted), nil
}

func normalizeLoopTokenTickEventPayload(raw json.RawMessage) (json.RawMessage, error) {
	var tick map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tick); err != nil {
		return nil, fmt.Errorf("%w: token_tick payload is invalid: %w", looppkg.ErrValidation, err)
	}
	var taskRunID string
	if taskRunRaw := tick[loopRunEventPayloadKeyTaskRunID]; len(bytes.TrimSpace(taskRunRaw)) > 0 {
		if err := json.Unmarshal(taskRunRaw, &taskRunID); err != nil {
			return nil, fmt.Errorf("%w: token_tick payload task_run_id is invalid: %w", looppkg.ErrValidation, err)
		}
	}
	tokensUsedRaw := tick[columnTokensUsed]
	if len(bytes.TrimSpace(tokensUsedRaw)) == 0 {
		return nil, fmt.Errorf("%w: token_tick payload tokens_used is required", looppkg.ErrValidation)
	}
	var tokensUsed int64
	if err := json.Unmarshal(tokensUsedRaw, &tokensUsed); err != nil {
		return nil, fmt.Errorf("%w: token_tick payload tokens_used is invalid: %w", looppkg.ErrValidation, err)
	}
	var terminal bool
	if terminalRaw := tick[loopRunEventPayloadKeyTerminal]; len(bytes.TrimSpace(terminalRaw)) > 0 {
		if err := json.Unmarshal(terminalRaw, &terminal); err != nil {
			return nil, fmt.Errorf("%w: token_tick payload terminal is invalid: %w", looppkg.ErrValidation, err)
		}
	}
	data, err := json.Marshal(map[string]any{
		loopRunEventPayloadKeyTaskRunID: strings.TrimSpace(taskRunID),
		columnTokensUsed:                tokensUsed,
		loopRunEventPayloadKeyTerminal:  terminal,
	})
	if err != nil {
		return nil, fmt.Errorf("store: marshal token_tick loop run event payload: %w", err)
	}
	return data, nil
}

func boundedLoopRunEventTextPayload(value string) (json.RawMessage, error) {
	bounded := diagnostics.RedactAndBound(taskpkg.RedactClaimTokens(value), maxLoopRunEventPayloadBytes/2)
	data, err := json.Marshal(map[string]any{
		loopRunEventPayloadKeyText: bounded,
		"truncated":                true,
	})
	if err != nil {
		return nil, fmt.Errorf("store: marshal bounded loop run event payload: %w", err)
	}
	return data, nil
}

func cloneLoopEventRawJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}
