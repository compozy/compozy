//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/workspace"
)

func TestLoopGoalManagedRuntimeIntegration(t *testing.T) {
	t.Run("Should terminalize a wedged action and advance the Loop through the real scheduler", func(t *testing.T) {
		testLoopActionLivenessIntegration(t)
	})

	t.Run("Should stall after one node fails across generations despite a successful sibling", func(t *testing.T) {
		testLoopFailureBreakerIntegration(t)
	})

	t.Run("Should create a run-owned session when origin participation differs from the Loop Run", func(t *testing.T) {
		fixture := newLoopGoalManagedRuntimeFixture(t, "origin-participation", nil, withoutInitialGoalBinding())
		originParticipation := daemonTestLiveParticipation(
			string(fixture.run.WorkspaceID),
			"goal-origin-live",
		)
		originID, originIdentity := fixture.createOriginSession(
			t,
			"origin-participation",
			originParticipation,
		)
		request := fixture.bindingRequest("origin-participation")
		request.OriginSessionID = originID
		request.PinnedCreationProfileRef = originIdentity.CreationProfileRef
		request.PinnedCreationDigest = originIdentity.CreationDigest
		request.StaticPolicySpecDigest = originIdentity.PolicySpecDigest
		localParticipation := participation.LocalSpec()
		request.NetworkParticipation = &localParticipation

		binding, err := fixture.runtime.BindActionSession(testutil.Context(t), request)
		if err != nil {
			t.Fatalf("BindActionSession() error = %v", err)
		}
		if binding.SessionID == originID {
			t.Fatalf("binding.SessionID = %q, want a run-owned session distinct from origin", binding.SessionID)
		}
		if got, want := binding.Ownership, string(goalpkg.BindingOwnershipRunOwned); got != want {
			t.Fatalf("binding.Ownership = %q, want %q", got, want)
		}
		bound, err := fixture.manager.Status(testutil.Context(t), binding.SessionID)
		if err != nil {
			t.Fatalf("Status(bound session) error = %v", err)
		}
		if got := bound.NetworkParticipation; got != localParticipation {
			t.Fatalf("bound session participation = %#v, want Loop Run snapshot %#v", got, localParticipation)
		}
		origin, err := fixture.manager.Status(testutil.Context(t), originID)
		if err != nil {
			t.Fatalf("Status(origin session) error = %v", err)
		}
		if got := origin.NetworkParticipation; got != originParticipation {
			t.Fatalf("origin participation = %#v, want unchanged %#v", got, originParticipation)
		}
	})

	t.Run("Should converge Stop after session materialization and before binding activation", func(t *testing.T) {
		materialized := make(chan struct{})
		releaseActivation := make(chan struct{})
		fixture := newLoopGoalManagedRuntimeFixture(
			t,
			"stop-create-race",
			newHarnessIntegrationDriver(),
			withoutInitialGoalBinding(),
			withGoalRuntimeSessionManager(func(manager *session.Manager) SessionManager {
				return &delayedEnsureCreatedManager{
					Manager: manager, materialized: materialized, release: releaseActivation,
				}
			}),
		)
		request := fixture.bindingRequest("stop-create-race")
		result := make(chan error, 1)
		go func() {
			_, err := fixture.runtime.BindActionSession(testutil.Context(t), request)
			result <- err
		}()
		select {
		case <-materialized:
		case <-time.After(time.Second):
			t.Fatal("managed session creation did not materialize")
		}

		aggregate, err := looppkg.NewService(
			fixture.goalStore,
			looppkg.DefinitionResolverFunc(
				func(context.Context, looppkg.WorkspaceID, string) (*looppkg.ResolvedDefinition, error) {
					return nil, looppkg.ErrDefinitionNotFound
				},
			),
			managedTestGoalRunPolicyResolver(),
			looppkg.WithClock(time.Now),
		)
		if err != nil {
			t.Fatalf("loop.NewService() error = %v", err)
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator-stop-create", taskpkg.OriginKindCLI, "cli")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		if err := aggregate.Stop(
			testutil.Context(t), fixture.run.WorkspaceID, fixture.run.ID, looppkg.StopReasonOperator, actor,
		); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		cleanupStore, ok := fixture.goalStore.(goalSessionCleanupStore)
		if !ok {
			t.Fatalf("goal store = %T, want goalSessionCleanupStore", fixture.goalStore)
		}
		outboxStore, ok := fixture.goalStore.(goalSessionOutboxStore)
		if !ok {
			t.Fatalf("goal store = %T, want goalSessionOutboxStore", fixture.goalStore)
		}
		if pending, err := cleanupStore.ClaimGoalSessionCleanup(testutil.Context(t), 10); err != nil {
			t.Fatalf("ClaimGoalSessionCleanup(unsettled) error = %v", err)
		} else if len(pending) != 0 {
			t.Fatalf("unsettled cleanup = %#v, want hidden", pending)
		}
		close(releaseActivation)
		select {
		case bindErr := <-result:
			if bindErr == nil {
				t.Fatal("BindActionSession(after Stop) error = nil")
			}
		case <-time.After(time.Second):
			t.Fatal("BindActionSession did not converge after Stop")
		}

		relay := &goalSessionOutboxRelay{
			store: outboxStore, appender: fixture.manager,
			cleanupStore: cleanupStore, stopper: fixture.manager, now: time.Now,
		}
		if err := relay.DeliverPending(testutil.Context(t)); err != nil {
			t.Fatalf("DeliverPending() error = %v", err)
		}
		if _, active := fixture.manager.Get(request.DesiredSessionID); active {
			t.Fatalf("late-created session %q remained active", request.DesiredSessionID)
		}
		if pending, err := cleanupStore.ClaimGoalSessionCleanup(testutil.Context(t), 10); err != nil {
			t.Fatalf("ClaimGoalSessionCleanup(completed) error = %v", err)
		} else if len(pending) != 0 {
			t.Fatalf("completed cleanup = %#v, want none", pending)
		}
	})

	t.Run("Should carry one prompt through the real Manager slot and durable terminal await", func(t *testing.T) {
		fixture := newLoopGoalManagedRuntimeFixture(t, "terminal", nil)
		var reported atomic.Int64
		promptID := "goal-prompt-managed-runtime"
		ticket, err := fixture.preparePrompt(t, promptID, looppkg.ActionUsageReporterFunc(func(tokens int64) {
			reported.Store(tokens)
		}))
		if err != nil {
			t.Fatalf("PrepareActionPrompt() error = %v", err)
		}
		result, err := fixture.runtime.AwaitActionPrompt(testutil.Context(t), ticket)
		if err != nil {
			t.Fatalf("AwaitActionPrompt() error = %v", err)
		}
		if result.PromptID != promptID || result.Outcome != looppkg.ActionPromptOutcomeCompleted ||
			result.StopReason != looppkg.ActionStopEndTurn || result.Text != "reply" ||
			!result.TokensReported || result.TokensUsed != 3 || result.EventStartSeq < 1 ||
			result.EventEndSeq < result.EventStartSeq {
			t.Fatalf("managed prompt result = %#v", result)
		}
		if got := reported.Load(); got != 3 {
			t.Fatalf("reported action usage = %d, want 3", got)
		}
		checkpoint, err := fixture.goalStore.LoadCheckpoint(testutil.Context(t), fixture.key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if checkpoint.Phase != "judging" || checkpoint.TurnsUsed != 1 || checkpoint.PromptID != promptID {
			t.Fatalf("managed checkpoint = %#v", checkpoint)
		}
	})

	t.Run("Should atomically Stop an attached prompt and cancel its exact real Manager lease", func(t *testing.T) {
		driver := newHarnessIntegrationDriver()
		promptStarted := make(chan struct{})
		promptCanceled := make(chan struct{})
		driver.promptHook = func(
			ctx context.Context,
			proc *session.AgentProcess,
			req acp.PromptRequest,
		) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			close(promptStarted)
			go func() {
				defer close(events)
				<-ctx.Done()
				close(promptCanceled)
				events <- acp.AgentEvent{
					Type: acp.EventTypeDone, SessionID: proc.SessionID, TurnID: req.TurnID,
					Timestamp: time.Now().UTC(), StopReason: string(looppkg.ActionStopCancelled),
					PromptStopReason: acp.PromptStopReasonCancelled,
				}
			}()
			return events, nil
		}
		fixture := newLoopGoalManagedRuntimeFixture(t, "stop", driver)
		promptID := "goal-prompt-managed-stop"
		ticket, err := fixture.preparePrompt(t, promptID, nil)
		if err != nil {
			t.Fatalf("PrepareActionPrompt() error = %v", err)
		}
		select {
		case <-promptStarted:
		case <-time.After(time.Second):
			t.Fatal("managed driver prompt did not start")
		}

		aggregate, err := looppkg.NewService(
			fixture.goalStore,
			looppkg.DefinitionResolverFunc(func(
				context.Context,
				looppkg.WorkspaceID,
				string,
			) (*looppkg.ResolvedDefinition, error) {
				return nil, looppkg.ErrDefinitionNotFound
			}),
			managedTestGoalRunPolicyResolver(),
			looppkg.WithClock(time.Now),
			looppkg.WithGoalPromptLeaseRevoker(loopGoalPromptLeaseRevoker{sessions: fixture.manager}),
		)
		if err != nil {
			t.Fatalf("loop.NewService() error = %v", err)
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator-stop", taskpkg.OriginKindCLI, "cli")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		if err := aggregate.Stop(
			testutil.Context(t),
			fixture.run.WorkspaceID,
			fixture.run.ID,
			looppkg.StopReasonOperator,
			actor,
		); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		select {
		case <-promptCanceled:
		case <-time.After(time.Second):
			t.Fatal("managed driver prompt lease was not canceled")
		}
		result, err := fixture.runtime.AwaitActionPrompt(testutil.Context(t), ticket)
		if err != nil {
			t.Fatalf("AwaitActionPrompt(after Stop) error = %v", err)
		}
		if result.PromptID != promptID || result.Outcome != looppkg.ActionPromptOutcomeAmbiguous ||
			result.ReasonCode != looppkg.ReasonCodeGoalControlRevokedInFlight || result.StopReason != "" {
			t.Fatalf("stopped managed prompt result = %#v", result)
		}
		checkpoint, err := fixture.goalStore.LoadCheckpoint(testutil.Context(t), fixture.key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if checkpoint.ControlEpoch != 2 || checkpoint.Phase != "terminal" || checkpoint.Status != "paused" ||
			checkpoint.ControlActorKind != string(actor.Actor.Kind.Normalize()) ||
			checkpoint.ControlActorID != actor.Actor.Ref {
			t.Fatalf("stopped managed checkpoint = %#v", checkpoint)
		}
		run, err := fixture.goalStore.GetLoopRun(
			testutil.Context(t), fixture.run.WorkspaceID, fixture.run.ID,
		)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if run.Status != looppkg.StatusFailed {
			t.Fatalf("stopped managed Run status = %q, want failed", run.Status)
		}
	})

	t.Run("Should settle Pause through the real Manager and coordinator before one live Resume", func(t *testing.T) {
		driver := newHarnessIntegrationDriver()
		promptStarted := make(chan struct{})
		releasePrompt := make(chan struct{})
		driver.promptHook = func(
			_ context.Context,
			proc *session.AgentProcess,
			req acp.PromptRequest,
		) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			close(promptStarted)
			go func() {
				defer close(events)
				<-releasePrompt
				ts := time.Now().UTC()
				tokens := int64(4)
				events <- acp.AgentEvent{
					Type: acp.EventTypeAgentMessage, SessionID: proc.SessionID,
					TurnID: req.TurnID, Timestamp: ts, Text: "pause boundary reply",
				}
				events <- acp.AgentEvent{
					Type: acp.EventTypeDone, SessionID: proc.SessionID, TurnID: req.TurnID,
					Timestamp: ts, StopReason: string(looppkg.ActionStopEndTurn),
					PromptStopReason: acp.PromptStopReasonEndTurn,
					Usage:            &acp.TokenUsage{TurnID: req.TurnID, TotalTokens: &tokens, Timestamp: ts},
				}
			}()
			return events, nil
		}

		fixture := newLoopGoalCoordinatorRuntimeFixture(t, "pause-resume", driver)
		judge := loopGoalJudgeEvaluatorFunc(func(
			context.Context,
			goalpkg.JudgeRequest,
		) (goalpkg.JudgeResult, error) {
			t.Fatal("Goal judge ran after a pending Pause boundary")
			return goalpkg.JudgeResult{}, nil
		})
		executor, err := goalpkg.NewExecutor(goalpkg.Dependencies{
			Store: fixture.goalStore, Binder: fixture.runtime, Judge: judge,
			Budget: fixture.goalStore, Context: fixture.runtime, Recovery: fixture.runtime,
			Now: time.Now,
		})
		if err != nil {
			t.Fatalf("goal.NewExecutor() error = %v", err)
		}
		resultC := make(chan looppkg.ActionRawResult, 1)
		errC := make(chan error, 1)
		go func() {
			result, executeErr := executor.Execute(testutil.Context(t), fixture.node, looppkg.ActionExecutionInput{
				WorkspaceID: fixture.run.WorkspaceID, LoopRunID: fixture.run.ID,
				Generation: 1, NodeID: fixture.node.ID, ItemIndex: 0,
				Actor: fixture.actor, CorrelationID: fixture.workerClaim.Run.ID,
				WorkerModel: "worker-model", JudgeModel: "judge-model",
				CWD: fixture.workspaceRoot, GoalSegmentEpoch: 1,
				GoalContextNudgeRatio: new(fixture.run.GoalContextNudgeRatio),
			})
			if executeErr != nil {
				errC <- executeErr
				return
			}
			resultC <- result
		}()
		select {
		case <-promptStarted:
		case err := <-errC:
			t.Fatalf("Goal Execute() before prompt error = %v", err)
		case <-time.After(2 * time.Second):
			t.Fatal("managed Goal prompt did not start")
		}

		activationObserver := &activationDispatchObserver{}
		dispatcher, err := newTaskRunActivationDispatcher(fixture.taskStore, nil, activationObserver, nil)
		if err != nil {
			t.Fatalf("newTaskRunActivationDispatcher() error = %v", err)
		}
		state := &bootState{tasks: &taskRuntime{}, logger: discardLogger()}
		state.tasks.activation.Store(dispatcher)
		aggregate, err := looppkg.NewService(
			fixture.goalStore,
			looppkg.DefinitionResolverFunc(func(
				context.Context,
				looppkg.WorkspaceID,
				string,
			) (*looppkg.ResolvedDefinition, error) {
				return nil, looppkg.ErrDefinitionNotFound
			}),
			managedTestGoalRunPolicyResolver(),
			looppkg.WithClock(time.Now),
			looppkg.WithGoalRunActivator(loopGoalRunActivator{state: state}),
		)
		if err != nil {
			t.Fatalf("loop.NewService() error = %v", err)
		}
		pauseActor, err := taskpkg.DeriveHumanActorContext(
			"operator-pause", taskpkg.OriginKindCLI, "cli",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext(pause) error = %v", err)
		}
		if err := aggregate.Pause(
			testutil.Context(t), fixture.run.WorkspaceID, fixture.run.ID, pauseActor,
		); err != nil {
			t.Fatalf("Pause() error = %v", err)
		}
		close(releasePrompt)

		var raw looppkg.ActionRawResult
		select {
		case raw = <-resultC:
		case err := <-errC:
			t.Fatalf("Goal Execute() error = %v", err)
		case <-time.After(2 * time.Second):
			t.Fatal("Goal Execute() did not settle the paused prompt")
		}
		if raw.Control == nil || raw.Control.Disposition != looppkg.ActionDispositionPaused ||
			raw.Control.Cause != looppkg.ReasonCode(looppkg.TransitionCausePauseBoundary) ||
			raw.TokensUsed != 4 {
			t.Fatalf("paused Goal result = %#v", raw)
		}
		fixture.completeWorkerControl(t, raw)
		fixture.runNextCoordinator(t)

		pausedRun, err := fixture.goalStore.GetLoopRun(
			testutil.Context(t), fixture.run.WorkspaceID, fixture.run.ID,
		)
		if err != nil {
			t.Fatalf("GetLoopRun(paused) error = %v", err)
		}
		if pausedRun.Status != looppkg.StatusPaused || pausedRun.PauseRequested {
			t.Fatalf("paused Run = %#v", pausedRun)
		}
		fixture.assertAwaitingGoalWithoutWorker(t)

		resumeActor, err := taskpkg.DeriveHumanActorContext(
			"operator-resume", taskpkg.OriginKindCLI, "cli",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext(resume) error = %v", err)
		}
		if err := aggregate.Resume(
			testutil.Context(t), fixture.run.WorkspaceID, fixture.run.ID, resumeActor,
		); err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		if err := aggregate.Resume(
			testutil.Context(t), fixture.run.WorkspaceID, fixture.run.ID, resumeActor,
		); err != nil {
			t.Fatalf("Resume(idempotent) error = %v", err)
		}
		if got := activationObserver.runIDs(); len(got) != 1 {
			t.Fatalf("live Goal activations = %#v, want exactly one successor", got)
		}
		successor, err := fixture.taskStore.GetTaskRun(
			testutil.Context(t), activationObserver.runIDs()[0],
		)
		if err != nil {
			t.Fatalf("GetTaskRun(successor) error = %v", err)
		}
		var metadata loopGoalIntegrationRunMetadata
		if err := json.Unmarshal(successor.Metadata, &metadata); err != nil {
			t.Fatalf("decode successor metadata error = %v", err)
		}
		if successor.RunKind.Normalize() != taskpkg.RunKindWorker || !successor.IsLoopWorker() ||
			successor.LoopRunID != string(fixture.run.ID) || metadata.Generation != 1 ||
			metadata.NodeID != string(fixture.node.ID) || metadata.ItemIndex != 0 ||
			metadata.GoalSegmentEpoch != 2 {
			t.Fatalf("Goal successor = %#v metadata = %#v", successor, metadata)
		}
	})
}

