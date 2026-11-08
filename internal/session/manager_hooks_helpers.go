package session

import (
	"context"
	"encoding/json"

	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func (m *Manager) warnHookDispatch(ctx context.Context, session *Session, event hookspkg.HookEvent, err error) {
	if err == nil {
		return
	}
	ctx = m.hookLifecycleContext(ctx)

	m.sessionLogger(session).WarnContext(
		ctx,
		"session: hook dispatch failed",
		"hook_event", event.String(),
		"error", err,
	)
}

func hookMessageDetails(eventType string) (string, string, bool) {
	switch strings.TrimSpace(eventType) {
	case acp.EventTypeAgentMessage:
		return hookMessageRoleAssistant, hookMessageDeltaTypeText, true
	case acp.EventTypeThought:
		return hookMessageRoleAssistant, hookMessageDeltaTypeThought, true
	default:
		return "", "", false
	}
}

func nextPromptMessageID(turnID string, sequence int) string {
	base := strings.TrimSpace(turnID)
	if base == "" {
		base = "msg"
	}
	if sequence <= 0 {
		return base + "-message"
	}
	return fmt.Sprintf("%s-message-%d", base, sequence)
}

func cloneSessionRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

type lifecycleSessionContextKey struct{}

// hookDispatchContext carries the captured session and owned cleanup context into lifecycle hooks.
func hookDispatchContext(ctx context.Context, manager *Manager, session *Session) context.Context {
	if ctx == nil || session == nil {
		return ctx
	}

	ctx = context.WithValue(ctx, lifecycleSessionContextKey{}, session)
	if manager != nil {
		ctx = hookspkg.WithDispatchEventEmitter(ctx, hookDispatchEventEmitter{
			manager: manager,
			session: session,
		})
	}

	writer, ok := session.recorderHandle().(hookspkg.HookRunWriter)
	if !ok || writer == nil {
		return ctx
	}

	return hookspkg.WithHookRunWriter(ctx, writer)
}

// ToolHookContext attaches the active session's canonical hook audit writers to registry calls.
func (m *Manager) ToolHookContext(ctx context.Context, sessionID string) context.Context {
	if active, ok := m.Get(sessionID); ok {
		return hookDispatchContext(ctx, m, active)
	}
	return ctx
}

// ObserveSessionLifecycle orders native callbacks against the actual session's terminal transition.
func ObserveSessionLifecycle(ctx context.Context, created bool, observe func()) {
	session, ok := ctx.Value(lifecycleSessionContextKey{}).(*Session)
	if !ok || session == nil {
		observe()
		return
	}
	session.lifecycleNotifyMu.Lock()
	defer session.lifecycleNotifyMu.Unlock()
	if !created || session.Info().State == StateActive {
		observe()
	}
}
