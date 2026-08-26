package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (r *taskRuntime) submitDetachedHarnessWork(
	ctx context.Context,
	req detachedHarnessSubmitRequest,
) (*detachedHarnessSubmission, error) {
	if r == nil {
		return nil, errors.New("daemon: task runtime is required")
	}
	if r.detached == nil {
		return nil, errors.New("daemon: detached harness bridge is required")
	}
	return r.detached.submit(ctx, req)
}

func (r *taskRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if r.bridgeNotifications != nil {
		r.bridgeNotifications.shutdown()
	}
	if r.taskStatusProjection != nil {
		r.taskStatusProjection.shutdown()
	}
	if r.reentry != nil {
		r.reentry.shutdown()
	}
	var shutdownErr error
	if roles := r.roles.Load(); roles != nil {
		if err := roles.shutdown(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if r.loopActions != nil {
		if err := r.loopActions.shutdown(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if r.claimHandoff != nil {
		if err := r.claimHandoff.shutdown(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if r.wakeBridge != nil {
		if err := r.wakeBridge.shutdown(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	return shutdownErr
}

func recoverTaskRunsOnBoot(
	ctx context.Context,
	manager *taskpkg.Service,
	store taskStore,
	sessions taskBridgeSessionManager,
	actor taskpkg.ActorContext,
) (taskRecoveryStats, error) {
	terminalSettled, err := recoverPendingTerminalRunCommandsOnBoot(ctx, manager, sessions, actor)
	if err != nil {
		return taskRecoveryStats{}, err
	}
	expired, err := manager.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
		Reason: taskRecoveryReasonBoot,
	}, actor)
	if err != nil {
		return taskRecoveryStats{}, fmt.Errorf(
			"daemon: recover expired task run leases on boot: %w",
			err,
		)
	}

	runs, err := store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning,
	})
	if err != nil {
		return taskRecoveryStats{}, fmt.Errorf("daemon: list task runs for boot recovery: %w", err)
	}

	stats := taskRecoveryStats{terminalSettled: terminalSettled, requeued: len(expired)}
	for _, run := range runs {
		recovery, err := planTaskRunRecovery(ctx, sessions, run)
		if err != nil {
			return taskRecoveryStats{}, fmt.Errorf(
				"daemon: plan boot recovery for task run %q: %w",
				run.ID,
				err,
			)
		}
		if recovery == nil {
			continue
		}
		if _, err := manager.RecoverRunOnBoot(ctx, run.ID, *recovery, actor); err != nil {
			return taskRecoveryStats{}, fmt.Errorf(
				"daemon: recover task run %q on boot: %w",
				run.ID,
				err,
			)
		}
		switch recovery.Action.Normalize() {
		case taskpkg.RunBootRecoveryRequeue:
			stats.requeued++
		case taskpkg.RunBootRecoveryMarkRunning:
			stats.markedRunning++
		case taskpkg.RunBootRecoveryFail:
			stats.failed++
		}
	}

	return stats, nil
}

func recoverPendingTerminalRunCommandsOnBoot(
	ctx context.Context,
	manager *taskpkg.Service,
	sessions taskBridgeSessionManager,
	actor taskpkg.ActorContext,
) (int, error) {
	pending, err := manager.ListPendingTerminalRunCommands(ctx, actor)
	if err != nil {
		return 0, fmt.Errorf("daemon: list pending terminal run commands on boot: %w", err)
	}
	settled := 0
	for _, command := range pending {
		evidence := taskpkg.TerminalRunRecoveryEvidence{
			RunID: command.RunID, SessionID: command.SessionID,
			Disposition: taskpkg.TerminalRunRecoveryStopObserved,
		}
		if command.StopRequired && command.Phase != taskpkg.TerminalRunCommandPhaseStopConfirmed {
			if sessions == nil {
				return 0, errors.New("daemon: terminal task recovery requires a session manager")
			}
			observed, inspectErr := inspectTaskSessionRecovery(ctx, sessions, command.SessionID)
			if inspectErr != nil {
				return 0, fmt.Errorf(
					"daemon: inspect session for terminal task run %q: %w",
					command.RunID,
					inspectErr,
				)
			}
			evidence.State = observed.state
			evidence.Disposition = terminalRunRecoveryDisposition(observed)
		}
		if _, recoverErr := manager.RecoverTerminalRunCommandOnBoot(ctx, evidence, actor); recoverErr != nil {
			return 0, fmt.Errorf(
				"daemon: recover terminal task run %q on boot: %w",
				command.RunID,
				recoverErr,
			)
		}
		settled++
	}
	return settled, nil
}

func terminalRunRecoveryDisposition(
	evidence taskSessionRecoveryEvidence,
) taskpkg.TerminalRunRecoveryDisposition {
	if evidence.live || evidence.stopRequired {
		return taskpkg.TerminalRunRecoveryResumeStop
	}
	switch strings.TrimSpace(evidence.state) {
	case taskRecoverySessionMissing, string(session.StateStopped):
		return taskpkg.TerminalRunRecoveryStopObserved
	default:
		return taskpkg.TerminalRunRecoveryAmbiguous
	}
}

func planTaskRunRecovery(
	ctx context.Context,
	sessions taskBridgeSessionManager,
	run taskpkg.Run,
) (*taskpkg.RunBootRecovery, error) {
	if sessions == nil {
		return nil, errors.New("daemon: task recovery requires a session manager")
	}
	if run.IsNetworkWake() {
		return planNetworkWakeRunRecovery(run), nil
	}
	if run.IsCallActivation() {
		return nil, nil
	}

	evidence, err := inspectTaskSessionRecovery(ctx, sessions, strings.TrimSpace(run.SessionID))
	if err != nil {
		return nil, err
	}

	return planSessionRunRecovery(run.Status.Normalize(), evidence), nil
}

func planNetworkWakeRunRecovery(run taskpkg.Run) *taskpkg.RunBootRecovery {
	switch run.Status.Normalize() {
	case taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning:
		return &taskpkg.RunBootRecovery{
			Action:         taskpkg.RunBootRecoveryRequeue,
			Reason:         taskRecoveryReasonBoot,
			SessionState:   "network_wake",
			Classification: taskRecoveryClassificationOrphaned,
			Detail:         "network wake recovered through the durable task queue",
		}
	default:
		return nil
	}
}

func planSessionRunRecovery(
	status taskpkg.RunStatus,
	evidence taskSessionRecoveryEvidence,
) *taskpkg.RunBootRecovery {
	switch status {
	case taskpkg.TaskRunStatusClaimed:
		if evidence.live {
			return taskRunRecoveryFromEvidence(taskpkg.RunBootRecoveryMarkRunning, evidence)
		}
		return taskRunRecoveryFromEvidence(taskpkg.RunBootRecoveryRequeue, evidence)

	case taskpkg.TaskRunStatusStarting:
		if evidence.live {
			return taskRunRecoveryFromEvidence(taskpkg.RunBootRecoveryMarkRunning, evidence)
		}
		return taskRunRecoveryFromEvidence(taskpkg.RunBootRecoveryFail, evidence)

	case taskpkg.TaskRunStatusRunning:
		if evidence.live {
			return nil
		}
		return taskRunRecoveryFromEvidence(taskpkg.RunBootRecoveryFail, evidence)

	default:
		return nil
	}
}

func taskRunRecoveryFromEvidence(
	action taskpkg.RunBootRecoveryAction,
	evidence taskSessionRecoveryEvidence,
) *taskpkg.RunBootRecovery {
	return &taskpkg.RunBootRecovery{
		Action:         action,
		Reason:         taskRecoveryReasonBoot,
		SessionState:   evidence.state,
		Classification: evidence.classification,
		Detail:         evidence.detail,
		StopRequired:   action == taskpkg.RunBootRecoveryFail && evidence.stopRequired,
	}
}

func taskSessionRuntimeState(
	ctx context.Context,
	sessions taskBridgeSessionManager,
	sessionID string,
) (bool, string, error) {
	evidence, err := inspectTaskSessionRecovery(ctx, sessions, sessionID)
	if err != nil {
		return false, "", err
	}
	return evidence.live, evidence.state, nil
}

func isTaskSessionStateLive(state session.State) bool {
	switch state {
	case session.StateStarting, session.StateActive, session.StateStopping:
		return true
	default:
		return false
	}
}

func inspectTaskSessionRecovery(
	ctx context.Context,
	sessions taskBridgeSessionManager,
	sessionID string,
) (taskSessionRecoveryEvidence, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return taskSessionRecoveryEvidence{
			state:          taskRecoverySessionMissing,
			classification: taskRecoveryClassificationMissing,
			detail:         "run has no bound session",
		}, nil
	}

	info, err := sessions.Status(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return taskSessionRecoveryEvidence{
				state:          taskRecoverySessionMissing,
				classification: taskRecoveryClassificationMissing,
				detail:         "bound session metadata is missing",
			}, nil
		}
		return taskSessionRecoveryEvidence{}, err
	}
	if info == nil {
		return taskSessionRecoveryEvidence{
			state:          taskRecoverySessionMissing,
			classification: taskRecoveryClassificationMissing,
			detail:         "bound session metadata is missing",
		}, nil
	}

	evidence := taskSessionRecoveryEvidence{
		live:  isTaskSessionStateLive(info.State),
		state: string(info.State),
	}
	if evidence.state == "" {
		evidence.state = taskRecoverySessionMissing
	}
	if evidence.live {
		evidence.classification = taskRecoveryClassificationLive
		return evidence, nil
	}

	evidence.classification, evidence.detail = classifyRecoveredTaskSession(info, time.Now().UTC())
	evidence.stopRequired = taskSessionMatchesRecordedSubprocess(info.Liveness)
	return evidence, nil
}

func classifyRecoveredTaskSession(info *session.Info, now time.Time) (string, string) {
	if info == nil {
		return taskRecoveryClassificationMissing, "session metadata is unavailable"
	}
	if liveness := info.Liveness; liveness != nil {
		if strings.TrimSpace(liveness.StallState) == store.SessionStallStateDetected {
			return taskRecoveryClassificationStalled, firstTaskRecoveryDetail(
				liveness.StallReason,
				info.StopDetail,
				"session liveness monitor marked the process stalled",
			)
		}
		if lastActivityAt := taskSessionLastActivityAt(liveness); lastActivityAt != nil &&
			!lastActivityAt.IsZero() &&
			now.Sub(lastActivityAt.UTC()) >= session.DefaultLivenessStallAfter &&
			taskSessionMatchesRecordedSubprocess(liveness) {
			return taskRecoveryClassificationStalled, firstTaskRecoveryDetail(
				liveness.StallReason,
				store.SessionStallReasonActivityTimeout,
				info.StopDetail,
			)
		}
		if taskSessionMatchesRecordedSubprocess(liveness) {
			return taskRecoveryClassificationOrphaned, fmt.Sprintf(
				"subprocess pid %d is still alive without a live daemon owner",
				liveness.SubprocessPID,
			)
		}
	}
	return taskRecoveryClassificationCrashed, firstTaskRecoveryDetail(
		info.StopDetail,
		"bound session is not live",
	)
}

func taskSessionLastActivityAt(liveness *store.SessionLivenessMeta) *time.Time {
	if liveness == nil {
		return nil
	}
	if liveness.Activity != nil &&
		liveness.Activity.LastActivityAt != nil &&
		!liveness.Activity.LastActivityAt.IsZero() {
		return liveness.Activity.LastActivityAt
	}
	return liveness.LastUpdateAt
}

func taskSessionMatchesRecordedSubprocess(liveness *store.SessionLivenessMeta) bool {
	if liveness == nil || liveness.SubprocessPID <= 0 {
		return false
	}
	if liveness.SubprocessStartedAt == nil || liveness.SubprocessStartedAt.IsZero() {
		return false
	}
	return procutil.MatchesStartTime(liveness.SubprocessPID, *liveness.SubprocessStartedAt)
}

func firstTaskRecoveryDetail(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func taskSessionName(spec *taskpkg.StartTaskSession) string {
	if spec == nil {
		return "task:#0"
	}

	base := strings.TrimSpace(spec.Task.Title)
	if base == "" {
		base = strings.TrimSpace(spec.Task.Identifier)
	}
	if base == "" {
		base = strings.TrimSpace(spec.Run.ID)
	}
	return fmt.Sprintf("task:%s#%d", base, spec.Run.Attempt)
}

func taskSessionAgentName(taskRecord taskpkg.Task) string {
	if taskRecord.Owner == nil || taskRecord.Owner.IsZero() {
		return ""
	}
	owner := *taskRecord.Owner
	if owner.Kind.Normalize() != taskpkg.OwnerKindPool {
		return ""
	}
	return strings.TrimSpace(owner.Ref)
}

func taskStopCause(reason taskpkg.StopReason) session.StopCause {
	switch reason.Normalize() {
	case taskpkg.StopReasonCompleted:
		return session.CauseCompleted
	case taskpkg.StopReasonFailed:
		return session.CauseFailed
	case taskpkg.StopReasonShutdown:
		return session.CauseShutdown
	case taskpkg.StopReasonOrphanedRun:
		return session.CauseFailed
	default:
		return session.CauseUserRequested
	}
}

func taskStopDetail(reason taskpkg.StopReason) string {
	switch reason.Normalize() {
	case taskpkg.StopReasonCompleted:
		return "task completed"
	case taskpkg.StopReasonFailed:
		return "task failed"
	case taskpkg.StopReasonShutdown:
		return taskStopDetailShutdown
	case taskpkg.StopReasonOrphanedRun:
		return taskStopDetailOrphaned
	default:
		return taskStopDetailCancellation
	}
}
