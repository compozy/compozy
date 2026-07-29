package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestDaemonLoopAPIServiceShouldBuildRunWebURLFromEffectiveConfig(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(homePaths, "", compozyconfig.WriteScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}
	if _, err := compozyconfig.EditConfigOverlay(
		homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if err := editor.SetValue([]string{"http", "host"}, "127.0.0.1"); err != nil {
				return err
			}
			return editor.SetValue([]string{"http", "port"}, 43127)
		},
	); err != nil {
		t.Fatalf("EditConfigOverlay() error = %v", err)
	}

	service := &daemonLoopAPIService{homePaths: homePaths}
	endpoint, err := service.resolveLoopRunWebEndpoint(t.Context(), "ws-1")
	if err != nil {
		t.Fatalf("resolveLoopRunWebEndpoint() error = %v", err)
	}
	got := endpoint.runURL("run-123")
	const want = "http://127.0.0.1:43127/loop-runs/run-123"
	if got != want {
		t.Fatalf("loopRunWebEndpoint.runURL() = %q, want %q", got, want)
	}
}

func TestDaemonLoopAPIServiceShouldResolveRunWebEndpointBeforeStarting(t *testing.T) {
	t.Parallel()

	errResolve := errors.New("resolve effective config")
	startCalled := false
	aggregate := &loopApprovalAggregateStub{startFn: func(
		context.Context,
		looppkg.WorkspaceID,
		string,
		looppkg.Inputs,
		task.ActorContext,
	) (*looppkg.Run, error) {
		startCalled = true
		return nil, errors.New("unexpected Start call")
	}}
	service := &daemonLoopAPIService{
		aggregate:         aggregate,
		workspaceResolver: &loopAPIWorkspaceResolverErrorStub{err: errResolve},
	}

	_, err := service.RunLoop(
		t.Context(),
		"ws-1",
		"delivery",
		contract.RunLoopRequest{},
		dsl.StartHTTP,
		task.ActorContext{},
		false,
	)
	if !errors.Is(err, errResolve) {
		t.Fatalf("RunLoop() error = %v, want effective-config resolution error", err)
	}
	if startCalled {
		t.Fatal("RunLoop() called Start before resolving the run web endpoint")
	}
}

func TestDaemonLoopAPIServiceAnnotationsRequireDefinition(t *testing.T) {
	t.Parallel()

	t.Run("Should reject reads for a missing definition", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		service := &daemonLoopAPIService{
			catalog:     newResourceCatalog(looppkg.CloneResourceSpec),
			persistence: db,
		}

		_, err := service.GetLoopAnnotations(t.Context(), "ws-missing", "missing-loop")
		if !errors.Is(err, looppkg.ErrDefinitionNotFound) {
			t.Fatalf("GetLoopAnnotations(missing definition) error = %v, want ErrDefinitionNotFound", err)
		}
	})

	t.Run("Should reject writes for a missing definition without persisting them", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		service := &daemonLoopAPIService{
			catalog:     newResourceCatalog(looppkg.CloneResourceSpec),
			persistence: db,
		}
		request := contract.PutLoopAnnotationsRequest{Annotations: []contract.LoopAnnotationPayload{{
			NodeID: "ghost",
			X:      12,
			Y:      34,
		}}}

		_, err := service.PutLoopAnnotations(t.Context(), "ws-missing", "missing-loop", request)
		if !errors.Is(err, looppkg.ErrDefinitionNotFound) {
			t.Fatalf("PutLoopAnnotations(missing definition) error = %v, want ErrDefinitionNotFound", err)
		}
		annotations, listErr := db.ListLoopUIAnnotations(t.Context(), "ws-missing", "missing-loop")
		if listErr != nil {
			t.Fatalf("ListLoopUIAnnotations(after rejected write) error = %v", listErr)
		}
		if len(annotations) != 0 {
			t.Fatalf("annotations after rejected write = %#v, want empty", annotations)
		}
	})
}

