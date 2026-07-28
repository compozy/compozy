package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

const (
	providerNativeToolIDRead  = "Read"
	providerNativeToolIDWrite = "Write"
	providerNativeToolIDBash  = "Bash"
)

// ToolExecutionRequest captures one provider-native tool call before any real
// side effect happens.
type ToolExecutionRequest struct {
	ToolID   string
	ReadOnly bool
	Input    json.RawMessage
}

// ToolExecutionGateway authoritatively intercepts provider-native tool calls
// at the execution boundary.
type ToolExecutionGateway interface {
	Intercept(context.Context, ToolExecutionRequest) (ToolExecutionRequest, error)
}

type providerNativeToolPrecheck struct {
	turnID        string
	toolID        string
	inputDigest   string
	recordedAtUTC time.Time
}

type providerNativeToolUpdatePayload struct {
	Title     string          `json:"title,omitempty"`
	Kind      string          `json:"kind,omitempty"`
	ToolInput json.RawMessage `json:"rawInput,omitempty"`
	Meta      map[string]any  `json:"_meta,omitempty"`
}

func canonicalProviderNativeToolID(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "read", "read file":
		return providerNativeToolIDRead
	case "write", "write file":
		return providerNativeToolIDWrite
	case "bash", "create terminal":
		return providerNativeToolIDBash
	default:
		return strings.TrimSpace(name)
	}
}

func cloneToolExecutionRequest(req ToolExecutionRequest) ToolExecutionRequest {
	return ToolExecutionRequest{
		ToolID:   strings.TrimSpace(req.ToolID),
		ReadOnly: req.ReadOnly,
		Input:    CloneRawMessage(req.Input),
	}
}

func providerNativeToolInputDigest(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err == nil {
		canonical, marshalErr := json.Marshal(normalized)
		if marshalErr == nil {
			return string(canonical)
		}
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return string(raw)
	}
	return compact.String()
}

func decodeProviderNativeToolUpdate(raw json.RawMessage, fallbackTitle string) (string, json.RawMessage, bool) {
	if len(raw) == 0 {
		return "", nil, false
	}

	var payload providerNativeToolUpdatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", nil, false
	}

	toolID := canonicalProviderNativeToolID(providerNativeToolName(payload, fallbackTitle))
	if toolID == "" {
		return "", nil, false
	}

	return toolID, CloneRawMessage(payload.ToolInput), true
}

func decodeProviderNativePermissionRequest(
	request acpsdk.RequestPermissionRequest,
) (ToolExecutionRequest, bool, error) {
	toolID := providerNativePermissionToolID(request)
	if toolID == "" {
		return ToolExecutionRequest{}, false, nil
	}

	input, err := marshalPermissionRequestToolInput(request.ToolCall.RawInput)
	if err != nil {
		return ToolExecutionRequest{}, false, err
	}

	return ToolExecutionRequest{
		ToolID:   toolID,
		ReadOnly: providerNativePermissionReadOnly(request),
		Input:    input,
	}, true, nil
}

func providerNativePermissionToolID(request acpsdk.RequestPermissionRequest) string {
	if request.ToolCall.Kind != nil {
		switch *request.ToolCall.Kind {
		case acpsdk.ToolKindRead:
			return providerNativeToolIDRead
		case acpsdk.ToolKindEdit:
			return providerNativeToolIDWrite
		}
	}

	toolName := providerNativeToolName(
		providerNativeToolUpdatePayload{
			Title: firstNonEmptyString(requestPermissionToolTitle(request)),
			Meta:  permissionRequestMetaMap(request.ToolCall.Meta),
		},
		requestPermissionToolTitle(request),
	)
	return canonicalProviderNativeToolID(toolName)
}

func providerNativePermissionReadOnly(request acpsdk.RequestPermissionRequest) bool {
	return request.ToolCall.Kind != nil && *request.ToolCall.Kind == acpsdk.ToolKindRead
}

func requestPermissionToolTitle(request acpsdk.RequestPermissionRequest) string {
	if request.ToolCall.Title == nil {
		return ""
	}
	return strings.TrimSpace(*request.ToolCall.Title)
}

func permissionRequestMetaMap(raw any) map[string]any {
	typed, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return typed
}

func marshalPermissionRequestToolInput(raw any) (json.RawMessage, error) {
	if raw == nil {
		return nil, nil
	}
	input, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return input, nil
}

func providerNativeToolName(payload providerNativeToolUpdatePayload, fallbackTitle string) string {
	if len(payload.Meta) > 0 {
		for _, value := range payload.Meta {
			nested, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if toolName := stringMapValue(nested, "toolName"); toolName != "" {
				return toolName
			}
		}
	}
	return firstNonEmptyString(payload.Title, payload.Kind, fallbackTitle)
}

