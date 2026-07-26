package loop_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/task"
)

func TestServiceParticipationShouldResolvePersistAndValidateLoopOwnership(t *testing.T) {
	t.Parallel()

	t.Run("Should keep copied non-network definitions Local in start and dry-run", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		copied := definition
		store := newFakeLoopStore()
		svc := newParticipationTestService(
			t,
			store,
			copied,
			loop.WithParticipationResolver(loopTestParticipationResolver(t, true)),
		)
		preview, err := svc.DryRun(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		})
		if err != nil {
			t.Fatalf("DryRun() error = %v", err)
		}
		if got := preview.ResolvedNetworkParticipation; got != participation.LocalSpec() {
			t.Fatalf("DryRun participation = %#v, want canonical Local", got)
		}
		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if got := run.NetworkSpecSnapshot(); got != participation.LocalSpec() {
			t.Fatalf("Start participation = %#v, want canonical Local", got)
		}
		if got := store.mustRun(t, run.ID).NetworkSpecSnapshot(); got != participation.LocalSpec() {
			t.Fatalf("stored participation = %#v, want canonical Local", got)
		}
	})

	t.Run("Should reject network nodes when resolved participation is Local", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			nodeID     dsl.NodeID
			mutateNode func(*dsl.Node)
		}{
			{
				name:   "Should reject agh network tools",
				nodeID: "send-update",
				mutateNode: func(node *dsl.Node) {
					node.Kind = "agh__network_send"
				},
			},
			{
				name:   "Should reject channel result harvests",
				nodeID: "await-reply",
				mutateNode: func(node *dsl.Node) {
					node.Harvest = &dsl.HarvestSpec{Kind: "channel_result", Window: "30s"}
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				definition := validDefinition()
				definition.Graph.Nodes[2].ID = tc.nodeID
				tc.mutateNode(&definition.Graph.Nodes[2])
				store := newFakeLoopStore()
				svc := newParticipationTestService(
					t,
					store,
					definition,
					loop.WithParticipationResolver(loopTestParticipationResolver(t, true)),
				)
				inputs := loop.Inputs{Values: map[string]any{"tasks": "task-ref"}}
				if _, err := svc.DryRun(context.Background(), "ws-1", "valid-loop", inputs); !errors.Is(
					err,
					participation.ErrLoopRequiresLive,
				) || !strings.Contains(err.Error(), string(tc.nodeID)) {
					t.Fatalf("DryRun() error = %v, want loop_requires_live naming %s", err, tc.nodeID)
				}
				if _, err := svc.Start(
					context.Background(),
					"ws-1",
					"valid-loop",
					inputs,
					humanActor(t),
				); !errors.Is(err, participation.ErrLoopRequiresLive) ||
					!strings.Contains(err.Error(), string(tc.nodeID)) {
					t.Fatalf("Start() error = %v, want loop_requires_live naming %s", err, tc.nodeID)
				}
				if got := store.createCount(); got != 0 {
					t.Fatalf("CreateLoopRun calls = %d, want 0", got)
				}
			})
		}
	})

	t.Run("Should resolve one live loop-run snapshot from the definition", func(t *testing.T) {
		t.Parallel()

		definition := liveLoopTestDefinition()
		store := newFakeLoopStore()
		svc := newParticipationTestService(
			t,
			store,
			definition,
			loop.WithParticipationResolver(loopTestParticipationResolver(t, true)),
		)
		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		spec := run.NetworkSpecSnapshot()
		if spec.Mode != participation.ModeLive || spec.Source != participation.SourceLoopDefinition ||
			spec.ChannelStrategy != participation.StrategyLoopRun || strings.TrimSpace(spec.ChannelID) == "" {
			t.Fatalf("Start participation = %#v, want live loop-definition snapshot", spec)
		}
		if got := store.mustRun(t, run.ID).NetworkSpecSnapshot(); got != spec {
			t.Fatalf("stored participation = %#v, want %#v", got, spec)
		}
	})

	t.Run("Should carry the live snapshot through started and terminal hooks", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		hooks := &participationLifecycleHookDispatcher{}
		svc := newParticipationTestService(
			t,
			store,
			liveLoopTestDefinition(),
			loop.WithParticipationResolver(loopTestParticipationResolver(t, true)),
			loop.WithHookDispatcher(hooks),
		)
		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		want := run.NetworkSpecSnapshot()
		if got := hooks.started.ResolvedNetworkParticipation; got == nil || *got != want {
			t.Fatalf("loop.started participation = %#v, want %#v", got, want)
		}
		if err := svc.Transition(
			context.Background(),
			run.ID,
			loop.StatusDone,
			loop.TransitionCauseContract,
		); err != nil {
			t.Fatalf("Transition(done) error = %v", err)
		}
		if got := hooks.terminal.ResolvedNetworkParticipation; got == nil || *got != want {
			t.Fatalf("loop.terminal participation = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve automation-job provenance for a per-fire request", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		store := newFakeLoopStore()
		svc := newParticipationTestService(
			t,
			store,
			definition,
			loop.WithParticipationResolver(loopTestParticipationResolver(t, true)),
		)
		mode := participation.ModeLive
		strategy := participation.StrategyLoopRun
		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
			NetworkParticipation: &participation.Request{
				Mode:            &mode,
				ChannelStrategy: &strategy,
			},
			NetworkParticipationSource: participation.SourceAutomationJob,
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		spec := run.NetworkSpecSnapshot()
		if spec.Mode != participation.ModeLive || spec.Source != participation.SourceAutomationJob ||
			spec.ChannelStrategy != participation.StrategyLoopRun || strings.TrimSpace(spec.ChannelID) == "" {
			t.Fatalf("Start participation = %#v, want live automation-job snapshot", spec)
		}
		if got := store.mustRun(t, run.ID).NetworkSpecSnapshot(); got != spec {
			t.Fatalf("stored participation = %#v, want %#v", got, spec)
		}
	})

	t.Run("Should reject live while unavailable before creating a run", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc := newParticipationTestService(
			t,
			store,
			liveLoopTestDefinition(),
			loop.WithParticipationResolver(loopTestParticipationResolver(t, false)),
		)
		_, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if !errors.Is(err, participation.ErrUnavailable) {
			t.Fatalf("Start() error = %v, want network participation unavailable", err)
		}
		if got := store.createCount(); got != 0 {
			t.Fatalf("CreateLoopRun calls = %d, want 0", got)
		}
	})
}

func TestServiceTransitionShouldEnforceTruthyStatusFSM(t *testing.T) {
	t.Parallel()

	t.Run("Should reject false done and failed coercions from truthful statuses", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			from loop.Status
			to   loop.Status
		}{
			{name: "Should reject exhausted to done", from: loop.StatusExhausted, to: loop.StatusDone},
			{name: "Should reject stalled to done", from: loop.StatusStalled, to: loop.StatusDone},
			{
				name: "Should reject needs approval to done",
				from: loop.StatusNeedsApproval,
				to:   loop.StatusDone,
			},
			{
				name: "Should reject needs approval to failed without operator stop",
				from: loop.StatusNeedsApproval,
				to:   loop.StatusFailed,
			},
			{name: "Should reject paused to done", from: loop.StatusPaused, to: loop.StatusDone},
			{
				name: "Should reject paused to failed without operator stop",
				from: loop.StatusPaused,
				to:   loop.StatusFailed,
			},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				store := newFakeLoopStore()
				run := seedFakeRun(store, tt.from)
				svc := newTestService(t, store, validDefinition())

				err := svc.Transition(context.Background(), run.ID, tt.to, loop.TransitionCauseContract)
				if !errors.Is(err, loop.ErrInvalidTransition) {
					t.Fatalf("Transition(%s -> %s) error = %v, want ErrInvalidTransition", tt.from, tt.to, err)
				}
				stored := store.mustRun(t, run.ID)
				if stored.Status != tt.from {
					t.Fatalf("stored status = %q, want original %q", stored.Status, tt.from)
				}
			})
		}
	})

	t.Run("Should allow running paused edges and reject illegal paused edges", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		err := svc.Transition(
			context.Background(),
			run.ID,
			loop.StatusPaused,
			loop.TransitionCausePauseBoundary,
		)
		if err != nil {
			t.Fatalf("Transition(running -> paused) error = %v", err)
		}
		err = svc.Transition(
			context.Background(),
			run.ID,
			loop.StatusRunning,
			loop.TransitionCauseOperatorResume,
		)
		if err != nil {
			t.Fatalf("Transition(paused -> running) error = %v", err)
		}

		done := seedFakeRun(store, loop.StatusDone)
		err = svc.Transition(
			context.Background(),
			done.ID,
			loop.StatusPaused,
			loop.TransitionCausePauseBoundary,
		)
		if !errors.Is(err, loop.ErrInvalidTransition) {
			t.Fatalf("Transition(done -> paused) error = %v, want ErrInvalidTransition", err)
		}
		watching := seedFakeRun(store, loop.StatusWatching)
		err = svc.Transition(
			context.Background(),
			watching.ID,
			loop.StatusPaused,
			loop.TransitionCausePauseBoundary,
		)
		if !errors.Is(err, loop.ErrInvalidTransition) {
			t.Fatalf("Transition(watching -> paused) error = %v, want ErrInvalidTransition", err)
		}
	})
}