func managedTestGoalRunPolicyResolver() looppkg.GoalRunPolicyResolver {
	return looppkg.GoalRunPolicyResolverFunc(func(
		context.Context,
		looppkg.WorkspaceID,
	) (*looppkg.GoalRunPolicy, error) {
		return &looppkg.GoalRunPolicy{ContextNudgeRatio: 0.8}, nil
	})
}

type loopGoalJudgeEvaluatorFunc func(context.Context, goalpkg.JudgeRequest) (goalpkg.JudgeResult, error)

type loopGoalIntegrationRunMetadata struct {
	Generation       int    `json:"generation"`
	NodeID           string `json:"node_id"`
	ItemIndex        int    `json:"item_index"`
	GoalSegmentEpoch int64  `json:"goal_segment_epoch"`
}

func (f loopGoalJudgeEvaluatorFunc) EvaluateGoal(
	ctx context.Context,
	req goalpkg.JudgeRequest,
) (goalpkg.JudgeResult, error) {
	return f(ctx, req)
}

type loopGoalCoordinatorRuntimeFixture struct {
	runtime       *loopGoalRuntime
	goalStore     loopGoalProductionStore
	taskStore     taskStore
	taskManager   *taskpkg.Service
	run           looppkg.Run
	node          loopdsl.Node
	actor         taskpkg.ActorContext
	workerClaim   *taskpkg.ClaimResult
	workspaceRoot string
}

