package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

var ErrRestartContinuationSkipped = errors.New("session: restart continuation requires operator input")

// ResumeInterruptedSessions admits ordinary sessions; delegated and task sessions keep their owning recovery path.
func (m *Manager) ResumeInterruptedSessions(ctx context.Context) error {
	infos, err := m.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.Type != SessionTypeUser || info.StopReason != store.StopShutdown {
			continue
		}
		if _, err := m.ResumeInterruptedSession(ctx, info.ID); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			m.logger.WarnContext(ctx, "session: restart continuation unavailable", "session_id", info.ID, "error", err)
		}
	}
	return nil
}

// ResumeInterruptedSession consumes a durable interruption through the existing prompt admission owner.
func (m *Manager) ResumeInterruptedSession(ctx context.Context, id string) (bool, error) {
	if _, held := m.restartInputsHeld.LoadOrStore(id, true); held {
		return false, fmt.Errorf("%w: restart recovery already running", ErrPromptInProgress)
	}
	defer m.restartInputsHeld.Delete(id)
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return false, err
	}
	checkpoint, found, err := m.pendingRestartCheckpoint(ctx, &meta)
	if err != nil || !found {
		return false, err
	}
	reason, err := m.restartSkipReason(ctx, &meta, checkpoint)
	if err != nil {
		return false, err
	}
	if reason != "" {
		return false, m.skipRestartContinuation(ctx, &meta, checkpoint, reason)
	}
	if err := m.appendRestartEvent(ctx, &meta, restartAttemptEvent, checkpoint, ""); err != nil {
		return false, err
	}
	active, err := m.resume(ctx, id, false)
	if err != nil {
		return false, err
	}
	current := active.Meta()
	digest, digestErr := restartPolicyDigest(active.AgentDefinition())
	if digestErr != nil || !sameRestartBinding(checkpoint, &current) ||
		(checkpoint.PolicyDigest != "" && digest != checkpoint.PolicyDigest) {
		return false, errors.Join(m.StopWithCause(ctx, id, CauseShutdown, "Restart continuation needs review."),
			digestErr, m.skipRestartContinuation(ctx, &current, checkpoint, "binding_changed_during_resume"))
	}
	result, err := m.admitRestartContinuation(ctx, active, checkpoint)
	if err != nil {
		if errors.Is(err, ErrPromptInProgress) {
			return false, m.skipRestartContinuation(ctx, &current, checkpoint, "new_turn_started")
		}
		stopErr := m.StopWithCause(ctx, id, CauseShutdown, "Restart continuation needs review.")
		return false, errors.Join(err, stopErr,
			m.skipRestartContinuation(ctx, &current, checkpoint, "continuation_dispatch_uncertain"))
	}
	if result.Events != nil {
		m.startTrackedPromptTask(func() {
			drainPromptSource(result.Events)
		})
	}
	if err := m.appendRestartEvent(
		ctx,
		&current,
		restartResumedEvent,
		checkpoint,
		"Resumed after CompozyOS restart.",
	); err != nil {
		return true, err
	}
	m.restartInputsHeld.Delete(id)
	m.startNextQueuedInputPrompt(id)
	return true, nil
}

func (m *Manager) pendingRestartCheckpoint(
	ctx context.Context, meta *store.SessionMeta,
) (store.SessionRestartCheckpoint, bool, error) {
	if sessionMetaStopReason(meta) != store.StopShutdown {
		return store.SessionRestartCheckpoint{}, false, nil
	}
	rows, err := m.Events(ctx, meta.ID, store.EventQuery{Type: restartInterruptedEvent, Limit: 1,
		Archive: store.EventArchiveUnarchived})
	if err != nil || len(rows) == 0 {
		return store.SessionRestartCheckpoint{}, false, err
	}
	checkpoint, err := decodeRestartCheckpoint(rows[0])
	if err != nil {
		return store.SessionRestartCheckpoint{}, false, err
	}
	boundary, found, err := m.latestRestartBoundary(ctx, meta)
	if err != nil || !found || boundary.TurnID != checkpoint.TurnID {
		return store.SessionRestartCheckpoint{}, false, err
	}
	for _, eventType := range []string{restartResumedEvent, restartSkippedEvent} {
		settled, err := m.restartEventExists(ctx, meta.ID, checkpoint.TurnID, eventType)
		if err != nil || settled {
			return store.SessionRestartCheckpoint{}, false, err
		}
	}
	terminal, err := m.Events(ctx, meta.ID, store.EventQuery{
		TurnID: checkpoint.TurnID, Type: acp.EventTypeDone, Limit: 1,
	})
	if err != nil {
		return checkpoint, false, err
	}
	if len(terminal) > 0 {
		event, err := transcript.UnmarshalAgentEvent(terminal[0].Content)
		if err != nil {
			return checkpoint, false, err
		}
		if event.PromptStopReason.Valid() && event.PromptStopReason != acp.PromptStopReasonCancelled {
			checkpoint.Reason = "turn_completed"
		}
	}
	return checkpoint, true, nil
}

func (m *Manager) skipRestartContinuation(
	ctx context.Context, meta *store.SessionMeta, checkpoint store.SessionRestartCheckpoint, reason string,
) error {
	checkpoint.Reason = reason
	if err := m.appendRestartEvent(ctx, meta, restartSkippedEvent, checkpoint,
		"Interrupted by CompozyOS restart. Automatic continuation paused: "+strings.ReplaceAll(reason, "_", " ")+
			". Review the session before resuming."); err != nil {
		return err
	}
	return fmt.Errorf("%w: %s", ErrRestartContinuationSkipped, reason)
}

func (m *Manager) admitRestartContinuation(
	ctx context.Context, active *Session, checkpoint store.SessionRestartCheckpoint,
) (SendPromptResult, error) {
	identity := "restart:" + checkpoint.TurnID
	req, err := m.parseSyntheticPromptRequest(ctx, active.ID, SyntheticPromptOpts{
		Message: restartContinueMessage, TurnID: "turn-" + restartEventID(checkpoint.TurnID, "continue"),
		Metadata: acp.PromptSyntheticMeta{Kind: restartContinuationKind, Reason: "daemon_restarted",
			WakeEventID: restartEventID(checkpoint.TurnID, restartInterruptedEvent)},
	})
	if err != nil {
		return SendPromptResult{}, err
	}
	req.messageID, req.idempotencyKey = identity, identity
	meta := active.Meta()
	req.runtime, err = normalizePromptRuntimeSelectionFromMeta(&meta, nil)
	if err != nil {
		return SendPromptResult{}, err
	}
	admission, err := m.newPromptAdmissionRequest(meta.WorkspaceID, req,
		store.SessionPromptOperationPrompt, BusyInputModeQueue)
	if err != nil {
		return SendPromptResult{}, err
	}
	unlock := m.lockPromptAdmission(meta.WorkspaceID, active.ID, identity)
	defer unlock()
	return m.submitAdmittedDirectPrompt(ctx, active, req, BusyInputModeQueue, admission)
}