func TestEffectiveConfigShouldMergeLayersAndClampCeilings(t *testing.T) {
	t.Parallel()

	t.Run("Should merge definition defaults loop config and per run overrides", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		def.Contract.IterationCap = 3
		def.Contract.NoProgress.Window = 2
		def.Contract.Budget.Tokens = 100
		def.Contract.Budget.WallClockSec = 10
		def.Contract.ModelDefaults = &dsl.ModelDefaults{
			Worker: "definition-worker",
			Judge:  "definition-judge",
		}
		requireNode(t, &def, "fan").MaxParallel = 2
		appendGate(&def, dsl.Node{
			ID:           "review_gate",
			Class:        dsl.NodeClassControl,
			Kind:         string(dsl.ControlGate),
			MaxRevisions: 2,
		})
		resolved := compileDefinition(t, def)
		defaults := loop.LoopDefaults{
			Delivery: loop.LoopConfig{
				IterationCap:     new(6),
				NoProgressWindow: new(99),
				FanOutWidth:      new(99),
				GateMaxRevisions: new(99),
				ModelDefaults: &loop.ModelDefaults{
					Worker: new("default-worker"),
					Judge:  new("default-judge"),
				},
			},
		}
		stored := loop.LoopConfig{
			IterationCap:     new(7),
			NoProgressWindow: new(8),
			FanOutWidth:      new(12),
			ModelDefaults: &loop.ModelDefaults{
				Worker: new("stored-worker"),
			},
		}
		escalate := dsl.BudgetExceededEscalate
		perRun := loop.LoopConfig{
			IterationCap:     new(8),
			BudgetOnExceeded: &escalate,
			FanOutWidth:      new(99),
			GateMaxRevisions: new(99),
			ModelDefaults: &loop.ModelDefaults{
				Judge: new("per-run-judge"),
			},
		}

		effective, err := loop.ResolveEffectiveConfig(resolved, defaults, &stored, perRun)
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig() error = %v", err)
		}
		if effective.IterationCap != 8 {
			t.Fatalf("IterationCap = %d, want per-run override 8", effective.IterationCap)
		}
		if effective.NoProgressWindow != 8 {
			t.Fatalf("NoProgressWindow = %d, want stored override 8", effective.NoProgressWindow)
		}
		if effective.FanOutWidth != loop.LoopMaxFanoutWidth {
			t.Fatalf(
				"FanOutWidth = %d, want clamped ceiling %d",
				effective.FanOutWidth,
				loop.LoopMaxFanoutWidth,
			)
		}
		if effective.GateMaxRevisions != loop.LoopMaxGateRevisions {
			t.Fatalf(
				"GateMaxRevisions = %d, want clamped ceiling %d",
				effective.GateMaxRevisions,
				loop.LoopMaxGateRevisions,
			)
		}
		if effective.BudgetOnExceeded != dsl.BudgetExceededEscalate {
			t.Fatalf("BudgetOnExceeded = %q, want escalate", effective.BudgetOnExceeded)
		}
		if effective.ModelDefaults.Worker != "stored-worker" {
			t.Fatalf("ModelDefaults.Worker = %q, want stored-worker", effective.ModelDefaults.Worker)
		}
		if effective.ModelDefaults.Judge != "per-run-judge" {
			t.Fatalf("ModelDefaults.Judge = %q, want per-run-judge", effective.ModelDefaults.Judge)
		}
	})

	t.Run("Should clamp an override above a ceiling instead of honoring it", func(t *testing.T) {
		t.Parallel()

		resolved := compileDefinition(t, validDefinition())
		perRun := loop.LoopConfig{
			NoProgressWindow: new(loop.LoopMaxNoProgressWindow + 10),
			FanOutWidth:      new(loop.LoopMaxFanoutWidth + 10),
			GateMaxRevisions: new(loop.LoopMaxGateRevisions + 10),
		}

		effective, err := loop.ResolveEffectiveConfig(
			resolved,
			loop.LoopDefaults{},
			nil,
			perRun,
		)
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig() error = %v", err)
		}
		if effective.NoProgressWindow != loop.LoopMaxNoProgressWindow {
			t.Fatalf(
				"NoProgressWindow = %d, want %d",
				effective.NoProgressWindow,
				loop.LoopMaxNoProgressWindow,
			)
		}
		if effective.FanOutWidth != loop.LoopMaxFanoutWidth {
			t.Fatalf("FanOutWidth = %d, want %d", effective.FanOutWidth, loop.LoopMaxFanoutWidth)
		}
		if effective.GateMaxRevisions != loop.LoopMaxGateRevisions {
			t.Fatalf("GateMaxRevisions = %d, want %d", effective.GateMaxRevisions, loop.LoopMaxGateRevisions)
		}
	})
}

func TestServiceStartShouldUseDefaultsResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should seed effective config from workspace defaults resolver", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		halt := dsl.BudgetExceededHalt
		iterationCap := 12
		var resolvedWorkspace loop.WorkspaceID
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithDefaultsResolver(func(
				_ context.Context,
				ws loop.WorkspaceID,
			) (loop.LoopDefaults, error) {
				resolvedWorkspace = ws
				return loop.LoopDefaults{
					Delivery: loop.LoopConfig{
						IterationCap:     new(iterationCap),
						NoProgressWindow: new(5),
						BudgetTokens:     new(0),
						BudgetWallSec:    new(0),
						BudgetOnExceeded: &halt,
						FanOutWidth:      new(1),
					},
				}, nil
			}),
		)

		run, err := svc.Start(context.Background(), "ws-config", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if resolvedWorkspace != "ws-config" {
			t.Fatalf("defaults resolver workspace = %q, want ws-config", resolvedWorkspace)
		}
		if run.IterationCap != 12 {
			t.Fatalf("IterationCap = %d, want resolver default 12", run.IterationCap)
		}
		if run.DefinitionDigest == "" || len(run.DefinitionSnapshot) == 0 {
			t.Fatalf(
				"definition pinning = digest:%q snapshot:%d, want populated",
				run.DefinitionDigest,
				len(run.DefinitionSnapshot),
			)
		}
		iterationCap = 99
		snapshot, err := store.GetLoopDefinitionSnapshot(context.Background(), "ws-config", run.DefinitionDigest)
		if err != nil {
			t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
		}
		if snapshot.Digest != run.DefinitionDigest || snapshot.Version != run.DefinitionVersion {
			t.Fatalf("snapshot = %#v, want digest/version from run %#v", snapshot, run)
		}
		hydrated, err := loop.LoadExecutedDefinitionSnapshot(snapshot.Definition, snapshot.Digest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		if got, want := hydrated.EffectiveConfig.IterationCap, 12; got != want {
			t.Fatalf("pinned iteration cap = %d, want %d after defaults mutation", got, want)
		}
	})
}

