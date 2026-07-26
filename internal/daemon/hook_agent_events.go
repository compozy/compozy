package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

const (
	hookAgentEventsRejectedKey = "rejected"
)

const (
	hookAgentEventsCompletedKey = "completed"
	hookAgentEventsBlockedKey   = "blocked"
	hookAgentEventsDenyKey      = "deny"
	hookAgentEventsFailedKey    = "failed"
	hookAgentEventsPendingKey   = "pending"
	hookAgentEventsResolvedKey  = "resolved"
	hookAgentEventsToolCallKey  = "tool_call"
)

type hookAgentToolPayload struct {
	SessionUpdate string          `json:"sessionUpdate"`
	Status        string          `json:"status,omitempty"`
	Title         string          `json:"title,omitempty"`
	Kind          string          `json:"kind,omitempty"`
	ToolCallID    string          `json:"toolCallId,omitempty"`
	ToolInput     json.RawMessage `json:"rawInput,omitempty"`
	ToolResult    json.RawMessage `json:"rawOutput,omitempty"`
	Meta          map[string]any  `json:"_meta,omitempty"`
}

type hookAgentPermissionPayload struct {
	RequestID string                      `json:"request_id"`
	Decision  string                      `json:"decision,omitempty"`
	ToolInput json.RawMessage             `json:"tool_input,omitempty"`
	Options   []hookspkg.PermissionOption `json:"options,omitempty"`
	ToolCall  hookspkg.PermissionToolCall `json:"tool_call"`
}

const hookPermissionDecisionDenied = "denied"

