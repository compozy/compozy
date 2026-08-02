package daemon

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

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
	info, err := r.sessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil || !confirmedLoopActionSessionDeath(info) {
		return false, nil
	}
	checkpoint, err := r.captureDeathResumeCheckpoint(ctx, strings.TrimSpace(sessionID))
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
		strings.TrimSpace(sessionID),
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
	return info != nil && info.State == session.StateStopped && info.StopReason == storepkg.StopAgentCrashed
}

func confirmedLoopActionDeathCause(info *session.Info) string {
	if info != nil && info.Failure != nil && strings.TrimSpace(info.Failure.Summary) != "" {
		return "confirmed session death: " + strings.TrimSpace(info.Failure.Summary)
	}
	return "confirmed session death: agent process exited"
}