func newLoopGoalCoordinatorRuntimeFixture(
	t *testing.T,
	suffix string,
	driver *harnessIntegrationDriver,
) loopGoalCoordinatorRuntimeFixture {
	t.Helper()
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	workspaceRoot := homePaths.HomeDir + "/workspace"
	resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
	daemonInstance, deps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Daemon.Shutdown() error = %v", err)
		}
	})

	goalStore, ok := daemonInstance.registry.(loopGoalProductionStore)
	if !ok {
		t.Fatalf("registry = %T, want loopGoalProductionStore", daemonInstance.registry)
	}
	tasks, ok := daemonInstance.registry.(taskStore)
	if !ok {
		t.Fatalf("registry = %T, want taskStore", daemonInstance.registry)
	}
	outputs, ok := daemonInstance.registry.(looppkg.GenerationOutputReader)
	if !ok {
		t.Fatalf("registry = %T, want GenerationOutputReader", daemonInstance.registry)
	}
	queueStore, ok := daemonInstance.registry.(store.SessionInputQueueStore)
	if !ok {
		t.Fatalf("registry = %T, want SessionInputQueueStore", daemonInstance.registry)
	}
	now := time.Now().UTC()
	if err := daemonInstance.registry.InsertWorkspace(testutil.Context(t), workspace.Workspace{
		ID: resolvedWorkspace.ID, Name: resolvedWorkspace.Name, RootDir: resolvedWorkspace.RootDir,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	manager := newHarnessIntegrationManager(
		t,
		homePaths,
		deps,
		resolvedWorkspace,
		driver,
		session.WithSessionCatalog(daemonInstance.registry),
		session.WithSessionCreationStore(goalStore),
		session.WithSessionInputQueueStore(queueStore),
	)
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Manager.Shutdown() error = %v", err)
		}
	})

	loopName := "goal-managed-coordinator-" + suffix
	handleLabel := "managed-" + suffix
	resolved := compileManagedGoalDefinition(t, loopName, resolvedWorkspace.Agents[0].Name, handleLabel)
	run := looppkg.Run{
		ID:          looppkg.RunID("looprun-goal-coordinator-" + suffix),
		WorkspaceID: looppkg.WorkspaceID(resolvedWorkspace.ID),
		LoopName:    loopName, Status: looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly, CreatedAt: now,
		StartedAt: now, LastProgressAt: now, Inputs: map[string]any{},
		IterationCap: 3, BudgetTokens: 100, BudgetWallSec: 60,
		BudgetOnExceeded: loopdsl.BudgetExceededHalt,
	}
	applyResolvedLoopRunPinningForTest(t, &run, now, resolved)
	if _, err := goalStore.CreateLoopRunForStart(
		testutil.Context(t), run, loopdsl.ConcurrencyAllow,
	); err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}

	coordinatorRunner, err := looppkg.NewCoordinatorRunner(
		tasks, goalStore, outputs, discardLogger(),
	)
	if err != nil {
		t.Fatalf("loop.NewCoordinatorRunner() error = %v", err)
	}
	taskManager, err := taskpkg.NewManager(
		taskpkg.WithStore(tasks),
		taskpkg.WithCoordinatorRunner(coordinatorRunner),
		taskpkg.WithGenerationStateFinalizer(looppkg.NewStoreFinalizer()),
		taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
			return looppkg.Status(status).Valid()
		}),
		taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
			return looppkg.Status(status).Terminal()
		}),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("loop-action", "daemon.loop-action")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	initialClaim := claimLoopRunForIntegration(
		t, taskManager, tasks, run, taskpkg.RunKindCoordinator, "", actor,
	)
	if _, err := taskManager.StartRun(
		testutil.Context(t),
		initialClaim.Run.ID,
		taskpkg.StartRun{
			IdempotencyKey: "start-" + initialClaim.Run.ID,
			ClaimToken:     initialClaim.ClaimToken,
		},
		actor,
	); err != nil {
		t.Fatalf("StartRun(initial coordinator) error = %v", err)
	}
	workerClaim := claimLoopRunForIntegration(
		t, taskManager, tasks, run, taskpkg.RunKindWorker, "", actor,
	)

	policyGate := &loopSessionPolicyGate{
		workspaceResolver: &harnessIntegrationWorkspaceResolver{resolved: resolvedWorkspace},
	}
	runtime := newLoopGoalProductionRuntime(
		goalStore,
		manager,
		homePaths.HomeDir,
		policyGate,
		time.Now,
		discardLogger(),
	)
	return loopGoalCoordinatorRuntimeFixture{
		runtime: runtime, goalStore: goalStore, taskStore: tasks, taskManager: taskManager,
		run: run, node: resolved.Definition.Graph.Nodes[0], actor: actor,
		workerClaim: workerClaim, workspaceRoot: workspaceRoot,
	}
}

