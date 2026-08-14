package loop_test

import (
	"bytes"
	"context"
	"encoding/json"
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

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/task"
)

func TestNodeLifecycleConfigShouldResolveAndPinAdmissionValues(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve node then loop config then shipped defaults", func(t *testing.T) {
		t.Parallel()

		loopAttempts := 5
		loopBase := 2 * time.Second
		loopConfig := loop.LifecycleConfig{
			RetryMaxAttempts: new(loopAttempts),
			RetryBackoffBase: new(loopBase),
		}
		node := dsl.Node{
			NodeLifecycleState: &dsl.NodeLifecycleState{Deadline: "5m"},
			Retry: &dsl.RetrySpec{
				MaxAttempts: 7,
				Backoff:     &dsl.BackoffSpec{Max: "12s"},
				NonRetryable: []string{
					string(loop.FailurePayloadDeclared),
				},
			},
		}

		resolved, err := loop.ResolveNodeLifecycleConfig(node, &loopConfig, loop.DefaultLifecycleConfig())
		if err != nil {
			t.Fatalf("ResolveNodeLifecycleConfig() error = %v", err)
		}
		if resolved.RetryMaxAttempts != 7 || resolved.RetryBackoffBase != 2*time.Second ||
			resolved.RetryBackoffMax != 12*time.Second || resolved.LivenessSilenceWindow != 30*time.Minute {
			t.Fatalf("resolved lifecycle = %#v, want node > loop config > defaults", resolved)
		}
		if resolved.Deadline == nil || *resolved.Deadline != 5*time.Minute {
			t.Fatalf("resolved deadline = %v, want 5m", resolved.Deadline)
		}
		if resolved.Sources["retry.max_attempts"] != loop.LifecycleSourceNode ||
			resolved.Sources["retry.backoff_base"] != loop.LifecycleSourceLoopConfig ||
			resolved.Sources["liveness.silence_window"] != loop.LifecycleSourceDefault {
			t.Fatalf("resolved sources = %#v, want node/loop/default provenance", resolved.Sources)
		}

		loopConfig.RetryMaxAttempts = new(1)
		loopConfig.RetryBackoffBase = new(9 * time.Second)
		node.Retry.MaxAttempts = 2
		node.Retry.NonRetryable[0] = string(loop.FailureTransport)
		if resolved.RetryMaxAttempts != 7 || resolved.RetryBackoffBase != 2*time.Second ||
			resolved.RetryNonRetryable[0] != string(loop.FailurePayloadDeclared) {
			t.Fatalf("pinned lifecycle changed after source mutation: %#v", resolved)
		}
	})

	t.Run("Should resolve removed overrides to shipped defaults and breaker globally", func(t *testing.T) {
		t.Parallel()

		resolved, err := loop.ResolveNodeLifecycleConfig(dsl.Node{}, nil, loop.DefaultLifecycleConfig())
		if err != nil {
			t.Fatalf("ResolveNodeLifecycleConfig() error = %v", err)
		}
		if resolved.RetryMaxAttempts != 3 || resolved.WaitAdmissionAttempts != 3 ||
			resolved.AdmissionHorizon != 168*time.Hour {
			t.Fatalf("resolved lifecycle = %#v, want shipped non-empty defaults", resolved)
		}
		breaker, err := loop.ResolveGlobalBreakerPolicy(5, time.Minute)
		if err != nil {
			t.Fatalf("ResolveGlobalBreakerPolicy() error = %v", err)
		}
		if breaker.Source != loop.BreakerGlobalSource || breaker.Threshold != 5 ||
			breaker.ProbeInterval != time.Minute {
			t.Fatalf("breaker = %#v, want reload-scoped global policy", breaker)
		}
	})
}

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
				name:   "Should reject compozy network tools",
				nodeID: "send-update",
				mutateNode: func(node *dsl.Node) {
					node.Kind = "compozy__network_send"
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
		startCtx, cancelStart := context.WithCancel(context.Background())
		hooks.cancelStarted = cancelStart
		defer cancelStart()
		run, err := svc.Start(startCtx, "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		want := run.NetworkSpecSnapshot()
		if got := hooks.started.ResolvedNetworkParticipation; got == nil || *got != want {
			t.Fatalf("loop.started participation = %#v, want %#v", got, want)
		}
		if !hooks.startedActive || !hooks.startedDeadline {
			t.Fatalf(
				"loop.started context active/deadline = %v/%v, want true/true",
				hooks.startedActive,
				hooks.startedDeadline,
			)
		}
		transitionCtx, cancelTransition := context.WithCancel(context.Background())
		hooks.cancelTerminal = cancelTransition
		defer cancelTransition()
		if err := svc.Transition(
			transitionCtx,
			run.ID,
			loop.StatusDone,
			loop.TransitionCauseContract,
		); err != nil {
			t.Fatalf("Transition(done) error = %v", err)
		}
		if got := hooks.terminal.ResolvedNetworkParticipation; got == nil || *got != want {
			t.Fatalf("loop.terminal participation = %#v, want %#v", got, want)
		}
		if !hooks.terminalActive || !hooks.terminalDeadline {
			t.Fatalf(
				"loop.terminal context active/deadline = %v/%v, want true/true",
				hooks.terminalActive,
				hooks.terminalDeadline,
			)
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

	t.Run("Should require an operator cancellation cause for live statuses", func(t *testing.T) {
		t.Parallel()

		for _, from := range []loop.Status{loop.StatusRunning, loop.StatusWatching} {
			t.Run("Should gate "+string(from)+" to canceled", func(t *testing.T) {
				t.Parallel()

				store := newFakeLoopStore()
				run := seedFakeRun(store, from)
				svc := newTestService(t, store, validDefinition())
				if err := svc.Transition(
					context.Background(), run.ID, loop.StatusCanceled, loop.TransitionCauseContract,
				); !errors.Is(err, loop.ErrInvalidTransition) {
					t.Fatalf("Transition(%s -> canceled, contract) error = %v, want ErrInvalidTransition", from, err)
				}
				if err := svc.Transition(
					context.Background(), run.ID, loop.StatusCanceled, loop.TransitionCauseOperatorCancel,
				); err != nil {
					t.Fatalf("Transition(%s -> canceled, operator) error = %v", from, err)
				}
			})
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
		def.Contract.RuntimeDefaults = &dsl.RuntimeDefaults{
			Worker: dsl.RuntimeSpec{Model: "definition-worker"},
			Judge:  dsl.RuntimeSpec{Model: "definition-judge"},
		}
		def.Contract.RuntimeRules = []dsl.RuntimeRule{{
			Match:   dsl.RuntimeMatch{Complexity: "high"},
			Runtime: dsl.RuntimeSpec{Provider: "definition-provider"},
		}}
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
				RuntimeDefaults: &loop.RuntimeDefaults{
					Worker: loop.RuntimeSpec{Model: "default-worker"},
					Judge:  loop.RuntimeSpec{Model: "default-judge"},
				},
				RuntimeRules: []loop.RuntimeRule{{
					Match:   loop.RuntimeMatch{Type: "frontend"},
					Runtime: loop.RuntimeSpec{Provider: "default-provider"},
				}},
			},
		}
		stored := loop.LoopConfig{
			IterationCap:     new(7),
			NoProgressWindow: new(8),
			FanOutWidth:      new(12),
			RuntimeDefaults: &loop.RuntimeDefaults{
				Worker: loop.RuntimeSpec{Model: "stored-worker"},
			},
			RuntimeRules: []loop.RuntimeRule{{
				Match:   loop.RuntimeMatch{Type: "frontend"},
				Runtime: loop.RuntimeSpec{Model: "stored-model"},
			}},
		}
		escalate := dsl.BudgetExceededEscalate
		perRun := loop.LoopConfig{
			IterationCap:     new(8),
			BudgetOnExceeded: &escalate,
			FanOutWidth:      new(99),
			GateMaxRevisions: new(99),
			RuntimeDefaults: &loop.RuntimeDefaults{
				Judge: loop.RuntimeSpec{Model: "per-run-judge"},
			},
			RuntimeRules: []loop.RuntimeRule{{
				Match:   loop.RuntimeMatch{ID: "task_01"},
				Runtime: loop.RuntimeSpec{Reasoning: "max"},
			}},
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
		if effective.RuntimeDefaults.Worker.Model != "stored-worker" {
			t.Fatalf(
				"RuntimeDefaults.Worker.Model = %q, want stored-worker",
				effective.RuntimeDefaults.Worker.Model,
			)
		}
		if effective.RuntimeDefaults.Judge.Model != "per-run-judge" {
			t.Fatalf(
				"RuntimeDefaults.Judge.Model = %q, want per-run-judge",
				effective.RuntimeDefaults.Judge.Model,
			)
		}
		if len(effective.RuntimeRules) != 3 {
			t.Fatalf("RuntimeRules = %#v, want definition/default/stored order", effective.RuntimeRules)
		}
		if effective.RuntimeRules[0].Match.Complexity != "high" ||
			effective.RuntimeRules[1].Runtime.Provider != "default-provider" ||
			effective.RuntimeRules[2].Runtime.Model != "stored-model" {
			t.Fatalf("RuntimeRules = %#v, want ordered config layers", effective.RuntimeRules)
		}
		if len(effective.RunRuntimeRules) != 1 ||
			effective.RunRuntimeRules[0].Match.ID != "task_01" ||
			effective.RunRuntimeRules[0].Runtime.Reasoning != "max" {
			t.Fatalf("RunRuntimeRules = %#v, want separated per-run rule", effective.RunRuntimeRules)
		}
	})

	t.Run("Should select watch runtime defaults only for watch definitions", func(t *testing.T) {
		t.Parallel()

		defaults := loop.LoopDefaults{
			Delivery: loop.LoopConfig{RuntimeDefaults: &loop.RuntimeDefaults{
				Worker: loop.RuntimeSpec{Model: "delivery-model"},
			}},
			Watch: loop.LoopConfig{RuntimeDefaults: &loop.RuntimeDefaults{
				Worker: loop.RuntimeSpec{Model: "watch-model"},
			}},
		}
		delivery, err := loop.ResolveEffectiveConfig(
			&loop.ResolvedDefinition{Definition: dsl.Definition{}}, defaults, nil, loop.LoopConfig{},
		)
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig(delivery) error = %v", err)
		}
		watch, err := loop.ResolveEffectiveConfig(&loop.ResolvedDefinition{Definition: dsl.Definition{
			Graph: dsl.Graph{Nodes: []dsl.Node{{
				ID: "watch", Class: dsl.NodeClassSource, Kind: string(dsl.SourceWatchSource),
			}}},
		}}, defaults, nil, loop.LoopConfig{})
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig(watch) error = %v", err)
		}
		if delivery.RuntimeDefaults.Worker.Model != "delivery-model" ||
			watch.RuntimeDefaults.Worker.Model != "watch-model" {
			t.Fatalf(
				"runtime defaults delivery/watch = %q/%q, want delivery-model/watch-model",
				delivery.RuntimeDefaults.Worker.Model,
				watch.RuntimeDefaults.Worker.Model,
			)
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

	t.Run("Should resolve configured inputs at the shared dry run and persisted start seam", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		definition.Inputs["auto_commit"] = dsl.Input{Type: dsl.InputTypeBoolean, Default: false}
		store := newFakeLoopStore()
		svc := newTestServiceWithOptions(
			t,
			store,
			definition,
			loop.WithInputDefaultsResolver(func(
				_ context.Context,
				ws loop.WorkspaceID,
				name string,
			) (loop.InputDefaultLayers, error) {
				if ws != "ws-inputs" || name != "valid-loop" {
					t.Fatalf("input defaults target = %s/%s, want ws-inputs/valid-loop", ws, name)
				}
				return loop.InputDefaultLayers{
					Global:    map[string]any{"tasks": "configured-task"},
					Workspace: map[string]any{"auto_commit": true},
				}, nil
			}),
		)

		preview, err := svc.DryRun(context.Background(), "ws-inputs", "valid-loop", loop.Inputs{})
		if err != nil {
			t.Fatalf("DryRun() error = %v", err)
		}
		if got, want := preview.ResolvedInputs["tasks"], "configured-task"; got != want {
			t.Fatalf("DryRun tasks = %#v, want %#v", got, want)
		}
		if got, want := preview.InputOrigins["tasks"], loop.InputOriginGlobal; got != want {
			t.Fatalf("DryRun tasks origin = %q, want %q", got, want)
		}
		if got, want := preview.InputOrigins["auto_commit"], loop.InputOriginWorkspace; got != want {
			t.Fatalf("DryRun auto_commit origin = %q, want %q", got, want)
		}
		if store.createCount() != 0 {
			t.Fatalf("DryRun create count = %d, want 0", store.createCount())
		}

		run, err := svc.Start(
			context.Background(),
			"ws-inputs",
			"valid-loop",
			loop.Inputs{Values: map[string]any{"tasks": "explicit-task"}},
			humanActor(t),
		)
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if got, want := run.Inputs["tasks"], "explicit-task"; got != want {
			t.Fatalf("Start tasks = %#v, want %#v", got, want)
		}
		if store.createCount() != 1 {
			t.Fatalf("Start create count = %d, want 1", store.createCount())
		}
	})

	t.Run("Should reject configured type mismatches before creating a run", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		definition.Inputs["auto_commit"] = dsl.Input{Type: dsl.InputTypeBoolean, Default: false}
		store := newFakeLoopStore()
		svc := newTestServiceWithOptions(
			t,
			store,
			definition,
			loop.WithInputDefaultsResolver(func(
				context.Context,
				loop.WorkspaceID,
				string,
			) (loop.InputDefaultLayers, error) {
				return loop.InputDefaultLayers{
					Workspace: map[string]any{"auto_commit": "true"},
				}, nil
			}),
		)

		_, err := svc.Start(
			context.Background(),
			"ws-inputs",
			"valid-loop",
			loop.Inputs{Values: map[string]any{"tasks": "task-ref"}},
			humanActor(t),
		)
		validation, ok := loop.AsInputDefaultError(err)
		if !ok || validation.Key != "auto_commit" || validation.Reason != loop.InputDefaultReasonTypeMismatch {
			t.Fatalf("Start() error = %#v, want typed auto_commit type mismatch", err)
		}
		if store.createCount() != 0 {
			t.Fatalf("Start create count = %d, want 0", store.createCount())
		}
	})
}

func TestServiceInlineGoalStartAndReplaceShouldSharePinnedStartPath(t *testing.T) {
	t.Parallel()

	t.Run("Should start a snapshot-only session Goal with pinned origin and policy", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		definition := validDefinition()
		definition.Contract.Goal = "Complete {{ .inputs.tasks }}"
		definition.Contract.DefinitionOfDone = "{{ .inputs.tasks }} is complete"
		svc := newTestService(t, store, definition)
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
		reason, reasonMatched := errors.AsType[*loop.ReasonError](err)
		if !reasonMatched || reason.Code != loop.ReasonCodeGoalJudgeUnavailable {
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
			loop.WithRunIDFactory(func() (loop.RunID, error) { return "looprun-goal-policy", nil }),
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
		reason, reasonMatched := errors.AsType[*loop.ReasonError](err)
		if !reasonMatched || reason.Code != loop.ReasonCodeActiveRunExists {
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
		reason, reasonMatched := errors.AsType[*loop.ReasonError](err)
		if !reasonMatched || reason.Code != loop.ReasonCodeAncestryCycle {
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

	t.Run("Should resume a wait with the pinned admission policy", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		resolved := compileDefinition(t, validDefinition())
		pinnedAttempts := 7
		effective, err := loop.ResolveEffectiveConfig(
			resolved,
			loop.DefaultLoopDefaults(),
			nil,
			loop.LoopConfig{Lifecycle: &loop.LifecycleConfig{WaitAdmissionAttempts: &pinnedAttempts}},
		)
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig() error = %v", err)
		}
		definition, digest, err := loop.BuildExecutedDefinitionSnapshot(resolved, effective)
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		run := seedFakeRun(store, loop.StatusRunning)
		run.Generation = 1
		run.DefinitionVersion = resolved.DefinitionVersion
		run.DefinitionDigest = digest
		store.seed(run)
		store.snapshots[string(run.WorkspaceID)+"/"+digest] = loop.DefinitionSnapshot{
			WorkspaceID: run.WorkspaceID,
			Digest:      digest,
			Version:     run.DefinitionVersion,
			Definition:  definition,
		}
		currentAttempts := 9
		if err := store.UpsertLoopConfig(
			context.Background(),
			run.WorkspaceID,
			run.LoopName,
			loop.LoopConfig{Lifecycle: &loop.LifecycleConfig{WaitAdmissionAttempts: &currentAttempts}},
		); err != nil {
			t.Fatalf("UpsertLoopConfig() error = %v", err)
		}
		svc := newTestService(t, store, validDefinition())
		lifecycle, ok := svc.(loop.NodeLifecycleService)
		if !ok {
			t.Fatal("Service does not implement NodeLifecycleService")
		}

		if _, err := lifecycle.ResumeNodeWait(
			context.Background(),
			run.WorkspaceID,
			run.ID,
			"release",
			0,
			json.RawMessage(`{}`),
			humanActor(t),
		); err != nil {
			t.Fatalf("ResumeNodeWait() error = %v", err)
		}
		if got := store.lastWaitResumeMutation(t).AdmissionAttempts; got != pinnedAttempts {
			t.Fatalf(
				"AdmissionAttempts = %d, want pinned %d instead of current %d",
				got,
				pinnedAttempts,
				currentAttempts,
			)
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
		var cancelApprove context.CancelFunc
		var activationActive bool
		var activationDeadline bool
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithGoalRunActivator(loop.GoalRunActivatorFunc(func(ctx context.Context, run task.Run) {
				cancelApprove()
				_, activationDeadline = ctx.Deadline()
				activationActive = ctx.Err() == nil
				activated = run
			})),
		)
		approveCtx, cancel := context.WithCancel(context.Background())
		cancelApprove = cancel
		defer cancel()
		if err := svc.Approve(
			approveCtx,
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
		if !activationActive || !activationDeadline {
			t.Fatalf(
				"Goal activation context active/deadline = %v/%v, want true/true",
				activationActive,
				activationDeadline,
			)
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

	t.Run("Should let child config override an inherited parent environment", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc := newTestService(t, store, validDefinition())
		childEnvironment := dsl.EnvironmentSpec{
			Mode:        dsl.EnvironmentWorktree,
			WorktreeRef: "child-feature",
		}
		if err := svc.Configure(context.Background(), "ws-1", "valid-loop", loop.LoopConfig{
			Environment: &childEnvironment,
		}); err != nil {
			t.Fatalf("Configure(child environment) error = %v", err)
		}
		parentEnvironment := dsl.EnvironmentSpec{Mode: dsl.EnvironmentPerRun}
		preview, err := svc.DryRun(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values:               map[string]any{"tasks": "task-ref"},
			InheritedEnvironment: &parentEnvironment,
		})
		if err != nil {
			t.Fatalf("DryRun(inherited environment) error = %v", err)
		}
		if preview.EffectiveConfig.Environment != childEnvironment {
			t.Fatalf(
				"effective child environment = %#v, want %#v",
				preview.EffectiveConfig.Environment,
				childEnvironment,
			)
		}
		explicitRunEnvironment := dsl.EnvironmentSpec{Mode: dsl.EnvironmentRoot}
		preview, err = svc.DryRun(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values:               map[string]any{"tasks": "task-ref"},
			InheritedEnvironment: &parentEnvironment,
			ConfigOverrides:      loop.LoopConfig{Environment: &explicitRunEnvironment},
		})
		if err != nil {
			t.Fatalf("DryRun(explicit environment) error = %v", err)
		}
		if preview.EffectiveConfig.Environment != explicitRunEnvironment {
			t.Fatalf(
				"effective run environment = %#v, want %#v",
				preview.EffectiveConfig.Environment,
				explicitRunEnvironment,
			)
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

	t.Run("Should reject config for a missing definition without persisting it", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		svc, err := loop.NewService(
			store,
			loop.DefinitionResolverFunc(func(
				context.Context,
				loop.WorkspaceID,
				string,
			) (*loop.ResolvedDefinition, error) {
				return nil, nil
			}),
			testGoalRunPolicyResolver(0.8),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Configure(context.Background(), "ws-1", "missing-loop", loop.LoopConfig{
			IterationCap: new(3),
		})
		if !errors.Is(err, loop.ErrDefinitionNotFound) {
			t.Fatalf("Configure(missing definition) error = %v, want ErrDefinitionNotFound", err)
		}
		if _, err := store.GetLoopConfig(
			context.Background(),
			"ws-1",
			"missing-loop",
		); !errors.Is(err, loop.ErrConfigNotFound) {
			t.Fatalf("GetLoopConfig(after rejected configure) error = %v, want ErrConfigNotFound", err)
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

	t.Run("Should resolve run workspace global and definition values by presence", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		def.Inputs = map[string]dsl.Input{
			"task_name":   {Type: dsl.InputTypeString, Required: true},
			"auto_commit": {Type: dsl.InputTypeBoolean, Default: false},
			"retries":     {Type: dsl.InputTypeNumber, Default: 5},
			"reviewer":    {Type: dsl.InputTypeString, Default: "reviewer"},
		}
		resolved, err := loop.ResolveInputDefaults(
			def,
			"valid-loop",
			map[string]any{"task_name": "run-task", "auto_commit": false},
			loop.InputDefaultLayers{
				Global: map[string]any{"auto_commit": true, "retries": int64(0)},
				Workspace: map[string]any{
					"auto_commit": true,
					"reviewer":    "",
				},
			},
		)
		if err != nil {
			t.Fatalf("ResolveInputDefaults() error = %v", err)
		}
		wantValues := map[string]any{
			"task_name": "run-task", "auto_commit": false, "retries": int64(0), "reviewer": "",
		}
		if !reflect.DeepEqual(resolved.Values, wantValues) {
			t.Fatalf("resolved values = %#v, want %#v", resolved.Values, wantValues)
		}
		wantOrigins := map[string]loop.InputOrigin{
			"task_name":   loop.InputOriginRun,
			"auto_commit": loop.InputOriginRun,
			"retries":     loop.InputOriginGlobal,
			"reviewer":    loop.InputOriginWorkspace,
		}
		if !reflect.DeepEqual(resolved.Origins, wantOrigins) {
			t.Fatalf("resolved origins = %#v, want %#v", resolved.Origins, wantOrigins)
		}
	})

	t.Run("Should return typed errors for unknown and mismatched inputs", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		def.Inputs = map[string]dsl.Input{
			"task_name":   {Type: dsl.InputTypeString, Required: true},
			"auto_commit": {Type: dsl.InputTypeBoolean, Default: false},
		}
		cases := []struct {
			name   string
			run    map[string]any
			layers loop.InputDefaultLayers
			key    string
			reason loop.InputDefaultReason
		}{
			{
				name: "Should reject an unknown configured key",
				run:  map[string]any{"task_name": "task-09"},
				layers: loop.InputDefaultLayers{
					Workspace: map[string]any{"legacy_provider": "coderabbit"},
				},
				key: "legacy_provider", reason: loop.InputDefaultReasonUnknownInput,
			},
			{
				name: "Should reject an unknown run key",
				run:  map[string]any{"task_name": "task-09", "legacy_provider": "coderabbit"},
				key:  "legacy_provider", reason: loop.InputDefaultReasonUnknownInput,
			},
			{
				name: "Should reject a configured type mismatch",
				run:  map[string]any{"task_name": "task-09"},
				layers: loop.InputDefaultLayers{
					Global: map[string]any{"auto_commit": "true"},
				},
				key: "auto_commit", reason: loop.InputDefaultReasonTypeMismatch,
			},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := loop.ResolveInputDefaults(def, "valid-loop", tt.run, tt.layers)
				validation, ok := loop.AsInputDefaultError(err)
				if !ok || validation.Loop != "valid-loop" || validation.Key != tt.key ||
					validation.Reason != tt.reason {
					t.Fatalf("ResolveInputDefaults() error = %#v, want valid-loop/%s/%s", err, tt.key, tt.reason)
				}
			})
		}
	})

	t.Run("Should validate definition defaults without consulting configured layers", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		def.Inputs = map[string]dsl.Input{
			"auto_commit": {Type: dsl.InputTypeBoolean, Default: "true"},
		}
		_, err := loop.NewCompiler().Compile(def)
		validation, ok := loop.AsInputDefaultError(err)
		if !ok || validation.Key != "auto_commit" || validation.Reason != loop.InputDefaultReasonTypeMismatch {
			t.Fatalf("Compile() error = %#v, want typed input_default type mismatch", err)
		}
	})
}

func TestServiceDryRunShouldReturnPlanPreviewWithoutState(t *testing.T) {
	t.Parallel()

	t.Run("Should return plan preview without creating loop run state", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		definition := validDefinition()
		definition.Contract.Goal = "Complete {{ .inputs.tasks }}"
		definition.Contract.DefinitionOfDone = "{{ .inputs.tasks }} is complete"
		svc := newTestService(t, store, definition)

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
		if got, want := preview.InputOrigins["tasks"], loop.InputOriginRun; got != want {
			t.Fatalf("InputOrigins[tasks] = %q, want %q", got, want)
		}
		if len(preview.Nodes) == 0 {
			t.Fatal("Nodes length = 0, want materialized gen-1 nodes")
		}
		if got, want := preview.Contract.Goal, "Complete {{ .inputs.tasks }}"; got != want {
			t.Fatalf("Contract.Goal = %q, want raw authored contract %q", got, want)
		}
		if got, want := preview.MaterializedContract.Goal, "Complete task-ref"; got != want {
			t.Fatalf("MaterializedContract.Goal = %q, want %q", got, want)
		}
		if got, want := preview.MaterializedContract.DefinitionOfDone, "task-ref is complete"; got != want {
			t.Fatalf("MaterializedContract.DefinitionOfDone = %q, want %q", got, want)
		}
		if store.createCount() != 0 {
			t.Fatalf("CreateLoopRun calls = %d, want 0", store.createCount())
		}
	})

	t.Run("Should reject invalid effective runtime before creating loop run state", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		definition.Contract.RuntimeDefaults = &dsl.RuntimeDefaults{
			Worker: dsl.RuntimeSpec{Provider: "flarp"},
		}
		store := newFakeLoopStore()
		svc := newTestServiceWithOptions(
			t,
			store,
			definition,
			loop.WithRuntimeCatalog(rejectingServiceRuntimeCatalogFactory{}),
		)
		inputs := loop.Inputs{Values: map[string]any{"tasks": "task-ref"}}

		if _, err := svc.DryRun(context.Background(), "ws-1", "valid-loop", inputs); err == nil {
			t.Fatal("DryRun() error = nil, want runtime validation")
		} else if validation, ok := loop.AsRuntimeValidationError(err); !ok ||
			len(validation.Items) != 1 || validation.Items[0].Reason != "unknown_provider" {
			t.Fatalf("DryRun() error = %v, want unknown_provider runtime validation", err)
		}
		if _, err := svc.Start(
			context.Background(),
			"ws-1",
			"valid-loop",
			inputs,
			humanActor(t),
		); err == nil {
			t.Fatal("Start() error = nil, want runtime validation")
		} else if validation, ok := loop.AsRuntimeValidationError(err); !ok ||
			len(validation.Items) != 1 || validation.Items[0].Reason != "unknown_provider" {
			t.Fatalf("Start() error = %v, want unknown_provider runtime validation", err)
		}
		if store.createCount() != 0 {
			t.Fatalf("CreateLoopRun calls = %d, want 0 after runtime validation", store.createCount())
		}
	})

	t.Run("Should reject a definition snapshot that cannot round trip", func(t *testing.T) {
		t.Parallel()

		resolved := compileDefinition(t, validDefinition())
		delete(resolved.Templates, "nodes.agent.params.prompt")
		store := newFakeLoopStore()
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
			loop.WithClock(func() time.Time {
				return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
			}),
			loop.WithRunIDFactory(func() (loop.RunID, error) {
				return "looprun-preview", nil
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		_, err = svc.DryRun(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		})
		if !errors.Is(err, loop.ErrValidation) || !strings.Contains(
			err.Error(),
			`template manifest key "nodes.agent.params.prompt" added during hydration`,
		) {
			t.Fatalf("DryRun() error = %v, want template key diagnostic", err)
		}
		if store.createCount() != 0 {
			t.Fatalf("CreateLoopRun calls = %d, want 0 after snapshot validation", store.createCount())
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
	definition.Contract.RuntimeDefaults = nil
	if strings.TrimSpace(judgeModel) != "" {
		definition.Contract.RuntimeDefaults = &dsl.RuntimeDefaults{
			Judge: dsl.RuntimeSpec{Model: judgeModel},
		}
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

func TestServiceCancellationShouldRecordCanceledTerminalTruth(t *testing.T) {
	t.Parallel()

	t.Run("Should cancel one node through its prompt and enqueue on_cancel", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		store.cancellationSessionIDs = []string{"session-agent"}
		definition := validDefinition()
		definition.Graph.Nodes[2].Normalize()
		definition.Graph.Nodes[2].OnCancel = []dsl.EffectSpec{{
			Emit: &dsl.EmitSpec{Kind: "agent_canceled"},
		}}
		var canceled []string
		var killed []string
		svc := newTestServiceWithOptions(
			t,
			store,
			definition,
			loop.WithCancellationSessionController(loop.CancellationSessionControllerFuncs{
				Cancel: func(_ context.Context, sessionID, _ string) error {
					canceled = append(canceled, sessionID)
					return nil
				},
				Kill: func(_ context.Context, sessionID, _ string) error {
					killed = append(killed, sessionID)
					return nil
				},
			}),
		)
		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if err := svc.CancelNode(
			context.Background(), run.WorkspaceID, run.ID, "agent", "operator request", humanActor(t),
		); err != nil {
			t.Fatalf("CancelNode() error = %v", err)
		}
		request := store.lastCancellationRequest(t)
		if request.NodeID != "agent" || len(request.Effects) != 1 ||
			request.Effects[0].Trigger != loop.EffectTriggerOnCancel {
			t.Fatalf("node cancellation request = %#v, want one on_cancel intent", request)
		}
		if !reflect.DeepEqual(canceled, []string{"session-agent"}) || len(killed) != 0 {
			t.Fatalf("cancel delivery = %#v killed = %#v", canceled, killed)
		}
		if !reflect.DeepEqual(store.cancellationStates, []loop.CancelState{
			loop.CancelStateDelivering, loop.CancelStateDraining,
		}) {
			t.Fatalf("node cancellation states = %#v", store.cancellationStates)
		}
		if got := store.mustRun(t, run.ID).Status; got != loop.StatusRunning {
			t.Fatalf("node cancellation changed Run status to %q", got)
		}
	})

	t.Run("Should kill one node without node-trigger effects", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		store.cancellationSessionIDs = []string{"session-agent"}
		definition := validDefinition()
		definition.Graph.Nodes[2].Normalize()
		definition.Graph.Nodes[2].OnCancel = []dsl.EffectSpec{{
			Emit: &dsl.EmitSpec{Kind: "must_not_fire"},
		}}
		var killed []string
		var activated task.Run
		svc := newTestServiceWithOptions(
			t,
			store,
			definition,
			loop.WithCancellationSessionController(loop.CancellationSessionControllerFuncs{
				Kill: func(_ context.Context, sessionID, _ string) error {
					killed = append(killed, sessionID)
					return nil
				},
			}),
			loop.WithCoordinatorRunActivator(loop.CoordinatorRunActivatorFunc(func(
				_ context.Context,
				run task.Run,
			) {
				activated = run
			})),
		)
		run, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if err := svc.KillNode(
			context.Background(), run.WorkspaceID, run.ID, "agent", "unsafe tool", humanActor(t),
		); err != nil {
			t.Fatalf("KillNode() error = %v", err)
		}
		request := store.lastCancellationRequest(t)
		if len(request.Effects) != 0 {
			t.Fatalf("kill effects = %#v, want none", request.Effects)
		}
		if !reflect.DeepEqual(killed, []string{"session-agent"}) || len(store.cancellationStates) != 0 {
			t.Fatalf("kill delivery = %#v states = %#v", killed, store.cancellationStates)
		}
		if activated.LoopRunID != string(run.ID) || activated.WorkspaceID != string(run.WorkspaceID) {
			t.Fatalf("node kill activation = %#v, want killed Run identity", activated)
		}
	})

	t.Run("Should propagate parent close only from parent to owned children", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		runLoopNode := func(id dsl.NodeID, policy dsl.ParentClosePolicy) dsl.Node {
			return dsl.Node{
				ID: id, Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
				Params:             dsl.NodeParams{"loop": "child-loop", "mode": string(dsl.RunLoopAwait)},
				NodeLifecycleState: &dsl.NodeLifecycleState{OnParentClose: policy},
			}
		}
		definition.Graph.Nodes[2] = runLoopNode("agent", "")
		definition.Graph.Nodes = append(
			definition.Graph.Nodes,
			runLoopNode("cancel_child", dsl.ParentCloseCancel),
			runLoopNode("abandon_child", dsl.ParentCloseAbandon),
		)
		definition.Graph.Edges = append(
			definition.Graph.Edges,
			dsl.Edge{From: "fan", To: "cancel_child"},
			dsl.Edge{From: "fan", To: "abandon_child"},
		)
		definition.Contract.NoProgress.HashFields = []string{"nodes.agent.output.loop_run_id"}
		store := newFakeLoopStore()
		svc := newTestService(t, store, definition)
		parent, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start(parent) error = %v", err)
		}
		children := []loop.Run{
			{ID: "child-terminate", WorkspaceID: parent.WorkspaceID, LoopName: "child-loop", Status: loop.StatusRunning,
				ParentLoopRunID: parent.ID},
			{ID: "child-cancel", WorkspaceID: parent.WorkspaceID, LoopName: "child-loop", Status: loop.StatusRunning,
				ParentLoopRunID: parent.ID},
			{ID: "child-abandon", WorkspaceID: parent.WorkspaceID, LoopName: "child-loop", Status: loop.StatusRunning,
				ParentLoopRunID: parent.ID},
		}
		store.mu.Lock()
		storedParent := store.runs[parent.ID]
		storedParent.Generation = 1
		store.runs[parent.ID] = storedParent
		for _, child := range children {
			store.runs[child.ID] = child
		}
		store.generationOutputs[parent.ID] = []loop.GenerationOutput{
			{Generation: 1, NodeID: "agent", Status: "awaiting_child", ChildLoopRunID: "child-terminate"},
			{Generation: 1, NodeID: "cancel_child", Status: "awaiting_child", ChildLoopRunID: "child-cancel"},
			{Generation: 1, NodeID: "abandon_child", Status: "awaiting_child", ChildLoopRunID: "child-abandon"},
		}
		store.mu.Unlock()

		if err := svc.CancelRun(
			context.Background(), parent.WorkspaceID, parent.ID, "operator request", humanActor(t),
		); err != nil {
			t.Fatalf("CancelRun(parent) error = %v", err)
		}
		if got := store.mustRun(t, "child-terminate"); got.Status != loop.StatusCanceled ||
			got.CancelKind != loop.RunCancelKill {
			t.Fatalf("terminate child = %#v", got)
		}
		if got := store.mustRun(t, "child-cancel"); got.Status != loop.StatusRunning ||
			!got.CancelRequested || got.CancelKind != loop.RunCancelCancel {
			t.Fatalf("cancel child = %#v", got)
		}
		if got := store.mustRun(t, "child-abandon"); got.Status != loop.StatusRunning || got.CancelRequested {
			t.Fatalf("abandoned child = %#v", got)
		}
	})

	t.Run("Should keep the parent cancellation draining when child propagation fails", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		definition.Graph.Nodes[2] = dsl.Node{
			ID: "agent", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
			Params: dsl.NodeParams{"loop": "child-loop", "mode": string(dsl.RunLoopAwait)},
		}
		definition.Contract.NoProgress.HashFields = []string{"nodes.agent.output.loop_run_id"}
		store := newFakeLoopStore()
		var activated task.Run
		svc := newTestServiceWithOptions(
			t,
			store,
			definition,
			loop.WithCoordinatorRunActivator(loop.CoordinatorRunActivatorFunc(func(
				_ context.Context,
				run task.Run,
			) {
				activated = run
			})),
		)
		parent, err := svc.Start(context.Background(), "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start(parent) error = %v", err)
		}
		child := loop.Run{
			ID: "child-parent-close-error", WorkspaceID: parent.WorkspaceID, LoopName: "child-loop",
			Status: loop.StatusRunning, ParentLoopRunID: parent.ID,
		}
		wantErr := errors.New("child cancellation unavailable")
		store.mu.Lock()
		storedParent := store.runs[parent.ID]
		storedParent.Generation = 1
		store.runs[parent.ID] = storedParent
		store.runs[child.ID] = child
		store.generationOutputs[parent.ID] = []loop.GenerationOutput{{
			Generation: 1, NodeID: "agent", Status: "awaiting_child", ChildLoopRunID: string(child.ID),
		}}
		store.cancellationErrByRun = map[loop.RunID]error{child.ID: wantErr}
		store.mu.Unlock()

		err = svc.CancelRun(
			context.Background(), parent.WorkspaceID, parent.ID, "operator request", humanActor(t),
		)
		if !errors.Is(err, wantErr) {
			t.Fatalf("CancelRun(parent) error = %v, want child propagation error", err)
		}
		if got := store.mustRun(t, parent.ID); !got.CancelRequested || got.Status != loop.StatusRunning {
			t.Fatalf("parent Run = %#v, want durable draining cancellation", got)
		}
		if !reflect.DeepEqual(store.cancellationStates, []loop.CancelState{
			loop.CancelStateDelivering, loop.CancelStateDraining,
		}) {
			t.Fatalf("parent cancellation states = %#v, want delivery to continue", store.cancellationStates)
		}
		if activated.LoopRunID != string(parent.ID) {
			t.Fatalf("coordinator activation = %#v, want parent drain activation", activated)
		}
	})

	t.Run("Should cooperatively cancel a live run", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusRunning)
		var activated task.Run
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithCoordinatorRunActivator(loop.CoordinatorRunActivatorFunc(func(
				_ context.Context,
				run task.Run,
			) {
				activated = run
			})),
		)

		if err := svc.CancelRun(context.Background(), "ws-1", run.ID, "operator request", humanActor(t)); err != nil {
			t.Fatalf("CancelRun() error = %v", err)
		}
		stored := store.mustRun(t, run.ID)
		if stored.Status != loop.StatusRunning || !stored.CancelRequested ||
			stored.CancelKind != loop.RunCancelCancel {
			t.Fatalf("stored Run = %#v, want a draining cancellation", stored)
		}
		if activated.LoopRunID != string(run.ID) || activated.WorkspaceID != string(run.WorkspaceID) {
			t.Fatalf("coordinator activation = %#v, want canceled Run identity", activated)
		}
		if !reflect.DeepEqual(store.cancellationStates, []loop.CancelState{
			loop.CancelStateDelivering, loop.CancelStateDraining,
		}) {
			t.Fatalf("run cancellation states = %#v, want visible drain", store.cancellationStates)
		}
	})

	t.Run("Should accept committed cancellation and retry deferred delivery", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		store.cancellationSessionIDs = []string{"session-work"}
		run := seedFakeRun(store, loop.StatusRunning)
		var deliveries int
		var activated task.Run
		deliveryErr := errors.New("provider cancellation unavailable")
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithCancellationSessionController(loop.CancellationSessionControllerFuncs{
				Cancel: func(context.Context, string, string) error {
					deliveries++
					if deliveries == 1 {
						return deliveryErr
					}
					return nil
				},
			}),
			loop.WithCoordinatorRunActivator(loop.CoordinatorRunActivatorFunc(func(
				_ context.Context,
				run task.Run,
			) {
				activated = run
			})),
		)

		if err := svc.CancelRun(
			context.Background(), run.WorkspaceID, run.ID, "operator request", humanActor(t),
		); err != nil {
			t.Fatalf("CancelRun() error = %v, want durable request accepted", err)
		}
		got := store.cancellationStates
		want := []loop.CancelState{loop.CancelStateDelivering}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("cancellation states = %#v, want %#v before retry", got, want)
		}

		request := store.lastCancellationRequest(t)
		store.pendingCancellations = []loop.PendingCancellation{{
			WorkspaceID: request.WorkspaceID,
			RunID:       request.RunID,
			State:       loop.CancelStateDelivering,
			Reason:      request.Reason,
			RequestedAt: request.RequestedAt,
			RequestedBy: request.Actor.Actor,
			SessionIDs:  []string{"session-work"},
		}}
		reconciler, ok := svc.(loop.CancellationReconciler)
		if !ok {
			t.Fatalf("service = %T, want CancellationReconciler", svc)
		}
		actor, err := task.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		reconciled, err := reconciler.ReconcilePendingCancellations(context.Background(), 10, actor)
		if err != nil {
			t.Fatalf("ReconcilePendingCancellations() error = %v", err)
		}
		if reconciled != 1 || deliveries != 2 {
			t.Fatalf("reconciled/deliveries = %d/%d, want 1/2", reconciled, deliveries)
		}
		if got, want := store.cancellationStates, []loop.CancelState{
			loop.CancelStateDelivering,
			loop.CancelStateDraining,
		}; !reflect.DeepEqual(got, want) {
			t.Fatalf("cancellation states = %#v, want %#v after retry", got, want)
		}
		if activated.LoopRunID != string(run.ID) {
			t.Fatalf("coordinator activation = %#v, want retried Run", activated)
		}
	})

	t.Run("Should preserve a terminal completion that wins the race", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusDone)
		svc := newTestService(t, store, validDefinition())

		if err := svc.CancelRun(context.Background(), "ws-1", run.ID, "late request", humanActor(t)); err != nil {
			t.Fatalf("CancelRun() error = %v", err)
		}
		if got := store.mustRun(t, run.ID).Status; got != loop.StatusDone {
			t.Fatalf("stored status = %q, want done", got)
		}
		if len(store.cancellationStates) != 0 {
			t.Fatalf("terminal cancellation states = %#v, want no delivery or advancement", store.cancellationStates)
		}
	})

	t.Run("Should make repeated cooperative cancel a no-op", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		store.cancellationSessionIDs = []string{"session-work"}
		run := seedFakeRun(store, loop.StatusRunning)
		var deliveries int
		svc := newTestServiceWithOptions(
			t,
			store,
			validDefinition(),
			loop.WithCancellationSessionController(loop.CancellationSessionControllerFuncs{
				Cancel: func(context.Context, string, string) error {
					deliveries++
					return nil
				},
			}),
		)
		for range 2 {
			if err := svc.CancelRun(
				context.Background(), run.WorkspaceID, run.ID, "operator request", humanActor(t),
			); err != nil {
				t.Fatalf("CancelRun() error = %v", err)
			}
		}
		if deliveries != 1 || !reflect.DeepEqual(store.cancellationStates, []loop.CancelState{
			loop.CancelStateDelivering, loop.CancelStateDraining,
		}) {
			t.Fatalf("repeated cancel deliveries/states = %d/%#v, want one drain", deliveries, store.cancellationStates)
		}
	})

	t.Run("Should kill a live run with the distinct kill cause", func(t *testing.T) {
		t.Parallel()

		store := newFakeLoopStore()
		run := seedFakeRun(store, loop.StatusRunning)
		svc := newTestService(t, store, validDefinition())

		if err := svc.KillRun(context.Background(), "ws-1", run.ID, "unsafe tool", humanActor(t)); err != nil {
			t.Fatalf("KillRun() error = %v", err)
		}
		if got := store.mustRun(t, run.ID); got.Status != loop.StatusCanceled {
			t.Fatalf("stored status = %q, want canceled", got.Status)
		}
		if got := store.lastTransition(t).cause; got != loop.TransitionCauseOperatorKill {
			t.Fatalf("transition cause = %q, want operator_kill", got)
		}
	})

	t.Run("Should revoke an exact Goal prompt lease only after cancellation commits", func(t *testing.T) {
		t.Parallel()

		base := newFakeLoopStore()
		run := seedFakeRun(base, loop.StatusRunning)
		lease := loop.GoalPromptLease{
			QueueEntryID: "queue-goal-stop", SessionID: "session-goal-stop", OwnerKind: "goal",
			LoopRunID: string(run.ID), TaskRunID: "task-run-goal-stop", RunGeneration: 1,
			PromptAttempt: 2, ControlEpoch: 3, BindingEpoch: 4,
			PromptID: "prompt-goal-stop", PromptKind: "work",
		}
		base.cancellationLeases = []loop.GoalPromptLease{lease}
		var revoked []loop.GoalPromptLease
		var reasons []string
		var cancelStop context.CancelFunc
		var revocationActive bool
		var revocationDeadline bool
		svc := newTestServiceWithOptions(
			t,
			base,
			validDefinition(),
			loop.WithGoalPromptLeaseRevoker(loop.GoalPromptLeaseRevokerFunc(
				func(ctx context.Context, got loop.GoalPromptLease, reason string) error {
					cancelStop()
					_, revocationDeadline = ctx.Deadline()
					revocationActive = ctx.Err() == nil
					revoked = append(revoked, got)
					reasons = append(reasons, reason)
					return nil
				},
			)),
		)
		actor := humanActor(t)
		stopCtx, cancel := context.WithCancel(context.Background())
		cancelStop = cancel
		defer cancel()
		if err := svc.CancelRun(stopCtx, run.WorkspaceID, run.ID, "operator request", actor); err != nil {
			t.Fatalf("CancelRun() error = %v", err)
		}
		if got := base.mustRun(t, run.ID); got.Status != loop.StatusRunning || !got.CancelRequested {
			t.Fatalf("stored Run = %#v, want cancellation to remain draining", got)
		}
		request := base.lastCancellationRequest(t)
		if request.WorkspaceID != run.WorkspaceID || request.RunID != run.ID || request.Actor != actor ||
			request.RequestedAt.IsZero() {
			t.Fatalf("cancellation request = %#v", request)
		}
		if !reflect.DeepEqual(revoked, []loop.GoalPromptLease{lease}) ||
			!reflect.DeepEqual(reasons, []string{string(loop.TransitionCauseOperatorCancel)}) {
			t.Fatalf("post-commit revocations = %#v reasons = %#v", revoked, reasons)
		}
		if !revocationActive || !revocationDeadline {
			t.Fatalf(
				"Goal revocation context active/deadline = %v/%v, want true/true",
				revocationActive,
				revocationDeadline,
			)
		}
	})

	t.Run("Should not revoke a Goal prompt lease when cancellation rolls back", func(t *testing.T) {
		t.Parallel()

		base := newFakeLoopStore()
		run := seedFakeRun(base, loop.StatusRunning)
		wantErr := errors.New("atomic cancellation failed")
		base.cancellationErr = wantErr
		var revokeCalls int
		svc := newTestServiceWithOptions(
			t,
			base,
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
		if err := svc.CancelRun(
			context.Background(), run.WorkspaceID, run.ID, "operator request", humanActor(t),
		); !errors.Is(err, wantErr) {
			t.Fatalf("CancelRun() error = %v, want %v", err, wantErr)
		}
		if revokeCalls != 0 {
			t.Fatalf("post-commit revoke calls = %d, want 0", revokeCalls)
		}
		if got := base.mustRun(t, run.ID); got.Status != loop.StatusRunning {
			t.Fatalf("stored status = %q, want running", got.Status)
		}
	})
}

