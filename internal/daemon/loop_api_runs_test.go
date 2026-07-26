package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

func TestDaemonLoopAPIServiceApproveLoopRun(t *testing.T) {
	t.Parallel()

	t.Run("Should deny approval by the Loop starter agent session", func(t *testing.T) {
		t.Parallel()

		aggregate := &loopApprovalAggregateStub{
			run: loopApprovalRun("sess-author"),
			approveFn: func(
				context.Context,
				looppkg.WorkspaceID,
				looppkg.RunID,
				looppkg.NodeID,
				looppkg.GateDecision,
				task.ActorContext,
			) error {
				t.Fatal("Approve() should not be called for self-approval")
				return nil
			},
		}
		service := &daemonLoopAPIService{aggregate: aggregate}
		actor := mustLoopApprovalActor(t, "sess-author")

		err := service.ApproveLoopRun(
			t.Context(),
			"ws-1",
			"run-1",
			contract.ApproveLoopRunRequest{GateID: "human", Decision: contract.LoopGateDecisionApprove},
			actor,
		)
		if !errors.Is(err, task.ErrPermissionDenied) {
			t.Fatalf("ApproveLoopRun() error = %v, want ErrPermissionDenied", err)
		}
	})

	t.Run("Should delegate approval for a different agent session", func(t *testing.T) {
		t.Parallel()

		approveCalled := false
		aggregate := &loopApprovalAggregateStub{
			run: loopApprovalRun("sess-author"),
			approveFn: func(
				_ context.Context,
				ws looppkg.WorkspaceID,
				runID looppkg.RunID,
				gateID looppkg.NodeID,
				decision looppkg.GateDecision,
				approveActor task.ActorContext,
			) error {
				approveCalled = true
				if ws != looppkg.WorkspaceID("ws-1") ||
					runID != looppkg.RunID("run-1") ||
					gateID != looppkg.NodeID("human") ||
					decision != looppkg.GateDecisionApprove {
					t.Fatalf("Approve() = %s/%s/%s/%s", ws, runID, gateID, decision)
				}
				if approveActor.Actor.Ref != "sess-reviewer" {
					t.Fatalf("Approve() actor = %#v, want sess-reviewer", approveActor.Actor)
				}
				return nil
			},
		}
		service := &daemonLoopAPIService{aggregate: aggregate}
		actor := mustLoopApprovalActor(t, "sess-reviewer")

		if err := service.ApproveLoopRun(
			t.Context(),
			"ws-1",
			"run-1",
			contract.ApproveLoopRunRequest{GateID: "human", Decision: contract.LoopGateDecisionApprove},
			actor,
		); err != nil {
			t.Fatalf("ApproveLoopRun() error = %v", err)
		}
		if !approveCalled {
			t.Fatal("Approve() was not called")
		}
	})
}

func loopApprovalRun(starterSession string) *looppkg.Run {
	return &looppkg.Run{
		ID:          looppkg.RunID("run-1"),
		WorkspaceID: looppkg.WorkspaceID("ws-1"),
		Status:      looppkg.StatusNeedsApproval,
		StartedBy: task.ActorIdentity{
			Kind: task.ActorKindAgentSession,
			Ref:  starterSession,
		},
	}
}

func mustLoopApprovalActor(t *testing.T, sessionID string) task.ActorContext {
	t.Helper()
	actor, err := task.DeriveAgentSessionActorContextForOrigin(
		sessionID,
		"ws-1",
		task.OriginKindUDS,
		"loop_approve",
	)
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContextForOrigin() error = %v", err)
	}
	return actor
}

type loopApprovalAggregateStub struct {
	run       *looppkg.Run
	approveFn func(
		context.Context,
		looppkg.WorkspaceID,
		looppkg.RunID,
		looppkg.NodeID,
		looppkg.GateDecision,
		task.ActorContext,
	) error
}

func (s *loopApprovalAggregateStub) Start(
	context.Context,
	looppkg.WorkspaceID,
	string,
	looppkg.Inputs,
	task.ActorContext,
) (*looppkg.Run, error) {
	return nil, errors.New("unexpected Start call")
}

func (s *loopApprovalAggregateStub) StartInline(
	context.Context,
	looppkg.WorkspaceID,
	dsl.Definition,
	looppkg.Inputs,
	looppkg.RunOrigin,
	task.ActorContext,
) (*looppkg.Run, error) {
	return nil, errors.New("unexpected StartInline call")
}

func (s *loopApprovalAggregateStub) ReplaceInline(
	context.Context,
	looppkg.RunID,
	looppkg.WorkspaceID,
	dsl.Definition,
	looppkg.Inputs,
	looppkg.RunOrigin,
	task.ActorContext,
) (looppkg.InlineReplaceResult, error) {
	return looppkg.InlineReplaceResult{}, errors.New("unexpected ReplaceInline call")
}

func (s *loopApprovalAggregateStub) ClearInlineGoal(
	context.Context,
	looppkg.WorkspaceID,
	string,
	task.ActorContext,
) error {
	return errors.New("unexpected ClearInlineGoal call")
}

func (s *loopApprovalAggregateStub) DryRun(
	context.Context,
	looppkg.WorkspaceID,
	string,
	looppkg.Inputs,
) (*looppkg.PlanPreview, error) {
	return nil, errors.New("unexpected DryRun call")
}

func (s *loopApprovalAggregateStub) Stop(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
	looppkg.StopReason,
	task.ActorContext,
) error {
	return errors.New("unexpected Stop call")
}

func (s *loopApprovalAggregateStub) Pause(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
	task.ActorContext,
) error {
	return errors.New("unexpected Pause call")
}

func (s *loopApprovalAggregateStub) Resume(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
	task.ActorContext,
) error {
	return errors.New("unexpected Resume call")
}

func (s *loopApprovalAggregateStub) Approve(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	runID looppkg.RunID,
	gateID looppkg.NodeID,
	decision looppkg.GateDecision,
	actor task.ActorContext,
) error {
	if s.approveFn == nil {
		return errors.New("unexpected Approve call")
	}
	return s.approveFn(ctx, ws, runID, gateID, decision, actor)
}

func (s *loopApprovalAggregateStub) Configure(
	context.Context,
	looppkg.WorkspaceID,
	string,
	looppkg.LoopConfig,
) error {
	return errors.New("unexpected Configure call")
}

func (s *loopApprovalAggregateStub) GetConfig(
	context.Context,
	looppkg.WorkspaceID,
	string,
) (*looppkg.LoopConfig, error) {
	return nil, errors.New("unexpected GetConfig call")
}

func (s *loopApprovalAggregateStub) GetConfigSnapshot(
	context.Context,
	looppkg.WorkspaceID,
	string,
) (looppkg.ConfigSnapshot, error) {
	return looppkg.ConfigSnapshot{}, errors.New("unexpected GetConfigSnapshot call")
}

func (s *loopApprovalAggregateStub) Get(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
) (*looppkg.Run, error) {
	if s.run == nil {
		return nil, looppkg.ErrRunNotFound
	}
	run := *s.run
	return &run, nil
}

func (s *loopApprovalAggregateStub) Transition(
	context.Context,
	looppkg.RunID,
	looppkg.Status,
	looppkg.TransitionCause,
) error {
	return errors.New("unexpected Transition call")
}