func compileManagedGoalDefinition(
	t *testing.T,
	loopName string,
	agent string,
	handleLabel string,
) *looppkg.ResolvedDefinition {
	t.Helper()
	definition := loopdsl.Definition{
		APIVersion:  loopdsl.APIVersion,
		Kind:        loopdsl.KindLoop,
		Meta:        loopdsl.Meta{Name: loopName, Version: 1},
		Concurrency: loopdsl.ConcurrencyAllow,
		Contract: loopdsl.Contract{
			Goal: "Settle a managed Goal pause", DefinitionOfDone: "The Goal converges",
			IterationCap: 3, NoProgress: loopdsl.NoProgress{Window: 1},
			Budget: loopdsl.Budget{
				Tokens: 100, WallClockSec: 60, OnExceeded: loopdsl.BudgetExceededHalt,
			},
		},
		Graph: loopdsl.Graph{
			Nodes: []loopdsl.Node{{
				ID: "converge", Class: loopdsl.NodeClassAction, Kind: string(loopdsl.ActionGoal),
				Session: &loopdsl.SessionSpec{Handle: handleLabel, Mode: loopdsl.SessionModeContinuous},
				Params: loopdsl.NodeParams{
					"agent": agent, "objective": "Finish the durable objective",
					"judge": []any{map[string]any{
						"id": "done", "type": "agent-judge", "agent": "reviewer",
						"rubric": "Approve when the objective is complete",
					}},
					"max_turns": 3, "on_exhausted": loopdsl.GoalOnExhaustedHalt,
					"output_schema": map[string]any{
						"status": map[string]any{
							"type": "string", "enum": []any{"complete", "blocked"},
						},
					},
				},
			}},
			Edges: []loopdsl.Edge{},
		},
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		t.Fatalf("Compile(managed Goal definition) error = %v", err)
	}
	return resolved
}