func TestServiceNodePauseShouldRetryCancellationDelivery(t *testing.T) {
	t.Parallel()

	base := newFakeLoopStore()
	store := &nodePauseRetryStore{Store: base}
	deliveries := 0
	svc := newTestServiceWithOptions(
		t,
		store,
		validDefinition(),
		loop.WithCancellationSessionController(loop.CancellationSessionControllerFuncs{
			Cancel: func(context.Context, string, string) error {
				deliveries++
				if deliveries == 1 {
					return errors.New("temporary cancellation delivery failure")
				}
				return nil
			},
		}),
	)
	nodes := svc.(loop.NodeLifecycleService)
	actor := humanActor(t)
	if _, err := nodes.PauseNode(
		context.Background(), "ws-1", "run-1", "worker", loop.NodePauseCancel, "repair", actor,
	); err == nil {
		t.Fatal("PauseNode(first) error = nil, want delivery failure")
	}
	result, err := nodes.PauseNode(
		context.Background(), "ws-1", "run-1", "worker", loop.NodePauseCancel, "repair", actor,
	)
	if err != nil {
		t.Fatalf("PauseNode(retry) error = %v", err)
	}
	if result.Applied || deliveries != 2 {
		t.Fatalf("PauseNode(retry) = %#v deliveries=%d, want replayed control and redelivery", result, deliveries)
	}
}

