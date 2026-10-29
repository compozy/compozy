package daemon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	core "github.com/compozy/compozy/internal/api/core"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	mcppkg "github.com/compozy/compozy/internal/mcp"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	goalCommandCursorModel       = "grok-4.5[effort=high,fast=true]"
	goalCommandActiveCursorModel = "grok-4.5[effort=medium,fast=false]"
)

func TestSessionGoalDefinitionShouldKeepFreeFormClausesInsideAgentJudgeRubric(t *testing.T) {
	t.Parallel()

	t.Run("Should compile a canonical agent judge without creating a command criterion", func(t *testing.T) {
		t.Parallel()

		objective, err := session.ParseGoalObjective(
			"Ship the release\nverify: $(touch /tmp/never-run)\nconstraints: preserve audit history",
		)
		if err != nil {
			t.Fatalf("ParseGoalObjective() error = %v", err)
		}
		definition := buildSessionGoalDefinition(
			objective,
			"operator-agent",
			goalCommandCursorModel,
			7,
		)
		resolved, err := looppkg.NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile(synthetic Goal) error = %v", err)
		}
		if definition.Meta.Name != looppkg.InlineGoalLoopName ||
			definition.Concurrency != dsl.ConcurrencyAllow || len(resolved.Definition.Graph.Nodes) != 1 {
			t.Fatalf("synthetic definition = %#v", resolved.Definition)
		}
		var params dsl.GoalParams
		if err := resolved.Definition.Graph.Nodes[0].Params.Decode(&params); err != nil {
			t.Fatalf("Decode(GoalParams) error = %v", err)
		}
		if params.Agent != "operator-agent" || params.MaxTurns != 7 || len(params.Judge) != 1 {
			t.Fatalf("Goal params = %#v", params)
		}
		if definition.Contract.RuntimeDefaults == nil ||
			definition.Contract.RuntimeDefaults.Judge.Model != goalCommandCursorModel {
			t.Fatalf(
				"Goal judge model default = %#v, want %s",
				definition.Contract.RuntimeDefaults,
				goalCommandCursorModel,
			)
		}
		criterion := params.Judge[0]
		if criterion.ID != "objective_satisfied" || criterion.Type != dsl.CriterionAgentJudge ||
			criterion.Agent != "operator-agent" ||
			criterion.Check != "" || !strings.Contains(criterion.Rubric, "$(touch /tmp/never-run)") {
			t.Fatalf("Goal judge = %#v", criterion)
		}
	})
}