func claimLoopRunForIntegration(
	t *testing.T,
	manager *taskpkg.Service,
	store taskStore,
	run looppkg.Run,
	kind taskpkg.RunKind,
	runID string,
	actor taskpkg.ActorContext,
) *taskpkg.ClaimResult {
	t.Helper()
	queued, err := store.ListTaskRunsByStatus(
		testutil.Context(t), []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued},
	)
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus() error = %v", err)
	}
	selectedID := runID
	for _, candidate := range queued {
		if selectedID == "" && candidate.LoopRunID == string(run.ID) &&
			candidate.RunKind.Normalize() == kind.Normalize() {
			selectedID = candidate.ID
			break
		}
	}
	if selectedID == "" {
		t.Fatalf("no queued %q Run for Loop %q", kind.Normalize(), run.ID)
	}
	claimedBy := actor.Actor
	claim, err := manager.ClaimNextRun(testutil.Context(t), taskpkg.ClaimCriteria{
		RunID: selectedID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: string(run.WorkspaceID),
		RunKind: kind, ClaimerSessionID: "integration-" + selectedID, ClaimedBy: &claimedBy,
		LeaseDuration: taskpkg.DefaultRunLeaseDuration, Now: time.Now().UTC(),
	}, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(%q) error = %v", kind.Normalize(), err)
	}
	return claim
}

