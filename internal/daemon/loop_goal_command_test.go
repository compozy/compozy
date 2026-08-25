package daemon

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should preserve the origin Network snapshot and apply a per-run worker runtime", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
		status, ok := fixture.service.sessionStatus.(*goalCommandSessionStatus)
		if !ok {
			t.Fatalf("session status type = %T, want *goalCommandSessionStatus", fixture.service.sessionStatus)
		}
		network := daemonTestLiveParticipation(fixture.workspaceID, "goal-origin-command")
		status.info.NetworkParticipation = network
		decision, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, fixture.sessionID,
			session.PromptCaller{Kind: string(taskpkg.ActorKindHuman), ID: "operator", Source: "http"},
			session.GoalCommand{
				Verb: session.GoalCommandVerbSet, Objective: "Ship with the selected worker",
				Runtime: &session.RuntimeSelection{
					Provider: "cursor", Model: "grok-4.5", ReasoningEffort: "high", Speed: speedpkg.SpeedFast,
				},
			},
		)
		if err != nil {
			t.Fatalf("Handle(set runtime) error = %v", err)
		}
		assertGoalCommandOutcome(t, decision, session.GoalOutcomeStarted, "")
		run := fixture.mustRun(t, decision.Result.Snapshot.RunID)
		if got := run.NetworkSpecSnapshot(); got != network {
			t.Fatalf("run network snapshot = %#v, want exact origin snapshot %#v", got, network)
		}
		snapshot, err := fixture.db.GetLoopDefinitionSnapshot(testutil.Context(t), run.WorkspaceID, run.DefinitionDigest)
		if err != nil {
			t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
		}
		resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		worker := resolved.EffectiveConfig.RuntimeDefaults.Worker
		if worker.Provider != "cursor" || worker.Model != "grok-4.5" || worker.Reasoning != "high" ||
			worker.Speed != speedpkg.SpeedFast {
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
		definition, err := fixture.db.GetLoopDefinitionSnapshot(testutil.Context(t), run.WorkspaceID, run.DefinitionDigest)
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

	t.Run("Should return a stable runtime-invalid result for an unknown provider", func(t *testing.T) {
		t.Parallel()

		fixture := newGoalCommandHandlerFixture(t)
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

	t.Run("Should isolate eight concurrent child Goals by origin runtime and network", func(t *testing.T) {
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
			childNetwork := daemonTestLiveParticipation(fixture.workspaceID, "goal-child-"+string(rune('a'+index)))
			child := &session.Info{
				ID: childID, ProfileID: parent.ProfileID, AgentName: parent.AgentName, Provider: parent.Provider,
				Model: parent.Model, WorkspaceID: parent.WorkspaceID, NetworkParticipation: childNetwork,
				Lineage: &store.SessionLineage{ParentSessionID: parentID}, Type: parent.Type, State: session.StateActive,
			}
			status.sessions[childID] = child
			creationDigest, err := profile.CreationDigest(store.SessionCreationOptions{
				SessionID: childID, NetworkOwnerKey: "session:" + childID,
				NetworkParticipation: childNetwork, SessionType: string(session.SessionTypeUser),
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
				ProfileID: child.ProfileID, ID: childID, AgentName: child.AgentName, Provider: child.Provider,
				WorkspaceID:         child.WorkspaceID,
				SessionNetworkState: &store.SessionNetworkState{NetworkSpec: childNetwork},
				SessionType:         string(child.Type), State: string(child.State), RuntimeStatus: store.SessionRuntimeUnbound,
				CreatedAt: parent.CreatedAt, UpdatedAt: parent.UpdatedAt,
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
			if snapshot == nil || snapshot.OriginSessionID != result.childID || snapshot.BoundSessionID != result.childID {
				t.Fatalf("Goal snapshot for %q = %#v, want isolated origin/binding", result.childID, snapshot)
			}
			if _, exists := runIDs[snapshot.RunID]; exists {
				t.Fatalf("duplicate child Goal run ID %q", snapshot.RunID)
			}
			runIDs[snapshot.RunID] = struct{}{}
			run := fixture.mustRun(t, snapshot.RunID)
			if run.Origin.SessionID != result.childID || run.StartedBy.Ref != parentID {
				t.Fatalf("child Goal run %q identity = %#v, want origin %q and actor %q", snapshot.RunID, run.Origin, result.childID, parentID)
			}
			child := status.sessions[result.childID]
			if got := run.NetworkSpecSnapshot(); got != child.NetworkParticipation {
				t.Fatalf("child Goal run %q network = %#v, want %#v", snapshot.RunID, got, child.NetworkParticipation)
			}
			definition, err := fixture.db.GetLoopDefinitionSnapshot(testutil.Context(t), run.WorkspaceID, run.DefinitionDigest)
			if err != nil {
				t.Fatalf("GetLoopDefinitionSnapshot(%q) error = %v", result.childID, err)
			}
			resolved, err := looppkg.LoadExecutedDefinitionSnapshot(definition.Definition, run.DefinitionDigest)
			if err != nil {
				t.Fatalf("LoadExecutedDefinitionSnapshot(%q) error = %v", result.childID, err)
			}
			worker := resolved.EffectiveConfig.RuntimeDefaults.Worker
			wantModel := "child-model-" + string(result.childID[len(result.childID)-1])
			if worker.Provider != "cursor" || worker.Model != wantModel || worker.Reasoning != "high" || worker.Speed != speedpkg.SpeedFast {
				t.Fatalf("child Goal run %q runtime = %#v, want cursor/%s/high/fast", snapshot.RunID, worker, wantModel)
			}
		}

		sibling, err := fixture.service.Handle(
			testutil.Context(t), fixture.workspaceID, "session-goal-child-b",
			session.PromptCaller{Kind: string(taskpkg.ActorKindAgentSession), ID: "session-goal-child-a", Source: "uds"},
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
	profile := store.SessionCreationProfile{
		Version: store.SessionCreationProfileVersion, AgentName: "operator-agent",
		ProfileID: profileOwner.ID,
		Provider:  "cursor", Model: profileModel, WorkspaceID: workspaceID, CWD: workspaceRoot,
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
		ProfileID: profileOwner.ID, ID: sessionID, AgentName: profile.AgentName, Provider: profile.Provider,
		WorkspaceID:         workspaceID,
		SessionNetworkState: &store.SessionNetworkState{NetworkSpec: participation.LocalSpec()},
		SessionType:         string(session.SessionTypeUser), State: string(session.StateActive),
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
		looppkg.WithDefaultsResolver(newLoopDefaultsResolver(homePaths, nil)),
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
		WorkspaceID: workspaceID, NetworkParticipation: participation.LocalSpec(),
		Lineage: &store.SessionLineage{ParentSessionID: "agent-operator"},
		Type:    session.SessionTypeUser, State: session.StateActive,
	}}
	return goalCommandHandlerFixture{
		service: &daemonLoopAPIService{
			aggregate: aggregate, persistence: db, goalPersistence: db,
			homePaths: homePaths, now: func() time.Time { return now },
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
