package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

var ErrRestartContinuationSkipped = errors.New("session: restart continuation requires operator input")

// ResumeInterruptedSessions admits ordinary sessions through a bounded pool; delegated and task
// sessions keep their owning recovery path.
func (m *Manager) ResumeInterruptedSessions(ctx context.Context) error {
	infos, err := m.ListAll(ctx)
	if err != nil {
		return err
	}
	waiting := m.restartWaitingSessions(ctx, infos)
	if err := runRestartResumes(ctx, planRestartResumes(infos), restartResumeConcurrency,
		func(ctx context.Context, id string) {
			resumed, err := m.ResumeInterruptedSession(ctx, id)
			if err == nil && !resumed && waiting[id] {
				err = m.resumeRestartWaitingSession(ctx, id)
				resumed = err == nil
			}
			if err != nil && ctx.Err() == nil {
				m.logger.WarnContext(ctx, "session: restart continuation unavailable", "session_id", id, "error", err)
			} else if !resumed && ctx.Err() == nil {
				m.logger.InfoContext(ctx, "session: restart continuation skipped", "session_id", id,
					"reason", "no_pending_interruption")
			}
		}); err != nil {
		return err
	}
	if err := m.recoverSpawnShutdownWakes(ctx, infos); err != nil {
		return err
	}
	m.recoverChildSettleWakes(ctx, infos)
	m.retryChildSettleWakes(ctx)
	return ctx.Err()
}

// Idle instance mains and required worker parents reopen their binding without replaying a
// completed turn. Preserve a refused binding in the existing restart history as well as the log.
func (m *Manager) resumeRestartWaitingSession(ctx context.Context, id string) error {
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return err
	}
	if err := m.requireSettledRestartTurn(ctx, &meta); err != nil {
		return err
	}
	var checkpoint store.SessionRestartCheckpoint
	if meta.RestartBoundary != nil {
		checkpoint = *meta.RestartBoundary
	}
	err = m.reopenRestartWaitingSession(ctx, &meta)
	if err == nil {
		m.logger.InfoContext(ctx, "session: waiting session reopened after restart", "session_id", id)
		return nil
	}
	checkpoint.Reason = "resume_failed"
	if refusal, ok := errors.AsType[*MonitorOwnerIneligibleError](err); ok {
		checkpoint.Reason = refusal.Reason
	} else if errors.Is(err, ErrMonitorOwnerStopped) {
		checkpoint.Reason = "stop_not_shutdown"
	}
	if checkpoint.TurnID == "" {
		stops, readErr := m.Events(ctx, id, store.EventQuery{Type: EventTypeSessionStopped, Limit: 1})
		if readErr != nil {
			return errors.Join(err, readErr)
		}
		if len(stops) > 0 {
			checkpoint.TurnID = stops[0].TurnID
		} else {
			checkpoint.TurnID = "turn-" + restartEventID(id, meta.UpdatedAt.UTC().Format(time.RFC3339Nano))
		}
	}
	return errors.Join(err, m.appendRestartEvent(ctx, &meta, restartSkippedEvent, checkpoint,
		"Session remains stopped after daemon restart: "+strings.ReplaceAll(checkpoint.Reason, "_", " ")+"."))
}

// ResumeIdleSessionAfterRestart reopens a qualified settled binding for waiting work.
func (m *Manager) ResumeIdleSessionAfterRestart(ctx context.Context, id string) error {
	if err := m.checkPromptAdmissionSuperseded(ctx, id); err != nil {
		return err
	}
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return err
	}
	if meta.RestartBoundary == nil {
		return &MonitorOwnerIneligibleError{Reason: restartBindingUnknownReason}
	}
	if err := m.requireSettledRestartTurn(ctx, &meta); err != nil {
		return err
	}
	instance, err := m.restartWaitingInstance(ctx, &meta)
	if err != nil {
		return err
	}
	purpose := resumeIdleOwner
	if instance != nil && instance.Main {
		purpose = resumeIdleInstanceMain
	}
	return m.resumeMonitorOwnerBinding(ctx, meta.ID, *meta.RestartBoundary, purpose)
}

func (m *Manager) requireSettledRestartTurn(ctx context.Context, meta *store.SessionMeta) error {
	checkpoint := meta.RestartBoundary
	if checkpoint == nil {
		return nil
	}
	interrupted, err := m.restartEventExists(ctx, meta.ID, checkpoint.TurnID, restartInterruptedEvent)
	if err != nil {
		return err
	}
	if interrupted {
		// A previously skipped interruption must retain its operator review requirement.
		return fmt.Errorf("%w: interrupted turn already settled", ErrRestartContinuationSkipped)
	}
	return nil
}

func (m *Manager) restartWaitingInstance(ctx context.Context, meta *store.SessionMeta) (*InstanceCeiling, error) {
	if normalizeSessionType(Type(meta.SessionType)) != SessionTypeUser || m.instanceCeiling == nil {
		return nil, nil
	}
	instance, err := m.instanceCeiling(ctx, meta.ID, "")
	if err != nil {
		return nil, err
	}
	if instance != nil && instance.Main && !instance.ActiveMain {
		return nil, &MonitorOwnerIneligibleError{Reason: restartInstanceInactiveReason}
	}
	return instance, nil
}

