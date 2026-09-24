package daemon

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/session"
)

const (
	// clarifyKeepaliveMethod is the ACP extension notification holding one
	// blocked clarify tool call open. Notifications are ignorable by spec,
	// so agents without a handler behave exactly as today. See ADR-001.
	clarifyKeepaliveMethod = "_compozy/clarify_ping"
	// clarifyKeepaliveInterval is the fixed ping cadence: a 2x margin under
	// the observed ~60s agent-side idle limit. Not a config key by design.
	clarifyKeepaliveInterval = 30 * time.Second
	// clarifyKeepaliveSendTimeout bounds one ping so a slow agent connection
	// never stalls the wait loop. Delivery stays fail-open past the timeout.
	clarifyKeepaliveSendTimeout = 5 * time.Second
)

// ClarifyPingParams is one keepalive tick's identity-only payload. It carries
// no tokens, answers, or question content beyond what pending events expose.
type ClarifyPingParams struct {
	SessionID string
	RequestID string
	Seq       uint64
	AskedAt   time.Time
	Deadline  time.Time
}

// ClarifyKeepalive delivers one keepalive ping for a pending clarification; failures are advisory.
// The payload shape keeps ping numbering in the broker and the resolver stateless.
type ClarifyKeepalive interface {
	Ping(ctx context.Context, ping ClarifyPingParams) error
}

// clarifyPingWireParams renders the ACP notification params with a null deadline when unbounded.
func clarifyPingWireParams(ping ClarifyPingParams) map[string]any {
	var deadline any
	if !ping.Deadline.IsZero() {
		deadline = ping.Deadline.UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"session_id": ping.SessionID,
		"request_id": ping.RequestID,
		"seq":        ping.Seq,
		"asked_at":   ping.AskedAt.UTC().Format(time.RFC3339),
		"deadline":   deadline,
	}
}

// clarifyAgentExtensionNotifier resolves a session to its live agent process for extension delivery.
type clarifyAgentExtensionNotifier interface {
	NotifyAgentExtension(ctx context.Context, sessionID, method string, params any) error
}

// The ping path exists only while *session.Manager keeps this method; pin the
// seam at compile time so a rename surfaces as a build break, not silent loss
// of keepalives.
var _ clarifyAgentExtensionNotifier = (*session.Manager)(nil)

// clarifyKeepaliveForSessions builds the broker keepalive at the composition root (nil without live-process delivery).
func clarifyKeepaliveForSessions(sessions SessionManager) ClarifyKeepalive {
	notifier, ok := sessions.(clarifyAgentExtensionNotifier)
	if !ok || notifier == nil {
		return nil
	}
	return newClarifyKeepaliveResolver(notifier)
}

// clarifyKeepaliveResolver adapts session live-process delivery to the broker seam.
type clarifyKeepaliveResolver struct {
	notify func(ctx context.Context, sessionID, method string, params any) error
}

func newClarifyKeepaliveResolver(
	notifier clarifyAgentExtensionNotifier,
) ClarifyKeepalive {
	if notifier == nil {
		return nil
	}
	return &clarifyKeepaliveResolver{notify: notifier.NotifyAgentExtension}
}

func (k *clarifyKeepaliveResolver) Ping(ctx context.Context, ping ClarifyPingParams) error {
	if k == nil || k.notify == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: clarification keepalive context is required")
	}
	if strings.TrimSpace(ping.SessionID) == "" || strings.TrimSpace(ping.RequestID) == "" {
		return errors.New("daemon: clarification keepalive session and request IDs are required")
	}
	if ping.Seq == 0 {
		return errors.New("daemon: clarification keepalive sequence starts at 1")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return k.notify(ctx, ping.SessionID, clarifyKeepaliveMethod, clarifyPingWireParams(ping))
}

// realClarifyPingTicker adapts time.Ticker to the bridge test seam.
func realClarifyPingTicker(interval time.Duration) (<-chan time.Time, func()) {
	ticker := time.NewTicker(interval)
	return ticker.C, ticker.Stop
}