func TestServiceInlineGoalStartAndReplaceShouldSharePinnedStartPath(t *testing.T) {
	t.Parallel()

	t.Run("Should start a snapshot-only session Goal with pinned origin and policy", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc := newTestService(t, store, validDefinition())
		run, err := svc.StartInline(
			context.Background(),
			"ws-1",
			inlineGoalDefinition("ship the release", "judge-v1"),
			loop.Inputs{},
			inlineGoalOrigin("session-origin"),
			humanActor(t),
		)
		if err != nil {
			t.Fatalf("StartInline() error = %v", err)
		}
		if run.LoopName != loop.InlineGoalLoopName || run.Origin.Kind != loop.RunOriginSession ||
			run.Origin.SessionID != "session-origin" || run.GoalContextNudgeRatio != 0.8 {
			t.Fatalf("StartInline() run = %#v", run)
		}
		if len(run.DefinitionSnapshot) == 0 || store.createCount() != 1 {
			t.Fatalf("StartInline() snapshot bytes = %d creates = %d", len(run.DefinitionSnapshot), store.createCount())
		}
	})

	t.Run("Should reject a missing resolved judge before any write", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc := newTestService(t, store, validDefinition())
		_, err := svc.StartInline(
			context.Background(),
			"ws-1",
			inlineGoalDefinition("ship the release", ""),
			loop.Inputs{},
			inlineGoalOrigin("session-origin"),
			humanActor(t),
		)
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeGoalJudgeUnavailable {
			t.Fatalf("StartInline() error = %v, want %q", err, loop.ReasonCodeGoalJudgeUnavailable)
		}
		if store.createCount() != 0 {
			t.Fatalf("StartInline() creates = %d, want 0", store.createCount())
		}
	})

	t.Run("Should prepare replacement before atomically revoking the expected Run", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		old := seedFakeRun(store, loop.StatusRunning)
		old.LoopName = loop.InlineGoalLoopName
		origin := inlineGoalOrigin("session-origin")
		old.Origin = &origin
		store.seed(old)
		svc := newTestService(t, store, validDefinition())
		result, err := svc.ReplaceInline(
			context.Background(),
			old.ID,
			"ws-1",
			inlineGoalDefinition("ship the safer release", "judge-v1"),
			loop.Inputs{},
			inlineGoalOrigin("session-origin"),
			humanActor(t),
		)
		if err != nil {
			t.Fatalf("ReplaceInline() error = %v", err)
		}
		if result.ReplacedRunID != old.ID || result.Run == nil || result.Run.ID == old.ID {
			t.Fatalf("ReplaceInline() result = %#v", result)
		}
		if got := store.mustRun(t, old.ID); got.Status != loop.StatusFailed {
			t.Fatalf("replaced Run status = %q, want failed", got.Status)
		}
	})

	t.Run("Should report cleanup failure without denying a committed replacement", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		old := seedFakeRun(store, loop.StatusRunning)
		old.LoopName = loop.InlineGoalLoopName
		origin := inlineGoalOrigin("session-origin")
		old.Origin = &origin
		store.seed(old)
		store.inlineReplaceRevokedPromptLeases = []loop.GoalPromptLease{{
			LoopRunID: string(old.ID), TaskRunID: "task-run-cleanup", JudgeAttemptID: "judge-cleanup",
		}}
		cleanupErr := errors.New("stop judge session")
		var logs bytes.Buffer
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
			loop.WithGoalPromptLeaseRevoker(loop.GoalPromptLeaseRevokerFunc(func(
				context.Context,
				loop.GoalPromptLease,
				string,
			) error {
				return cleanupErr
			})),
		)

		result, err := svc.ReplaceInline(
			context.Background(),
			old.ID,
			"ws-1",
			inlineGoalDefinition("ship the safer release", "judge-v1"),
			loop.Inputs{},
			inlineGoalOrigin("session-origin"),
			humanActor(t),
		)
		if err != nil {
			t.Fatalf("ReplaceInline() error = %v, want committed success", err)
		}
		if result.ReplacedRunID != old.ID || result.Run == nil {
			t.Fatalf("ReplaceInline() result = %#v, want committed replacement", result)
		}
		if got := logs.String(); !strings.Contains(got, cleanupErr.Error()) ||
			!strings.Contains(got, "judge-cleanup") {
			t.Fatalf("cleanup log = %q, want error and judge attempt identity", got)
		}
	})

	t.Run("Should leave the old Run live when replacement compilation fails", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		old := seedFakeRun(store, loop.StatusRunning)
		old.LoopName = loop.InlineGoalLoopName
		origin := inlineGoalOrigin("session-origin")
		old.Origin = &origin
		store.seed(old)
		svc := newTestService(t, store, validDefinition())
		invalid := inlineGoalDefinition("ship the release", "judge-v1")
		invalid.Graph.Nodes[0].Params["objective"] = ""
		if _, err := svc.ReplaceInline(
			context.Background(), old.ID, "ws-1", invalid, loop.Inputs{},
			inlineGoalOrigin("session-origin"), humanActor(t),
		); err == nil {
			t.Fatal("ReplaceInline(invalid) error = nil")
		}
		if got := store.mustRun(t, old.ID); got.Status != loop.StatusRunning {
			t.Fatalf("old Run status = %q, want running", got.Status)
		}
	})

	t.Run("Should clear the newest inline Goal through the atomic store boundary", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		active := seedFakeRun(store, loop.StatusRunning)
		active.LoopName = loop.InlineGoalLoopName
		origin := inlineGoalOrigin("session-origin")
		active.Origin = &origin
		store.seed(active)
		svc := newTestService(t, store, validDefinition())
		if err := svc.ClearInlineGoal(
			context.Background(),
			"ws-1",
			"session-origin",
			humanActor(t),
		); err != nil {
			t.Fatalf("ClearInlineGoal() error = %v", err)
		}
		if got := store.mustRun(t, active.ID); got.Status != loop.StatusFailed {
			t.Fatalf("cleared Run status = %q, want failed", got.Status)
		}
	})
}

func TestServiceStartShouldPinGoalRunPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve and persist the workspace Goal context policy", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		resolved := compileDefinition(t, validDefinition())
		var resolvedWorkspace loop.WorkspaceID
		svc, err := loop.NewService(
			store,
			loop.DefinitionResolverFunc(func(
				context.Context,
				loop.WorkspaceID,
				string,
			) (*loop.ResolvedDefinition, error) {
				return resolved, nil
			}),
			loop.GoalRunPolicyResolverFunc(func(
				_ context.Context,
				ws loop.WorkspaceID,
			) (*loop.GoalRunPolicy, error) {
				resolvedWorkspace = ws
				return &loop.GoalRunPolicy{ContextNudgeRatio: 0}, nil
			}),
			loop.WithClock(func() time.Time {
				return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
			}),
			loop.WithRunIDFactory(func() loop.RunID { return "looprun-goal-policy" }),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		run, err := svc.Start(context.Background(), "ws-goal-policy", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if resolvedWorkspace != "ws-goal-policy" {
			t.Fatalf("Goal policy resolver workspace = %q, want ws-goal-policy", resolvedWorkspace)
		}
		if run.GoalContextNudgeRatio != 0 {
			t.Fatalf("GoalContextNudgeRatio = %v, want explicit zero", run.GoalContextNudgeRatio)
		}
		if stored := store.mustRun(t, run.ID); stored.GoalContextNudgeRatio != 0 {
			t.Fatalf("stored GoalContextNudgeRatio = %v, want explicit zero", stored.GoalContextNudgeRatio)
		}
	})

	t.Run("Should fail before creating a Run when Goal policy resolution fails", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		resolved := compileDefinition(t, validDefinition())
		wantErr := errors.New("workspace Goal config unavailable")
		svc, err := loop.NewService(
			store,
			loop.DefinitionResolverFunc(func(
				context.Context,
				loop.WorkspaceID,
				string,
			) (*loop.ResolvedDefinition, error) {
				return resolved, nil
			}),
			loop.GoalRunPolicyResolverFunc(func(
				context.Context,
				loop.WorkspaceID,
			) (*loop.GoalRunPolicy, error) {
				return nil, wantErr
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		_, err = svc.Start(context.Background(), "ws-goal-policy", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if !errors.Is(err, wantErr) {
			t.Fatalf("Start() error = %v, want %v", err, wantErr)
		}
		if store.createCount() != 0 {
			t.Fatalf("CreateLoopRun calls = %d, want 0", store.createCount())
		}
	})
}

func TestServiceStartShouldEnforceConcurrencyAndAncestry(t *testing.T) {
	t.Parallel()

	t.Run("Should reject forbid concurrency when an active run exists", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		_, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if !errors.Is(err, loop.ErrConcurrencyConflict) {
			t.Fatalf("Start() error = %v, want ErrConcurrencyConflict", err)
		}
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeActiveRunExists {
			t.Fatalf("Start() reason = %#v, want active run reason code", reason)
		}
	})

	t.Run("Should create a queued run when queue concurrency sees an active run", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		seedFakeRun(store, loop.StatusRunning)
		def := validDefinition()
		def.Concurrency = dsl.ConcurrencyQueue
		svc := newTestService(t, store, def)

		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if run.Status != loop.StatusQueued {
			t.Fatalf("run.Status = %q, want queued", run.Status)
		}
	})

	t.Run("Should reject a run loop target already present in the parent chain", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		parent := seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		_, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values:          map[string]any{"tasks": "task-ref"},
			ParentLoopRunID: parent.ID,
		}, humanActor(t))
		if !errors.Is(err, loop.ErrAncestryRejected) {
			t.Fatalf("Start() error = %v, want ErrAncestryRejected", err)
		}
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeAncestryCycle {
			t.Fatalf("Start() reason = %#v, want ancestry cycle reason code", reason)
		}
	})
}

