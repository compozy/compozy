package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
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
		if definition.Contract.ModelDefaults == nil ||
			definition.Contract.ModelDefaults.Judge != goalCommandCursorModel {
			t.Fatalf(
				"Goal judge model default = %#v, want %s",
				definition.Contract.ModelDefaults,
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
		if first.StartedBy.Kind != taskpkg.ActorKindHuman || first.StartedBy.Ref != "operator" ||
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
		if run := fixture.mustRun(t, replaced.Result.Snapshot.RunID); !run.PauseRequested {
			t.Fatal("paused Goal pause_requested = false")
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

	t.Run("Should pin the reconciled active model for a provider-native default session", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixtureWithModels(t, "", goalCommandCursorModel)
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

	t.Run("Should reject invalid origin sessions before creating a Run", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			mutate     func(*goalCommandSessionStatus)
			wantReason session.GoalReasonCode
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
				name: "Should reject an origin without a persisted creation profile",
				mutate: func(status *goalCommandSessionStatus) {
					status.info.ID = "session-missing-creation-profile"
				},
				wantReason: session.GoalReasonCode(looppkg.ReasonCodeGoalOriginProfileUnavailable),
			},
			{
				name: "Should reject an origin whose active agent differs from its immutable profile",
				mutate: func(status *goalCommandSessionStatus) {
					status.info.AgentName = "different-agent"
				},
				wantReason: session.GoalReasonCode(looppkg.ReasonCodeGoalOriginProfileUnavailable),
			},
			{
				name: "Should reject an origin whose active provider differs from its immutable profile",
				mutate: func(status *goalCommandSessionStatus) {
					status.info.Provider = "different-provider"
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
}

type goalCommandHandlerFixture struct {
	service     *daemonLoopAPIService
	db          loopGoalProductionStore
	workspaceID string
	sessionID   string
	profileRef  string
}

func newGoalCommandHandlerFixture(t *testing.T) goalCommandHandlerFixture {
	t.Helper()
	return newGoalCommandHandlerFixtureWithModels(
		t,
		goalCommandCursorModel,
		goalCommandActiveCursorModel,
	)
}

func newGoalCommandHandlerFixtureWithModels(
	t *testing.T,
	profileModel string,
	activeModel string,
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
	workspaceID := "ws-goal-command"
	workspaceRoot := t.TempDir()
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: workspaceID, Name: workspaceID, RootDir: workspaceRoot,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	profile := store.SessionCreationProfile{
		Version: store.SessionCreationProfileVersion, AgentName: "operator-agent",
		Provider: "cursor", Model: profileModel, WorkspaceID: workspaceID, CWD: workspaceRoot,
		SandboxMode: store.SessionCreationSandboxNone, Permissions: "default",
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
		SessionID:            sessionID,
		NetworkOwnerKey:      "session:" + sessionID,
		NetworkParticipation: participation.LocalSpec(),
		SessionType:          string(session.SessionTypeUser),
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
		ID: sessionID, AgentName: profile.AgentName, Provider: profile.Provider,
		WorkspaceID:         workspaceID,
		SessionNetworkState: &store.SessionNetworkState{NetworkSpec: participation.LocalSpec()},
		SessionType:         string(session.SessionTypeUser), State: string(session.StateActive),
		CreatedAt: now, UpdatedAt: now,
	}, identity); err != nil {
		t.Fatalf("RegisterSessionWithCreationIdentity() error = %v", err)
	}
	runIDs := []looppkg.RunID{
		"run-goal-command-1",
		"run-goal-command-2",
		"run-goal-command-3",
		"run-goal-command-4",
	}
	nextRunID := 0
	aggregate, err := looppkg.NewService(
		db,
		looppkg.DefinitionResolverFunc(
			func(context.Context, looppkg.WorkspaceID, string) (*looppkg.ResolvedDefinition, error) {
				return nil, looppkg.ErrDefinitionNotFound
			},
		),
		newGoalRunPolicyResolver(homePaths, nil),
		looppkg.WithDefaultsResolver(newLoopDefaultsResolver(homePaths, nil)),
		looppkg.WithClock(func() time.Time { return now }),
		looppkg.WithRunIDFactory(func() looppkg.RunID {
			if nextRunID >= len(runIDs) {
				return looppkg.RunID("run-goal-command-overflow")
			}
			value := runIDs[nextRunID]
			nextRunID++
			return value
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	status := &goalCommandSessionStatus{info: &session.Info{
		ID: sessionID, AgentName: profile.AgentName, Provider: profile.Provider,
		Model:       activeModel,
		WorkspaceID: workspaceID, NetworkParticipation: participation.LocalSpec(),
		Type: session.SessionTypeUser, State: session.StateActive,
	}}
	return goalCommandHandlerFixture{
		service: &daemonLoopAPIService{
			aggregate: aggregate, persistence: db, goalPersistence: db,
			homePaths: homePaths, now: func() time.Time { return now },
			sessionStatus: status, creationStore: db,
		},
		db: db, workspaceID: workspaceID, sessionID: sessionID, profileRef: profileRef,
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
	wantJudgeModel string,
) {
	t.Helper()
	snapshot, err := f.db.GetLoopDefinitionSnapshot(testutil.Context(t), run.WorkspaceID, run.DefinitionDigest)
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
	if resolved.EffectiveConfig.ModelDefaults.Judge != wantJudgeModel {
		t.Fatalf(
			"pinned Goal judge model = %q, want %q",
			resolved.EffectiveConfig.ModelDefaults.Judge,
			wantJudgeModel,
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
	info *session.Info
}

func (s *goalCommandSessionStatus) Status(context.Context, string) (*session.Info, error) {
	if s == nil || s.info == nil {
		return nil, errors.New("session not found")
	}
	cloned := *s.info
	return &cloned, nil
}
