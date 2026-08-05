package task

import (
	"context"
	"errors"
	"strings"
)

const (
	wakeSuppressionSessionNotLive = "session_not_live"
	wakeSuppressionOptOut         = "wake_creator_disabled"
	wakeSuppressionSelfWake       = "self_wake"
	wakeSuppressionDeliveryFailed = "delivery_failed"
)

type wakeDispatchRequest struct {
	taskRecord         Task
	runID              string
	executingSessionID string
	wakeEventID        string
	reason             WakeReason
	summary            string
	actor              ActorContext
}

func (m *Service) dispatchTerminalWake(ctx context.Context, taskRecord Task, run Run, actor ActorContext) {
	status := run.Status.Normalize()
	m.dispatchWakeCreator(ctx, &wakeDispatchRequest{
		taskRecord:         taskRecord,
		runID:              run.ID,
		executingSessionID: run.SessionID,
		wakeEventID:        wakeEventID("terminal", taskRecord.ID, run.ID, status.String()),
		reason:             WakeReasonTerminal,
		summary:            terminalWakeSummary(taskRecord, run),
		actor:              actor,
	})
}

func (m *Service) dispatchBlockedWake(
	ctx context.Context,
	taskRecord Task,
	block TaskBlock,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) {
	runID := ""
	executingSessionID := ""
	if release != nil {
		runID = strings.TrimSpace(release.Run.ID)
		executingSessionID = strings.TrimSpace(release.PreviousRun.SessionID)
	}
	m.dispatchWakeCreator(ctx, &wakeDispatchRequest{
		taskRecord:         taskRecord,
		runID:              runID,
		executingSessionID: executingSessionID,
		wakeEventID:        wakeEventID("blocked", taskRecord.ID, block.ID),
		reason:             WakeReasonBlocked,
		summary:            blockedWakeSummary(taskRecord, block),
		actor:              actor,
	})
}

func (m *Service) dispatchNeedsAttentionWake(
	ctx context.Context,
	taskRecord Task,
	block TaskBlock,
	reason string,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) {
	runID := ""
	executingSessionID := ""
	if release != nil {
		runID = strings.TrimSpace(release.Run.ID)
		executingSessionID = strings.TrimSpace(release.PreviousRun.SessionID)
	}
	m.dispatchWakeCreator(ctx, &wakeDispatchRequest{
		taskRecord:         taskRecord,
		runID:              runID,
		executingSessionID: executingSessionID,
		wakeEventID:        wakeEventID("needs_attention", taskRecord.ID, block.ID),
		reason:             WakeReasonNeedsAttention,
		summary:            needsAttentionWakeSummary(taskRecord, reason),
		actor:              actor,
	})
}

func (m *Service) dispatchWakeCreator(ctx context.Context, req *wakeDispatchRequest) {
	if m == nil || req == nil {
		return
	}
	taskRecord := req.taskRecord
	if taskRecord.CreatedBy.Kind.Normalize() != ActorKindAgentSession {
		return
	}
	creatorSessionID := strings.TrimSpace(taskRecord.CreatedBy.Ref)
	event := WakeEvent{
		WakeEventID: strings.TrimSpace(req.wakeEventID),
		WorkspaceID: strings.TrimSpace(taskRecord.WorkspaceID),
		TaskID:      strings.TrimSpace(taskRecord.ID),
		RunID:       strings.TrimSpace(req.runID),
		Reason:      req.reason,
		Summary:     req.summary,
	}.Normalize()
	if err := event.Validate(); err != nil {
		m.logWakeDispatchError("task: invalid wake event", err, event, taskRecord.ID)
		return
	}
	auditEventID, err := m.reserveTaskEventID()
	if err != nil {
		m.logWakeDispatchError("task: reserve wake audit identity", err, event, taskRecord.ID)
		return
	}
	reserved, err := m.reserveWakeEvent(ctx, event.TaskID, event.WakeEventID)
	if err != nil {
		m.logWakeDispatchError("task: wake dedupe check failed", err, event, taskRecord.ID)
		return
	}
	if !reserved {
		return
	}
	if !taskRecord.WakeCreator {
		m.recordWakeSuppressed(ctx, auditEventID, taskRecord, event, req, wakeSuppressionOptOut, nil)
		return
	}
	if creatorSessionID == "" {
		m.recordWakeSuppressed(ctx, auditEventID, taskRecord, event, req, wakeSuppressionSessionNotLive, nil)
		return
	}
	if strings.TrimSpace(req.executingSessionID) == creatorSessionID {
		m.recordWakeSuppressed(ctx, auditEventID, taskRecord, event, req, wakeSuppressionSelfWake, nil)
		return
	}
	notifier := m.wakeNotifier
	if notifier == nil {
		notifier = noopWakeNotifier{}
	}
	if err := notifier.WakeCreator(ctx, creatorSessionID, event); err != nil {
		if errors.Is(err, ErrSessionNotLive) {
			m.recordWakeSuppressed(ctx, auditEventID, taskRecord, event, req, wakeSuppressionSessionNotLive, nil)
			return
		}
		m.recordWakeSuppressed(ctx, auditEventID, taskRecord, event, req, wakeSuppressionDeliveryFailed, err)
		return
	}
	m.recordWakeDelivered(ctx, auditEventID, taskRecord, event, req)
}
