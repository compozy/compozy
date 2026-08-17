package session

import (
	"context"
	"strings"
)

const actingSessionActorKind = "agent_session"

type actingSessionContextKey struct{}

// WithActingSession records the trusted session that initiated a cross-session action.
func WithActingSession(ctx context.Context, sessionID string) context.Context {
	actorID := strings.TrimSpace(sessionID)
	if ctx == nil || actorID == "" {
		return ctx
	}
	return context.WithValue(ctx, actingSessionContextKey{}, actorID)
}

func actingSessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	actorID, ok := ctx.Value(actingSessionContextKey{}).(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(actorID)
}

func resolvedByFromContext(ctx context.Context, fallback string) string {
	if actorID := actingSessionID(ctx); actorID != "" {
		return actingSessionActorKind + ":" + actorID
	}
	return strings.TrimSpace(fallback)
}
