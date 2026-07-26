package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/tools"
)

func actionContextWithNodeTimeout(
	ctx context.Context,
	rawTimeout string,
) (context.Context, context.CancelFunc, error) {
	timeout, err := parseActionTimeout(rawTimeout)
	if err != nil {
		return nil, nil, err
	}
	if timeout <= 0 {
		return ctx, func() {}, nil
	}
	nextCtx, cancel := context.WithTimeout(ctx, timeout)
	return nextCtx, cancel, nil
}

func parseActionTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	timeout, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%w: parse action timeout %q: %w", ErrValidation, trimmed, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("%w: action timeout must be positive", ErrValidation)
	}
	return timeout, nil
}

func actionSessionHandle(spec *dsl.SessionSpec) string {
	if spec != nil {
		if handle := strings.TrimSpace(spec.Handle); handle != "" {
			return handle
		}
	}
	return defaultActionSessionHandle
}

func harvestKind(node dsl.Node) string {
	if node.Harvest == nil {
		return harvestKindSync
	}
	return strings.TrimSpace(node.Harvest.Kind)
}

func outputFromRaw(raw ActionRawResult) (ActionOutput, error) {
	value := raw.Value
	if value == nil && len(bytes.TrimSpace(raw.Structured)) > 0 {
		var decoded any
		if err := json.Unmarshal(raw.Structured, &decoded); err != nil {
			return ActionOutput{}, fmt.Errorf("decode action structured output: %w", err)
		}
		value = decoded
	}
	return ActionOutput{
		Structured:     cloneRawMessage(raw.Structured),
		Value:          value,
		Text:           raw.Text,
		SessionID:      raw.SessionID,
		EventStartSeq:  raw.EventStartSeq,
		EventEndSeq:    raw.EventEndSeq,
		TokensUsed:     raw.TokensUsed,
		ChildLoopRunID: raw.ChildLoopRunID,
		Status:         raw.Status,
	}, nil
}

func validateEventRange(raw ActionRawResult) error {
	if strings.TrimSpace(raw.SessionID) == "" {
		return fmt.Errorf("%w: event range session_id is required", ErrValidation)
	}
	if raw.EventStartSeq <= 0 || raw.EventEndSeq <= 0 {
		return fmt.Errorf("%w: event range start and end sequence are required", ErrValidation)
	}
	if raw.EventEndSeq < raw.EventStartSeq {
		return fmt.Errorf("%w: event range end precedes start", ErrValidation)
	}
	return nil
}

func validateSyncHarvest(node dsl.Node) error {
	kind := harvestKind(node)
	switch kind {
	case "", harvestKindSync:
		return nil
	default:
		return fmt.Errorf(
			"%w: harvest kind %q is not supported for action kind %q",
			ErrValidation,
			kind,
			node.Kind,
		)
	}
}

func normalizeActionToolScope(in ActionExecutionInput) tools.Scope {
	scope := in.ToolScope
	if scope.WorkspaceID == "" {
		scope.WorkspaceID = string(in.WorkspaceID)
	}
	if scope.ActorKind == "" && in.Actor.Actor.Kind != "" {
		scope.ActorKind = string(in.Actor.Actor.Kind.Normalize())
	}
	return scope
}

func actionToolCallID(node dsl.Node, in ActionExecutionInput) string {
	if in.ItemIndex > 0 {
		return fmt.Sprintf("%s:%d", node.ID, in.ItemIndex)
	}
	return string(node.ID)
}

func metadataString(metadata map[string]json.RawMessage, key string) string {
	raw, ok := metadata[key]
	if !ok || len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	// Optional tool metadata is best-effort; malformed values degrade to the zero value.
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func metadataInt64(metadata map[string]json.RawMessage, key string) int64 {
	raw, ok := metadata[key]
	if !ok || len(bytes.TrimSpace(raw)) == 0 {
		return 0
	}
	// Optional tool metadata is best-effort; malformed values degrade to the zero value.
	var number int64
	if err := json.Unmarshal(raw, &number); err == nil {
		return number
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