func (f loopGoalCoordinatorRuntimeFixture) completeWorkerControl(
	t *testing.T,
	raw looppkg.ActionRawResult,
) {
	t.Helper()
	payload, err := json.Marshal(raw.Control)
	if err != nil {
		t.Fatalf("json.Marshal(ActionControl) error = %v", err)
	}
	completed, err := f.taskManager.CompleteRunLease(testutil.Context(t), taskpkg.LeaseCompletion{
		RunID: f.workerClaim.Run.ID, ClaimToken: f.workerClaim.ClaimToken,
		Result: taskpkg.RunResult{
			TokensUsed: raw.TokensUsed,
			CoordinatorControl: &taskpkg.CoordinatorControlResult{
				Kind: looppkg.CoordinatorControlKindLoopAction, Payload: payload,
			},
		},
		TokensUsed: raw.TokensUsed, Now: time.Now().UTC(),
	}, f.actor)
	if err != nil {
		t.Fatalf("CompleteRunLease(Goal control) error = %v", err)
	}
	if completed.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
		t.Fatalf("completed Goal worker status = %q", completed.Status)
	}
}

func (f loopGoalCoordinatorRuntimeFixture) runNextCoordinator(t *testing.T) {
	t.Helper()
	reconciler, ok := f.taskStore.(loopCoordinatorBootReconciler)
	if !ok {
		t.Fatalf("task store = %T, want loopCoordinatorBootReconciler", f.taskStore)
	}
	runs, err := reconciler.ReconcileLoopCoordinatorsOnBoot(
		testutil.Context(t), f.actor.Origin, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("ReconcileLoopCoordinatorsOnBoot() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("reconciled coordinators = %#v, want exactly one", runs)
	}
	claim := claimLoopRunForIntegration(
		t, f.taskManager, f.taskStore, f.run, taskpkg.RunKindCoordinator, runs[0].ID, f.actor,
	)
	if _, err := f.taskManager.StartRun(
		testutil.Context(t),
		claim.Run.ID,
		taskpkg.StartRun{
			IdempotencyKey: "start-" + claim.Run.ID,
			ClaimToken:     claim.ClaimToken,
		},
		f.actor,
	); err != nil {
		t.Fatalf("StartRun(settlement coordinator) error = %v", err)
	}
}

func (f loopGoalCoordinatorRuntimeFixture) assertAwaitingGoalWithoutWorker(t *testing.T) {
	t.Helper()
	outputs, ok := f.taskStore.(looppkg.GenerationOutputReader)
	if !ok {
		t.Fatalf("task store = %T, want GenerationOutputReader", f.taskStore)
	}
	generationOutputs, err := outputs.ListGenerationOutputs(testutil.Context(t), f.run.ID, 1)
	if err != nil {
		t.Fatalf("ListGenerationOutputs() error = %v", err)
	}
	if len(generationOutputs) != 1 ||
		generationOutputs[0].Status != looppkg.GenerationOutputStatusAwaitingGoal {
		t.Fatalf("paused generation outputs = %#v", generationOutputs)
	}
	active, err := f.taskStore.ListTaskRunsByStatus(testutil.Context(t), []taskpkg.RunStatus{
		taskpkg.TaskRunStatusQueued,
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning,
	})
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus(active) error = %v", err)
	}
	for _, candidate := range active {
		if candidate.LoopRunID == string(f.run.ID) && candidate.RunKind.Normalize() == taskpkg.RunKindWorker {
			t.Fatalf("paused Goal still has in-flight worker = %#v", candidate)
		}
	}
	checkpoint, err := f.goalStore.LoadCheckpoint(testutil.Context(t), goalpkg.TurnKey{
		WorkspaceID: f.run.WorkspaceID, LoopRunID: f.run.ID,
		Generation: 1, NodeID: f.node.ID, ItemIndex: 0,
	})
	if err != nil {
		t.Fatalf("LoadCheckpoint(paused) error = %v", err)
	}
	if checkpoint.Phase != "awaiting_control" || checkpoint.Status != "paused" ||
		checkpoint.TurnsUsed != 1 || checkpoint.ControlEpoch != 1 {
		t.Fatalf("paused Goal checkpoint = %#v", checkpoint)
	}
}