func TestServiceControlMethodsShouldPreserveStatusContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should pause by setting intent without changing status", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		if err := svc.Pause(context.Background(), "ws-1", run.ID, humanActor(t)); err != nil {
			t.Fatalf("Pause() error = %v", err)
		}
		stored := store.mustRun(t, run.ID)
		if stored.Status != loop.StatusRunning || !stored.PauseRequested {
			t.Fatalf("stored run = %#v, want running with pause_requested", stored)
		}
		if err := svc.Pause(context.Background(), "ws-1", run.ID, humanActor(t)); err != nil {
			t.Fatalf("Pause() second call error = %v, want idempotent nil", err)
		}
	})

	t.Run("Should resume by clearing intent or transitioning paused to running", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		running := seedFakeRun(store, loop.StatusRunning)
		running.PauseRequested = true
		store.seed(running)
		paused := seedFakeRun(store, loop.StatusPaused)
		paused.ID = "run-paused-resume"
		paused.PauseRequested = true
		store.seed(paused)
		svc := newTestService(t, store, validDefinition())

		if err := svc.Resume(context.Background(), "ws-1", running.ID, humanActor(t)); err != nil {
			t.Fatalf("Resume(running) error = %v", err)
		}
		if got := store.mustRun(t, running.ID); got.PauseRequested {
			t.Fatalf("running PauseRequested = true, want false")
		}
		if err := svc.Resume(context.Background(), "ws-1", paused.ID, humanActor(t)); err != nil {
			t.Fatalf("Resume(paused) error = %v", err)
		}
		if got := store.mustRun(t, paused.ID); got.Status != loop.StatusRunning || got.PauseRequested {
			t.Fatalf("paused after resume = %#v, want running and not requested", got)
		}
	})

	t.Run("Should approve or reject only needs approval runs", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		approvable := seedFakeRun(store, loop.StatusNeedsApproval)
		approvable.ID = "run-approve"
		approvable.ActiveGateID = "review_gate"
		approvable.ActiveHumanCriteria = []byte(`[{"id":"human","type":"human","outcome":"awaiting_approval"}]`)
		store.seed(approvable)
		rejectable := seedFakeRun(store, loop.StatusNeedsApproval)
		rejectable.ID = "run-reject"
		rejectable.ActiveGateID = "review_gate"
		rejectable.ActiveHumanCriteria = []byte(`[{"id":"human","type":"human","outcome":"awaiting_approval"}]`)
		store.seed(rejectable)
		running := seedFakeRun(store, loop.StatusRunning)
		running.ID = "run-running-approve"
		store.seed(running)
		svc := newTestService(t, store, validDefinition())
		actor := humanActor(t)

		err := svc.Approve(
			context.Background(),
			"ws-1",
			approvable.ID,
			"review_gate",
			loop.GateDecisionApprove,
			actor,
		)
		if err != nil {
			t.Fatalf("Approve(approve) error = %v", err)
		}
		if got := store.mustRun(t, approvable.ID); got.Status != loop.StatusRunning {
			t.Fatalf("approved status = %q, want running", got.Status)
		}
		decisions, err := store.ListLoopGateDecisions(context.Background(), "ws-1", approvable.ID, 0, "review_gate")
		if err != nil {
			t.Fatalf("ListLoopGateDecisions(approve) error = %v", err)
		}
		if decisions["human"].Decision != gate.HumanDecisionApprove {
			t.Fatalf("decision[human] = %#v, want approve", decisions["human"])
		}
		err = svc.Approve(
			context.Background(),
			"ws-1",
			rejectable.ID,
			"review_gate",
			loop.GateDecisionReject,
			actor,
		)
		if err != nil {
			t.Fatalf("Approve(reject) error = %v", err)
		}
		if got := store.mustRun(t, rejectable.ID); got.Status != loop.StatusBlocked {
			t.Fatalf("rejected status = %q, want blocked", got.Status)
		}
		err = svc.Approve(
			context.Background(),
			"ws-1",
			running.ID,
			"review_gate",
			loop.GateDecisionApprove,
			actor,
		)
		if !errors.Is(err, loop.ErrInvalidTransition) {
			t.Fatalf("Approve(running) error = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("Should reject approvals for a stale gate id", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusNeedsApproval)
		run.ID = "run-stale-gate"
		run.ActiveGateID = "current_gate"
		run.ActiveHumanCriteria = []byte(`[{"id":"human","type":"human","outcome":"awaiting_approval"}]`)
		store.seed(run)
		svc := newTestService(t, store, validDefinition())

		err := svc.Approve(
			context.Background(),
			"ws-1",
			run.ID,
			"old_gate",
			loop.GateDecisionApprove,
			humanActor(t),
		)
		if !errors.Is(err, loop.ErrInvalidTransition) {
			t.Fatalf("Approve(stale gate) error = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("Should reject request changes for synthetic budget gate", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusNeedsApproval)
		run.ID = "run-budget-request-changes"
		run.ActiveGateID = loop.BudgetGateID
		store.seed(run)
		svc := newTestService(t, store, validDefinition())

		err := svc.Approve(
			context.Background(),
			"ws-1",
			run.ID,
			loop.BudgetGateID,
			loop.GateDecisionRequestChanges,
			humanActor(t),
		)
		if !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("Approve(budget request_changes) error = %v, want ErrValidation", err)
		}
		if got := store.mustRun(t, run.ID); got.Status != loop.StatusNeedsApproval {
			t.Fatalf("budget run status = %q, want needs-approval", got.Status)
		}
	})
}

func TestServiceShouldAllocateTypedGoalGrantsFromDurableControl(t *testing.T) {
	t.Run("Should extend the effective turn limit by the pinned node limit exactly once", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusNeedsApproval)
		run.ID = "run-goal-turn-extension"
		run.ActiveGateID = loop.SyntheticGoalGateID(
			"converge",
			1,
			0,
			loop.ReasonCodeGoalTurnsExhausted,
		)
		definition := singleNodeDefinition(validGoalNode("converge", ""))
		definition.Graph.Nodes[0].Params["max_turns"] = 4
		resolved, err := loop.NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile(Goal definition) error = %v", err)
		}
		effective, err := loop.ResolveEffectiveConfig(
			resolved,
			loop.DefaultLoopDefaults(),
			nil,
			loop.LoopConfig{},
		)
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig() error = %v", err)
		}
		encoded, digest, err := loop.BuildExecutedDefinitionSnapshot(resolved, effective)
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		run.DefinitionVersion = resolved.DefinitionVersion
		run.DefinitionDigest = digest
		store.seed(run)
		store.snapshots[string(run.WorkspaceID)+"/"+run.DefinitionDigest] = loop.DefinitionSnapshot{
			WorkspaceID: run.WorkspaceID,
			Digest:      run.DefinitionDigest,
			Version:     run.DefinitionVersion,
			Definition:  encoded,
		}
		store.goalControl = &loop.GoalControlState{
			WorkspaceID:  run.WorkspaceID,
			LoopRunID:    run.ID,
			Generation:   1,
			NodeID:       "converge",
			ControlEpoch: 1,
			GoalStatus:   "budget-limited",
			TurnsUsed:    4,
			TurnLimit:    4,
			TaskRunID:    "goal-segment-1",
			GateID:       run.ActiveGateID,
			Cause:        loop.ReasonCodeGoalTurnsExhausted,
			RunStatus:    loop.StatusNeedsApproval,
		}
		var activated task.Run
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithGoalRunActivator(loop.GoalRunActivatorFunc(func(_ context.Context, run task.Run) {
				activated = run
			})),
		)
		if err := svc.Approve(
			context.Background(),
			run.WorkspaceID,
			run.ID,
			run.ActiveGateID,
			loop.GateDecisionApprove,
			humanActor(t),
		); err != nil {
			t.Fatalf("Approve(turn extension) error = %v", err)
		}
		grant := store.lastGoalReactivation(t)
		if grant.Kind != loop.GoalGrantTurnExtension || grant.Scope != loop.GoalGrantScopeTurnLimit ||
			grant.TurnIncrement != 4 {
			t.Fatalf("turn-extension grant = %#v", grant)
		}
		if len(grant.Decisions) != 1 || grant.Decisions[0].GateID != run.ActiveGateID {
			t.Fatalf("turn-extension decisions = %#v", grant.Decisions)
		}
		if activated.ID != "goal-successor" {
			t.Fatalf("activated Goal successor = %#v, want goal-successor", activated)
		}
	})

	t.Run("Should map approval causes to exact budget and reseed grant scopes", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name      string
			cause     loop.ReasonCode
			openTurn  bool
			wantKind  loop.GoalGrantKind
			wantScope loop.GoalGrantScope
		}{
			{
				name: "session-origin reseed", cause: loop.ReasonCodeGoalReseedConfirmationRequired,
				wantKind: loop.GoalGrantReseed, wantScope: loop.GoalGrantScopeRotateBinding,
			},
			{
				name: "pre-submit budget", cause: loop.ReasonCodeGoalBudgetFenced,
				wantKind: loop.GoalGrantBudget, wantScope: loop.GoalGrantScopeWorkAndSettle,
			},
			{
				name: "open-turn budget", cause: loop.ReasonCodeGoalBudgetFenced, openTurn: true,
				wantKind: loop.GoalGrantBudget, wantScope: loop.GoalGrantScopeSettleCurrent,
			},
		}
		for _, tc := range cases {
			t.Run("Should allocate "+tc.name, func(t *testing.T) {
				t.Parallel()

				store := newFakeLoopStore()
				run := seedFakeRun(store, loop.StatusNeedsApproval)
				run.ID = loop.RunID("run-goal-grant-" + strings.ReplaceAll(tc.name, " ", "-"))
				run.ActiveGateID = loop.SyntheticGoalGateID("converge", 1, 0, tc.cause)
				store.seed(run)
				store.goalControl = &loop.GoalControlState{
					WorkspaceID: run.WorkspaceID, LoopRunID: run.ID, Generation: 1,
					NodeID: "converge", ControlEpoch: 1, GoalStatus: "awaiting-control",
					TurnsUsed: 1, TurnLimit: 3, TaskRunID: "goal-segment-1",
					GateID: run.ActiveGateID, Cause: tc.cause, OpenTurn: tc.openTurn,
					RunStatus: loop.StatusNeedsApproval,
				}
				svc := newTestService(t, store, validDefinition())
				if err := svc.Approve(
					context.Background(), run.WorkspaceID, run.ID, run.ActiveGateID,
					loop.GateDecisionApprove, humanActor(t),
				); err != nil {
					t.Fatalf("Approve(%s) error = %v", tc.name, err)
				}
				grant := store.lastGoalReactivation(t)
				if grant.Kind != tc.wantKind || grant.Scope != tc.wantScope || grant.TurnIncrement != 0 {
					t.Fatalf("%s grant = %#v", tc.name, grant)
				}
			})
		}
	})

	t.Run("Should consume a plain Resume grant at successor reactivation", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusPaused)
		run.ID = "run-goal-plain-resume"
		store.seed(run)
		store.goalControl = &loop.GoalControlState{
			WorkspaceID: run.WorkspaceID, LoopRunID: run.ID, Generation: 1,
			NodeID: "converge", ControlEpoch: 1, GoalStatus: "paused",
			TurnsUsed: 1, TurnLimit: 3, TaskRunID: "goal-segment-1",
			Cause: loop.ReasonCode(loop.TransitionCausePauseBoundary), RunStatus: loop.StatusPaused,
		}
		svc := newTestService(t, store, validDefinition())
		if err := svc.Resume(context.Background(), run.WorkspaceID, run.ID, humanActor(t)); err != nil {
			t.Fatalf("Resume(plain Goal control) error = %v", err)
		}
		grant := store.lastGoalReactivation(t)
		if grant.Kind != loop.GoalGrantPlainResume || grant.Scope != loop.GoalGrantScopeReactivate ||
			grant.TurnIncrement != 0 {
			t.Fatalf("plain-resume grant = %#v", grant)
		}
	})
}