func (m *Manager) reopenRestartWaitingSession(ctx context.Context, meta *store.SessionMeta) error {
	instance, err := m.restartWaitingInstance(ctx, meta)
	if err != nil {
		return err
	}
	if meta.RestartBoundary != nil {
		return m.ResumeWaitingSession(ctx, meta.ID)
	}
	if instance == nil || !instance.ActiveMain {
		return &MonitorOwnerIneligibleError{Reason: restartBindingUnknownReason}
	}
	return m.resumeUnboundMainAfterRestart(ctx, meta)
}

// An unbound main has no accepted provider route. Reopen its logical session and let the first prompt bind it.
func (m *Manager) resumeUnboundMainAfterRestart(ctx context.Context, meta *store.SessionMeta) (err error) {
	if err := requireUnboundShutdownResume(meta); err != nil {
		return err
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, meta.ID); err != nil {
		return &MonitorOwnerIneligibleError{Reason: "archived"}
	}
	workspace, err := m.resolveResumeWorkspace(ctx, meta)
	if err != nil {
		return &MonitorOwnerIneligibleError{Reason: "workspace_unavailable"}
	}
	if !workspace.Config.Session.Restart.AutoContinue {
		return &MonitorOwnerIneligibleError{Reason: restartDisabledReason}
	}
	rows, err := m.Events(ctx, meta.ID, store.EventQuery{Types: []string{
		acp.EventTypeUserMessage, acp.EventTypeSyntheticReentry,
	}, Limit: 1})
	if err != nil {
		return err
	}
	if len(rows) != 0 {
		return &MonitorOwnerIneligibleError{Reason: restartBindingUnknownReason}
	}
	if _, held := m.restartInputsHeld.LoadOrStore(meta.ID, true); held {
		return fmt.Errorf("%w: restart recovery already running", ErrPromptInProgress)
	}
	defer func() {
		m.restartInputsHeld.Delete(meta.ID)
		if err == nil {
			m.afterSessionResumed(ctx, meta.ID)
		}
	}()
	if _, err := m.resume(ctx, meta.ID, resumeUnboundAfterShutdown); err != nil {
		return err
	}
	return m.checkReopenedInstanceMain(ctx, meta.ID)
}

func requireUnboundShutdownResume(meta *store.SessionMeta) error {
	if State(meta.State) != StateStopped || sessionMetaStopReason(meta) != store.StopShutdown {
		return ErrMonitorOwnerStopped
	}
	if !isUnboundLogicalResume(meta) || meta.AcceptedRoute != nil || meta.RestartBoundary != nil {
		return &MonitorOwnerIneligibleError{Reason: restartBindingUnknownReason}
	}
	if restartFailureRequiresOperator(meta.Failure) {
		return &MonitorOwnerIneligibleError{Reason: "agent_failure"}
	}
	return nil
}

// Check again before dispatch so an instance transition during activation wins over recovery.
func (m *Manager) checkReopenedInstanceMain(ctx context.Context, id string) error {
	instance, err := m.instanceCeiling(ctx, id, "")
	if err == nil && instance != nil && instance.ActiveMain {
		return nil
	}
	if err == nil {
		err = &MonitorOwnerIneligibleError{Reason: restartInstanceInactiveReason}
	}
	cleanupCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	return errors.Join(
		err,
		m.StopWithCause(cleanupCtx, id, CauseShutdown, "Instance main is unavailable after restart."),
	)
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
	if normalizeSessionType(Type(meta.SessionType)) == SessionTypeSpawned {
		m.spawnMu.Lock()
		defer m.spawnMu.Unlock()
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
	active, err := m.resume(ctx, id, resumeWithoutDispatch)
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
	m.dispatchResumedInputs(ctx, id)
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
	canceled, err := m.restartTurnCanceled(ctx, meta.ID, checkpoint.TurnID)
	if err != nil || canceled {
		return store.SessionRestartCheckpoint{}, false, err
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
	opts := SyntheticPromptOpts{
		Message: restartContinueMessage, TurnID: "turn-" + restartEventID(checkpoint.TurnID, "continue"),
		Metadata: acp.PromptSyntheticMeta{Kind: restartContinuationKind, Reason: "daemon_restarted",
			WakeEventID: restartEventID(checkpoint.TurnID, restartInterruptedEvent)},
	}
	req, err := m.parseSyntheticPromptRequest(ctx, active.ID, opts)
	if err != nil {
		return SendPromptResult{}, err
	}
	instanceLineage, err := m.queueInstancePrompt(ctx, req)
	if err != nil {
		return SendPromptResult{}, err
	}
	if instanceLineage {
		// An instance lineage never dispatches non-human work directly: the durable selector applies its
		// paused hold and unattended accounting. The stable message id keeps a retried recovery on one row.
		opts.MessageID, opts.Priority = identity, 1
		entry, err := m.EnqueueSyntheticWake(ctx, active.ID, opts)
		if err != nil {
			return SendPromptResult{}, err
		}
		return SendPromptResult{
			Status: store.SessionPromptResultStatusQueued, Mode: BusyInputModeQueue,
			Delivery: store.SessionInputDeliveryAfterTurn, MessageID: entry.MessageID, QueueEntryID: entry.ID,
		}, nil
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