func TestDaemonLoopAPIServiceShouldManageScopedInputDefaultsWithoutCollapsingPresence(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	resolver := loopCatalogWorkspaceResolverForTest(
		t,
		"ws-input-defaults",
		workspaceRoot,
		time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	)
	service := &daemonLoopAPIService{homePaths: homePaths, workspaceResolver: resolver}

	global, err := service.PutLoopInputDefault(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		"auto_commit",
		contract.PutLoopInputDefaultRequest{Scope: contract.LoopInputDefaultsScopeGlobal, Value: false},
	)
	if err != nil {
		t.Fatalf("PutLoopInputDefault(global) error = %v", err)
	}
	if !global.Present || global.Value != false {
		t.Fatalf("global input default = %#v, want present explicit false", global)
	}

	workspace, err := service.PutLoopInputDefaults(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		contract.PutLoopInputDefaultsRequest{
			Scope: contract.LoopInputDefaultsScopeWorkspace,
			Values: map[string]any{
				"auto_commit": true,
				"retries":     float64(0),
				"reviewer":    "",
			},
		},
	)
	if err != nil {
		t.Fatalf("PutLoopInputDefaults(workspace) error = %v", err)
	}
	if workspace.Values["auto_commit"] != true {
		t.Fatalf("workspace auto_commit = %#v, want true", workspace.Values["auto_commit"])
	}
	switch retries := workspace.Values["retries"].(type) {
	case int64:
		if retries != 0 {
			t.Fatalf("workspace retries = %d, want zero", retries)
		}
	case float64:
		if retries != 0 {
			t.Fatalf("workspace retries = %f, want zero", retries)
		}
	default:
		t.Fatalf("workspace retries = %#v (%T), want numeric zero", retries, retries)
	}
	if reviewer, present := workspace.Values["reviewer"]; !present || reviewer != "" {
		t.Fatalf("workspace reviewer = %#v/%v, want present empty string", reviewer, present)
	}

	globalLayer, err := service.GetLoopInputDefaults(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		contract.LoopInputDefaultsScopeGlobal,
	)
	if err != nil {
		t.Fatalf("GetLoopInputDefaults(global) error = %v", err)
	}
	if value, present := globalLayer.Values["auto_commit"]; !present || value != false {
		t.Fatalf("global layer after workspace override = %#v, want explicit false preserved", globalLayer.Values)
	}

	deleted, err := service.DeleteLoopInputDefault(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		"auto_commit",
		contract.LoopInputDefaultsScopeWorkspace,
	)
	if err != nil {
		t.Fatalf("DeleteLoopInputDefault(workspace) error = %v", err)
	}
	if !deleted.Deleted {
		t.Fatalf("DeleteLoopInputDefault(workspace) = %#v, want deleted", deleted)
	}
	missing, err := service.GetLoopInputDefault(
		t.Context(),
		"ws-input-defaults",
		"review-and-fix",
		"auto_commit",
		contract.LoopInputDefaultsScopeWorkspace,
	)
	if err != nil {
		t.Fatalf("GetLoopInputDefault(workspace after delete) error = %v", err)
	}
	if missing.Present {
		t.Fatalf("workspace input default after delete = %#v, want absent", missing)
	}
}

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
	run     *looppkg.Run
	startFn func(
		context.Context,
		looppkg.WorkspaceID,
		string,
		looppkg.Inputs,
		task.ActorContext,
	) (*looppkg.Run, error)
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
	ctx context.Context,
	ws looppkg.WorkspaceID,
	name string,
	inputs looppkg.Inputs,
	actor task.ActorContext,
) (*looppkg.Run, error) {
	if s.startFn != nil {
		return s.startFn(ctx, ws, name, inputs, actor)
	}
	return nil, errors.New("unexpected Start call")
}

type loopAPIWorkspaceResolverErrorStub struct {
	err error
}

func (s *loopAPIWorkspaceResolverErrorStub) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{}, s.err
}

func (s *loopAPIWorkspaceResolverErrorStub) ResolveOrRegister(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{}, s.err
}

func (s *loopAPIWorkspaceResolverErrorStub) Get(
	context.Context,
	string,
) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, s.err
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