func TestServiceConfigMethodsShouldReadWriteRawOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Should configure clamp and read loop config", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc := newTestService(t, store, validDefinition())

		if err := svc.Configure(context.Background(), "ws-1", "valid-loop", loop.LoopConfig{
			EnabledChecks: []byte(`{"human":true}`),
			FanOutWidth:   new(loop.LoopMaxFanoutWidth + 1),
		}); err != nil {
			t.Fatalf("Configure() error = %v", err)
		}
		cfg, err := svc.GetConfig(context.Background(), "ws-1", "valid-loop")
		if err != nil {
			t.Fatalf("GetConfig() error = %v", err)
		}
		if cfg.FanOutWidth == nil || *cfg.FanOutWidth != loop.LoopMaxFanoutWidth {
			t.Fatalf("FanOutWidth = %#v, want clamped %d", cfg.FanOutWidth, loop.LoopMaxFanoutWidth)
		}
		if string(cfg.EnabledChecks) != `{"human":true}` {
			t.Fatalf("EnabledChecks = %s, want persisted JSON", cfg.EnabledChecks)
		}
		snapshot, err := svc.GetConfigSnapshot(context.Background(), "ws-1", "valid-loop")
		if err != nil {
			t.Fatalf("GetConfigSnapshot() error = %v", err)
		}
		if snapshot.Stored == nil || snapshot.Stored.FanOutWidth == nil ||
			*snapshot.Stored.FanOutWidth != loop.LoopMaxFanoutWidth {
			t.Fatalf("stored config = %#v, want clamped override", snapshot.Stored)
		}
		if snapshot.Effective.FanOutWidth != loop.LoopMaxFanoutWidth ||
			snapshot.Effective.GateMaxRevisions != 10 {
			t.Fatalf("effective config = %#v, want stored fan-out and delivery gate default", snapshot.Effective)
		}
	})

	t.Run("Should reject invalid config JSON", func(t *testing.T) {
		t.Parallel()

		svc := newTestService(t, newFakeLoopStore(), validDefinition())
		err := svc.Configure(context.Background(), "ws-1", "valid-loop", loop.LoopConfig{
			EnabledChecks: []byte(`{`),
		})
		if !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("Configure(invalid JSON) error = %v, want ErrValidation", err)
		}
	})
}

func TestServiceGetAndDefaultsShouldExposeRunState(t *testing.T) {
	t.Parallel()

	t.Run("Should return a workspace scoped run", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		got, err := svc.Get(context.Background(), "ws-1", run.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.ID != run.ID {
			t.Fatalf("Get().ID = %q, want %q", got.ID, run.ID)
		}
	})

	t.Run("Should use injected defaults in start effective config", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc := newTestServiceWithOptions(t, store, validDefinition(), loop.WithDefaults(loop.LoopDefaults{
			Delivery: loop.LoopConfig{BudgetTokens: new(4444)},
		}))

		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if run.BudgetTokens != 4444 {
			t.Fatalf("BudgetTokens = %d, want injected default 4444", run.BudgetTokens)
		}
	})
}

func TestResolveInputsShouldApplyDefaultsAndValidateTypes(t *testing.T) {
	t.Parallel()

	t.Run("Should apply defaults and clone nested values", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		def.Inputs = map[string]dsl.Input{
			"name": {Type: dsl.InputTypeString, Required: true},
			"flag": {Type: dsl.InputTypeBoolean, Default: true},
		}
		resolved, err := loop.ResolveInputs(def, loop.Inputs{Values: map[string]any{"name": "loop"}})
		if err != nil {
			t.Fatalf("ResolveInputs() error = %v", err)
		}
		if resolved["flag"] != true {
			t.Fatalf("resolved flag = %#v, want true", resolved["flag"])
		}
	})

	t.Run("Should reject invalid declared input types", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		def.Inputs = map[string]dsl.Input{"count": {Type: dsl.InputTypeNumber, Required: true}}
		_, err := loop.ResolveInputs(def, loop.Inputs{Values: map[string]any{"count": "not-a-number"}})
		if !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("ResolveInputs() error = %v, want ErrValidation", err)
		}
	})
}

func TestServiceDryRunShouldReturnPlanPreviewWithoutState(t *testing.T) {
	t.Parallel()

	t.Run("Should return plan preview without creating loop run state", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc := newTestService(t, store, validDefinition())

		preview, err := svc.DryRun(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		})
		if err != nil {
			t.Fatalf("DryRun() error = %v", err)
		}
		if preview.Generation != 1 {
			t.Fatalf("Generation = %d, want 1", preview.Generation)
		}
		if got, want := preview.ResolvedInputs["tasks"], "task-ref"; got != want {
			t.Fatalf("ResolvedInputs[tasks] = %#v, want %q", got, want)
		}
		if len(preview.Nodes) == 0 {
			t.Fatal("Nodes length = 0, want materialized gen-1 nodes")
		}
		if store.createCount() != 0 {
			t.Fatalf("CreateLoopRun calls = %d, want 0", store.createCount())
		}
	})
}

func TestCostShouldBeDisplayOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should derive cost as tokens multiplied by price", func(t *testing.T) {
		t.Parallel()

		cost := loop.DeriveDisplayCost(1500, 0.000002)
		if math.Abs(cost.USD-0.003) > 0.0000001 {
			t.Fatalf("USD = %.9f, want 0.003", cost.USD)
		}
	})

	t.Run("Should not expose a budget USD enforcement field", func(t *testing.T) {
		t.Parallel()

		if _, ok := reflect.TypeFor[loop.EffectiveConfig]().FieldByName("BudgetUSD"); ok {
			t.Fatal("EffectiveConfig exposes BudgetUSD, want token/wall budget enforcement only")
		}
	})
}