func TestDaemonGoalCommandHandlerShouldExecuteCanonicalSessionLifecycle(t *testing.T) {
	t.Parallel()

	// Invariant: a Clear that wins after Stop commits remains the authoritative result.
	// Owner: daemon command composition; canonical Goal lifecycle suite.
	t.Run("Should return cleared when Clear wins after Stop cancels the run", func(t *testing.T) {
		t.Parallel()
		fixture := newGoalCommandHandlerFixture(t)
		caller := session.PromptCaller{Kind: "human", ID: "operator", Source: "http"}
		started, err := fixture.service.Handle(t.Context(), fixture.workspaceID, fixture.sessionID,
			caller, session.GoalCommand{Verb: "set", Objective: "Preserve the winning clear"})
		if err != nil {
			t.Fatal(err)
		}
		actor, err := taskpkg.DeriveHumanActorContextForWorkspace("operator", fixture.workspaceID,
			taskpkg.OriginKindHTTP, "http")
		if err != nil {
			t.Fatal(err)
		}
		fixture.service.goalPersistence = clearCanceledGoalProjection{
			loopGoalAPIPersistence: fixture.service.goalPersistence,
			clear: func(ctx context.Context) error {
				return fixture.service.aggregate.ClearInlineGoal(ctx, looppkg.WorkspaceID(fixture.workspaceID),
					fixture.sessionID, actor)
			},
		}
		stopped, err := fixture.service.Handle(t.Context(), fixture.workspaceID, fixture.sessionID,
			caller, session.GoalCommand{Verb: "stop"})
		if err != nil {
			t.Fatal(err)
		}
		assertGoalCommandOutcome(t, stopped, session.GoalOutcomeCleared, "")
		if stopped.Result.Snapshot != nil {
			t.Fatal("concurrent Clear must not restore the hidden snapshot")
		}
		if _, err := core.GoalCommandResultPayloadFromSession(stopped.Result); err != nil {
			t.Fatalf("committed stop produced an invalid public result: %v", err)
		}
		if run := fixture.mustRun(t, started.Result.Snapshot.RunID); run.Status != looppkg.StatusCanceled {
			t.Fatalf("stopped run = %#v", run)
		}
	})

	// Invariant: unavailable usage telemetry never disables durable Goal controls.
	// Owner: daemon Goal read/control boundary; canonical Goal command suite.
	t.Run("Should retain status and controls when context event reads fail", func(t *testing.T) {
		t.Parallel()
		fixture := newGoalCommandHandlerFixture(t)
		caller := session.PromptCaller{Kind: "human", ID: "operator", Source: "http"}
		started, err := fixture.service.Handle(t.Context(), fixture.workspaceID, fixture.sessionID,
			caller, session.GoalCommand{Verb: "set", Objective: "Keep controls available"})
		if err != nil {
			t.Fatal(err)
		}
		fixture.service.goalPersistence = contextUnavailableGoalProjection{fixture.service.goalPersistence}
		fixture.service.goalContext = &loopGoalContextRuntime{sessions: failingGoalContextEvents{}}
		for _, verb := range []string{"status", "pause", "pause", "resume", "resume", "stop", "stop"} {
			result, err := fixture.service.Handle(t.Context(), fixture.workspaceID, fixture.sessionID,
				caller, session.GoalCommand{Verb: verb})
			if err != nil || result.Result == nil || result.Result.Outcome == session.GoalOutcomeError {
				t.Fatalf("%s unavailable: %#v %v", verb, result, err)
			}
			if result.Result.Snapshot == nil ||
				result.Result.Snapshot.Context.Diagnostic != "goal_context_unavailable" ||
				result.Result.Snapshot.Context.State != "unknown" {
				t.Fatalf("missing degraded context: %#v", result)
			}
		}
		if run := fixture.mustRun(t, started.Result.Snapshot.RunID); run.Status != looppkg.StatusCanceled {
			t.Fatalf("Stop did not cancel during telemetry outage: %#v", run)
		}
	})

	t.Run("Should start and project a Goal in User scope", func(t *testing.T) {
		t.Parallel()
		fixture := newGoalCommandHandlerFixtureWithRuntime(
			t,
			"cursor",
			goalCommandCursorModel,
			goalCommandActiveCursorModel,
			true,
		)
		objective := "Ship User work\nverify: make verify"
		result, err := fixture.service.Handle(
			t.Context(),
			"",
			fixture.sessionID,
			session.PromptCaller{Kind: "human", ID: "operator", Source: "http"},
			session.GoalCommand{Verb: session.GoalCommandVerbSet, Objective: objective},
		)
		if err != nil {
			t.Fatal(err)
		}
		assertGoalCommandOutcome(t, result, session.GoalOutcomeStarted, "")
		run := fixture.mustRun(t, result.Result.Snapshot.RunID)
		if run.WorkspaceID != "" || run.ProfileID != fixture.profileID {
			t.Fatalf("wrong Goal owner: %+v", run)
		}
	})

	// Invariant: terminal Run truth and live quarantine outrank stale active checkpoints.
	// Owner: daemon Goal snapshot projection; canonical Goal command suite.
	t.Run("Should project terminal runs and quarantined live work coherently", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			run         looppkg.Status
			quarantined bool
			want        string
		}{
			{looppkg.StatusCanceled, false, "paused"}, {looppkg.StatusFailed, false, "blocked"},
			{looppkg.StatusDone, false, "complete"}, {looppkg.StatusRunning, true, "blocked"},
			{looppkg.StatusRunning, false, "active"},
		} {
			projection := goalpkg.SessionProjection{RunStatus: test.run, Quarantined: test.quarantined,
				Checkpoint: &goalpkg.Checkpoint{Status: "active", TurnsUsed: 2, TurnLimit: 5}}
			snapshot := composeSessionGoalSnapshot(
				projection,
				dsl.GoalParams{},
				"goal",
				"objective",
				session.GoalContextSnapshot{},
			)
			if snapshot.Status != test.want || snapshot.Live != test.run.Live() || snapshot.TurnsUsed != 2 {
				t.Fatalf("snapshot = %#v for %#v", snapshot, test)
			}
			if test.quarantined && (snapshot.Cause == nil || *snapshot.Cause != "node_quarantined") {
				t.Fatal("quarantine has no actionable cause")
			}
		}
	})

	// Invariant: session Stop fences its Goal; removal cancels it without crossing scope boundaries.
	// Owner: daemon Goal lifecycle dispatcher; canonical session Goal command suite.
	t.Run("Should fence a stopped session quarantined Goal and cancel on removal", func(t *testing.T) {
		t.Parallel()
		fixture := newGoalCommandHandlerFixture(t)
		ctx := testutil.Context(t)
		started, err := fixture.service.Handle(ctx, fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: "human", ID: "operator", Source: "http"},
			session.GoalCommand{Verb: "set", Objective: "Retain history after stopping"})
		if err != nil {
			t.Fatal(err)
		}
		// Exercise the reported durable shape: a live Run whose Goal failed before
		// its first checkpoint and was quarantined in a later generation.
		db := fixture.db.(*globaldb.GlobalDB).DB()
		runID := started.Result.Snapshot.RunID
		if _, err := db.ExecContext(ctx, `UPDATE loop_runs SET generation = 2 WHERE id = ?`, runID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO loop_generation_outputs
			(loop_run_id, generation, node_id, status) VALUES (?, 2, 'goal', 'quarantined')`, runID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO loop_node_controls
			(loop_run_id, node_id, quarantined, quarantined_at, revision, updated_at)
			VALUES (?, 'goal', 1, ?, 1, ?)`, runID, time.Now().UTC(), time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		info := session.Info{ID: fixture.sessionID, WorkspaceID: fixture.workspaceID, ProfileID: fixture.profileID}
		for _, foreign := range []session.Info{
			{ID: info.ID, WorkspaceID: info.WorkspaceID, ProfileID: store.DefaultProfileID},
			{ID: info.ID, WorkspaceID: "foreign-workspace", ProfileID: info.ProfileID},
			{ID: "foreign-session", WorkspaceID: info.WorkspaceID, ProfileID: info.ProfileID},
		} {
			if err := fixture.service.StopSessionGoals(ctx, &foreign); err != nil {
				t.Fatal(err)
			}
			if !fixture.mustRun(t, started.Result.Snapshot.RunID).Status.Live() {
				t.Fatal("foreign session canceled the Goal")
			}
		}
		for range 2 {
			if err := fixture.service.StopSessionGoals(ctx, &info); err != nil {
				t.Fatal(err)
			}
		}
		run := fixture.mustRun(t, started.Result.Snapshot.RunID)
		if run.Status != looppkg.StatusPaused || run.PauseRequested {
			t.Fatalf("session Stop did not preserve a paused Goal intent: %#v", run)
		}
		if err := fixture.service.RemoveSessionGoals(ctx, &info); err != nil {
			t.Fatal(err)
		}
		run = fixture.mustRun(t, started.Result.Snapshot.RunID)
		if run.Status != looppkg.StatusCanceled {
			t.Fatalf("run after stop = %#v", run)
		}
		snapshot, err := fixture.service.GetSessionGoal(ctx, info.WorkspaceID, info.ID)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot == nil || snapshot.Live || snapshot.Status != "paused" || snapshot.RunStatus != "canceled" {
			t.Fatalf("canceled snapshot = %#v", snapshot)
		}
	})

	t.Run("Should preserve caller and origin identity across set replace controls and clear", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		ctx := testutil.Context(t)
		caller := session.PromptCaller{Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http"}

		started, err := fixture.service.Handle(ctx, fixture.workspaceID, fixture.sessionID, caller, session.GoalCommand{
			Verb:      "set",
			Objective: "Ship the release\nverify: make verify\nconstraints: preserve audit history",
		})
		if err != nil {
			t.Fatalf("Handle(set) error = %v", err)
		}
		assertGoalCommandOutcome(t, started, session.GoalOutcomeStarted, "")
		if started.Result.Snapshot == nil || started.Result.Snapshot.RunID != "run-goal-command-1" ||
			started.Result.Snapshot.TurnLimit != 7 || started.Result.Snapshot.Context.NudgeRatio != 0 {
			t.Fatalf("started Goal snapshot = %#v", started.Result.Snapshot)
		}
		first := fixture.mustRun(t, started.Result.Snapshot.RunID)
		if first.ProfileID != fixture.profileID || first.ProfileID == store.DefaultProfileID ||
			first.StartedBy.Kind != taskpkg.ActorKindHuman || first.StartedBy.Ref != "operator" ||
			first.StartedOrigin.Kind != taskpkg.OriginKindHTTP || first.Origin.CreationProfileRef != fixture.profileRef {
			t.Fatalf("started Goal identity = %#v", first)
		}
		fixture.assertPinnedGoalDefinition(t, first, goalCommandCursorModel)

		conflict, err := fixture.service.Handle(
			ctx,
			fixture.workspaceID,
			fixture.sessionID,
			caller,
			session.GoalCommand{
				Verb: "set", Objective: "Start a conflicting Goal",
			},
		)
		if err != nil {
			t.Fatalf("Handle(conflicting set) error = %v", err)
		}
		assertGoalCommandOutcome(t, conflict, session.GoalOutcomeError, session.GoalReasonReplaceRequired)
		if conflict.Result.Snapshot == nil || conflict.Result.Snapshot.RunID != string(first.ID) {
			t.Fatalf("replace-required snapshot = %#v", conflict.Result.Snapshot)
		}

		stale, err := fixture.service.Handle(ctx, fixture.workspaceID, fixture.sessionID, caller, session.GoalCommand{
			Verb: "replace", ExpectedRunID: "run-stale", Objective: "Ship a safer release",
		})
		if err != nil {
			t.Fatalf("Handle(stale replace) error = %v", err)
		}
		assertGoalCommandOutcome(t, stale, session.GoalOutcomeError, session.GoalReasonReplaceStale)
		if stale.Result.Snapshot == nil || stale.Result.Snapshot.RunID != string(first.ID) {
			t.Fatalf("stale replacement snapshot = %#v", stale.Result.Snapshot)
		}

		replaced, err := fixture.service.Handle(
			ctx,
			fixture.workspaceID,
			fixture.sessionID,
			caller,
			session.GoalCommand{
				Verb: "replace", ExpectedRunID: string(first.ID), Objective: "Ship a safer release",
			},
		)
		if err != nil {
			t.Fatalf("Handle(replace) error = %v", err)
		}
		assertGoalCommandOutcome(t, replaced, session.GoalOutcomeReplaced, "")
		if replaced.Result.ReplacedRunID == nil || *replaced.Result.ReplacedRunID != string(first.ID) ||
			replaced.Result.Snapshot == nil || replaced.Result.Snapshot.RunID != "run-goal-command-4" {
			t.Fatalf("replacement result = %#v", replaced.Result)
		}
		if replacedRun := fixture.mustRun(
			t,
			replaced.Result.Snapshot.RunID,
		); replacedRun.ProfileID != fixture.profileID {
			t.Fatalf("replaced Goal profile_id = %q, want %q", replacedRun.ProfileID, fixture.profileID)
		}

		paused, err := fixture.service.Handle(
			ctx,
			fixture.workspaceID,
			fixture.sessionID,
			caller,
			session.GoalCommand{Verb: "pause"},
		)
		if err != nil {
			t.Fatalf("Handle(pause) error = %v", err)
		}
		assertGoalCommandOutcome(t, paused, session.GoalOutcomePaused, "")
		if run := fixture.mustRun(
			t,
			replaced.Result.Snapshot.RunID,
		); run.Status != looppkg.StatusPaused ||
			run.PauseRequested {
			t.Fatalf("Goal before its first checkpoint must be effectively paused: %#v", run)
		}

		resumed, err := fixture.service.Handle(
			ctx,
			fixture.workspaceID,
			fixture.sessionID,
			caller,
			session.GoalCommand{Verb: "resume"},
		)
		if err != nil {
			t.Fatalf("Handle(resume) error = %v", err)
		}
		assertGoalCommandOutcome(t, resumed, session.GoalOutcomeResumed, "")
		if run := fixture.mustRun(t, replaced.Result.Snapshot.RunID); run.PauseRequested {
			t.Fatal("resumed Goal pause_requested = true")
		}

		status, err := fixture.service.Handle(
			ctx,
			fixture.workspaceID,
			fixture.sessionID,
			caller,
			session.GoalCommand{Verb: "status"},
		)
		if err != nil {
			t.Fatalf("Handle(status) error = %v", err)
		}
		assertGoalCommandOutcome(t, status, session.GoalOutcomeStatus, "")

		cleared, err := fixture.service.Handle(
			ctx,
			fixture.workspaceID,
			fixture.sessionID,
			caller,
			session.GoalCommand{Verb: "clear"},
		)
		if err != nil {
			t.Fatalf("Handle(clear) error = %v", err)
		}
		assertGoalCommandOutcome(t, cleared, session.GoalOutcomeCleared, "")
		if cleared.Result.Snapshot != nil {
			t.Fatalf("cleared Goal snapshot = %#v, want nil", cleared.Result.Snapshot)
		}

		missing, err := fixture.service.Handle(
			ctx,
			fixture.workspaceID,
			fixture.sessionID,
			caller,
			session.GoalCommand{Verb: "status"},
		)
		if err != nil {
			t.Fatalf("Handle(status after clear) error = %v", err)
		}
		assertGoalCommandOutcome(t, missing, session.GoalOutcomeError, session.GoalReasonNotActive)
	})

	// Invariant: Goal commands that create or clear a Goal advance the tool projection epoch, so
	// epoch-cached goal_get availability flips at the transition and is reused between them.
	// Owner: daemon Goal command dispatcher; canonical session Goal command suite.
	t.Run("Should flip cached goal_get availability as set and clear advance the epoch", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		ctx := testutil.Context(t)
		caller := session.PromptCaller{Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http"}
		epoch := mcppkg.NewProjectionEpoch()
		fixture.service.goalToolsChanged = func() { epoch.Advance() }
		native := &daemonNativeTools{
			deps:             &daemonNativeToolsDeps{Loops: func() core.LoopService { return fixture.service }},
			goalAvailability: newNativeGoalAvailabilityCache(epoch.Current, nil),
		}
		scope := toolspkg.Scope{WorkspaceID: fixture.workspaceID, SessionID: fixture.sessionID}
		requireGoalGet := func(label string, wantAvailable bool) {
			t.Helper()
			availability := native.goalGetAvailability(ctx, scope)
			if availability.Available != wantAvailable {
				t.Fatalf("goal_get availability %s = %#v, want available=%v", label, availability, wantAvailable)
			}
			if !wantAvailable && !slices.Contains(availability.ReasonCodes, toolspkg.ReasonGoalNotActive) {
				t.Fatalf("goal_get availability %s reasons = %v, want goal_not_active", label, availability.ReasonCodes)
			}
		}
		handle := func(verb string, objective string) session.GoalDispatchDecision {
			t.Helper()
			command := session.GoalCommand{Verb: verb, Objective: objective}
			decision, err := fixture.service.Handle(ctx, fixture.workspaceID, fixture.sessionID, caller, command)
			if err != nil {
				t.Fatalf("Handle(%s) error = %v", verb, err)
			}
			return decision
		}

		requireGoalGet("before set", false)
		beforeSet := epoch.Current()
		assertGoalCommandOutcome(t, handle("set", "Ship the release"), session.GoalOutcomeStarted, "")
		if epoch.Current() == beforeSet {
			t.Fatal("Handle(set) did not advance the tool projection epoch")
		}
		requireGoalGet("after set", true)

		beforeStatus := epoch.Current()
		assertGoalCommandOutcome(t, handle("status", ""), session.GoalOutcomeStatus, "")
		if epoch.Current() != beforeStatus {
			t.Fatal("Handle(status) advanced the tool projection epoch for a read-only command")
		}
		requireGoalGet("after status", true)

		assertGoalCommandOutcome(t, handle("clear", ""), session.GoalOutcomeCleared, "")
		requireGoalGet("after clear", false)
	})

	t.Run("Should pin the reconciled active model for a provider-native default session", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixtureWithRuntime(t, "cursor", "", goalCommandCursorModel)
		decision, err := fixture.service.Handle(
			testutil.Context(t),
			fixture.workspaceID,
			fixture.sessionID,
			session.PromptCaller{
				Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http",
			},
			session.GoalCommand{
				Verb:      "set",
				Objective: "Ship with the provider-native model\nverify: make verify",
			},
		)
		if err != nil {
			t.Fatalf("Handle(set provider-native model) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeStarted, "")
		if decision.Result.Snapshot == nil {
			t.Fatal("provider-native Goal snapshot = nil")
		}
		fixture.assertPinnedGoalDefinition(
			t,
			fixture.mustRun(t, decision.Result.Snapshot.RunID),
			goalCommandCursorModel,
		)
	})

	t.Run("Should rewrite draft into the ordinary idle-only prompt path", func(t *testing.T) {
		t.Parallel()

		service := &daemonLoopAPIService{aggregate: &loopApprovalAggregateStub{}}
		decision, err := service.Handle(
			t.Context(),
			"ws-1",
			"session-1",
			session.PromptCaller{Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "uds"},
			session.GoalCommand{Verb: "draft", Objective: "make this concrete"},
		)
		if err != nil {
			t.Fatalf("Handle(draft) error = %v", err)
		}
		if decision.Kind != session.GoalDispatchPrompt || !decision.BypassGoalParse ||
			decision.BusyPolicy != "reject-if-busy" || decision.BusyReason != session.GoalReasonDraftRequiresIdle ||
			!strings.Contains(decision.RewrittenMessage, "make this concrete") {
			t.Fatalf("draft decision = %#v", decision)
		}
	})

	t.Run("Should authorize an agent draft before rewriting it", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		status := fixture.service.sessionStatus.(*goalCommandSessionStatus)
		status.callers["other-agent"] = &session.Info{ID: "other-agent", WorkspaceID: fixture.workspaceID}
		decision, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "other-agent", Source: "http"},
			session.GoalCommand{Verb: session.GoalCommandVerbDraft, Objective: "do not rewrite this"},
		)
		if err != nil {
			t.Fatalf("Handle(unauthorized draft) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeError, session.GoalReasonCallerUnauthorized)
	})

	t.Run("Should propagate caller status store failures instead of returning unauthorized", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		status := fixture.service.sessionStatus.(*goalCommandSessionStatus)
		statusErr := errors.New("session status store unavailable")
		status.statusErrs = map[string]error{fixture.sessionID: statusErr}
		decision, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "agent-operator", Source: "http"},
			session.GoalCommand{Verb: session.GoalCommandVerbSet, Objective: "surface the store outage"},
		)
		if !errors.Is(err, statusErr) {
			t.Fatalf("Handle(status store failure) error = %v, want wrapped status error", err)
		}
		if decision != (session.GoalDispatchDecision{}) {
			t.Fatalf("Handle(status store failure) decision = %#v, want zero decision", decision)
		}
	})

	t.Run("Should apply a per-run worker runtime", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		decision, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http"},
			session.GoalCommand{
				Verb: session.GoalCommandVerbSet, Objective: "Ship with the selected worker",
				Runtime: &session.RuntimeSelection{
					Provider: "cursor", Model: "grok-4.5", ReasoningEffort: "high", Speed: speedpkg.SpeedFast,
					ACPOptions: []acp.SessionConfigOptionSelection{{ID: "thinking", BoolValue: new(true)}},
				},
			},
		)
		if err != nil {
			t.Fatalf("Handle(set runtime) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeStarted, "")
		run := fixture.mustRun(t, decision.Result.Snapshot.RunID)

		snapshot, err := fixture.db.GetLoopDefinitionSnapshot(
			testutil.Context(t),
			run.WorkspaceID, "", run.DefinitionDigest,
		)
		if err != nil {
			t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
		}
		resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		worker := resolved.EffectiveConfig.RuntimeDefaults.Worker
		if worker.Provider != "cursor" || worker.Model != "grok-4.5" || worker.Reasoning != "high" ||
			worker.Speed != speedpkg.SpeedFast || len(worker.ACPOptions) != 1 ||
			worker.ACPOptions[0].ID != "thinking" || worker.ACPOptions[0].BoolValue == nil ||
			!*worker.ACPOptions[0].BoolValue {
			t.Fatalf("run worker runtime = %#v, want selected runtime", worker)
		}
	})

	t.Run("Should resolve a provider-only runtime against the selected provider", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		decision, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http"},
			session.GoalCommand{
				Verb: session.GoalCommandVerbSet, Objective: "Switch providers without inheriting the origin model",
				Runtime: &session.RuntimeSelection{Provider: "openrouter", Speed: speedpkg.SpeedNormal},
			},
		)
		if err != nil {
			t.Fatalf("Handle(provider-only runtime) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeStarted, "")
		run := fixture.mustRun(t, decision.Result.Snapshot.RunID)
		definition, err := fixture.db.GetLoopDefinitionSnapshot(
			testutil.Context(t), run.WorkspaceID, "", run.DefinitionDigest,
		)
		if err != nil {
			t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
		}
		resolved, err := looppkg.LoadExecutedDefinitionSnapshot(definition.Definition, run.DefinitionDigest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		worker := resolved.EffectiveConfig.RuntimeDefaults.Worker
		if worker.Provider != "openrouter" || worker.Model == "" || worker.Model == goalCommandCursorModel {
			t.Fatalf("provider-only Goal runtime = %#v, want openrouter default model", worker)
		}
	})

	t.Run("Should reject an unknown Goal provider after cross-provider binding", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixtureWithRuntime(t, "codex", "gpt-6.1-sol", "gpt-6.1-sol")
		status := fixture.service.sessionStatus.(*goalCommandSessionStatus)
		status.info.Provider = "claude"
		status.info.Model = "claude-opus-5-5"
		decision, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http"},
			session.GoalCommand{
				Verb: session.GoalCommandVerbSet, Objective: "Reject an unknown Goal runtime provider",
				Runtime: &session.RuntimeSelection{Provider: "missing-provider", Model: "missing-model"},
			},
		)
		if err != nil {
			t.Fatalf("Handle(unknown provider) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeError, session.GoalReasonRuntimeInvalid)
		_, err = fixture.db.GetLoopRun(t.Context(), looppkg.WorkspaceID(fixture.workspaceID), "run-goal-command-1")
		if !errors.Is(err, looppkg.ErrRunNotFound) {
			t.Fatalf("rejected runtime created a Run: %v", err)
		}
	})

	t.Run("Should start and replace an agent-created Goal after a cross-provider runtime bind", func(t *testing.T) {
		t.Parallel()
		fixture := newGoalCommandHandlerFixtureWithRuntime(t, "codex", "gpt-6.1-sol", "gpt-6.1-sol")
		status := fixture.service.sessionStatus.(*goalCommandSessionStatus)
		status.callers["agent-operator"].Provider = "codex"
		status.info.Provider = "claude"
		status.info.Model = "claude-opus-5-5"
		status.info.ReasoningEffort = "xhigh"
		ctx := t.Context()
		before, err := fixture.db.GetSessionCreationIdentity(ctx, fixture.sessionID)
		if err != nil {
			t.Fatal(err)
		}
		caller := session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "agent-operator", Source: "uds"}
		command := session.GoalCommand{Verb: session.GoalCommandVerbSet, Objective: "Ship with a rebound runtime",
			Runtime: &session.RuntimeSelection{Provider: "claude", Model: "claude-opus-5-5", ReasoningEffort: "xhigh"}}
		for _, verb := range []string{session.GoalCommandVerbSet, session.GoalCommandVerbReplace} {
			command.Verb = verb
			decision, err := fixture.service.Handle(ctx, fixture.workspaceID, fixture.sessionID, caller, command)
			if err != nil {
				t.Fatal(err)
			}
			want := session.GoalOutcomeStarted
			if verb == session.GoalCommandVerbReplace {
				want = session.GoalOutcomeReplaced
			}
			assertGoalCommandOutcome(t, decision, want, "")
			run := fixture.mustRun(t, decision.Result.Snapshot.RunID)
			snapshot, err := fixture.db.GetLoopDefinitionSnapshot(ctx, run.WorkspaceID, "", run.DefinitionDigest)
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
			if err != nil {
				t.Fatal(err)
			}
			worker := resolved.EffectiveConfig.RuntimeDefaults.Worker
			if worker.Provider != "claude" || worker.Model != "claude-opus-5-5" || worker.Reasoning != "xhigh" {
				t.Fatalf("Goal worker runtime = %#v, want Claude Opus 5.5 xhigh", worker)
			}
			if run.Origin.CreationProfileRef != before.CreationProfileRef ||
				run.Origin.PolicySpecDigest != before.PolicySpecDigest || run.Origin.CreationDigest != before.CreationDigest {
				t.Fatalf("Goal changed immutable creation identity: %#v", run.Origin)
			}
			command.ExpectedRunID = decision.Result.Snapshot.RunID
		}
		after, err := fixture.db.GetSessionCreationIdentity(ctx, fixture.sessionID)
		if err != nil || after != before {
			t.Fatalf("creation identity changed: before=%#v after=%#v err=%v", before, after, err)
		}
	})

	t.Run("Should reject invalid origin sessions before creating a Run", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			mutate     func(*goalCommandSessionStatus)
			wantReason session.GoalReasonCode
			wantCheck  string
		}{
			{
				name: "Should reject a stopped origin",
				mutate: func(status *goalCommandSessionStatus) {
					status.info.State = session.StateStopped
				},
				wantReason: session.GoalReasonCode(looppkg.ReasonCodeGoalOriginInvalid),
			},
			{
				name: "Should reject a foreign-workspace origin",
				mutate: func(status *goalCommandSessionStatus) {
					status.info.WorkspaceID = "ws-foreign"
				},
				wantReason: session.GoalReasonCode(looppkg.ReasonCodeGoalOriginWorkspaceMismatch),
			},
			{
				name:      "Should reject an origin without a persisted creation profile",
				wantCheck: "creation_identity_missing",
				mutate: func(status *goalCommandSessionStatus) {
					status.info.ID = "session-missing-creation-profile"
				},
				wantReason: session.GoalReasonCode(looppkg.ReasonCodeGoalOriginProfileUnavailable),
			},
			{
				name:      "Should reject an origin whose active agent differs from its immutable profile",
				wantCheck: "creation_agent_mismatch",
				mutate: func(status *goalCommandSessionStatus) {
					status.info.AgentName = "different-agent"
				},
				wantReason: session.GoalReasonCode(looppkg.ReasonCodeGoalOriginProfileUnavailable),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				fixture := newGoalCommandHandlerFixture(t)
				status, ok := fixture.service.sessionStatus.(*goalCommandSessionStatus)
				if !ok {
					t.Fatalf("session status type = %T, want *goalCommandSessionStatus", fixture.service.sessionStatus)
				}
				var logs bytes.Buffer
				fixture.service.logger = slog.New(slog.NewJSONHandler(&logs, nil))
				tt.mutate(status)
				decision, err := fixture.service.Handle(
					testutil.Context(t),
					fixture.workspaceID,
					fixture.sessionID,
					session.PromptCaller{
						Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http",
					},
					session.GoalCommand{Verb: "set", Objective: "Ship without an invalid origin"},
				)
				if err != nil {
					t.Fatalf("Handle(set invalid origin) error = %v", err)
				}
				assertGoalCommandOutcome(t, decision, session.GoalOutcomeError, tt.wantReason)
				if tt.wantCheck != "" && !strings.Contains(logs.String(), `"check":"`+tt.wantCheck+`"`) {
					t.Fatalf("origin diagnostic = %s, want check %q", logs.String(), tt.wantCheck)
				}
				if _, err := fixture.db.GetLoopRun(
					testutil.Context(t),
					looppkg.WorkspaceID(fixture.workspaceID),
					"run-goal-command-1",
				); !errors.Is(err, looppkg.ErrRunNotFound) {
					t.Fatalf("GetLoopRun(after rejected origin) error = %v, want ErrRunNotFound", err)
				}
			})
		}
	})

	t.Run("Should map an authenticated agent caller into the durable Run actor", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		decision, err := fixture.service.Handle(
			testutil.Context(t),
			fixture.workspaceID,
			fixture.sessionID,
			session.PromptCaller{
				Kind: string(taskpkg.ActorKindAgentSession), ID: "agent-operator", Source: "uds",
			},
			session.GoalCommand{Verb: "set", Objective: "Ship through an agent-operated surface"},
		)
		if err != nil {
			t.Fatalf("Handle(agent set) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeStarted, "")
		if decision.Result.Snapshot == nil {
			t.Fatal("agent-started Goal snapshot = nil")
		}
		run := fixture.mustRun(t, decision.Result.Snapshot.RunID)
		if run.StartedBy.Kind != taskpkg.ActorKindAgentSession || run.StartedBy.Ref != "agent-operator" ||
			run.StartedOrigin.Kind != taskpkg.OriginKindUDS {
			t.Fatalf("agent-started Run identity = %#v", run)
		}
	})

	t.Run("Should reject an agent caller that is not an ancestor of the target session", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		status, ok := fixture.service.sessionStatus.(*goalCommandSessionStatus)
		if !ok {
			t.Fatalf("session status type = %T, want *goalCommandSessionStatus", fixture.service.sessionStatus)
		}
		status.info.Lineage = &store.SessionLineage{ParentSessionID: "different-agent"}
		status.callers["different-agent"] = &session.Info{
			ID: "different-agent", WorkspaceID: fixture.workspaceID,
		}
		decision, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "agent-operator", Source: "uds"},
			session.GoalCommand{Verb: session.GoalCommandVerbSet, Objective: "Reject this cross-tree Goal"},
		)
		if err != nil {
			t.Fatalf("Handle(unauthorized agent) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeError, session.GoalReasonCallerUnauthorized)
		if _, err := fixture.db.GetLoopRun(
			testutil.Context(t), looppkg.WorkspaceID(fixture.workspaceID), "run-goal-command-1",
		); !errors.Is(err, looppkg.ErrRunNotFound) {
			t.Fatalf("GetLoopRun(after unauthorized agent) error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("Should reject foreign and cyclic agent lineage before Goal creation", func(t *testing.T) {
		t.Parallel()

		t.Run("Should reject a foreign workspace intermediary", func(t *testing.T) {
			t.Parallel()
			fixture := newGoalCommandHandlerFixture(t)
			status := fixture.service.sessionStatus.(*goalCommandSessionStatus)
			status.info.Lineage = &store.SessionLineage{ParentSessionID: "foreign-intermediate"}
			status.callers["foreign-intermediate"] = &session.Info{
				ID: "foreign-intermediate", WorkspaceID: "ws-foreign",
			}
			decision, err := fixture.service.Handle(
				testutil.Context(t), fixture.workspaceID, fixture.sessionID,
				session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "agent-operator", Source: "uds"},
				session.GoalCommand{Verb: session.GoalCommandVerbSet, Objective: "reject foreign lineage"},
			)
			if err != nil {
				t.Fatalf("Handle(foreign intermediary) error = %v", err)
			}
			assertGoalCommandOutcome(t, decision, session.GoalOutcomeError, session.GoalReasonCallerUnauthorized)
		})

		t.Run("Should reject a session continued from the caller", func(t *testing.T) {
			t.Parallel()
			fixture := newGoalCommandHandlerFixture(t)
			status := fixture.service.sessionStatus.(*goalCommandSessionStatus)
			status.info.Lineage = &store.SessionLineage{
				ParentSessionID: "agent-operator", Kind: store.LineageKindContinue, OriginAgentName: "coder",
			}
			decision, err := fixture.service.Handle(
				testutil.Context(t), fixture.workspaceID, fixture.sessionID,
				session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "agent-operator", Source: "uds"},
				session.GoalCommand{Verb: session.GoalCommandVerbSet, Objective: "reject a continued session"},
			)
			if err != nil {
				t.Fatalf("Handle(continued session) error = %v", err)
			}
			assertGoalCommandOutcome(t, decision, session.GoalOutcomeError, session.GoalReasonCallerUnauthorized)
		})

		t.Run("Should reject a cyclic lineage", func(t *testing.T) {
			t.Parallel()
			fixture := newGoalCommandHandlerFixture(t)
			status := fixture.service.sessionStatus.(*goalCommandSessionStatus)
			status.info.Lineage = &store.SessionLineage{ParentSessionID: "cyclic-intermediate"}
			status.callers["cyclic-intermediate"] = &session.Info{
				ID: "cyclic-intermediate", WorkspaceID: fixture.workspaceID,
				Lineage: &store.SessionLineage{ParentSessionID: fixture.sessionID},
			}
			decision, err := fixture.service.Handle(
				testutil.Context(t), fixture.workspaceID, fixture.sessionID,
				session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "agent-operator", Source: "uds"},
				session.GoalCommand{Verb: session.GoalCommandVerbSet, Objective: "reject cyclic lineage"},
			)
			if err != nil {
				t.Fatalf("Handle(cyclic lineage) error = %v", err)
			}
			assertGoalCommandOutcome(t, decision, session.GoalOutcomeError, session.GoalReasonCallerUnauthorized)
		})
	})

	t.Run("Should isolate eight concurrent child Goals by origin runtime", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		status, ok := fixture.service.sessionStatus.(*goalCommandSessionStatus)
		if !ok {
			t.Fatalf("session status type = %T, want *goalCommandSessionStatus", fixture.service.sessionStatus)
		}
		profile, err := fixture.db.GetSessionCreationProfile(testutil.Context(t), fixture.profileRef)
		if err != nil {
			t.Fatalf("GetSessionCreationProfile() error = %v", err)
		}
		policyDigest, err := profile.PolicySpecDigest()
		if err != nil {
			t.Fatalf("PolicySpecDigest() error = %v", err)
		}
		status.sessions = make(map[string]*session.Info)
		parent := *status.info
		parentID := fixture.sessionID
		status.sessions[parentID] = &parent
		for index := range 8 {
			childID := "session-goal-child-" + string(rune('a'+index))

			child := &session.Info{
				ID:          childID,
				ProfileID:   parent.ProfileID,
				AgentName:   parent.AgentName,
				Provider:    parent.Provider,
				Model:       parent.Model,
				WorkspaceID: parent.WorkspaceID,

				Lineage: &store.SessionLineage{
					ParentSessionID: parentID,
				},
				Type:  parent.Type,
				State: session.StateActive,
			}
			status.sessions[childID] = child
			creationDigest, err := profile.CreationDigest(store.SessionCreationOptions{
				SessionID:   childID,
				SessionType: string(session.SessionTypeUser),
			})
			if err != nil {
				t.Fatalf("CreationDigest(%q) error = %v", childID, err)
			}
			identity := store.SessionCreationIdentity{
				CreationProfileRef: fixture.profileRef,
				PolicySpecDigest:   policyDigest,
				CreationDigest:     creationDigest,
			}
			if _, err := fixture.db.RegisterSessionWithCreationIdentity(testutil.Context(t), store.SessionInfo{
				ProfileID:   child.ProfileID,
				ID:          childID,
				AgentName:   child.AgentName,
				Provider:    child.Provider,
				WorkspaceID: child.WorkspaceID,

				SessionType: string(
					child.Type,
				),
				State:         string(child.State),
				RuntimeStatus: store.SessionRuntimeUnbound,
				CreatedAt:     parent.CreatedAt,
				UpdatedAt:     parent.UpdatedAt,
			}, identity); err != nil {
				t.Fatalf("RegisterSessionWithCreationIdentity(%q) error = %v", childID, err)
			}
		}

		type childResult struct {
			childID  string
			decision session.GoalDispatchDecision
			err      error
		}
		results := make(chan childResult, 8)
		for index := range 8 {
			childID := "session-goal-child-" + string(rune('a'+index))
			go func(childID string, index int) {
				decision, err := fixture.service.Handle(
					testutil.Context(t), fixture.workspaceID, childID,
					session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: parentID, Source: "uds"},
					session.GoalCommand{
						Verb: session.GoalCommandVerbSet, Objective: "Ship child " + childID,
						Runtime: &session.RuntimeSelection{
							Provider: "cursor", Model: "child-model-" + string(rune('a'+index)),
							ReasoningEffort: "high", Speed: speedpkg.SpeedFast,
						},
					},
				)
				results <- childResult{childID: childID, decision: decision, err: err}
			}(childID, index)
		}

		runIDs := make(map[string]struct{}, 8)
		for range 8 {
			result := <-results
			if result.err != nil {
				t.Fatalf("Handle(%q) error = %v", result.childID, result.err)
			}
			assertGoalCommandOutcome(t, result.decision, session.GoalOutcomeStarted, "")
			snapshot := result.decision.Result.Snapshot
			if snapshot == nil || snapshot.OriginSessionID != result.childID ||
				snapshot.BoundSessionID != result.childID {
				t.Fatalf("Goal snapshot for %q = %#v, want isolated origin/binding", result.childID, snapshot)
			}
			if _, exists := runIDs[snapshot.RunID]; exists {
				t.Fatalf("duplicate child Goal run ID %q", snapshot.RunID)
			}
			runIDs[snapshot.RunID] = struct{}{}
			run := fixture.mustRun(t, snapshot.RunID)
			if run.Origin.SessionID != result.childID || run.StartedBy.Ref != parentID {
				t.Fatalf(
					"child Goal run %q identity = %#v, want origin %q and actor %q",
					snapshot.RunID,
					run.Origin,
					result.childID,
					parentID,
				)
			}

			definition, err := fixture.db.GetLoopDefinitionSnapshot(
				testutil.Context(t),
				run.WorkspaceID, "", run.DefinitionDigest,
			)
			if err != nil {
				t.Fatalf("GetLoopDefinitionSnapshot(%q) error = %v", result.childID, err)
			}
			resolved, err := looppkg.LoadExecutedDefinitionSnapshot(definition.Definition, run.DefinitionDigest)
			if err != nil {
				t.Fatalf("LoadExecutedDefinitionSnapshot(%q) error = %v", result.childID, err)
			}
			worker := resolved.EffectiveConfig.RuntimeDefaults.Worker
			wantModel := "child-model-" + string(result.childID[len(result.childID)-1])
			if worker.Provider != "cursor" || worker.Model != wantModel || worker.Reasoning != "high" ||
				worker.Speed != speedpkg.SpeedFast {
				t.Fatalf("child Goal run %q runtime = %#v, want cursor/%s/high/fast", snapshot.RunID, worker, wantModel)
			}
		}

		sibling, err := fixture.service.Handle(
			testutil.Context(t),
			fixture.workspaceID,
			"session-goal-child-b",
			session.PromptCaller{
				Kind:   string(taskpkg.ActorKindAgentSession),
				ID:     "session-goal-child-a",
				Source: "uds",
			},
			session.GoalCommand{Verb: session.GoalCommandVerbStatus},
		)
		if err != nil {
			t.Fatalf("Handle(sibling status) error = %v", err)
		}
		assertGoalCommandOutcome(t, sibling, session.GoalOutcomeError, session.GoalReasonCallerUnauthorized)
	})
}

