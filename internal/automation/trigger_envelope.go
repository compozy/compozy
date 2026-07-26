package automation

import (
	"context"

	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"strconv"
	"strings"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

func (e *TriggerEngine) hookCompletionEnvelope(
	ctx context.Context,
	sessionID string,
	record hookspkg.HookRunRecord,
) (ActivationEnvelope, error) {
	if strings.TrimSpace(sessionID) == "" {
		return ActivationEnvelope{}, errors.New("automation: hook completion session id is required")
	}
	if strings.TrimSpace(record.HookName) == "" {
		return ActivationEnvelope{}, errors.New("automation: hook completion hook name is required")
	}
	if e.hookSessions == nil {
		return ActivationEnvelope{}, errors.New("automation: hook session resolver is required")
	}

	info, err := e.hookSessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return ActivationEnvelope{}, fmt.Errorf("automation: resolve hook session %q: %w", sessionID, err)
	}
	if info == nil {
		return ActivationEnvelope{}, fmt.Errorf("automation: resolve hook session %q: empty session", sessionID)
	}

	data := map[string]any{
		triggerSessionIDKey:     strings.TrimSpace(info.ID),
		triggerSessionNameKey:   strings.TrimSpace(info.Name),
		triggerSessionTypeKey:   string(info.Type),
		triggerAgentNameKey:     strings.TrimSpace(info.AgentName),
		triggerHookNameKey:      strings.TrimSpace(record.HookName),
		triggerHookEventKey:     record.Event.String(),
		triggerHookSourceKey:    record.Source.String(),
		triggerHookModeKey:      string(record.Mode),
		triggerHookOutcomeKey:   string(record.Outcome),
		triggerDispatchDepthKey: strconv.Itoa(record.DispatchDepth),
		triggerRequiredKey:      strconv.FormatBool(record.Required),
	}
	if !record.RecordedAt.IsZero() {
		data[triggerRecordedAtKey] = record.RecordedAt.UTC().Format(time.RFC3339Nano)
	}
	if trimmedError := strings.TrimSpace(record.Error); trimmedError != "" {
		data[triggerErrorKey] = trimmedError
	}

	workspaceID := strings.TrimSpace(info.WorkspaceID)
	return ActivationEnvelope{
		Kind:        "hook." + strings.TrimSpace(record.HookName) + ".completed",
		Scope:       scopeFromWorkspaceID(workspaceID),
		WorkspaceID: workspaceID,
		Source:      ActivationSourceHook,
		Data:        data,
	}, nil
}

