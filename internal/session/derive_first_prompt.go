package session

import (
	"context"
	"fmt"
	"strings"
)

// admitDeriveFirstMessage admits the derive's first message through the ordinary prompt
// admission store under "<derive key>:first" with a deterministic message id, so a
// retried derive completes an undispatched admission exactly once. The prompt runs
// detached; its delivery and completion are reported by the child's prompt events.
func (m *Manager) admitDeriveFirstMessage(ctx context.Context, spec deriveSpec, childID string) error {
	deliveryCtx, cancelDelivery := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelDelivery()
	_, err := m.SendPrompt(context.WithoutCancel(ctx), childID, SendPromptOpts{
		Message:         spec.message,
		MessageID:       deriveFirstMessageID(spec.workspaceID, spec.key),
		IdempotencyKey:  deriveFirstAdmissionKey(spec.key),
		DeliveryContext: deliveryCtx,
	})
	if err != nil {
		return fmt.Errorf("session: admit first message of derived session %q: %w", childID, err)
	}
	return nil
}

// recordImportedContextConsumption records the first dispatch that carried a derived
// child's imported context (its admission key and message id) in the child's meta.
func (m *Manager) recordImportedContextConsumption(
	session *Session,
	req promptRequest,
	replayBlock string,
) {
	if strings.TrimSpace(replayBlock) == "" || !session.hasImportedContext() {
		return
	}
	if !session.markImportedContextConsumed(
		strings.TrimSpace(req.idempotencyKey), strings.TrimSpace(req.messageID), m.now(),
	) {
		return
	}
	if err := m.persistSessionMetadataOnly(session); err != nil {
		m.sessionLogger(session).Warn(
			"session.imported_context.consumption_persist_failed",
			"error", err,
		)
	}
}
