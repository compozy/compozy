package session

import (
	"context"

	"strings"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Manager) dispatchSessionHealthUpdateAfter(
	ctx context.Context,
	previous heartbeat.SessionHealth,
	current heartbeat.SessionHealth,
) error {
	if m == nil || !sessionHealthHookTransition(previous, current) {
		return nil
	}
	if !m.shouldDispatchSessionHealthHook(current) {
		return nil
	}
	payload := hookspkg.SessionHealthUpdateAfterPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionHealthUpdateAfter,
			Timestamp: current.UpdatedAt.UTC(),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID:   current.SessionID,
			AgentName:   current.AgentName,
			WorkspaceID: current.WorkspaceID,
			State:       string(current.State),
			UpdatedAt:   current.UpdatedAt.UTC(),
		},
		Health:              string(current.Health),
		ActivePrompt:        current.ActivePrompt,
		Attachable:          current.Attachable,
		EligibleForWake:     current.EligibleForWake,
		IneligibilityReason: current.IneligibilityReason,
		LastActivityAt:      current.LastActivityAt.UTC(),
		LastPresenceAt:      current.LastPresenceAt.UTC(),
		LastError:           current.LastError,
	}
	_, err := m.hooks.authoredContext().DispatchSessionHealthUpdateAfter(ctx, payload)
	return err
}

func sessionHealthHookTransition(previous heartbeat.SessionHealth, current heartbeat.SessionHealth) bool {
	previous = previous.Normalize()
	current = current.Normalize()
	if strings.TrimSpace(current.SessionID) == "" {
		return false
	}
	if strings.TrimSpace(previous.SessionID) == "" {
		return true
	}
	return previous.State != current.State ||
		previous.Health != current.Health ||
		previous.EligibleForWake != current.EligibleForWake ||
		previous.IneligibilityReason != current.IneligibilityReason
}

func (m *Manager) shouldDispatchSessionHealthHook(current heartbeat.SessionHealth) bool {
	if m == nil {
		return false
	}
	sessionID := strings.TrimSpace(current.SessionID)
	if sessionID == "" {
		return false
	}
	now := current.UpdatedAt.UTC()
	if now.IsZero() {
		now = m.now()
	}
	m.sessionHealthHookMu.Lock()
	defer m.sessionHealthHookMu.Unlock()
	if m.sessionHealthHookLast == nil {
		m.sessionHealthHookLast = make(map[string]time.Time)
	}
	last := m.sessionHealthHookLast[sessionID]
	if !last.IsZero() && now.Sub(last) < m.sessionHealthHookMinInterval {
		return false
	}
	m.sessionHealthHookLast[sessionID] = now
	return true
}

func (m *Manager) detachedSessionHealthContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := ctx
	if base == nil && m != nil {
		base = m.lifecycleCtx
	}
	if base == nil {
		base = context.TODO()
	}
	return context.WithTimeout(context.WithoutCancel(base), defaultLifecycleTimeout)
}
