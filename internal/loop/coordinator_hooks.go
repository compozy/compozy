package loop

import (
	"context"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) dispatchGenerationPre(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	intent GenerationIntent,
) (bool, task.CoordinatorCompletionPlan) {
	if r.hooks == nil {
		return false, task.CoordinatorCompletionPlan{}
	}
	payload := hookspkg.LoopGenerationPrePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopGenerationPre,
			Timestamp: r.now().UTC(),
		},
		LoopContext:      coordinatorLoopContext(taskRun, run, int(intent.Generation)),
		Origin:           hookspkg.LoopGenerationOrigin(intent.Origin),
		ParentGeneration: intent.ParentGeneration,
		Status:           string(run.Status),
	}
	result, err := r.hooks.DispatchLoopGenerationPre(loopHookContext(ctx), payload)
	if result.Denied {
		return true, deniedGenerationPlan(run, int(intent.Generation), StatusFailed, result.DenyReason)
	}
	if err != nil {
		r.logger.Warn(
			"loop: generation pre hook failed open",
			"loop_run_id", run.ID,
			"loop_name", run.LoopName,
			"generation", intent.Generation,
			"origin", intent.Origin,
			"parent_generation", intent.ParentGeneration,
			"error", err,
		)
	}
	return false, task.CoordinatorCompletionPlan{}
}

func (r *CoordinatorRunner) dispatchGenerationPost(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	intent GenerationIntent,
) {
	if r.hooks == nil {
		return
	}
	payload := hookspkg.LoopGenerationPostPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopGenerationPost,
			Timestamp: r.now().UTC(),
		},
		LoopContext:      coordinatorLoopContext(taskRun, run, int(intent.Generation)),
		Origin:           hookspkg.LoopGenerationOrigin(intent.Origin),
		ParentGeneration: intent.ParentGeneration,
		Status:           "planned",
	}
	if _, err := r.hooks.DispatchLoopGenerationPost(loopHookContext(ctx), payload); err != nil {
		r.logger.Warn(
			"loop: generation post hook failed open",
			"loop_run_id", run.ID,
			"loop_name", run.LoopName,
			"generation", intent.Generation,
			"origin", intent.Origin,
			"parent_generation", intent.ParentGeneration,
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
	if r.hooks == nil {
		return plan
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		r.logger.Warn(
			"loop: decode gate hook payload failed open",
			"loop_run_id", run.ID,
			"loop_name", run.LoopName,
			"generation", plan.Snapshot.Generation,
			"error", err,
		)
		return plan
	}
	if len(payload.Verdicts) == 0 {
		return plan
	}
	bestGeneration := cloneInt64(run.BestGeneration)
	if payload.BestUpdate != nil {
		bestGeneration = new(payload.BestUpdate.Generation)
	}
	hookCtx := loopHookContext(ctx)
	var deniedTerminal *task.CoordinatorTerminal
	for _, verdict := range payload.Verdicts {
		pre := r.loopGateHookPayload(
			taskRun,
			run,
			plan,
			hookspkg.HookLoopGatePre,
			verdict.GateID,
			string(verdict.Outcome),
			verdict.Score,
			bestGeneration,
		)
		result, dispatchErr := r.hooks.DispatchLoopGatePre(hookCtx, pre)
		if result.Denied && plan.Terminal == nil && deniedTerminal == nil {
			deniedTerminal = deniedCoordinatorTerminal(StatusBlocked, result.DenyReason)
		} else if dispatchErr != nil {
			r.logGateHookFailure(run, plan.Snapshot.Generation, verdict.GateID, "pre", dispatchErr)
		}
	}
	if deniedTerminal != nil {
		plan.Terminal = deniedTerminal
	}
	for _, verdict := range payload.Verdicts {
		post := r.loopGateHookPayload(
			taskRun,
			run,
			plan,
			hookspkg.HookLoopGatePost,
			verdict.GateID,
			string(verdict.Outcome),
			verdict.Score,
			bestGeneration,
		)
		if _, dispatchErr := r.hooks.DispatchLoopGatePost(hookCtx, post); dispatchErr != nil {
			r.logGateHookFailure(run, plan.Snapshot.Generation, verdict.GateID, "post", dispatchErr)
		}
	}
	return plan
}

func (r *CoordinatorRunner) loopGateHookPayload(
	taskRun task.Run,
	run Run,
	plan task.CoordinatorCompletionPlan,
	event hookspkg.HookEvent,
	gateID string,
	outcome string,
	score *float64,
	bestGeneration *int64,
) hookspkg.LoopGatePayload {
	status, reasonCode := loopGateHookPlanState(plan)
	return hookspkg.LoopGatePayload{
		PayloadBase:    hookspkg.PayloadBase{Event: event, Timestamp: r.now().UTC()},
		LoopContext:    coordinatorLoopContext(taskRun, run, plan.Snapshot.Generation),
		GateID:         gateID,
		Outcome:        outcome,
		Score:          cloneFloat64(score),
		BestGeneration: cloneInt64(bestGeneration),
		Status:         status,
		ReasonCode:     reasonCode,
	}
}

func loopGateHookPlanState(plan task.CoordinatorCompletionPlan) (string, string) {
	if plan.Terminal == nil {
		return coordinatorPhaseEvaluated, ""
	}
	return plan.Terminal.Status, plan.Terminal.ReasonCode
}

func (r *CoordinatorRunner) logGateHookFailure(
	run Run,
	generation int,
	gateID string,
	phase string,
	err error,
) {
	r.logger.Warn(
		"loop: gate hook failed open",
		"loop_run_id", run.ID,
		"loop_name", run.LoopName,
		"generation", generation,
		"gate_id", gateID,
		"phase", phase,
		"error", err,
	)
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