type goalCommandHandlerFixture struct {
	service     *daemonLoopAPIService
	db          loopGoalProductionStore
	workspaceID string
	sessionID   string
	profileRef  string
	profileID   string
}

func newGoalCommandHandlerFixture(t *testing.T) goalCommandHandlerFixture {
	t.Helper()
	return newGoalCommandHandlerFixtureWithRuntime(
		t,
		"cursor",
		goalCommandCursorModel,
		goalCommandActiveCursorModel,
	)
}

func newGoalCommandHandlerFixtureWithRuntime(
	t *testing.T,
	profileProvider string,
	profileModel string,
	activeModel string,
	userScope ...bool,
) goalCommandHandlerFixture {
	t.Helper()

	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 10, 23, 45, 0, 0, time.UTC)
	homePaths := testHomePaths(t)
	writeDaemonFile(t, homePaths.ConfigFile, `
[goals]
max_turns = 7
context_nudge_ratio = 0.0
`)
	db := openDaemonTestGlobalDB(t)
	profiles, err := profilepkg.NewManager(
		profilepkg.WithStore(db), profilepkg.WithHomePaths(homePaths), profilepkg.WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("profile.NewManager() error = %v", err)
	}
	profileOwner, err := profiles.Create(ctx, profilepkg.CreateInput{Name: "goal-profile"})
	if err != nil {
		t.Fatalf("profiles.Create() error = %v", err)
	}
	workspaceID := "ws-goal-command"
	workspaceRoot := t.TempDir()
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: workspaceID, Name: workspaceID, RootDir: workspaceRoot,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	var userResolver *workspacepkg.Resolver
	var defaultsResolver workspacepkg.RuntimeResolver
	var apiResolver loopAPIWorkspaceResolver
	if len(userScope) > 0 && userScope[0] {
		workspaceID = ""
		userResolver, err = workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithProfileAvailabilityChecker(profiles),
		)
		if err != nil {
			t.Fatal(err)
		}
		defaultsResolver = userResolver
		apiResolver = userResolver
	}
	profile := store.SessionCreationProfile{
		Version: store.SessionCreationProfileVersion, AgentName: "operator-agent",
		ProfileID: profileOwner.ID,
		Provider:  profileProvider, Model: profileModel, WorkspaceID: workspaceID, CWD: workspaceRoot,
		Permissions: "default",
	}
	profileRef, err := db.PutSessionCreationProfile(ctx, profile)
	if err != nil {
		t.Fatalf("PutSessionCreationProfile() error = %v", err)
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		t.Fatalf("PolicySpecDigest() error = %v", err)
	}
	sessionID := "session-goal-command"
	creationDigest, err := profile.CreationDigest(store.SessionCreationOptions{
		SessionID: sessionID,

		SessionType: string(session.SessionTypeUser),
	})
	if err != nil {
		t.Fatalf("CreationDigest() error = %v", err)
	}
	identity := store.SessionCreationIdentity{
		CreationProfileRef: profileRef,
		PolicySpecDigest:   policyDigest,
		CreationDigest:     creationDigest,
	}
	if _, err := db.RegisterSessionWithCreationIdentity(ctx, store.SessionInfo{
		ProfileID: profileOwner.ID, ID: sessionID, AgentName: profile.AgentName, Provider: profile.Provider,
		WorkspaceID: workspaceID,

		SessionType: string(session.SessionTypeUser), State: string(session.StateActive),
		RuntimeStatus: store.SessionRuntimeUnbound,
		CreatedAt:     now, UpdatedAt: now,
	}, identity); err != nil {
		t.Fatalf("RegisterSessionWithCreationIdentity() error = %v", err)
	}
	runIDs := []looppkg.RunID{
		"run-goal-command-1",
		"run-goal-command-2",
		"run-goal-command-3",
		"run-goal-command-4",
		"run-goal-command-5",
		"run-goal-command-6",
		"run-goal-command-7",
		"run-goal-command-8",
		"run-goal-command-9",
		"run-goal-command-10",
		"run-goal-command-11",
		"run-goal-command-12",
		"run-goal-command-13",
		"run-goal-command-14",
		"run-goal-command-15",
		"run-goal-command-16",
	}
	nextRunID := 0
	var nextRunMu sync.Mutex
	aggregate, err := looppkg.NewService(
		db,
		looppkg.DefinitionResolverFunc(
			func(context.Context, looppkg.WorkspaceID, string, string) (*looppkg.ResolvedDefinition, error) {
				return nil, looppkg.ErrDefinitionNotFound
			},
		),
		newGoalRunPolicyResolver(homePaths, nil),
		looppkg.WithDefaultsResolver(newLoopDefaultsResolver(homePaths, defaultsResolver)),
		looppkg.WithClock(func() time.Time { return now }),
		looppkg.WithRunIDFactory(func() (looppkg.RunID, error) {
			nextRunMu.Lock()
			defer nextRunMu.Unlock()
			if nextRunID >= len(runIDs) {
				return looppkg.RunID("run-goal-command-overflow"), nil
			}
			value := runIDs[nextRunID]
			nextRunID++
			return value, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	status := &goalCommandSessionStatus{targetID: sessionID, callers: map[string]*session.Info{
		"agent-operator": {
			ID: "agent-operator", WorkspaceID: workspaceID,
		},
	}, info: &session.Info{
		ProfileID: profileOwner.ID, ID: sessionID, AgentName: profile.AgentName, Provider: profile.Provider,
		Model:       activeModel,
		WorkspaceID: workspaceID,
		Lineage:     &store.SessionLineage{ParentSessionID: "agent-operator"},
		Type:        session.SessionTypeUser, State: session.StateActive,
	}}
	return goalCommandHandlerFixture{
		service: &daemonLoopAPIService{
			aggregate: aggregate, persistence: db, goalPersistence: db,
			homePaths: homePaths, workspaceResolver: apiResolver, now: func() time.Time { return now },
			sessionStatus: status, creationStore: db,
		},
		db: db, workspaceID: workspaceID, sessionID: sessionID, profileRef: profileRef, profileID: profileOwner.ID,
	}
}

func (f goalCommandHandlerFixture) mustRun(t *testing.T, runID string) looppkg.Run {
	t.Helper()
	run, err := f.db.GetLoopRun(testutil.Context(t), looppkg.WorkspaceID(f.workspaceID), looppkg.RunID(runID))
	if err != nil {
		t.Fatalf("GetLoopRun(%q) error = %v", runID, err)
	}
	return run
}

func (f goalCommandHandlerFixture) assertPinnedGoalDefinition(
	t *testing.T,
	run looppkg.Run,
	wantRuntimeModel string,
) {
	t.Helper()
	snapshot, err := f.db.GetLoopDefinitionSnapshot(testutil.Context(t), run.WorkspaceID, "", run.DefinitionDigest)
	if err != nil {
		t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
	}
	var params dsl.GoalParams
	if err := resolved.Definition.Graph.Nodes[0].Params.Decode(&params); err != nil {
		t.Fatalf("Decode(GoalParams) error = %v", err)
	}
	if len(params.Judge) != 1 || params.Judge[0].Type != dsl.CriterionAgentJudge ||
		params.Judge[0].Check != "" || !strings.Contains(params.Judge[0].Rubric, "make verify") {
		t.Fatalf("pinned Goal judge = %#v", params.Judge)
	}
	if resolved.EffectiveConfig.RuntimeDefaults.Judge.Model != wantRuntimeModel {
		t.Fatalf(
			"pinned Goal judge model = %q, want %q",
			resolved.EffectiveConfig.RuntimeDefaults.Judge.Model,
			wantRuntimeModel,
		)
	}
}

func assertGoalCommandOutcome(
	t *testing.T,
	decision session.GoalDispatchDecision,
	wantOutcome session.GoalCommandOutcome,
	wantReason session.GoalReasonCode,
) {
	t.Helper()
	if decision.Kind != session.GoalDispatchRespond || decision.Result == nil ||
		decision.Result.Outcome != wantOutcome {
		t.Fatalf("Goal decision = %#v, want outcome %q", decision, wantOutcome)
	}
	if wantReason == "" {
		if decision.Result.ReasonCode != nil {
			t.Fatalf("Goal reason = %q, want nil", *decision.Result.ReasonCode)
		}
		return
	}
	if decision.Result.ReasonCode == nil || *decision.Result.ReasonCode != wantReason {
		t.Fatalf("Goal reason = %#v, want %q", decision.Result.ReasonCode, wantReason)
	}
}

type goalCommandSessionStatus struct {
	info       *session.Info
	targetID   string
	callers    map[string]*session.Info
	sessions   map[string]*session.Info
	statusErrs map[string]error
}

func (s *goalCommandSessionStatus) Status(_ context.Context, id string) (*session.Info, error) {
	if s == nil || s.info == nil {
		return nil, errors.New("session not found")
	}
	if err, ok := s.statusErrs[strings.TrimSpace(id)]; ok {
		return nil, err
	}
	if caller, ok := s.callers[strings.TrimSpace(id)]; ok {
		cloned := *caller
		return &cloned, nil
	}
	if child, ok := s.sessions[strings.TrimSpace(id)]; ok {
		cloned := *child
		return &cloned, nil
	}
	if s.targetID != "" && strings.TrimSpace(id) != s.targetID {
		return nil, errors.New("session not found")
	}
	cloned := *s.info
	return &cloned, nil
}

// These fakes replace read-only I/O boundaries; control transitions use the real store.
type contextUnavailableGoalProjection struct{ loopGoalAPIPersistence }

func (s contextUnavailableGoalProjection) GetSessionGoalProjection(ctx context.Context, ws looppkg.WorkspaceID,
	sessionID string) (goalpkg.SessionProjection, error) {
	projection, err := s.loopGoalAPIPersistence.GetSessionGoalProjection(ctx, ws, sessionID)
	if err == nil && projection.Found {
		projection.Checkpoint = &goalpkg.Checkpoint{ContextState: "known", UsageSequence: new(int64(100)),
			SessionID: sessionID, ContextNudgeRatio: 0.8, Status: "active"}
	}
	return projection, err
}

type failingGoalContextEvents struct{}

func (failingGoalContextEvents) Events(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
	return nil, errors.New("query events: injected read outage")
}

type clearCanceledGoalProjection struct {
	loopGoalAPIPersistence
	clear func(context.Context) error
}

func (s clearCanceledGoalProjection) GetSessionGoalProjection(ctx context.Context, ws looppkg.WorkspaceID,
	sessionID string) (goalpkg.SessionProjection, error) {
	projection, err := s.loopGoalAPIPersistence.GetSessionGoalProjection(ctx, ws, sessionID)
	if err != nil || projection.RunStatus != looppkg.StatusCanceled || projection.Cleared {
		return projection, err
	}
	if err := s.clear(ctx); err != nil {
		return goalpkg.SessionProjection{}, err
	}
	return s.loopGoalAPIPersistence.GetSessionGoalProjection(ctx, ws, sessionID)
}
