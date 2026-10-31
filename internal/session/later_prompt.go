package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// agentMessageToLater reports whether an agent message targets a parked
// session, which queues the message instead of resuming the target.
func (m *Manager) agentMessageToLater(
	ctx context.Context,
	target promptAdmissionTarget,
	req promptRequest,
) (bool, error) {
	if target.session != nil || req.meta.Origin == nil {
		return false, nil
	}
	return m.sessionInLater(ctx, target.workspaceID, req.target)
}

// enqueueAdmittedLaterPrompt queues an agent message for a parked session. The
// message waits until the operator resumes the session; the parked row counts
// it as pending so the Later card can show it.
func (m *Manager) enqueueAdmittedLaterPrompt(
	ctx context.Context,
	sessionID string,
	admissionReq store.SessionPromptAdmissionRequest,
) (SendPromptResult, error) {
	if m.inputQueue == nil {
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrSessionInLater, sessionID)
	}
	generation, err := m.currentInputGeneration(ctx, sessionID)
	if err != nil {
		return SendPromptResult{}, err
	}
	admission, _, _, created, err := m.inputQueue.EnqueueAdmitted(ctx, admissionReq, generation, "", false)
	if err != nil {
		return SendPromptResult{}, err
	}
	result, err := sendPromptResultFromAdmission(admission)
	if err != nil {
		return SendPromptResult{}, err
	}
	result.TargetInLater = true
	result.Replayed = !created
	if created {
		if _, err := m.publishLaterCatalogUpsert(ctx, sessionID); err != nil {
			m.logger.Warn("session: publish later pending message", "session_id", sessionID, "error", err)
		}
	}
	m.replyWatchSendResult(ctx, admission, nil)
	return result, nil
}
