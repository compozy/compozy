package loop

import (
	"context"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/task"
)

func (r *CoordinatorRunner) dispatchGenerationPre(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	generation int,
) (bool, task.CoordinatorCompletionPlan) {
	if r.hooks == nil {
		return false, task.CoordinatorCompletionPlan{}
	}
	payload := hookspkg.LoopGenerationPrePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopGenerationPre,
			Timestamp: r.now().UTC(),
		},
		LoopContext: coordinatorLoopContext(taskRun, run, generation),
		Status:      string(run.Status),
	}
	result, err := r.hooks.DispatchLoopGenerationPre(loopHookContext(ctx), payload)
	if result.Denied {
		return true, deniedGenerationPlan(run, generation, StatusFailed, result.DenyReason)
	}
	if err != nil {
		r.logger.Warn(
			"loop: generation pre hook failed open",
			"loop_run_id", run.ID,
			"loop_name", run.LoopName,
			"generation", generation,
			"error", err,
		)
	}
	return false, task.CoordinatorCompletionPlan{}
}

func (r *CoordinatorRunner) dispatchGenerationPost(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	plan task.CoordinatorCompletionPlan,
) {
	if r.hooks == nil {
		return
	}
	payload := hookspkg.LoopGenerationPostPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopGenerationPost,
			Timestamp: r.now().UTC(),
		},
		LoopContext: coordinatorLoopContext(taskRun, run, plan.Snapshot.Generation),
		Status:      "planned",
	}
	if _, err := r.hooks.DispatchLoopGenerationPost(loopHookContext(ctx), payload); err != nil {
		r.logger.Warn(
			"loop: generation post hook failed open",
			"loop_run_id", run.ID,
			"loop_name", run.LoopName,
			"generation", plan.Snapshot.Generation,
			"error", err,
		)
	}
}

func (r *CoordinatorRunner) dispatchGateHooks(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	plan task.CoordinatorCompletionPlan,
) task.CoordinatorCompletionPlan {
	if r.hooks == nil || plan.Terminal == nil {
		return plan
	}
	generation := plan.Snapshot.Generation
	pre := hookspkg.LoopGatePrePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopGatePre,
			Timestamp: r.now().UTC(),
		},
		LoopContext: coordinatorLoopContext(taskRun, run, generation),
		Status:      plan.Terminal.Status,
		ReasonCode:  plan.Terminal.ReasonCode,
	}
	result, err := r.hooks.DispatchLoopGatePre(loopHookContext(ctx), pre)
	if result.Denied {
		plan.Terminal = deniedCoordinatorTerminal(StatusBlocked, result.DenyReason)
	} else if err != nil {
		r.logger.Warn(
			"loop: gate pre hook failed open",
			"loop_run_id", run.ID,
			"loop_name", run.LoopName,
			"generation", generation,
			"error", err,
		)
	}
	post := hookspkg.LoopGatePostPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopGatePost,
			Timestamp: r.now().UTC(),
		},
		LoopContext: coordinatorLoopContext(taskRun, run, generation),
		Status:      plan.Terminal.Status,
		ReasonCode:  plan.Terminal.ReasonCode,
	}
	if _, err := r.hooks.DispatchLoopGatePost(loopHookContext(ctx), post); err != nil {
		r.logger.Warn(
			"loop: gate post hook failed open",
			"loop_run_id", run.ID,
			"loop_name", run.LoopName,
			"generation", generation,
			"error", err,
		)
	}
	return plan
}

func deniedGenerationPlan(
	run Run,
	generation int,
	status Status,
	reason string,
) task.CoordinatorCompletionPlan {
	return task.CoordinatorCompletionPlan{
		Snapshot: task.GenerationSnapshot{
			LoopRunID:  string(run.ID),
			Generation: generation,
			Payload:    GenerationSnapshotPayload{},
		},
		Terminal: deniedCoordinatorTerminal(status, reason),
	}
}

func deniedCoordinatorTerminal(status Status, reason string) *task.CoordinatorTerminal {
	terminal := &task.CoordinatorTerminal{
		Status:     string(status),
		Cause:      string(TransitionCauseContract),
		ReasonCode: "hook_denied",
	}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		terminal.ReasonCode = trimmed
	}
	return terminal
}

func coordinatorLoopContext(taskRun task.Run, run Run, generation int) hookspkg.LoopContext {
	networkSpec := taskRun.NetworkSpecSnapshot()
	return hookspkg.LoopContext{
		LoopRunID:                    string(run.ID),
		ParentLoopRunID:              string(run.ParentLoopRunID),
		WorkspaceID:                  string(run.WorkspaceID),
		LoopName:                     strings.TrimSpace(run.LoopName),
		Generation:                   generation,
		TaskID:                       strings.TrimSpace(taskRun.TaskID),
		RunID:                        strings.TrimSpace(taskRun.ID),
		RunKind:                      taskRun.RunKind.Normalize().String(),
		ResolvedNetworkParticipation: participation.CloneSpec(networkSpec),
		SessionID:                    strings.TrimSpace(taskRun.SessionID),
		OriginKind:                   string(taskRun.Origin.Kind.Normalize()),
		OriginRef:                    strings.TrimSpace(taskRun.Origin.Ref),
	}
}