func inlineGoalDefinition(objective string, judgeModel string) dsl.Definition {
	definition := validDefinition()
	definition.Meta.Name = loop.InlineGoalLoopName
	definition.Meta.Version = 1
	definition.Concurrency = dsl.ConcurrencyAllow
	definition.Inputs = map[string]dsl.Input{}
	definition.Contract.Goal = objective
	definition.Contract.DefinitionOfDone = "The objective is satisfied according to the Goal judge."
	definition.Contract.ModelDefaults = nil
	if strings.TrimSpace(judgeModel) != "" {
		definition.Contract.ModelDefaults = &dsl.ModelDefaults{Judge: judgeModel}
	}
	definition.Graph = dsl.Graph{Nodes: []dsl.Node{{
		ID:    "goal",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionGoal),
		Session: &dsl.SessionSpec{
			Mode: dsl.SessionModeContinuous,
		},
		Params: dsl.NodeParams{
			"agent":     "operator-agent",
			"objective": objective,
			"judge": []dsl.GateCriterion{{
				ID: "objective_satisfied", Type: dsl.CriterionAgentJudge, Rubric: objective,
			}},
			"max_turns": 3,
			"output_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{
						"type": "string", "enum": []string{"complete", "blocked"},
					},
				},
			},
		},
	}}, Edges: []dsl.Edge{}}
	return definition
}

func inlineGoalOrigin(sessionID string) loop.RunOrigin {
	return loop.RunOrigin{
		Kind:               loop.RunOriginSession,
		SessionID:          sessionID,
		CreationProfileRef: "profile:sha256:origin",
		PolicySpecDigest:   "policy:sha256:origin",
		CreationDigest:     "creation:sha256:origin",
	}
}

func TestServiceConstructorAndReasonErrorsShouldBeStable(t *testing.T) {
	t.Parallel()

	t.Run("Should reject missing service dependencies", func(t *testing.T) {
		t.Parallel()

		resolver := loop.DefinitionResolverFunc(func(
			context.Context,
			loop.WorkspaceID,
			string,
		) (*loop.ResolvedDefinition, error) {
			return nil, nil
		})
		goalPolicy := testGoalRunPolicyResolver(0.8)
		if _, err := loop.NewService(nil, resolver, goalPolicy); !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("NewService(nil store) error = %v, want ErrValidation", err)
		}
		if _, err := loop.NewService(newFakeLoopStore(), nil, goalPolicy); !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("NewService(nil resolver) error = %v, want ErrValidation", err)
		}
		if _, err := loop.NewService(newFakeLoopStore(), resolver, nil); !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("NewService(nil Goal policy resolver) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should render reason errors with deterministic metadata", func(t *testing.T) {
		t.Parallel()

		err := &loop.ReasonError{
			Code: loop.ReasonCodeActiveRunExists,
			Err:  loop.ErrConcurrencyConflict,
			Meta: map[string]string{
				"status":        "running",
				"active_run_id": "run-1",
			},
		}
		want := "active_loop_run_exists: loop: concurrency conflict " +
			"(active_run_id=run-1, status=running)"
		if got := err.Error(); got != want {
			t.Fatalf("ReasonError.Error() = %q, want %q", got, want)
		}
		if !errors.Is(err, loop.ErrConcurrencyConflict) {
			t.Fatalf("ReasonError does not unwrap ErrConcurrencyConflict")
		}
	})
}

func TestServiceStopShouldFailRunWithOperatorCause(t *testing.T) {
	t.Parallel()

	t.Run("Should transition a live run to failed with operator stop cause", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		if err := svc.Stop(context.Background(), "ws-1", run.ID, loop.StopReasonOperator, humanActor(t)); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stored := store.mustRun(t, run.ID)
		if stored.Status != loop.StatusFailed {
			t.Fatalf("stored status = %q, want failed", stored.Status)
		}
		transition := store.lastTransition(t)
		if transition.cause != loop.TransitionCauseOperatorStop {
			t.Fatalf("transition cause = %q, want operator_stop", transition.cause)
		}
	})

	t.Run("Should reject stop for an already terminal run", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusDone)
		svc := newTestService(t, store, validDefinition())

		err := svc.Stop(context.Background(), "ws-1", run.ID, loop.StopReasonOperator, humanActor(t))
		if !errors.Is(err, loop.ErrInvalidTransition) {
			t.Fatalf("Stop() error = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("Should reject an unknown stop reason", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		err := svc.Stop(context.Background(), "ws-1", run.ID, loop.StopReason("unknown"), humanActor(t))
		if !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("Stop(unknown reason) error = %v, want ErrValidation", err)
		}
		if got := store.mustRun(t, run.ID); got.Status != loop.StatusRunning {
			t.Fatalf("stored status = %q, want original running", got.Status)
		}
	})

	t.Run("Should atomically stop Goal state before revoking its exact prompt lease", func(t *testing.T) {
		t.Parallel()

		base := newFakeLoopStore()
		run := seedFakeRun(base, loop.StatusRunning)
		lease := loop.GoalPromptLease{
			QueueEntryID: "queue-goal-stop", SessionID: "session-goal-stop", OwnerKind: "goal",
			LoopRunID: string(run.ID), TaskRunID: "task-run-goal-stop", RunGeneration: 1,
			PromptAttempt: 2, ControlEpoch: 3, BindingEpoch: 4,
			PromptID: "prompt-goal-stop", PromptKind: "work",
		}
		store := &atomicGoalStopStore{
			fakeLoopStore: base,
			result: loop.GoalRunStopResult{
				RevokedPromptLeases: []loop.GoalPromptLease{lease},
			},
		}
		var revoked []loop.GoalPromptLease
		var reasons []string
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithGoalPromptLeaseRevoker(loop.GoalPromptLeaseRevokerFunc(
				func(_ context.Context, got loop.GoalPromptLease, reason string) error {
					revoked = append(revoked, got)
					reasons = append(reasons, reason)
					return nil
				},
			)),
		)
		actor := humanActor(t)
		if err := svc.Stop(context.Background(), run.WorkspaceID, run.ID, loop.StopReasonOperator, actor); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if got := base.mustRun(t, run.ID); got.Status != loop.StatusFailed {
			t.Fatalf("stored status = %q, want failed", got.Status)
		}
		request := store.lastRequest(t)
		if request.WorkspaceID != run.WorkspaceID || request.RunID != run.ID ||
			request.ExpectedStatus != loop.StatusRunning || request.Actor != actor || request.StoppedAt.IsZero() {
			t.Fatalf("atomic Goal stop request = %#v", request)
		}
		if !reflect.DeepEqual(revoked, []loop.GoalPromptLease{lease}) ||
			!reflect.DeepEqual(reasons, []string{string(loop.TransitionCauseOperatorStop)}) {
			t.Fatalf("post-commit revocations = %#v reasons = %#v", revoked, reasons)
		}
	})

	t.Run("Should not revoke a Goal prompt lease when the atomic stop rolls back", func(t *testing.T) {
		t.Parallel()

		base := newFakeLoopStore()
		run := seedFakeRun(base, loop.StatusRunning)
		wantErr := errors.New("atomic Goal stop failed")
		store := &atomicGoalStopStore{fakeLoopStore: base, err: wantErr}
		var revokeCalls int
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithGoalPromptLeaseRevoker(loop.GoalPromptLeaseRevokerFunc(func(
				context.Context,
				loop.GoalPromptLease,
				string,
			) error {
				revokeCalls++
				return nil
			})),
		)
		if err := svc.Stop(
			context.Background(), run.WorkspaceID, run.ID, loop.StopReasonOperator, humanActor(t),
		); !errors.Is(err, wantErr) {
			t.Fatalf("Stop() error = %v, want %v", err, wantErr)
		}
		if revokeCalls != 0 {
			t.Fatalf("post-commit revoke calls = %d, want 0", revokeCalls)
		}
		if got := base.mustRun(t, run.ID); got.Status != loop.StatusRunning {
			t.Fatalf("stored status = %q, want running", got.Status)
		}
	})
}

func newTestService(t *testing.T, store *fakeLoopStore, def dsl.Definition) loop.Service {
	t.Helper()

	return newTestServiceWithOptions(t, store, def)
}

