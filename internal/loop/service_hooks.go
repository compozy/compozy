package loop

import (
	"context"
	"log/slog"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/task"
)

func (s *service) dispatchLoopStarted(ctx context.Context, run Run, actor task.ActorContext) {
	if s.hooks == nil {
		return
	}
	payload := hookspkg.LoopStartedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopStarted,
			Timestamp: run.CreatedAt,
		},
		LoopContext: serviceLoopContext(run, actor),
		Status:      string(run.Status),
		Cause:       string(TransitionCauseStart),
	}
	_, err := s.hooks.DispatchLoopStarted(loopHookContext(ctx), payload)
	s.reportTerminalHookFailure(hookspkg.HookLoopStarted, err, payload)
}

func (s *service) dispatchCoordinatorTerminal(
	ctx context.Context,
	run Run,
	cause TransitionCause,
	at time.Time,
) {
	if s.hooks == nil {
		return
	}
	payload := hookspkg.LoopTerminalPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopTerminal,
			Timestamp: at,
		},
		LoopContext: serviceLoopContext(run, task.ActorContext{}),
		Status:      string(run.Status),
		Cause:       string(cause),
	}
	_, err := s.hooks.DispatchLoopTerminal(loopHookContext(ctx), payload)
	s.reportTerminalHookFailure(hookspkg.HookLoopTerminal, err, payload)
}

func (s *service) reportTerminalHookFailure(
	event hookspkg.HookEvent,
	err error,
	payload hookspkg.LoopLifecyclePayload,
) {
	if err == nil {
		return
	}
	slog.Warn(
		"loop: lifecycle hook failed after aggregate mutation",
		"hook_event", event,
		"loop_run_id", payload.LoopRunID,
		"loop_name", payload.LoopName,
		"error", err,
	)
}

func serviceLoopContext(run Run, actor task.ActorContext) hookspkg.LoopContext {
	return hookspkg.LoopContext{
		LoopRunID:       string(run.ID),
		ParentLoopRunID: string(run.ParentLoopRunID),
		WorkspaceID:     string(run.WorkspaceID),
		LoopName:        run.LoopName,
		Generation:      run.Generation,
		ResolvedNetworkParticipation: participation.CloneSpec(
			run.NetworkSpecSnapshot(),
		),
		ActorKind:  string(actor.Actor.Kind.Normalize()),
		ActorID:    actor.Actor.Ref,
		OriginKind: string(actor.Origin.Kind.Normalize()),
		OriginRef:  actor.Origin.Ref,
	}
}

func loopHookContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}
