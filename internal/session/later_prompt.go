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
	target *promptAdmissionTarget,
	req *promptRequest,
) (bool, error) {
	if target.session != nil || req.meta.Origin == nil {
		return false, nil
	}
	return m.sessionInLater(ctx, target.workspaceID, req.target)
}

// enqueueAdmittedOfflinePrompt persists input without starting a stopped provider.
// The instance reconciler or the operator resumes its queue when the hold ends.
func (m *Manager) enqueueAdmittedOfflinePrompt(
	ctx context.Context,
	sessionID string,
	admissionReq store.SessionPromptAdmissionRequest,
	later, supersede bool,
) (SendPromptResult, error) {
	if m.inputQueue == nil {
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrSessionInLater, sessionID)
	}
	generation, err := m.currentInputGeneration(ctx, sessionID)
	if err != nil {
		return SendPromptResult{}, err
	}
	admission, _, _, created, err := m.inputQueue.EnqueueAdmitted(ctx, admissionReq, generation, "", supersede)
	if err != nil {
		return SendPromptResult{}, err
	}
	result, err := sendPromptResultFromAdmission(admission)
	if err != nil {
		return SendPromptResult{}, err
	}
	result.TargetInLater = later
	result.Replayed = !created
	// The operator may have resumed the target since the Later check; the
	// selector delivers the message then, and does nothing while it is stopped.
	m.startNextQueuedInputPrompt(sessionID)
	if created && later {
		if _, err := m.publishLaterCatalogUpsert(ctx, sessionID); err != nil {
			m.logger.Warn("session: publish later pending message", "session_id", sessionID, "error", err)
		}
	}
	m.replyWatchSendResult(ctx, admission, nil)
	return result, nil
}