func newTestServiceWithOptions(
	t *testing.T,
	store loop.Store,
	def dsl.Definition,
	opts ...loop.Option,
) loop.Service {
	t.Helper()

	resolved := compileDefinition(t, def)
	options := []loop.Option{
		loop.WithClock(func() time.Time {
			return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
		}),
		loop.WithRunIDFactory(func() loop.RunID {
			return loop.RunID("looprun-test")
		}),
	}
	options = append(options, opts...)
	svc, err := loop.NewService(
		store,
		loop.DefinitionResolverFunc(func(
			context.Context,
			loop.WorkspaceID,
			string,
		) (*loop.ResolvedDefinition, error) {
			return resolved, nil
		}),
		testGoalRunPolicyResolver(0.8),
		options...,
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}

func testGoalRunPolicyResolver(ratio float64) loop.GoalRunPolicyResolver {
	return loop.GoalRunPolicyResolverFunc(func(
		context.Context,
		loop.WorkspaceID,
	) (*loop.GoalRunPolicy, error) {
		return &loop.GoalRunPolicy{ContextNudgeRatio: ratio}, nil
	})
}

func liveLoopTestDefinition() dsl.Definition {
	definition := validDefinition()
	live := participation.ModeLive
	loopRun := participation.StrategyLoopRun
	definition.NetworkParticipation = &participation.Request{
		Mode:            &live,
		ChannelStrategy: &loopRun,
	}
	definition.Graph.Nodes[2].Harvest = &dsl.HarvestSpec{
		Kind:   "channel_result",
		Window: "30s",
	}
	return definition
}

func loopTestParticipationResolver(t *testing.T, available bool) participation.Resolver {
	t.Helper()
	defaults := participation.Bounds{
		MaxWakes:         4,
		MaxWakeWallTime:  "30s",
		MaxTotalWallTime: "2m",
		MaxInputTokens:   4096,
		MaxOutputTokens:  4096,
		MaxWakeDepth:     4,
		CoalesceWindow:   "250ms",
	}
	resolver, err := participation.NewResolver(participation.ResolverOptions{
		Defaults: defaults,
		Limits: participation.Limits{
			MaxWakes:          16,
			MaxWakeWallTime:   "5m",
			MaxTotalWallTime:  "30m",
			MaxInputTokens:    1_000_000,
			MaxOutputTokens:   1_000_000,
			MaxWakeDepth:      16,
			MinCoalesceWindow: "10ms",
			MaxCoalesceWindow: "5s",
		},
		Availability: func(context.Context) (bool, error) {
			return available, nil
		},
	})
	if err != nil {
		t.Fatalf("participation.NewResolver() error = %v", err)
	}
	return resolver
}

func newParticipationTestService(
	t *testing.T,
	store loop.Store,
	definition dsl.Definition,
	opts ...loop.Option,
) loop.Service {
	t.Helper()
	resolved := compileDefinition(t, validDefinition())
	resolved.Definition = definition
	options := []loop.Option{
		loop.WithClock(func() time.Time {
			return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
		}),
		loop.WithRunIDFactory(func() loop.RunID { return "looprun-test" }),
	}
	options = append(options, opts...)
	svc, err := loop.NewService(
		store,
		loop.DefinitionResolverFunc(func(
			context.Context,
			loop.WorkspaceID,
			string,
		) (*loop.ResolvedDefinition, error) {
			return resolved, nil
		}),
		testGoalRunPolicyResolver(0.8),
		options...,
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}

type participationLifecycleHookDispatcher struct {
	loop.HookDispatcher
	started  hookspkg.LoopStartedPayload
	terminal hookspkg.LoopTerminalPayload
}

func (d *participationLifecycleHookDispatcher) DispatchLoopStarted(
	_ context.Context,
	payload hookspkg.LoopStartedPayload,
) (hookspkg.LoopStartedPayload, error) {
	d.started = payload
	return payload, nil
}

func (d *participationLifecycleHookDispatcher) DispatchLoopTerminal(
	_ context.Context,
	payload hookspkg.LoopTerminalPayload,
) (hookspkg.LoopTerminalPayload, error) {
	d.terminal = payload
	return payload, nil
}

type atomicGoalStopStore struct {
	*fakeLoopStore
	mu       sync.Mutex
	requests []loop.GoalRunStopRequest
	result   loop.GoalRunStopResult
	err      error
}

func (s *atomicGoalStopStore) StopGoalRun(
	ctx context.Context,
	request loop.GoalRunStopRequest,
) (loop.GoalRunStopResult, error) {
	s.mu.Lock()
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	if s.err != nil {
		return loop.GoalRunStopResult{}, s.err
	}
	if err := s.CompareAndSwapLoopRunStatus(
		ctx,
		request.RunID,
		request.ExpectedStatus,
		loop.StatusFailed,
		loop.TransitionCauseOperatorStop,
		request.StoppedAt,
	); err != nil {
		return loop.GoalRunStopResult{}, err
	}
	return s.result, nil
}

func (s *atomicGoalStopStore) lastRequest(t *testing.T) loop.GoalRunStopRequest {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		t.Fatal("atomic Goal stop request count = 0, want 1")
	}
	return s.requests[len(s.requests)-1]
}

func compileDefinition(t *testing.T, def dsl.Definition) *loop.ResolvedDefinition {
	t.Helper()

	resolved, err := loop.NewCompiler(
		loop.WithCompilerToolSchemaSource(fakeToolSchemas{}),
	).Compile(def)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return resolved
}

func seedFakeRun(store *fakeLoopStore, status loop.Status) loop.Run {
	run := loop.Run{
		ID:                loop.RunID("run-" + string(status)),
		WorkspaceID:       "ws-1",
		LoopName:          "valid-loop",
		Status:            status,
		ReattemptStrategy: loop.ReattemptFailedOnly,
		CreatedAt:         time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		LastProgressAt:    time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		BudgetOnExceeded:  dsl.BudgetExceededHalt,
		Inputs:            map[string]any{"tasks": "task-ref"},
	}
	store.seed(run)
	return run
}

type fakeTransition struct {
	runID loop.RunID
	from  loop.Status
	to    loop.Status
	cause loop.TransitionCause
}

type fakeLoopStore struct {
	mu                               sync.Mutex
	runs                             map[loop.RunID]loop.Run
	configs                          map[string]loop.LoopConfig
	snapshots                        map[string]loop.DefinitionSnapshot
	decisions                        map[string]map[string]gate.HumanDecision
	transitions                      []fakeTransition
	goalControl                      *loop.GoalControlState
	goalReactivations                []loop.GoalReactivationRequest
	inlineReplaceRevokedPromptLeases []loop.GoalPromptLease
	creates                          int
}

func newFakeLoopStore() *fakeLoopStore {
	return &fakeLoopStore{
		runs:      map[loop.RunID]loop.Run{},
		configs:   map[string]loop.LoopConfig{},
		snapshots: map[string]loop.DefinitionSnapshot{},
		decisions: map[string]map[string]gate.HumanDecision{},
	}
}

func (s *fakeLoopStore) seed(run loop.Run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
}

func (s *fakeLoopStore) mustRun(t *testing.T, id loop.RunID) loop.Run {
	t.Helper()
	run, err := s.GetLoopRunByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLoopRunByID(%q) error = %v", id, err)
	}
	return run
}

func (s *fakeLoopStore) lastTransition(t *testing.T) fakeTransition {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.transitions) == 0 {
		t.Fatal("transition count = 0, want at least one")
	}
	return s.transitions[len(s.transitions)-1]
}

func (s *fakeLoopStore) lastGoalReactivation(t *testing.T) loop.GoalReactivationRequest {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.goalReactivations) == 0 {
		t.Fatal("Goal reactivation count = 0, want at least one")
	}
	return s.goalReactivations[len(s.goalReactivations)-1]
}

func (s *fakeLoopStore) createCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.creates
}

func (s *fakeLoopStore) CreateLoopRunForStart(
	_ context.Context,
	run loop.Run,
	policy dsl.ConcurrencyPolicy,
) (loop.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy == "" {
		policy = dsl.ConcurrencyForbid
	}
	active := s.activeLoopRunLocked(run.WorkspaceID, run.LoopName)
	switch policy {
	case dsl.ConcurrencyForbid:
		if active != nil {
			return loop.Run{}, &loop.ReasonError{
				Code: loop.ReasonCodeActiveRunExists,
				Err:  loop.ErrConcurrencyConflict,
				Meta: map[string]string{
					"active_run_id": string(active.ID),
					"loop_name":     active.LoopName,
					"status":        string(active.Status),
				},
			}
		}
		run.Status = loop.StatusRunning
	case dsl.ConcurrencyQueue:
		if active != nil {
			run.Status = loop.StatusQueued
		} else {
			run.Status = loop.StatusRunning
		}
	case dsl.ConcurrencyAllow:
		run.Status = loop.StatusRunning
	default:
		return loop.Run{}, loop.ErrValidation
	}
	s.creates++
	s.runs[run.ID] = run
	s.snapshots[string(run.WorkspaceID)+"/"+run.DefinitionDigest] = loop.DefinitionSnapshot{
		WorkspaceID: run.WorkspaceID,
		Digest:      run.DefinitionDigest,
		Version:     run.DefinitionVersion,
		Definition:  run.DefinitionSnapshot,
		ByteSize:    len(run.DefinitionSnapshot),
		CreatedAt:   run.StartedAt,
		LastUsedAt:  run.StartedAt,
	}
	return run, nil
}

func (s *fakeLoopStore) CreateInlineLoopRunForStart(
	ctx context.Context,
	run loop.Run,
) (loop.Run, error) {
	return s.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
}