func dispatchACPAgentHookEvent(
	ctx context.Context,
	logger *slog.Logger,
	hooks hookRuntime,
	sessionCtx hookspkg.SessionContext,
	event any,
	timestamp time.Time,
) {
	if hooks == nil {
		return
	}
	agentEvent, ok := normalizeHookAgentEvent(event)
	if !ok {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	if sessionCtx.SessionID == "" {
		sessionCtx.SessionID = strings.TrimSpace(agentEvent.SessionID)
	}

	switch agentEvent.Type {
	case acp.EventTypeToolCall, acp.EventTypeToolResult:
		dispatchToolHookEvent(ctx, logger, hooks, sessionCtx, agentEvent, timestamp)
	case acp.EventTypePermission:
		dispatchPermissionHookEvent(ctx, logger, hooks, sessionCtx, agentEvent, timestamp)
	}
}

func dispatchToolHookEvent(
	ctx context.Context,
	logger *slog.Logger,
	hooks hookRuntime,
	sessionCtx hookspkg.SessionContext,
	event acp.AgentEvent,
	defaultTimestamp time.Time,
) {
	raw, ok := decodeHookAgentToolPayload(event.Raw)
	if !ok {
		return
	}
	base := hookspkg.PayloadBase{Timestamp: hookEventTimestamp(event.Timestamp, defaultTimestamp)}
	turn := hookspkg.TurnContext{TurnID: strings.TrimSpace(event.TurnID)}
	ref := hookspkg.ToolCallRef{
		ToolCallID: firstNonEmpty(strings.TrimSpace(event.ToolCallID), strings.TrimSpace(raw.ToolCallID)),
		ToolID:     hookAgentToolName(raw, strings.TrimSpace(event.Title)),
		ReadOnly:   strings.EqualFold(strings.TrimSpace(raw.Kind), "read"),
	}

	updateType := strings.ToLower(strings.TrimSpace(raw.SessionUpdate))
	status := strings.ToLower(strings.TrimSpace(raw.Status))
	switch {
	case updateType == hookAgentEventsToolCallKey && !event.ToolPrechecked && status != hookAgentEventsPendingKey:
		_, err := hooks.DispatchToolPreCall(ctx, hookspkg.ToolPreCallPayload{
			PayloadBase:    withHookEvent(base, hookspkg.HookToolPreCall),
			SessionContext: sessionCtx,
			TurnContext:    turn,
			ToolCallRef:    ref,
			ToolInput:      acp.CloneRawMessage(raw.ToolInput),
		})
		warnHookAgentDispatch(ctx, logger, hookspkg.HookToolPreCall, err)
	case updateType == "tool_call_update" && status == hookAgentEventsCompletedKey:
		_, err := hooks.DispatchToolPostCall(ctx, hookspkg.ToolPostCallPayload{
			PayloadBase:    withHookEvent(base, hookspkg.HookToolPostCall),
			SessionContext: sessionCtx,
			TurnContext:    turn,
			ToolCallRef:    ref,
			Title:          firstNonEmpty(strings.TrimSpace(event.Title), strings.TrimSpace(raw.Title)),
			ToolInput:      acp.CloneRawMessage(raw.ToolInput),
			ToolResult:     acp.CloneRawMessage(raw.ToolResult),
		})
		warnHookAgentDispatch(ctx, logger, hookspkg.HookToolPostCall, err)
	case updateType == "tool_call_update" && status == hookAgentEventsFailedKey:
		_, err := hooks.DispatchToolPostError(ctx, hookspkg.ToolPostErrorPayload{
			PayloadBase:    withHookEvent(base, hookspkg.HookToolPostError),
			SessionContext: sessionCtx,
			TurnContext:    turn,
			ToolCallRef:    ref,
			Title:          firstNonEmpty(strings.TrimSpace(event.Title), strings.TrimSpace(raw.Title)),
			ToolInput:      acp.CloneRawMessage(raw.ToolInput),
			Error:          firstNonEmpty(strings.TrimSpace(event.Error), strings.TrimSpace(string(raw.ToolResult))),
		})
		warnHookAgentDispatch(ctx, logger, hookspkg.HookToolPostError, err)
	}
}

func dispatchPermissionHookEvent(
	ctx context.Context,
	logger *slog.Logger,
	hooks hookRuntime,
	sessionCtx hookspkg.SessionContext,
	event acp.AgentEvent,
	defaultTimestamp time.Time,
) {
	raw, ok := decodeHookAgentPermissionPayload(event.Raw)
	if !ok {
		return
	}
	base := hookspkg.PayloadBase{Timestamp: hookEventTimestamp(event.Timestamp, defaultTimestamp)}
	turn := hookspkg.TurnContext{TurnID: strings.TrimSpace(event.TurnID)}
	decision := firstNonEmpty(strings.TrimSpace(event.Decision), strings.TrimSpace(raw.Decision))
	decisionClass := hookPermissionDecisionClass(decision)

	switch {
	case decision == "":
		_, err := hooks.DispatchPermissionRequest(ctx, hookspkg.PermissionRequestPayload{
			PayloadBase:    withHookEvent(base, hookspkg.HookPermissionRequest),
			SessionContext: sessionCtx,
			TurnContext:    turn,
			RequestID:      firstNonEmpty(strings.TrimSpace(event.RequestID), strings.TrimSpace(raw.RequestID)),
			Action:         strings.TrimSpace(event.Action),
			Resource:       strings.TrimSpace(event.Resource),
			DecisionClass:  decisionClass,
			ToolInput:      acp.CloneRawMessage(raw.ToolInput),
			ToolCall:       clonePermissionToolCall(raw.ToolCall),
			Options:        clonePermissionOptions(raw.Options),
		})
		warnHookAgentDispatch(ctx, logger, hookspkg.HookPermissionRequest, err)
	case hookPermissionDenied(decision):
		_, err := hooks.DispatchPermissionDenied(ctx, hookspkg.PermissionDeniedPayload{
			PayloadBase:    withHookEvent(base, hookspkg.HookPermissionDenied),
			SessionContext: sessionCtx,
			TurnContext:    turn,
			RequestID:      firstNonEmpty(strings.TrimSpace(event.RequestID), strings.TrimSpace(raw.RequestID)),
			Action:         strings.TrimSpace(event.Action),
			Resource:       strings.TrimSpace(event.Resource),
			Decision:       decision,
			DecisionClass:  decisionClass,
			ToolInput:      acp.CloneRawMessage(raw.ToolInput),
			ToolCall:       clonePermissionToolCall(raw.ToolCall),
		})
		warnHookAgentDispatch(ctx, logger, hookspkg.HookPermissionDenied, err)
	default:
		_, err := hooks.DispatchPermissionResolved(ctx, hookspkg.PermissionResolvedPayload{
			PayloadBase:    withHookEvent(base, hookspkg.HookPermissionResolved),
			SessionContext: sessionCtx,
			TurnContext:    turn,
			RequestID:      firstNonEmpty(strings.TrimSpace(event.RequestID), strings.TrimSpace(raw.RequestID)),
			Action:         strings.TrimSpace(event.Action),
			Resource:       strings.TrimSpace(event.Resource),
			Decision:       decision,
			DecisionClass:  decisionClass,
			ToolInput:      acp.CloneRawMessage(raw.ToolInput),
			ToolCall:       clonePermissionToolCall(raw.ToolCall),
		})
		warnHookAgentDispatch(ctx, logger, hookspkg.HookPermissionResolved, err)
	}
}

func normalizeHookAgentEvent(event any) (acp.AgentEvent, bool) {
	switch typed := event.(type) {
	case acp.AgentEvent:
		return typed, true
	case *acp.AgentEvent:
		if typed == nil {
			return acp.AgentEvent{}, false
		}
		return *typed, true
	default:
		return acp.AgentEvent{}, false
	}
}

func decodeHookAgentToolPayload(raw json.RawMessage) (hookAgentToolPayload, bool) {
	if len(raw) == 0 {
		return hookAgentToolPayload{}, false
	}
	var payload hookAgentToolPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return hookAgentToolPayload{}, false
	}
	return payload, true
}

