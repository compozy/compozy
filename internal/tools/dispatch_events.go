package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"time"
)

// ToolEventData carries per-outcome event details.
type ToolEventData struct {
	StartedAt time.Time
	Input     json.RawMessage
	Result    ToolResult
	Err       error
}

func (r *RuntimeRegistry) emit(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
	kind ToolCallEventKind,
	data ToolEventData,
) error {
	if r.events == nil {
		return nil
	}
	event := buildToolCallEvent(target, req, kind, data)
	if err := r.events.EmitToolEvent(ctx, event); err != nil {
		return NewToolError(
			ErrorCodeBackendFailed,
			target.descriptor.ID,
			fmt.Sprintf("tool %q observability emit failed", target.descriptor.ID),
			fmt.Errorf("%w: %w", ErrToolBackendFailed, err),
			ReasonBackendUnhealthy,
		)
	}
	return nil
}

func buildToolCallEvent(
	target *dispatchTarget,
	req CallRequest,
	kind ToolCallEventKind,
	data ToolEventData,
) ToolCallEvent {
	descriptor := target.descriptor
	event := ToolCallEvent{
		Kind:          kind,
		ToolID:        descriptor.ID,
		DisplayTitle:  descriptor.DisplayTitle,
		SourceKind:    descriptor.Source.Kind,
		SourceOwner:   descriptor.Source.Owner,
		WorkspaceID:   req.WorkspaceID,
		SessionID:     req.SessionID,
		AgentName:     req.AgentName,
		Risk:          descriptor.Risk,
		ReadOnly:      descriptor.ReadOnly,
		Destructive:   descriptor.Destructive,
		OpenWorld:     descriptor.OpenWorld,
		ApprovalMode:  target.view.Decision.SystemPermissionMode,
		Decision:      target.view.Decision.RegistryPolicyResult,
		ReasonCodes:   append([]ReasonCode(nil), target.view.Decision.ReasonCodes...),
		CorrelationID: req.CorrelationID,
		InputDigest:   digestRaw(redactInputForEvents(req.Input, req.SensitiveInputFields)),
	}
	if !data.StartedAt.IsZero() {
		event.DurationMS = time.Since(data.StartedAt).Milliseconds()
	}
	event.RedactedInputFields = redactedInputFields(req.Input, req.SensitiveInputFields)
	if data.Result.Bytes > 0 || data.Result.Truncated {
		event.ResultBytes = data.Result.Bytes
		event.Truncated = data.Result.Truncated
		event.ResultDigest = digestToolResult(data.Result)
		event.ResultRedactionPaths = resultRedactionPaths(data.Result.Redactions)
	}
	if data.Err != nil {
		if toolErr, ok := errors.AsType[*ToolError](data.Err); ok {
			event.ErrorCode = toolErr.Code
			event.ReasonCodes = appendUniqueReasons(event.ReasonCodes, toolErr.ReasonCodes...)
		}
	}
	return event
}

func redactInputForEvents(input json.RawMessage, fields []string) json.RawMessage {
	redacted, _, err := redactRawJSON(normalizeCallInput(input), "$.input", normalizeSensitiveFields(fields), nil)
	if err != nil {
		return json.RawMessage(`{"invalid":true}`)
	}
	return redacted
}

func redactedInputFields(input json.RawMessage, fields []string) []string {
	_, redactions, err := redactRawJSON(normalizeCallInput(input), "$.input", normalizeSensitiveFields(fields), nil)
	if err != nil {
		return nil
	}
	values := make([]string, 0, len(redactions))
	for _, redaction := range redactions {
		values = append(values, redaction.Path)
	}
	return values
}

func digestToolResult(result ToolResult) string {
	data, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resultRedactionPaths(redactions []Redaction) []string {
	paths := make([]string, 0, len(redactions))
	for _, redaction := range redactions {
		if redaction.Path != "" {
			paths = append(paths, redaction.Path)
		}
	}
	return paths
}

func appendUniqueReasons(existing []ReasonCode, incoming ...ReasonCode) []ReasonCode {
	for _, reason := range incoming {
		if reason != "" && !slices.Contains(existing, reason) {
			existing = append(existing, reason)
		}
	}
	return existing
}
