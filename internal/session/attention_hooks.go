package session

import (
	"context"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func (m *Manager) dispatchSessionAttentionChanged(
	ctx context.Context,
	info *Info,
	event AttentionEvent,
) {
	if m == nil || info == nil {
		return
	}
	hookCtx := ctx
	if hookCtx == nil {
		hookCtx = m.fallbackLifecycleContext()
	}
	hookCtx = context.WithoutCancel(hookCtx)
	if active, ok := m.Get(info.ID); ok {
		hookCtx = hookDispatchContext(hookCtx, m, active)
	}
	_, err := m.hooks.attention().DispatchSessionAttentionChanged(hookCtx, hookspkg.SessionAttentionChangedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionAttentionChanged,
			Timestamp: event.At,
		},
		SessionContext: hookSessionContextFromInfo(info),
		From:           string(event.From),
		To:             string(event.To),
		Class:          string(event.Class),
		At:             event.At,
	})
	if err != nil {
		m.logger.WarnContext(
			hookCtx,
			"session: attention hook dispatch failed",
			"session_id", info.ID,
			"error", err,
		)
	}
}