type loopGoalManagedRuntimeFixture struct {
	runtime       *loopGoalRuntime
	goalStore     loopGoalProductionStore
	manager       *session.Manager
	run           looppkg.Run
	key           goalpkg.TurnKey
	binding       looppkg.ActionSessionBinding
	taskRunID     string
	workspaceRoot string
	agentName     string
}

type loopGoalManagedRuntimeFixtureConfig struct {
	bindInitial bool
	decorate    func(*session.Manager) SessionManager
}

type loopGoalManagedRuntimeFixtureOption func(*loopGoalManagedRuntimeFixtureConfig)

func withoutInitialGoalBinding() loopGoalManagedRuntimeFixtureOption {
	return func(config *loopGoalManagedRuntimeFixtureConfig) { config.bindInitial = false }
}

func withGoalRuntimeSessionManager(
	decorate func(*session.Manager) SessionManager,
) loopGoalManagedRuntimeFixtureOption {
	return func(config *loopGoalManagedRuntimeFixtureConfig) { config.decorate = decorate }
}

type delayedEnsureCreatedManager struct {
	*session.Manager
	materialized chan struct{}
	release      <-chan struct{}
}

func (m *delayedEnsureCreatedManager) EnsureCreated(
	ctx context.Context,
	opts session.CreateOpts,
) (session.EnsureCreatedResult, error) {
	result, err := m.Manager.EnsureCreated(ctx, opts)
	close(m.materialized)
	select {
	case <-m.release:
		return result, err
	case <-ctx.Done():
		return session.EnsureCreatedResult{}, ctx.Err()
	}
}

func newLoopGoalManagedRuntimeFixture(
	t *testing.T,
	suffix string,
	driver *harnessIntegrationDriver,
	options ...loopGoalManagedRuntimeFixtureOption,
) loopGoalManagedRuntimeFixture {
	t.Helper()
	config := loopGoalManagedRuntimeFixtureConfig{bindInitial: true}
	for _, option := range options {
		option(&config)
	}
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	workspaceRoot := homePaths.HomeDir + "/workspace"
	resolvedWorkspace := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
	daemonInstance, deps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Daemon.Shutdown() error = %v", err)
		}
	})

	goalStore, ok := daemonInstance.registry.(loopGoalProductionStore)
	if !ok {
		t.Fatalf("registry = %T, want loopGoalProductionStore", daemonInstance.registry)
	}
	tasks, ok := daemonInstance.registry.(taskStore)
	if !ok {
		t.Fatalf("registry = %T, want taskStore", daemonInstance.registry)
	}
	queueStore, ok := daemonInstance.registry.(store.SessionInputQueueStore)
	if !ok {
		t.Fatalf("registry = %T, want SessionInputQueueStore", daemonInstance.registry)
	}
	now := time.Now().UTC()
	if err := daemonInstance.registry.InsertWorkspace(testutil.Context(t), workspace.Workspace{
		ID: resolvedWorkspace.ID, Name: resolvedWorkspace.Name, RootDir: resolvedWorkspace.RootDir,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	if driver == nil {
		driver = newHarnessIntegrationDriver()
	}
	manager := newHarnessIntegrationManager(
		t,
		homePaths,
		deps,
		resolvedWorkspace,
		driver,
		session.WithSessionCatalog(daemonInstance.registry),
		session.WithSessionCreationStore(goalStore),
		session.WithSessionInputQueueStore(queueStore),
	)
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Manager.Shutdown() error = %v", err)
		}
	})

	run := looppkg.Run{
		ID:          looppkg.RunID("looprun-goal-managed-" + suffix),
		WorkspaceID: looppkg.WorkspaceID(resolvedWorkspace.ID),
		LoopName:    "goal-managed", Status: looppkg.StatusRunning, Generation: 1,
		ReattemptStrategy: looppkg.ReattemptFailedOnly, CreatedAt: now, StartedAt: now,
		LastProgressAt: now, Inputs: map[string]any{},
	}
	applyLoopRunPinningForTest(t, &run, now)
	run.BudgetTokens = 100
	run.BudgetWallSec = 60
	if _, err := goalStore.CreateLoopRunForStart(
		testutil.Context(t), run, loopdsl.ConcurrencyAllow,
	); err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}

	actor := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "goal-managed-test"}
	origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "goal-managed-test"}
	taskRecord := taskpkg.Task{
		ID: "task-goal-managed-" + suffix, Scope: taskpkg.ScopeWorkspace,
		WorkspaceID: resolvedWorkspace.ID, Title: "Run managed Goal prompt",
		Status: taskpkg.TaskStatusInProgress, Priority: taskpkg.PriorityHigh,
		CreatedBy: actor, Origin: origin, CreatedAt: now, UpdatedAt: now,
	}
	if err := tasks.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	taskRunID := "taskrun-goal-managed-" + suffix
	if err := tasks.CreateTaskRun(testutil.Context(t), taskpkg.Run{
		ID: taskRunID, TaskID: taskRecord.ID, Attempt: 1, RunKind: taskpkg.RunKindWorker,
		LoopRunID: string(run.ID), Status: taskpkg.TaskRunStatusRunning,
		ClaimedBy: &actor, Origin: origin,
		Metadata: json.RawMessage(
			`{"generation":1,"node_id":"converge","item_index":0,"goal_segment_epoch":1}`,
		),
		QueuedAt: now, ClaimedAt: now, StartedAt: now,
	}); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	key := goalpkg.TurnKey{
		WorkspaceID: run.WorkspaceID, LoopRunID: run.ID, Generation: 1,
		NodeID: "converge", ItemIndex: 0,
	}
	if _, err := goalStore.CreateCheckpoint(testutil.Context(t), goalpkg.CreateCheckpointRequest{
		Checkpoint: goalpkg.Checkpoint{
			Key: key, ControlEpoch: 1, Phase: "idle", Status: "active", TurnLimit: 3,
			TaskRunID: taskRunID, ContextState: loopManagedContextStateUnknown,
			ContextNudgeRatio: 0.8, UpdatedAt: now,
		},
	}); err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	policyGate := &loopSessionPolicyGate{
		workspaceResolver: &harnessIntegrationWorkspaceResolver{resolved: resolvedWorkspace},
	}
	runtimeSessions := SessionManager(manager)
	if config.decorate != nil {
		runtimeSessions = config.decorate(manager)
	}
	runtime := newLoopGoalProductionRuntime(
		goalStore,
		runtimeSessions,
		homePaths.HomeDir,
		policyGate,
		time.Now,
		discardLogger(),
	)
	fixture := loopGoalManagedRuntimeFixture{
		runtime: runtime, goalStore: goalStore, manager: manager,
		run: run, key: key, taskRunID: taskRunID,
		workspaceRoot: resolvedWorkspace.RootDir, agentName: resolvedWorkspace.Agents[0].Name,
	}
	if config.bindInitial {
		binding, err := runtime.BindActionSession(testutil.Context(t), fixture.bindingRequest(suffix))
		if err != nil {
			t.Fatalf("BindActionSession() error = %v", err)
		}
		fixture.binding = binding
	}
	return fixture
}