func (s *fakeLoopStore) ReplaceInlineLoopRun(
	_ context.Context,
	request loop.InlineReplaceStoreRequest,
) (loop.InlineReplaceStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Run == nil {
		return loop.InlineReplaceStoreResult{}, loop.ErrValidation
	}
	old, ok := s.runs[request.ExpectedRunID]
	if !ok || old.WorkspaceID != request.Run.WorkspaceID || !old.Status.Live() ||
		old.Origin.Kind != loop.RunOriginSession || old.Origin.SessionID != request.Run.Origin.SessionID {
		return loop.InlineReplaceStoreResult{}, &loop.ReasonError{
			Code: loop.ReasonCodeGoalReplaceStale,
			Err:  loop.ErrTransitionConflict,
		}
	}
	old.Status = loop.StatusFailed
	s.runs[old.ID] = old
	created := *request.Run
	created.Status = loop.StatusRunning
	s.runs[created.ID] = created
	s.creates++
	s.snapshots[string(created.WorkspaceID)+"/"+created.DefinitionDigest] = loop.DefinitionSnapshot{
		WorkspaceID: created.WorkspaceID,
		Digest:      created.DefinitionDigest,
		Version:     created.DefinitionVersion,
		Definition:  created.DefinitionSnapshot,
		ByteSize:    len(created.DefinitionSnapshot),
		CreatedAt:   created.StartedAt,
		LastUsedAt:  created.StartedAt,
	}
	return loop.InlineReplaceStoreResult{
		ReplacedRunID: old.ID,
		ReplacedRun:   old,
		Run:           created,
		RevokedPromptLeases: append(
			[]loop.GoalPromptLease(nil),
			s.inlineReplaceRevokedPromptLeases...,
		),
	}, nil
}

func (s *fakeLoopStore) ClearInlineGoal(
	_ context.Context,
	request loop.InlineGoalClearStoreRequest,
) (loop.InlineGoalClearStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var newest *loop.Run
	for _, candidate := range s.runs {
		if candidate.WorkspaceID != request.WorkspaceID || candidate.Origin == nil ||
			candidate.Origin.Kind != loop.RunOriginSession ||
			candidate.Origin.SessionID != request.OriginSessionID {
			continue
		}
		current := candidate
		if newest == nil || current.CreatedAt.After(newest.CreatedAt) {
			newest = &current
		}
	}
	if newest == nil {
		return loop.InlineGoalClearStoreResult{}, &loop.ReasonError{
			Code: loop.ReasonCodeGoalNotActive,
			Err:  loop.ErrTransitionConflict,
		}
	}
	result := loop.InlineGoalClearStoreResult{Run: *newest}
	if newest.Status.Live() {
		newest.Status = loop.StatusFailed
		s.runs[newest.ID] = *newest
		result.Run = *newest
		result.Terminalized = true
	}
	return result, nil
}

func (s *fakeLoopStore) GetLoopRun(
	_ context.Context,
	ws loop.WorkspaceID,
	runID loop.RunID,
) (loop.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok || run.WorkspaceID != ws {
		return loop.Run{}, loop.ErrRunNotFound
	}
	return run, nil
}

func (s *fakeLoopStore) GetLoopRunByID(_ context.Context, runID loop.RunID) (loop.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return loop.Run{}, loop.ErrRunNotFound
	}
	return run, nil
}

func (s *fakeLoopStore) LoadAwaitingGoalControl(
	_ context.Context,
	workspaceID loop.WorkspaceID,
	runID loop.RunID,
) (loop.GoalControlState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.goalControl == nil || s.goalControl.WorkspaceID != workspaceID || s.goalControl.LoopRunID != runID {
		return loop.GoalControlState{}, false, nil
	}
	return *s.goalControl, true, nil
}

func (s *fakeLoopStore) ReactivateGoalRun(
	_ context.Context,
	req loop.GoalReactivationRequest,
) (loop.GoalReactivationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.goalControl == nil || s.goalControl.ControlEpoch != req.State.ControlEpoch {
		return loop.GoalReactivationResult{}, loop.ErrTransitionConflict
	}
	s.goalReactivations = append(s.goalReactivations, req)
	run, ok := s.runs[req.State.LoopRunID]
	if !ok {
		return loop.GoalReactivationResult{}, loop.ErrRunNotFound
	}
	run.Status = loop.StatusRunning
	run.ActiveGateID = ""
	run.ActiveHumanCriteria = []byte(`[]`)
	s.runs[run.ID] = run
	s.goalControl = nil
	return loop.GoalReactivationResult{
		Run:          task.Run{ID: "goal-successor"},
		ControlEpoch: req.State.ControlEpoch + 1,
		GrantID:      int64(len(s.goalReactivations)),
	}, nil
}

func (s *fakeLoopStore) GetLoopDefinitionSnapshot(
	_ context.Context,
	ws loop.WorkspaceID,
	digest string,
) (loop.DefinitionSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.snapshots[string(ws)+"/"+digest]
	if !ok {
		return loop.DefinitionSnapshot{}, loop.ErrRunNotFound
	}
	return snapshot, nil
}

func (s *fakeLoopStore) FindActiveLoopRun(
	_ context.Context,
	ws loop.WorkspaceID,
	loopName string,
) (*loop.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeLoopRunLocked(ws, loopName), nil
}

func (s *fakeLoopStore) activeLoopRunLocked(ws loop.WorkspaceID, loopName string) *loop.Run {
	var oldest *loop.Run
	for _, run := range s.runs {
		if run.WorkspaceID != ws || run.LoopName != loopName || !run.Status.Live() {
			continue
		}
		candidate := run
		if oldest == nil || candidate.CreatedAt.Before(oldest.CreatedAt) {
			oldest = &candidate
		}
	}
	return oldest
}

func (s *fakeLoopStore) CompareAndSwapLoopRunStatus(
	_ context.Context,
	runID loop.RunID,
	from loop.Status,
	to loop.Status,
	cause loop.TransitionCause,
	_ time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return loop.ErrRunNotFound
	}
	if run.Status != from {
		return loop.ErrTransitionConflict
	}
	run.Status = to
	if to == loop.StatusRunning || to == loop.StatusPaused || to.Terminal() {
		run.PauseRequested = false
		run.ActiveGateID = ""
		run.ActiveHumanCriteria = []byte(`[]`)
	}
	s.runs[runID] = run
	s.transitions = append(
		s.transitions,
		fakeTransition{runID: runID, from: from, to: to, cause: cause},
	)
	return nil
}

func (s *fakeLoopStore) RecordLoopGateDecisions(
	_ context.Context,
	records []loop.GateDecisionRecord,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range records {
		key := gateDecisionKey(record.WorkspaceID, record.RunID, record.Generation, record.GateID)
		if s.decisions[key] == nil {
			s.decisions[key] = map[string]gate.HumanDecision{}
		}
		s.decisions[key][record.CriterionID] = gate.HumanDecision{
			Decision: gate.HumanDecisionKind(record.Decision),
			Actor:    record.Actor,
			Note:     record.Note,
		}
	}
	return nil
}

func (s *fakeLoopStore) ListLoopGateDecisions(
	_ context.Context,
	ws loop.WorkspaceID,
	runID loop.RunID,
	generation int,
	gateID loop.NodeID,
) (map[string]gate.HumanDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.decisions[gateDecisionKey(ws, runID, generation, gateID)]
	out := map[string]gate.HumanDecision{}
	maps.Copy(out, stored)
	return out, nil
}

func gateDecisionKey(ws loop.WorkspaceID, runID loop.RunID, generation int, gateID loop.NodeID) string {
	return string(ws) + "/" + string(runID) + "/" + strconv.Itoa(generation) + "/" + string(gateID)
}

func (s *fakeLoopStore) SetLoopRunPauseRequested(
	_ context.Context,
	ws loop.WorkspaceID,
	runID loop.RunID,
	requested bool,
	actor task.ActorContext,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok || run.WorkspaceID != ws {
		return loop.ErrRunNotFound
	}
	run.PauseRequested = requested
	if requested {
		run.ControlActor = actor.Actor
		run.ControlRequestedAt = time.Now().UTC()
	} else {
		run.ControlActor = task.ActorIdentity{}
		run.ControlRequestedAt = time.Time{}
	}
	s.runs[runID] = run
	return nil
}

func (s *fakeLoopStore) UpsertLoopConfig(
	_ context.Context,
	ws loop.WorkspaceID,
	loopName string,
	cfg loop.LoopConfig,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[string(ws)+"/"+loopName] = cfg
	return nil
}

func (s *fakeLoopStore) GetLoopConfig(
	_ context.Context,
	ws loop.WorkspaceID,
	loopName string,
) (*loop.LoopConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, ok := s.configs[string(ws)+"/"+loopName]
	if !ok {
		return nil, loop.ErrConfigNotFound
	}
	return &cfg, nil
}

func humanActor(t *testing.T) task.ActorContext {
	t.Helper()
	actor, err := task.DeriveHumanActorContext("operator", task.OriginKindCLI, "cli")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	return actor
}