func sessionEnvelope(kind string, sess *session.Session) (ActivationEnvelope, error) {
	if sess == nil {
		return ActivationEnvelope{}, errors.New("automation: session is required")
	}
	info := sess.Info()
	if info == nil {
		return ActivationEnvelope{}, errors.New("automation: session info is required")
	}

	workspaceID := strings.TrimSpace(info.WorkspaceID)
	data := map[string]any{
		triggerSessionIDKey:    strings.TrimSpace(info.ID),
		triggerSessionNameKey:  strings.TrimSpace(info.Name),
		triggerSessionTypeKey:  string(info.Type),
		triggerAgentNameKey:    strings.TrimSpace(info.AgentName),
		triggerStateKey:        string(info.State),
		triggerWorkspaceKey:    strings.TrimSpace(info.Workspace),
		triggerWorkspaceIDKey:  workspaceID,
		triggerACPSessionIDKey: strings.TrimSpace(info.ACPSessionID),
	}
	if !info.CreatedAt.IsZero() {
		data[triggerCreatedAtKey] = info.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if !info.UpdatedAt.IsZero() {
		data[triggerUpdatedAtKey] = info.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	if kind == sessionEventStopped {
		if stopReason := strings.TrimSpace(string(info.StopReason)); stopReason != "" {
			data[triggerStopReasonKey] = stopReason
		}
		if stopDetail := strings.TrimSpace(info.StopDetail); stopDetail != "" {
			data[triggerStopDetailKey] = stopDetail
		}
	}

	return ActivationEnvelope{
		Kind:        kind,
		Scope:       scopeFromWorkspaceID(workspaceID),
		WorkspaceID: workspaceID,
		Source:      ActivationSourceObserver,
		Data:        data,
	}, nil
}

func memoryConsolidatedEnvelope(event MemoryConsolidatedEvent) (ActivationEnvelope, error) {
	workspaceID := strings.TrimSpace(event.WorkspaceID)
	data := cloneAnyMap(event.Data)
	if data == nil {
		data = make(map[string]any)
	}
	if !event.Timestamp.IsZero() {
		data[triggerCompletedAtKey] = event.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	if workspaceID != "" {
		data[triggerWorkspaceIDKey] = workspaceID
	}

	envelope := ActivationEnvelope{
		Kind:        "memory.consolidated",
		Scope:       scopeFromWorkspaceID(workspaceID),
		WorkspaceID: workspaceID,
		Source:      ActivationSourceObserver,
		Data:        data,
	}
	return envelope, envelope.Validate("envelope")
}

func webhookEnvelope(request WebhookRequest, trigger Trigger) (ActivationEnvelope, error) {
	endpoint, err := FormatWebhookEndpoint(trigger.EndpointSlug, trigger.WebhookID)
	if err != nil {
		return ActivationEnvelope{}, err
	}
	data := cloneAnyMap(request.Data)
	if data == nil {
		data = make(map[string]any)
	}
	if _, exists := data["payload"]; !exists {
		data["payload"] = string(request.Payload)
	}
	data["endpoint"] = endpoint
	data["endpoint_slug"] = strings.TrimSpace(trigger.EndpointSlug)
	data["webhook_id"] = strings.TrimSpace(trigger.WebhookID)
	data["timestamp"] = request.Timestamp.UTC().Format(time.RFC3339Nano)

	return ActivationEnvelope{
		Kind:        triggerEventWebhook,
		Scope:       request.Scope,
		WorkspaceID: strings.TrimSpace(request.WorkspaceID),
		Source:      ActivationSourceWebhook,
		Data:        data,
	}, nil
}

func webhookSignaturePayload(timestamp time.Time, payload []byte) []byte {
	message := strconv.FormatInt(timestamp.UTC().Unix(), 10) + "."
	out := make([]byte, 0, len(message)+len(payload))
	out = append(out, []byte(message)...)
	out = append(out, payload...)
	return out
}

func decodeWebhookSignature(signature string) ([]byte, error) {
	trimmed := strings.TrimSpace(signature)
	if !strings.HasPrefix(trimmed, webhookSignaturePrefix) {
		return nil, ErrWebhookSignatureInvalid
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(trimmed, webhookSignaturePrefix))
	if err != nil || len(decoded) != sha256.Size {
		return nil, ErrWebhookSignatureInvalid
	}
	return decoded, nil
}

func webhookDeliveryClaimActive(run Run, now time.Time, window time.Duration) bool {
	if window <= 0 {
		window = DefaultWebhookFreshnessWindow
	}
	claimedAt := time.Time{}
	if run.StartedAt != nil {
		claimedAt = run.StartedAt.UTC()
	} else if run.ScheduledAt != nil {
		claimedAt = run.ScheduledAt.UTC()
	}
	if claimedAt.IsZero() {
		return true
	}
	return claimedAt.Add(window).After(now.UTC())
}

func webhookDeliveryRunID(triggerID string, deliveryID string) string {
	sum := sha256.Sum256([]byte(webhookDeliveryKey(triggerID, deliveryID)))
	return "run_wbh_" + hex.EncodeToString(sum[:12])
}

func webhookDeliveryFireID(triggerID string, deliveryID string) string {
	sum := sha256.Sum256([]byte(webhookDeliveryKey(triggerID, deliveryID)))
	return "webhook:" + hex.EncodeToString(sum[:])
}

func scopeFromWorkspaceID(workspaceID string) Scope {
	if strings.TrimSpace(workspaceID) == "" {
		return AutomationScopeGlobal
	}
	return AutomationScopeWorkspace
}

func pointerToActivationEnvelope(envelope ActivationEnvelope) *ActivationEnvelope {
	cloned := envelope
	if envelope.Data != nil {
		cloned.Data = cloneAnyMap(envelope.Data)
	}
	return &cloned
}

type triggerSessionObserver struct {
	engine *TriggerEngine
}
