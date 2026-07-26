package extensionpkg

import (
	"context"
	"log/slog"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/session"
)

// BridgeDeliveryNotifier projects prompt-time ACP events into the bridge
// delivery broker while preserving an optional downstream notifier chain.
type BridgeDeliveryNotifier struct {
	broker     *bridgepkg.Broker
	downstream session.Notifier
}

var _ session.Notifier = (*BridgeDeliveryNotifier)(nil)
var _ session.AgentEventNotifier = (*BridgeDeliveryNotifier)(nil)

// NewBridgeDeliveryNotifier wraps the provided downstream notifier with
// session-to-bridge delivery projection.
func NewBridgeDeliveryNotifier(broker *bridgepkg.Broker, downstream session.Notifier) *BridgeDeliveryNotifier {
	return &BridgeDeliveryNotifier{
		broker:     broker,
		downstream: downstream,
	}
}

// OnSessionCreated forwards the lifecycle callback unchanged.
func (n *BridgeDeliveryNotifier) OnSessionCreated(ctx context.Context, sess *session.Session) {
	if n == nil || n.downstream == nil {
		return
	}
	n.downstream.OnSessionCreated(ctx, sess)
}

// OnSessionStopped fails unfinished bridge deliveries before forwarding the lifecycle callback.
func (n *BridgeDeliveryNotifier) OnSessionStopped(ctx context.Context, sess *session.Session) {
	if n == nil {
		return
	}
	if n.broker != nil && sess != nil {
		if err := n.broker.FailSession(ctx, sess.ID, ""); err != nil {
			slog.ErrorContext(ctx, "extension: fail session delivery projection",
				"session_id", sess.ID,
				"error", err,
			)
		}
	}
	if n.downstream != nil {
		n.downstream.OnSessionStopped(ctx, sess)
	}
}

// OnAgentEvent projects ACP runtime output into the delivery broker before forwarding.
func (n *BridgeDeliveryNotifier) OnAgentEvent(ctx context.Context, sessionID string, payload any) {
	if n == nil {
		return
	}
	n.projectAgentEvent(ctx, sessionID, payload)
	if n.downstream != nil {
		n.downstream.OnAgentEvent(ctx, sessionID, payload)
	}
}

// OnAgentEventForSession preserves the richer session-aware notifier path when
// the downstream chain supports it.
func (n *BridgeDeliveryNotifier) OnAgentEventForSession(
	ctx context.Context,
	sess *session.Session,
	payload any,
) {
	if n == nil {
		return
	}

	sessionID := ""
	if sess != nil {
		sessionID = sess.ID
	}
	n.projectAgentEvent(ctx, sessionID, payload)

	if aware, ok := n.downstream.(session.AgentEventNotifier); ok {
		aware.OnAgentEventForSession(ctx, sess, payload)
		return
	}
	if n.downstream != nil {
		n.downstream.OnAgentEvent(ctx, sessionID, payload)
	}
}

func (n *BridgeDeliveryNotifier) projectAgentEvent(
	ctx context.Context,
	sessionID string,
	payload any,
) {
	if n == nil || n.broker == nil {
		return
	}
	event, ok := payload.(acp.AgentEvent)
	if !ok {
		return
	}
	projected, err := projectionEventFromAgentEvent(&event)
	if err != nil {
		slog.ErrorContext(ctx, "extension: canonicalize bridge delivery event",
			"session_id", sessionID,
			"event_type", event.Type,
			"turn_id", event.TurnID,
			"error", err,
		)
		return
	}
	if err := n.broker.ProjectEvent(ctx, sessionID, projected); err != nil {
		slog.ErrorContext(ctx, "extension: project bridge delivery event",
			"session_id", sessionID,
			"event_type", event.Type,
			"turn_id", event.TurnID,
			"error", err,
		)
	}
}