func stringMapValue(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func providerNativeToolPatchUnsupported(
	original ToolExecutionRequest,
	patched ToolExecutionRequest,
) bool {
	if canonicalProviderNativeToolID(original.ToolID) != canonicalProviderNativeToolID(patched.ToolID) {
		return true
	}
	return providerNativeToolInputDigest(original.Input) != providerNativeToolInputDigest(patched.Input)
}

func (p *AgentProcess) interceptProviderNativeTool(
	ctx context.Context,
	request ToolExecutionRequest,
) (ToolExecutionRequest, error) {
	normalized := cloneToolExecutionRequest(request)
	normalized.ToolID = canonicalProviderNativeToolID(normalized.ToolID)
	if normalized.ToolID == "" {
		return normalized, nil
	}

	if p == nil || p.toolGateway == nil {
		return normalized, nil
	}

	patched, err := p.toolGateway.Intercept(ctx, normalized)
	if err != nil {
		return ToolExecutionRequest{}, err
	}
	p.recordProviderNativeToolPrecheck(normalized.ToolID, normalized.Input)
	return cloneToolExecutionRequest(patched), nil
}

func (p *AgentProcess) interceptProviderNativePermissionRequest(
	ctx context.Context,
	request acpsdk.RequestPermissionRequest,
) (bool, error) {
	original, ok, err := decodeProviderNativePermissionRequest(request)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	normalized := cloneToolExecutionRequest(original)
	normalized.ToolID = canonicalProviderNativeToolID(normalized.ToolID)
	if normalized.ToolID == "" || p == nil || p.toolGateway == nil {
		return normalized.ToolID != "", nil
	}

	patched, err := p.toolGateway.Intercept(ctx, normalized)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			p.recordProviderNativeToolPrecheck(normalized.ToolID, normalized.Input)
		}
		return true, err
	}
	if providerNativeToolPatchUnsupported(normalized, patched) {
		p.recordProviderNativeToolPrecheck(normalized.ToolID, normalized.Input)
		return true, fmt.Errorf(
			"%w: provider-native permission callbacks do not support tool input patches",
			ErrPermissionDenied,
		)
	}

	p.recordProviderNativeToolPrecheck(normalized.ToolID, normalized.Input)
	return true, nil
}

func (p *AgentProcess) recordProviderNativeToolPrecheck(toolID string, input json.RawMessage) {
	if p == nil {
		return
	}

	precheck := providerNativeToolPrecheck{
		turnID:        strings.TrimSpace(p.activeTurnID()),
		toolID:        canonicalProviderNativeToolID(toolID),
		inputDigest:   providerNativeToolInputDigest(input),
		recordedAtUTC: timeNowUTC(),
	}
	if precheck.turnID == "" || precheck.toolID == "" {
		return
	}

	p.toolPrecheckMu.Lock()
	defer p.toolPrecheckMu.Unlock()

	p.pruneProviderNativeToolPrechecksLocked(precheck.recordedAtUTC)
	p.toolPrechecks = append(p.toolPrechecks, precheck)
}

func (p *AgentProcess) markToolEventPrechecked(event AgentEvent) AgentEvent {
	if p == nil || event.Type != EventTypeToolCall {
		return event
	}

	toolID, input, ok := decodeProviderNativeToolUpdate(event.Raw, event.Title)
	if !ok {
		return event
	}

	if !p.consumeProviderNativeToolPrecheck(strings.TrimSpace(event.TurnID), toolID, input) {
		return event
	}

	event.ToolPrechecked = true
	return event
}

func (p *AgentProcess) consumeProviderNativeToolPrecheck(turnID string, toolID string, input json.RawMessage) bool {
	if p == nil {
		return false
	}

	normalizedTurnID := strings.TrimSpace(turnID)
	normalizedToolID := canonicalProviderNativeToolID(toolID)
	if normalizedTurnID == "" || normalizedToolID == "" {
		return false
	}

	inputDigest := providerNativeToolInputDigest(input)
	now := timeNowUTC()

	p.toolPrecheckMu.Lock()
	defer p.toolPrecheckMu.Unlock()

	p.pruneProviderNativeToolPrechecksLocked(now)
	for i, candidate := range p.toolPrechecks {
		if candidate.turnID != normalizedTurnID || candidate.toolID != normalizedToolID {
			continue
		}
		if inputDigest != "" && candidate.inputDigest != "" && candidate.inputDigest != inputDigest {
			continue
		}
		p.toolPrechecks = append(p.toolPrechecks[:i], p.toolPrechecks[i+1:]...)
		return true
	}
	return false
}

func (p *AgentProcess) pruneProviderNativeToolPrechecksLocked(now time.Time) {
	if p == nil || len(p.toolPrechecks) == 0 {
		return
	}

	cutoff := now.Add(-time.Minute)
	kept := p.toolPrechecks[:0]
	for _, candidate := range p.toolPrechecks {
		if !candidate.recordedAtUTC.Before(cutoff) {
			kept = append(kept, candidate)
		}
	}
	p.toolPrechecks = kept
}