func (f loopGoalManagedRuntimeFixture) bindingRequest(suffix string) looppkg.ActionSessionBindRequest {
	return looppkg.ActionSessionBindRequest{
		WorkspaceID: f.run.WorkspaceID, LoopRunID: f.run.ID, Generation: 1,
		NodeID: "converge", ItemIndex: 0, Agent: f.agentName,
		CWD: f.workspaceRoot, Handle: "goal:managed:" + suffix, Mode: "continuous",
		TargetBindingEpoch: 1, ExpectedControlEpoch: 1,
		ExpectedCheckpointPhase: "idle", ExpectedTaskRunID: f.taskRunID,
		BindingAttemptID: "binding-attempt-goal-managed-" + suffix,
		DesiredSessionID: "sess-goal-managed-" + suffix,
		MaxTurns:         3,
	}
}

func (f loopGoalManagedRuntimeFixture) createOriginSession(
	t *testing.T,
	suffix string,
	spec participation.Spec,
) (string, store.SessionCreationIdentity) {
	t.Helper()
	ctx := testutil.Context(t)
	request := f.bindingRequest("origin-" + suffix)
	request.NetworkParticipation = &spec
	profile, opts, err := f.runtime.resolveEffectiveCreationProfile(ctx, request, "")
	if err != nil {
		t.Fatalf("resolveEffectiveCreationProfile(origin) error = %v", err)
	}
	sessionID := "sess-goal-origin-" + suffix
	opts.DesiredSessionID = sessionID
	opts.NetworkOwnerKey = participation.OwnerKey(participation.OwnerRef{
		Kind: participation.OwnerKindSession,
		ID:   sessionID,
	})
	identity, err := bindingCreationIdentity(profile, opts, sessionID)
	if err != nil {
		t.Fatalf("bindingCreationIdentity(origin) error = %v", err)
	}
	opts.CreationProfile = cloneStoreCreationProfile(profile)
	opts.CreationIdentity = cloneStoreCreationIdentity(identity)
	if _, err := f.manager.EnsureCreated(ctx, opts); err != nil {
		t.Fatalf("EnsureCreated(origin) error = %v", err)
	}
	return sessionID, identity
}

func (f loopGoalManagedRuntimeFixture) preparePrompt(
	t *testing.T,
	promptID string,
	reporter looppkg.ActionUsageReporter,
) (looppkg.ActionPromptTicket, error) {
	t.Helper()
	return f.runtime.PrepareActionPrompt(testutil.Context(t), f.binding, looppkg.ActionPromptRequest{
		PromptID: promptID, Message: "Advance the managed Goal", Kind: "work",
		Owner: looppkg.ActionPromptOwner{
			LoopRunID: f.run.ID, TaskRunID: f.taskRunID, Generation: 1,
			NodeID: "converge", ItemIndex: 0, Turn: 1,
			ControlEpoch: 1, BindingEpoch: f.binding.BindingEpoch,
		},
		UsageReporter: reporter,
	})
}
