package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// resumeConfirmedDeadAction leaves supervised stops to their receipt owner and resumes confirmed interruptions.
func (r *loopActionRuntime) resumeConfirmedDeadAction(
	ctx context.Context,
	run taskpkg.Run,
	actor taskpkg.ActorContext,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
	deathStreakLimit int,
) (bool, error) {
	if r == nil || r.sessions == nil || strings.TrimSpace(sessionID) == "" {
		return false, nil
	}
	sessionID = strings.TrimSpace(sessionID)
	info, err := r.sessions.Status(ctx, sessionID)
	if err != nil {
		r.logger.Warn(
			"daemon: inspect Loop action session before confirmed-death continuation",
			"session_id", sessionID,
			"error", err,
		)
		return false, nil
	}
	if info != nil && (info.StopCause == session.CauseInactivity ||
		(info.StopReason == storepkg.StopTimeout && info.StopDetail == "inactivity")) {
		// The durable stop receipt owns recovery and fences this attempt's late result.
		return true, nil
	}
	if !confirmedLoopActionSessionDeath(info) {
		return false, nil
	}
	if info.StopReason == storepkg.StopShutdown {
		qualified, err := qualifyLoopRestart(ctx, r.sessions, sessionID)
		if err != nil || !qualified {
			return false, err
		}
	}
	checkpoint, err := r.captureDeathResumeCheckpoint(ctx, sessionID)
	if err != nil {
		return false, err
	}
	resumer, ok := r.store.(looppkg.DeadNodeResumer)
	if !ok {
		return false, fmt.Errorf("daemon: Loop store does not support confirmed-death continuation")
	}
	request, err := looppkg.DeadNodeResumeRequestForTaskRun(
		run,
		workspaceID,
		sessionID,
		checkpoint,
		deathStreakLimit,
		confirmedLoopActionDeathCause(info),
		actor,
		r.now().UTC(),
	)
	if err != nil {
		return false, err
	}
	result, err := resumer.ResumeDeadNode(ctx, request)
	if err != nil {
		return false, fmt.Errorf("daemon: resume confirmed-dead Loop action: %w", err)
	}
	if result.Continued {
		r.logger.Info(
			"daemon: resumed confirmed-dead Loop action",
			"loop_run_id", run.LoopRunID,
			"task_run_id", run.ID,
			"continuation_task_run_id", result.Run.ID,
			"issued_epoch", result.IssuedEpoch,
			"replay", result.Replay,
		)
	}
	return true, nil
}

func confirmedLoopActionSessionDeath(info *session.Info) bool {
	return info != nil && info.State == session.StateStopped &&
		(info.StopReason == storepkg.StopAgentCrashed || info.StopReason == storepkg.StopShutdown)
}

func qualifyLoopRestart(ctx context.Context, sessions loopActionSessionStatus, id string) (bool, error) {
	qualifier, ok := sessions.(interface {
		QualifyRestartContinuation(context.Context, string) (bool, error)
	})
	if !ok {
		return false, nil
	}
	return qualifier.QualifyRestartContinuation(ctx, id)
}

func confirmedLoopActionDeathCause(info *session.Info) string {
	if info != nil && info.StopReason == storepkg.StopShutdown {
		return "daemon restart interrupted the session; verify state before repeating actions"
	}
	if info != nil && info.Failure != nil && strings.TrimSpace(info.Failure.Summary) != "" {
		return "confirmed session death: " + strings.TrimSpace(info.Failure.Summary)
	}
	return "confirmed session death: agent process exited"
}

// recoverInterruptedRuns fences Loop cells before generic boot lease recovery can requeue their old attempts.
// The Loop store remains the sole owner of continuation identities and its death-streak breaker.
func (r *loopActionRuntime) recoverInterruptedRuns(ctx context.Context, actor taskpkg.ActorContext) error {
	runs, err := r.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{
		taskpkg.TaskRunStatusClaimed, taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusRunning,
	})
	if err != nil {
		return err
	}
	var errs []error
	for _, run := range runs {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if run.LoopRunID == "" || run.SessionID == "" {
			continue
		}
		if err := r.recoverInterruptedRun(ctx, run, actor); err != nil &&
			!errors.Is(err, session.ErrSessionNotFound) {
			errs = append(errs, fmt.Errorf("daemon: recover interrupted Loop run %q: %w", run.ID, err))
		}
	}
	return errors.Join(errs...)
}

func (r *loopActionRuntime) recoverInterruptedRun(
	ctx context.Context,
	run taskpkg.Run,
	actor taskpkg.ActorContext,
) error {
	info, err := r.sessions.Status(ctx, run.SessionID)
	if err != nil {
		return err
	}
	if info == nil || info.State != session.StateStopped || info.StopReason != storepkg.StopShutdown {
		return nil
	}
	limit, err := r.actionDeathStreakLimitForRun(ctx, run)
	if err != nil {
		return err
	}
	handled, err := r.resumeConfirmedDeadAction(
		ctx,
		run,
		actor,
		looppkg.WorkspaceID(info.WorkspaceID),
		run.SessionID,
		limit,
	)
	if err != nil && !errors.Is(err, session.ErrRestartContinuationSkipped) {
		return err
	}
	if !handled {
		return r.pauseInterruptedRun(ctx, run, actor, info.WorkspaceID, limit)
	}
	return nil
}

func (r *loopActionRuntime) pauseInterruptedRun(
	ctx context.Context, run taskpkg.Run, actor taskpkg.ActorContext, workspaceID string, limit int,
) error {
	request, err := looppkg.DeadNodeResumeRequestForTaskRun(run, looppkg.WorkspaceID(workspaceID),
		run.SessionID, nil, limit, "Restart continuation requires review.", actor, r.now().UTC())
	if err != nil {
		return err
	}
	pauser, ok := r.store.(looppkg.NodePauseStore)
	if !ok {
		return errors.New("daemon: Loop store does not support pausing interrupted nodes")
	}
	_, err = pauser.PauseNode(ctx, looppkg.NodePauseMutation{
		WorkspaceID: request.WorkspaceID, RunID: request.RunID, NodeID: request.NodeID,
		ItemIndex: new(request.ItemIndex), Mode: looppkg.NodePauseDrain, Reason: request.Cause,
		Actor: actor, RequestedAt: request.ConfirmedAt,
	})
	return err
}
