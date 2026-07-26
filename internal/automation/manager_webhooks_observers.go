package automation

import (
	"context"
	"errors"

	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/session"

	taskpkg "github.com/compozy/agh/internal/task"
)

// HandleWebhook validates, normalizes, and dispatches a webhook delivery
// through the running trigger engine.
func (m *Manager) HandleWebhook(ctx context.Context, request WebhookRequest) (TriggerResult, error) {
	engine, runtimeCtx, ok := m.triggerRuntime()
	if !ok {
		return TriggerResult{}, ErrManagerNotRunning
	}
	mergedCtx, cancel := mergedRuntimeContext(ctx, runtimeCtx)
	defer cancel()
	return engine.HandleWebhook(mergedCtx, request)
}

// FireExtensionTrigger routes one extension-originated ext.* event through the shared trigger engine.
func (m *Manager) FireExtensionTrigger(ctx context.Context, request ExtensionTriggerRequest) (TriggerResult, error) {
	if err := request.Validate("extension_trigger"); err != nil {
		return TriggerResult{}, err
	}

	engine, runtimeCtx, ok := m.triggerRuntime()
	if !ok {
		return TriggerResult{}, ErrManagerNotRunning
	}

	mergedCtx, cancel := mergedRuntimeContext(ctx, runtimeCtx)
	defer cancel()

	envelope := ActivationEnvelope{
		Kind:        strings.TrimSpace(request.Event),
		Scope:       request.Scope,
		WorkspaceID: strings.TrimSpace(request.WorkspaceID),
		Source:      ActivationSourceExtension,
		Data:        cloneJSONMap(request.Payload),
	}
	if envelope.Data == nil {
		envelope.Data = map[string]any{}
	}
	return engine.Fire(mergedCtx, envelope)
}

// SessionObserver exposes the existing session notifier seam for automation
// trigger ingress.
func (m *Manager) SessionObserver() session.Notifier {
	return managerSessionObserver{manager: m}
}

// HookTelemetrySink exposes the existing hook telemetry sink seam for
// hook-completion trigger ingress.
func (m *Manager) HookTelemetrySink() hookspkg.TelemetrySink {
	return managerHookTelemetrySink{manager: m}
}

// MemoryObserver exposes the automation memory-consolidation observer seam for
// callers that can publish completion events.
func (m *Manager) MemoryObserver() MemoryConsolidationObserver {
	return managerMemoryObserver{manager: m}
}

// RecordAutomationSessionTaskActor stores the trusted task-domain actor
// context for one automation-launched session.
func (m *Manager) RecordAutomationSessionTaskActor(sessionID string, actor taskpkg.ActorContext) error {
	if m == nil {
		return nil
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return errors.New("automation: session id is required")
	}
	if err := actor.Validate(); err != nil {
		return err
	}

	m.taskActorMu.Lock()
	defer m.taskActorMu.Unlock()
	m.sessionTaskActors[trimmedSessionID] = actor
	return nil
}

// TaskActorContextForSession returns the automation-linked task actor context
// previously recorded for one automation-launched session.
func (m *Manager) TaskActorContextForSession(sessionID string) (taskpkg.ActorContext, error) {
	if m == nil {
		return taskpkg.ActorContext{}, ErrSessionTaskActorNotFound
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return taskpkg.ActorContext{}, errors.New("automation: session id is required")
	}

	m.taskActorMu.RLock()
	actor, ok := m.sessionTaskActors[trimmedSessionID]
	m.taskActorMu.RUnlock()
	if !ok {
		return taskpkg.ActorContext{}, ErrSessionTaskActorNotFound
	}
	return actor, nil
}

// DeleteAutomationSessionTaskActor removes any recorded task actor context for
// the supplied automation-launched session.
func (m *Manager) DeleteAutomationSessionTaskActor(sessionID string) {
	if m == nil {
		return
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return
	}

	m.taskActorMu.Lock()
	defer m.taskActorMu.Unlock()
	delete(m.sessionTaskActors, trimmedSessionID)
}