func newTestService(t *testing.T, store *fakeLoopStore, def dsl.Definition) loop.Service {
	t.Helper()

	return newTestServiceWithOptions(t, store, def)
}

type nodePauseRetryStore struct {
	loop.Store
	calls int
}

func (s *nodePauseRetryStore) PauseNode(
	context.Context,
	loop.NodePauseMutation,
) (loop.NodePauseResult, error) {
	s.calls++
	return loop.NodePauseResult{
		Control:    loop.NodeControl{LoopRunID: "run-1", NodeID: "worker", Paused: true},
		SessionIDs: []string{"session-worker"},
		Applied:    s.calls == 1,
	}, nil
}

func (s *nodePauseRetryStore) ResumeNode(
	context.Context,
	loop.NodeResumeMutation,
) (loop.NodeResumeResult, error) {
	return loop.NodeResumeResult{}, errors.New("unexpected ResumeNode call")
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
		loop.WithRunIDFactory(func() (loop.RunID, error) {
			return loop.RunID("looprun-test"), nil
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
		loop.WithRunIDFactory(func() (loop.RunID, error) { return "looprun-test", nil }),
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
	started          hookspkg.LoopStartedPayload
	terminal         hookspkg.LoopTerminalPayload
	cancelStarted    context.CancelFunc
	cancelTerminal   context.CancelFunc
	startedActive    bool
	startedDeadline  bool
	terminalActive   bool
	terminalDeadline bool
}

func (d *participationLifecycleHookDispatcher) DispatchLoopStarted(
	ctx context.Context,
	payload hookspkg.LoopStartedPayload,
) (hookspkg.LoopStartedPayload, error) {
	d.cancelStarted()
	_, d.startedDeadline = ctx.Deadline()
	d.startedActive = ctx.Err() == nil
	d.started = payload
	return payload, nil
}

func (d *participationLifecycleHookDispatcher) DispatchLoopTerminal(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) (hookspkg.LoopTerminalPayload, error) {
	d.cancelTerminal()
	_, d.terminalDeadline = ctx.Deadline()
	d.terminalActive = ctx.Err() == nil
	d.terminal = payload
	return payload, nil
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

type rejectingServiceRuntimeCatalogFactory struct{}

func (rejectingServiceRuntimeCatalogFactory) ForWorkspace(
	context.Context,
	loop.WorkspaceID,
) (loop.RuntimeCatalog, error) {
	return rejectingServiceRuntimeCatalog{}, nil
}

type rejectingServiceRuntimeCatalog struct{}

func (rejectingServiceRuntimeCatalog) CanonicalProvider(provider string) string {
	return strings.TrimSpace(provider)
}

func (rejectingServiceRuntimeCatalog) ValidateRuntime(_ context.Context, runtime loop.RuntimeSpec) error {
	if runtime.Provider != "flarp" {
		return nil
	}
	return loop.NewRuntimeValidationError(loop.RuntimeValidationItem{
		Field: "provider", Value: runtime.Provider, Reason: "unknown_provider",
	})
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
	cancellationRequests             []loop.CancellationMutation
	cancellationStates               []loop.CancelState
	cancellationSessionIDs           []string
	cancellationLeases               []loop.GoalPromptLease
	cancellationErr                  error
	cancellationErrByRun             map[loop.RunID]error
	pendingCancellations             []loop.PendingCancellation
	generationOutputs                map[loop.RunID][]loop.GenerationOutput
	waitResumeMutation               *loop.WaitResumeMutation
	creates                          int
}

func (s *fakeLoopStore) ListPendingCancellations(
	_ context.Context,
	limit int,
) ([]loop.PendingCancellation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancellationErr != nil {
		return nil, s.cancellationErr
	}
	count := min(limit, len(s.pendingCancellations))
	result := make([]loop.PendingCancellation, count)
	copy(result, s.pendingCancellations[:count])
	return result, nil
}

func newFakeLoopStore() *fakeLoopStore {
	return &fakeLoopStore{
		runs:              map[loop.RunID]loop.Run{},
		configs:           map[string]loop.LoopConfig{},
		snapshots:         map[string]loop.DefinitionSnapshot{},
		decisions:         map[string]map[string]gate.HumanDecision{},
		generationOutputs: map[loop.RunID][]loop.GenerationOutput{},
	}
}

func (s *fakeLoopStore) ListGenerationOutputs(
	_ context.Context,
	workspaceID loop.WorkspaceID,
	runID loop.RunID,
	generation int,
) ([]loop.GenerationOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok || run.WorkspaceID != workspaceID {
		return nil, loop.ErrRunNotFound
	}
	outputs := s.generationOutputs[runID]
	result := make([]loop.GenerationOutput, 0, len(outputs))
	for _, output := range outputs {
		if output.Generation == generation {
			result = append(result, output)
		}
	}
	return result, nil
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

func (s *fakeLoopStore) lastCancellationRequest(t *testing.T) loop.CancellationMutation {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cancellationRequests) == 0 {
		t.Fatal("cancellation request count = 0, want at least one")
	}
	return s.cancellationRequests[len(s.cancellationRequests)-1]
}

func (s *fakeLoopStore) lastWaitResumeMutation(t *testing.T) loop.WaitResumeMutation {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.waitResumeMutation == nil {
		t.Fatal("wait resume mutation = nil")
	}
	return *s.waitResumeMutation
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

func (s *fakeLoopStore) ResumeWait(
	_ context.Context,
	mutation loop.WaitResumeMutation,
) (loop.WaitResumeResult, error) {
	if err := mutation.Validate(); err != nil {
		return loop.WaitResumeResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	mutationCopy := mutation
	s.waitResumeMutation = &mutationCopy
	return loop.WaitResumeResult{Won: true}, nil
}

func (s *fakeLoopStore) RequestRunCancellation(
	_ context.Context,
	mutation loop.CancellationMutation,
) (loop.CancellationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancellationRequests = append(s.cancellationRequests, mutation)
	if err := s.cancellationErrByRun[mutation.RunID]; err != nil {
		return loop.CancellationResult{}, err
	}
	if s.cancellationErr != nil {
		return loop.CancellationResult{}, s.cancellationErr
	}
	run, ok := s.runs[mutation.RunID]
	if !ok || run.WorkspaceID != mutation.WorkspaceID {
		return loop.CancellationResult{}, loop.ErrRunNotFound
	}
	if run.Status.Terminal() {
		return loop.CancellationResult{Run: run, Terminal: true}, nil
	}
	if run.CancelRequested {
		return loop.CancellationResult{Run: run}, nil
	}
	run.CancelRequested = true
	run.CancelKind = mutation.Kind
	s.runs[run.ID] = run
	result := loop.CancellationResult{
		Run:                 run,
		Applied:             true,
		SessionIDs:          append([]string(nil), s.cancellationSessionIDs...),
		RevokedPromptLeases: append([]loop.GoalPromptLease(nil), s.cancellationLeases...),
	}
	if mutation.Kind == loop.RunCancelKill {
		s.terminalizeCancellationLocked(run, mutation, &result)
	}
	return result, nil
}

func (s *fakeLoopStore) AdvanceRunCancellation(
	_ context.Context,
	mutation loop.CancellationMutation,
	state loop.CancelState,
) (loop.CancellationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancellationStates = append(s.cancellationStates, state)
	if s.cancellationErr != nil {
		return loop.CancellationResult{}, s.cancellationErr
	}
	run, ok := s.runs[mutation.RunID]
	if !ok || run.WorkspaceID != mutation.WorkspaceID {
		return loop.CancellationResult{}, loop.ErrRunNotFound
	}
	result := loop.CancellationResult{Run: run, Applied: true}
	if state == loop.CancelStateDraining {
		coordinator := task.Run{
			ID:          "run-cancel-wake-" + string(mutation.RunID),
			WorkspaceID: string(mutation.WorkspaceID),
			LoopRunID:   string(mutation.RunID),
			RunKind:     task.RunKindCoordinator,
		}
		result.Coordinator = &coordinator
	}
	if state == loop.CancelStateCanceled {
		s.terminalizeCancellationLocked(run, mutation, &result)
	}
	return result, nil
}

func (s *fakeLoopStore) RequestNodeCancellation(
	_ context.Context,
	mutation loop.CancellationMutation,
) (loop.CancellationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancellationRequests = append(s.cancellationRequests, mutation)
	if err := s.cancellationErrByRun[mutation.RunID]; err != nil {
		return loop.CancellationResult{}, err
	}
	if s.cancellationErr != nil {
		return loop.CancellationResult{}, s.cancellationErr
	}
	run, ok := s.runs[mutation.RunID]
	if !ok || run.WorkspaceID != mutation.WorkspaceID {
		return loop.CancellationResult{}, loop.ErrRunNotFound
	}
	if run.Status.Terminal() {
		return loop.CancellationResult{Run: run, Terminal: true}, nil
	}
	result := loop.CancellationResult{
		Run: run, Applied: true, SessionIDs: append([]string(nil), s.cancellationSessionIDs...),
	}
	if mutation.Kind == loop.RunCancelKill {
		coordinator := task.Run{
			ID:          "run-node-kill-wake-" + string(mutation.RunID),
			WorkspaceID: string(mutation.WorkspaceID),
			LoopRunID:   string(mutation.RunID),
			RunKind:     task.RunKindCoordinator,
		}
		result.Coordinator = &coordinator
	}
	return result, nil
}

func (s *fakeLoopStore) AdvanceNodeCancellation(
	_ context.Context,
	mutation loop.CancellationMutation,
	state loop.CancelState,
) (loop.CancellationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancellationStates = append(s.cancellationStates, state)
	if s.cancellationErr != nil {
		return loop.CancellationResult{}, s.cancellationErr
	}
	run, ok := s.runs[mutation.RunID]
	if !ok || run.WorkspaceID != mutation.WorkspaceID {
		return loop.CancellationResult{}, loop.ErrRunNotFound
	}
	result := loop.CancellationResult{Run: run, Applied: true}
	if state == loop.CancelStateDraining {
		coordinator := task.Run{
			ID:          "run-node-cancel-wake-" + string(mutation.RunID),
			WorkspaceID: string(mutation.WorkspaceID),
			LoopRunID:   string(mutation.RunID),
			RunKind:     task.RunKindCoordinator,
		}
		result.Coordinator = &coordinator
	}
	return result, nil
}

func (s *fakeLoopStore) terminalizeCancellationLocked(
	run loop.Run,
	mutation loop.CancellationMutation,
	result *loop.CancellationResult,
) {
	from := run.Status
	run.Status = loop.StatusCanceled
	s.runs[run.ID] = run
	cause := loop.TransitionCauseOperatorCancel
	if mutation.Kind == loop.RunCancelKill {
		cause = loop.TransitionCauseOperatorKill
	}
	s.transitions = append(s.transitions, fakeTransition{runID: run.ID, from: from, to: run.Status, cause: cause})
	result.Run = run
	result.Terminal = true
	result.RevokedPromptLeases = append([]loop.GoalPromptLease(nil), s.cancellationLeases...)
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

func (s *fakeLoopStore) ReactivateLoopCoordinator(
	_ context.Context,
	req *loop.CoordinatorReactivationRequest,
) (loop.CoordinatorReactivationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req == nil {
		return loop.CoordinatorReactivationResult{}, loop.ErrValidation
	}
	run, ok := s.runs[req.Run.ID]
	if !ok || run.WorkspaceID != req.Run.WorkspaceID {
		return loop.CoordinatorReactivationResult{}, loop.ErrRunNotFound
	}
	if run.Status != req.Run.Status || run.ActiveGateID != req.Run.ActiveGateID {
		return loop.CoordinatorReactivationResult{}, loop.ErrTransitionConflict
	}
	for _, record := range req.Decisions {
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
	run.Status = loop.StatusRunning
	run.PauseRequested = false
	run.ActiveGateID = ""
	run.ActiveHumanCriteria = []byte(`[]`)
	s.runs[run.ID] = run
	s.transitions = append(s.transitions, fakeTransition{
		runID: run.ID,
		from:  req.Run.Status,
		to:    loop.StatusRunning,
		cause: req.Cause,
	})
	return loop.CoordinatorReactivationResult{
		Run: task.Run{ID: "coordinator-successor", LoopRunID: string(run.ID)},
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