func decodeHookAgentPermissionPayload(raw json.RawMessage) (hookAgentPermissionPayload, bool) {
	if len(raw) == 0 {
		return hookAgentPermissionPayload{}, false
	}
	var payload hookAgentPermissionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return hookAgentPermissionPayload{}, false
	}
	return payload, true
}

func withHookEvent(base hookspkg.PayloadBase, event hookspkg.HookEvent) hookspkg.PayloadBase {
	base.Event = event
	return base
}

func hookEventTimestamp(eventTimestamp time.Time, fallback time.Time) time.Time {
	if !eventTimestamp.IsZero() {
		return eventTimestamp
	}
	if !fallback.IsZero() {
		return fallback
	}
	return time.Now().UTC()
}

func hookAgentToolName(payload hookAgentToolPayload, fallback string) string {
	if len(payload.Meta) > 0 {
		for _, value := range payload.Meta {
			nested, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if toolName := strings.TrimSpace(stringMapValue(nested, "toolName")); toolName != "" {
				return toolName
			}
		}
	}
	return firstNonEmpty(strings.TrimSpace(payload.Title), strings.TrimSpace(payload.Kind), fallback)
}

func hookPermissionDecisionClass(decision string) string {
	if decision == "" {
		return string(SessionClassInteractive)
	}
	if hookPermissionDenied(decision) {
		return hookPermissionDecisionDenied
	}
	return hookAgentEventsResolvedKey
}

func hookPermissionDenied(decision string) bool {
	clean := strings.ToLower(strings.TrimSpace(decision))
	switch {
	case clean == "":
		return false
	case clean == daemonBlockKey, clean == hookAgentEventsBlockedKey:
		return true
	case clean == hookAgentEventsDenyKey,
		clean == hookPermissionDecisionDenied,
		clean == "reject",
		clean == hookAgentEventsRejectedKey:
		return true
	case strings.HasPrefix(clean, "block-"):
		return true
	case strings.HasPrefix(clean, "deny-"):
		return true
	case strings.HasPrefix(clean, "reject-"):
		return true
	default:
		return false
	}
}

func clonePermissionToolCall(src hookspkg.PermissionToolCall) hookspkg.PermissionToolCall {
	cloned := src
	if len(src.Locations) > 0 {
		cloned.Locations = append([]hookspkg.ToolLocation(nil), src.Locations...)
	}
	return cloned
}

func clonePermissionOptions(src []hookspkg.PermissionOption) []hookspkg.PermissionOption {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]hookspkg.PermissionOption, 0, len(src))
	cloned = append(cloned, src...)
	return cloned
}

func warnHookAgentDispatch(ctx context.Context, logger *slog.Logger, event hookspkg.HookEvent, err error) {
	if err == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	logger.WarnContext(ctx, "daemon: hook agent-event dispatch failed", "hook_event", event.String(), "error", err)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringMapValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key]
	if !ok {
		return ""
	}
	typed, ok := raw.(string)
	if !ok {
		return ""
	}
	return typed
}
