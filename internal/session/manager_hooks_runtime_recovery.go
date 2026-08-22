package session

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

const runtimeRecoveryDiagnosticMaxBytes = 2048

func (m *Manager) dispatchRuntimeRecoveryObservation(
	ctx context.Context,
	session *Session,
	turnID string,
	event hookspkg.HookEvent,
	attempt int,
	maxAttempts int,
	generation int64,
	failureKind string,
	failureDetail string,
) {
	if m == nil || session == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, session)
	payload := hookspkg.SessionRuntimeRecoveryPayload{
		PayloadBase:    hookspkg.PayloadBase{Event: event, Timestamp: m.now()},
		SessionContext: hookSessionContext(session),
		TurnContext:    hookspkg.TurnContext{TurnID: strings.TrimSpace(turnID)},
		Attempt:        attempt,
		MaxAttempts:    maxAttempts,
		Generation:     generation,
		FailureKind:    strings.TrimSpace(failureKind),
		FailureDetail: diagnostics.RedactAndBound(
			failureDetail,
			runtimeRecoveryDiagnosticMaxBytes,
		),
	}

	hooks := m.hooks.runtimeRecovery()
	var err error
	switch event {
	case hookspkg.HookSessionRuntimeRecoveryStarted:
		_, err = hooks.DispatchSessionRuntimeRecoveryStarted(ctx, payload)
	case hookspkg.HookSessionRuntimeRecoverySucceeded:
		_, err = hooks.DispatchSessionRuntimeRecoverySucceeded(ctx, payload)
	case hookspkg.HookSessionRuntimeRecoveryExhausted:
		_, err = hooks.DispatchSessionRuntimeRecoveryExhausted(ctx, payload)
	default:
		return
	}
	if err != nil {
		m.warnHookDispatch(ctx, session, event, err)
	}
}
