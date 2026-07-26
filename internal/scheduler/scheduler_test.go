package scheduler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/jonboulle/clockwork"
)

func TestRunOnceWakesOnlyEligibleIdleSessions(t *testing.T) {
	base := time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC)
	source := &fakeTaskSource{
		pending: []RunSnapshot{workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)},
		active: []taskpkg.Run{{
			ID:        "active-1",
			Status:    taskpkg.TaskRunStatusRunning,
			SessionID: "sess-busy",
		}},
	}
	sessions := &fakeSessionSource{sessions: []SessionSnapshot{
		sessionSnapshot("sess-busy", "ws-1", "active", false, []string{"go"}, base.Add(time.Second)),
		sessionSnapshot("sess-prompting", "ws-1", "active", true, []string{"go"}, base.Add(2*time.Second)),
		sessionSnapshot("sess-wrong-workspace", "ws-2", "active", false, []string{"go"}, base.Add(3*time.Second)),
		sessionSnapshot("sess-missing-capability", "ws-1", "active", false, []string{"docs"}, base.Add(4*time.Second)),
		sessionSnapshot("sess-stopped", "ws-1", "stopped", false, []string{"go"}, base.Add(5*time.Second)),
		sessionSnapshot("sess-idle", "ws-1", "active", false, []string{"go", "sqlite"}, base.Add(6*time.Second)),
	}}
	waker := &fakeWaker{}
	scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

	result, err := scheduler.RunOnce(testutil.Context(t))
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.WakeAttempts != 1 || result.WakeSucceeded != 1 {
		t.Fatalf("wake result = %#v, want one successful wake", result)
	}
	targets := waker.targetsSnapshot()
	if got, want := len(targets), 1; got != want {
		t.Fatalf("wake targets = %d, want %d", got, want)
	}
	if got, want := targets[0].Session.ID, "sess-idle"; got != want {
		t.Fatalf("woken session = %q, want %q", got, want)
	}
	if got, want := targets[0].Work.Run.ID, "run-1"; got != want {
		t.Fatalf("woken run = %q, want %q", got, want)
	}
}

func TestRunOnceRecordsNoMatchWithoutWakeMutation(t *testing.T) {
	base := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	source := &fakeTaskSource{
		pending: []RunSnapshot{workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)},
	}
	sessions := &fakeSessionSource{sessions: []SessionSnapshot{
		sessionSnapshot("sess-docs", "ws-1", "active", false, []string{"docs"}, base),
	}}
	waker := &fakeWaker{}
	scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

	result, err := scheduler.RunOnce(testutil.Context(t))
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.NoMatchRuns != 1 {
		t.Fatalf("NoMatchRuns = %d, want 1", result.NoMatchRuns)
	}
	if got, want := result.NoMatchRunIDs, []string{"run-1"}; !slices.Equal(got, want) {
		t.Fatalf("NoMatchRunIDs = %v, want %v", got, want)
	}
	if got := len(waker.targetsSnapshot()); got != 0 {
		t.Fatalf("wake targets = %d, want 0", got)
	}
	stats := scheduler.Stats()
	if stats.NoMatchRuns != 1 || stats.WakeAttempts != 0 {
		t.Fatalf("stats = %#v, want one no-match and no wake attempts", stats)
	}
}

func TestRunOnceEscalatesStarvedRuns(t *testing.T) {
	t.Parallel()

	t.Run("Should classify structural compatibility separately from momentary availability", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 7, 15, 11, 0, 0, 0, time.UTC)
		work := workSnapshot(
			"task-classify",
			"run-classify",
			taskpkg.ScopeWorkspace,
			"ws-1",
			[]string{"frontend"},
			base,
		)
		work.Task.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "frontend-agent"}
		work.Run.SetNetworkState(participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     "ws-1",
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "frontend",
			Source:          participation.SourceExplicitRequest,
			Bounds: participation.Bounds{
				MaxWakes:         1,
				MaxWakeWallTime:  "1m",
				MaxTotalWallTime: "1m",
				MaxInputTokens:   1,
				MaxOutputTokens:  1,
				MaxWakeDepth:     1,
				CoalesceWindow:   "100ms",
			},
		}, "", "", "")
		matching := sessionSnapshot(
			"sess-matching",
			"ws-1",
			"active",
			false,
			[]string{"frontend"},
			base,
		)
		matching.AgentName = "frontend-agent"
		matching.Channel = "frontend"

		with := func(mutator func(*SessionSnapshot)) SessionSnapshot {
			candidate := matching
			candidate.Capabilities = append([]string(nil), matching.Capabilities...)
			mutator(&candidate)
			return candidate
		}
		testCases := []struct {
			name          string
			sessions      []SessionSnapshot
			occupied      map[string]struct{}
			wantKind      CapacityKind
			wantAvailable []string
		}{
			{
				name:          "Should return available for a matching idle session",
				sessions:      []SessionSnapshot{matching},
				wantKind:      CapacityAvailable,
				wantAvailable: []string{"sess-matching"},
			},
			{
				name:     "Should return waiting for a matching session with an active lease",
				sessions: []SessionSnapshot{matching},
				occupied: map[string]struct{}{"sess-matching": {}},
				wantKind: CapacityWaiting,
			},
			{
				name: "Should return waiting for a matching prompting session",
				sessions: []SessionSnapshot{with(func(candidate *SessionSnapshot) {
					candidate.Prompting = true
				})},
				wantKind: CapacityWaiting,
			},
			{
				name: "Should return waiting for a matching starting session",
				sessions: []SessionSnapshot{with(func(candidate *SessionSnapshot) {
					candidate.State = "starting"
				})},
				wantKind: CapacityWaiting,
			},
			{
				name: "Should prefer a matching idle session over matching occupied capacity",
				sessions: []SessionSnapshot{
					matching,
					with(func(candidate *SessionSnapshot) { candidate.ID = "sess-idle" }),
				},
				occupied:      map[string]struct{}{"sess-matching": {}},
				wantKind:      CapacityAvailable,
				wantAvailable: []string{"sess-idle"},
			},
			{
				name:     "Should return unmatched when no session exists",
				wantKind: CapacityUnmatched,
			},
			{
				name: "Should return unmatched for a foreign workspace",
				sessions: []SessionSnapshot{with(func(candidate *SessionSnapshot) {
					candidate.WorkspaceID = "ws-2"
				})},
				wantKind: CapacityUnmatched,
			},
			{
				name: "Should ignore participation channels when classifying otherwise compatible capacity",
				sessions: []SessionSnapshot{with(func(candidate *SessionSnapshot) {
					candidate.Channel = "backend"
				})},
				wantKind:      CapacityAvailable,
				wantAvailable: []string{"sess-matching"},
			},
			{
				name: "Should return unmatched for a foreign owner pool",
				sessions: []SessionSnapshot{with(func(candidate *SessionSnapshot) {
					candidate.AgentName = "analytics-agent"
				})},
				wantKind: CapacityUnmatched,
			},
			{
				name: "Should return unmatched for known missing capabilities",
				sessions: []SessionSnapshot{with(func(candidate *SessionSnapshot) {
					candidate.Capabilities = []string{"docs"}
				})},
				wantKind: CapacityUnmatched,
			},
			{
				name: "Should return indeterminate for an unknown capability projection",
				sessions: []SessionSnapshot{with(func(candidate *SessionSnapshot) {
					candidate.Capabilities = nil
					candidate.CapabilityState = CapabilityStateUnknown
				})},
				wantKind: CapacityIndeterminate,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				disposition := classifyRunCapacity(&work, testCase.sessions, testCase.occupied)
				if disposition.Kind != testCase.wantKind {
					t.Fatalf("capacity kind = %q, want %q", disposition.Kind, testCase.wantKind)
				}
				availableIDs := make([]string, 0, len(disposition.Available))
				for _, candidate := range disposition.Available {
					availableIDs = append(availableIDs, candidate.ID)
				}
				if !slices.Equal(availableIDs, testCase.wantAvailable) {
					t.Fatalf("available sessions = %v, want %v", availableIDs, testCase.wantAvailable)
				}
			})
		}
	})

	for _, testCase := range []struct {
		name      string
		state     string
		prompting bool
		active    []taskpkg.Run
		capState  CapabilityState
	}{
		{
			name:  "Should hold convergence while a compatible session owns an active lease",
			state: "active",
			active: []taskpkg.Run{{
				ID:        "run-active",
				Status:    taskpkg.TaskRunStatusRunning,
				SessionID: "sess-capacity",
			}},
		},
		{
			name:      "Should hold convergence while a compatible session is prompting",
			state:     "active",
			prompting: true,
		},
		{
			name:  "Should hold convergence while a compatible session is starting",
			state: "starting",
		},
		{
			name:     "Should hold convergence while compatible capability projection is indeterminate",
			state:    "active",
			capState: CapabilityStateUnknown,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			base := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			work := workSnapshot(
				"task-capacity",
				"run-capacity",
				taskpkg.ScopeWorkspace,
				"ws-1",
				[]string{"frontend"},
				base,
			)
			source := &fakeTaskSource{pending: []RunSnapshot{work}, active: testCase.active}
			candidate := sessionSnapshot(
				"sess-capacity",
				"ws-1",
				testCase.state,
				testCase.prompting,
				[]string{"frontend"},
				base,
			)
			if testCase.capState != "" {
				candidate.CapabilityState = testCase.capState
				candidate.Capabilities = nil
			}
			sessions := &fakeSessionSource{sessions: []SessionSnapshot{candidate}}
			starvation := newFakeStarvationStore()
			escalator := &fakeEscalationActor{}
			scheduler := newTestScheduler(
				t,
				source,
				sessions,
				&fakeWaker{},
				WithClock(clockwork.NewFakeClockAt(base.Add(10*time.Minute))),
				WithEscalationActor(escalator),
				WithStarvationStore(starvation),
				WithStarvationThresholds(StarvationThresholds{
					FanOutAfter:         1,
					SpawnAfter:          2,
					EventAfter:          3,
					NeedsAttentionAfter: 4,
					MinQueuedAge:        time.Minute,
				}),
			)

			result, err := scheduler.RunOnce(testutil.Context(t))
			if err != nil {
				t.Fatalf("RunOnce() error = %v", err)
			}
			if result.StarvedRuns != 0 || result.NoMatchRuns != 0 {
				t.Fatalf(
					"capacity-held result = %#v, want no starvation or no-match classification",
					result,
				)
			}
			if result.CapacityWaitingRuns != 1 ||
				!slices.Equal(result.CapacityWaitingRunIDs, []string{work.Run.ID}) {
				t.Fatalf("capacity wait result = %#v, want held run %q", result, work.Run.ID)
			}
			if _, ok := starvation.snapshot(work.Run.ID); ok {
				t.Fatal("capacity-held run created a starvation budget")
			}
			if len(escalator.spawns()) != 0 || len(escalator.emitted()) != 0 || len(escalator.attention()) != 0 {
				t.Fatalf("capacity-held run produced convergence side effects: %#v", escalator)
			}
		})
	}

	t.Run("Should freeze and resume an existing starvation budget as compatible capacity changes", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)
		work := workSnapshot(
			"task-frozen",
			"run-frozen",
			taskpkg.ScopeWorkspace,
			"ws-1",
			[]string{"frontend"},
			base,
		)
		source := &fakeTaskSource{
			pending: []RunSnapshot{work},
			active: []taskpkg.Run{{
				ID:        "run-active",
				Status:    taskpkg.TaskRunStatusRunning,
				SessionID: "sess-capacity",
			}},
		}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-capacity", "ws-1", "active", false, []string{"frontend"}, base),
		}}
		starvation := newFakeStarvationStore()
		var capacityLogs bytes.Buffer
		seeded := taskpkg.RunStarvationMutation{
			RunID:          work.Run.ID,
			WakeCount:      3,
			FirstStarvedAt: base.Add(time.Minute),
			LastWakeAt:     base.Add(2 * time.Minute),
			EscalationTier: 1,
			UpdatedAt:      base.Add(2 * time.Minute),
		}
		if _, err := starvation.UpsertRunStarvation(testutil.Context(t), seeded); err != nil {
			t.Fatalf("UpsertRunStarvation(seed) error = %v", err)
		}
		scheduler := newTestScheduler(
			t,
			source,
			sessions,
			&fakeWaker{},
			WithClock(clockwork.NewFakeClockAt(base.Add(10*time.Minute))),
			WithLogger(slog.New(slog.NewJSONHandler(&capacityLogs, nil))),
			WithEscalationActor(&fakeEscalationActor{}),
			WithStarvationStore(starvation),
			WithStarvationThresholds(StarvationThresholds{
				FanOutAfter:         1,
				SpawnAfter:          8,
				EventAfter:          9,
				NeedsAttentionAfter: 10,
				MinQueuedAge:        time.Minute,
			}),
		)

		heldResult, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce(capacity held) error = %v", err)
		}
		if heldResult.CapacityWaitingRuns != 1 ||
			!slices.Equal(heldResult.CapacityWaitingRunIDs, []string{work.Run.ID}) {
			t.Fatalf("capacity wait result = %#v, want held run %q", heldResult, work.Run.ID)
		}
		held, ok := starvation.snapshot(work.Run.ID)
		if !ok {
			t.Fatal("capacity-held run cleared its durable starvation budget")
		}
		if held.WakeCount != seeded.WakeCount ||
			!held.FirstStarvedAt.Equal(seeded.FirstStarvedAt) ||
			!held.LastWakeAt.Equal(seeded.LastWakeAt) ||
			!held.UpdatedAt.Equal(seeded.UpdatedAt) {
			t.Fatalf("held budget = %#v, want unchanged %#v", held, seeded)
		}
		if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
			t.Fatalf("RunOnce(capacity held again) error = %v", err)
		}
		if got, want := scheduler.Stats().CapacityWaitingRuns, 2; got != want {
			t.Fatalf("CapacityWaitingRuns stats = %d, want %d", got, want)
		}
		if logOutput := capacityLogs.String(); !strings.Contains(logOutput, "scheduler.capacity_waiting") ||
			!strings.Contains(logOutput, work.Run.ID) {
			t.Fatalf("capacity wait logs = %q, want event and run id", logOutput)
		}

		source.mu.Lock()
		source.active = nil
		source.mu.Unlock()
		sessions.setSessions(nil)
		if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
			t.Fatalf("RunOnce(capacity absent) error = %v", err)
		}
		resumed, ok := starvation.snapshot(work.Run.ID)
		if !ok {
			t.Fatal("unmatched run did not retain its resumed starvation budget")
		}
		if got, want := resumed.WakeCount, seeded.WakeCount+1; got != want {
			t.Fatalf("resumed wake_count = %d, want %d", got, want)
		}
	})

	t.Run("Should reserve one idle session for the canonical first run and hold the later run", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)
		first := workSnapshot(
			"task-first",
			"run-first",
			taskpkg.ScopeWorkspace,
			"ws-1",
			[]string{"frontend"},
			base,
		)
		second := workSnapshot(
			"task-second",
			"run-second",
			taskpkg.ScopeWorkspace,
			"ws-1",
			[]string{"frontend"},
			base.Add(time.Second),
		)
		source := &fakeTaskSource{pending: []RunSnapshot{second, first}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-serial", "ws-1", "active", false, []string{"frontend"}, base),
		}}
		starvation := newFakeStarvationStore()
		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			source,
			sessions,
			waker,
			WithClock(clockwork.NewFakeClockAt(base.Add(10*time.Minute))),
			WithEscalationActor(&fakeEscalationActor{}),
			WithStarvationStore(starvation),
			WithStarvationThresholds(StarvationThresholds{
				FanOutAfter:         1,
				SpawnAfter:          2,
				EventAfter:          3,
				NeedsAttentionAfter: 4,
				MinQueuedAge:        time.Minute,
			}),
		)

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if got, want := result.StarvedRunIDs, []string{first.Run.ID}; !slices.Equal(got, want) {
			t.Fatalf("StarvedRunIDs = %v, want %v", got, want)
		}
		if result.NoMatchRuns != 0 {
			t.Fatalf("NoMatchRuns = %d, want 0 for same-cycle reserved capacity", result.NoMatchRuns)
		}
		if result.CapacityWaitingRuns != 1 ||
			!slices.Equal(result.CapacityWaitingRunIDs, []string{second.Run.ID}) {
			t.Fatalf("capacity wait result = %#v, want later run %q", result, second.Run.ID)
		}
		if targets := waker.targetsSnapshot(); len(targets) != 1 || targets[0].Work.Run.ID != first.Run.ID {
			t.Fatalf("wake targets = %#v, want only canonical first run", targets)
		}
		if _, ok := starvation.snapshot(second.Run.ID); ok {
			t.Fatal("same-cycle capacity wait created a starvation budget for the later run")
		}
	})

	t.Run("Should fan out wakes to every eligible session for a starved run", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
		work := workSnapshot("task-starved", "run-starved", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-a", "ws-1", "active", false, []string{"go"}, base.Add(time.Second)),
			sessionSnapshot("sess-b", "ws-1", "active", false, []string{"go"}, base.Add(2*time.Second)),
		}}
		waker := &fakeWaker{}
		clock := clockwork.NewFakeClockAt(base.Add(3 * time.Minute))
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clock))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.StarvedRuns != 1 {
			t.Fatalf("StarvedRuns = %d, want 1 (result %#v)", result.StarvedRuns, result)
		}
		if got, want := result.StarvedRunIDs, []string{"run-starved"}; !slices.Equal(got, want) {
			t.Fatalf("StarvedRunIDs = %v, want %v", got, want)
		}
		targets := waker.targetsSnapshot()
		if got, want := len(targets), 2; got != want {
			t.Fatalf("wake targets = %d, want %d (fan out to all eligible)", got, want)
		}
		woken := make(map[string]struct{}, len(targets))
		for idx := range targets {
			if got := targets[idx].Work.Run.ID; got != "run-starved" {
				t.Fatalf("woken run = %q, want run-starved", got)
			}
			woken[targets[idx].Session.ID] = struct{}{}
		}
		if _, ok := woken["sess-a"]; !ok {
			t.Fatalf("sess-a not woken; woken = %v", woken)
		}
		if _, ok := woken["sess-b"]; !ok {
			t.Fatalf("sess-b not woken; woken = %v", woken)
		}
	})

	t.Run("Should wake only one session for a freshly queued run", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 28, 13, 0, 0, 0, time.UTC)
		work := workSnapshot("task-fresh", "run-fresh", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-a", "ws-1", "active", false, []string{"go"}, base.Add(time.Second)),
			sessionSnapshot("sess-b", "ws-1", "active", false, []string{"go"}, base.Add(2*time.Second)),
		}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.StarvedRuns != 0 {
			t.Fatalf("StarvedRuns = %d, want 0", result.StarvedRuns)
		}
		if got, want := len(waker.targetsSnapshot()), 1; got != want {
			t.Fatalf("wake targets = %d, want %d (single pick for a fresh run)", got, want)
		}
	})

	t.Run("Should report a starved run that has no eligible session", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 28, 14, 0, 0, 0, time.UTC)
		work := workSnapshot("task-none", "run-none", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-docs", "ws-1", "active", false, []string{"docs"}, base.Add(time.Second)),
		}}
		waker := &fakeWaker{}
		clock := clockwork.NewFakeClockAt(base.Add(5 * time.Minute))
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clock))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.StarvedRuns != 1 {
			t.Fatalf("StarvedRuns = %d, want 1", result.StarvedRuns)
		}
		if result.NoMatchRuns != 1 {
			t.Fatalf("NoMatchRuns = %d, want 1 (starved but no eligible session)", result.NoMatchRuns)
		}
		if got := len(waker.targetsSnapshot()); got != 0 {
			t.Fatalf("wake targets = %d, want 0", got)
		}
	})
}

func TestRunConvergenceEscalationLadder(t *testing.T) {
	t.Parallel()

	const minQueuedAge = 2 * time.Minute
	ladder := func(fanOut, spawn, event, needsAttention int) StarvationThresholds {
		return StarvationThresholds{
			FanOutAfter:         fanOut,
			SpawnAfter:          spawn,
			EventAfter:          event,
			NeedsAttentionAfter: needsAttention,
			MinQueuedAge:        minQueuedAge,
		}
	}
	build := func(
		t *testing.T,
		base time.Time,
		source *fakeTaskSource,
		store *fakeStarvationStore,
		escalator *fakeEscalationActor,
		thresholds StarvationThresholds,
	) (*Scheduler, context.Context) {
		t.Helper()
		sessions := &fakeSessionSource{}
		clock := clockwork.NewFakeClockAt(base.Add(3 * minQueuedAge))
		scheduler := newTestScheduler(
			t,
			source,
			sessions,
			&fakeWaker{},
			WithClock(clock),
			WithEscalationActor(escalator),
			WithStarvationStore(store),
			WithStarvationThresholds(thresholds),
			WithStarvationAge(minQueuedAge),
		)
		return scheduler, testutil.Context(t)
	}

	t.Run("Should climb fan-out, spawn, event, needs_attention and then clear the budget", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 5, 28, 15, 0, 0, 0, time.UTC)
		work := workSnapshot("task-ladder", "run-ladder", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		store := newFakeStarvationStore()
		escalator := &fakeEscalationActor{}
		scheduler, ctx := build(t, base, source, store, escalator, ladder(1, 2, 3, 4))

		for cycle := 1; cycle <= 4; cycle++ {
			if _, err := scheduler.RunOnce(ctx); err != nil {
				t.Fatalf("RunOnce(cycle %d) error = %v", cycle, err)
			}
			switch cycle {
			case 2:
				if got := escalator.spawns(); !slices.Equal(got, []string{"run-ladder"}) {
					t.Fatalf("cycle 2 spawns = %v, want [run-ladder]", got)
				}
			case 3:
				if got := escalator.emitted(); !slices.Equal(got, []string{"run-ladder"}) {
					t.Fatalf("cycle 3 emits = %v, want [run-ladder]", got)
				}
			}
		}
		if got := escalator.spawns(); !slices.Equal(got, []string{"run-ladder"}) {
			t.Fatalf("spawn requested more than once (coalesce failed): %v", got)
		}
		if got := escalator.emitted(); !slices.Equal(got, []string{"run-ladder"}) {
			t.Fatalf("event emitted more than once (set-once failed): %v", got)
		}
		if got := escalator.attention(); !slices.Equal(got, []string{"run-ladder"}) {
			t.Fatalf("needs_attention = %v, want [run-ladder]", got)
		}
		if _, ok := store.snapshot("run-ladder"); ok {
			t.Fatal("starvation budget not cleared after needs_attention")
		}
	})

	t.Run("Should let an unresolvable spawn fall through to event and needs_attention", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 5, 28, 16, 0, 0, 0, time.UTC)
		work := workSnapshot("task-unres", "run-unres", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		store := newFakeStarvationStore()
		escalator := &fakeEscalationActor{spawnErr: ErrSpawnUnresolvable}
		scheduler, ctx := build(t, base, source, store, escalator, ladder(1, 2, 3, 4))

		for cycle := 1; cycle <= 4; cycle++ {
			if _, err := scheduler.RunOnce(ctx); err != nil {
				t.Fatalf("RunOnce(cycle %d) error = %v", cycle, err)
			}
		}
		if got := escalator.spawns(); len(got) != 0 {
			t.Fatalf("unresolvable spawn recorded a request: %v", got)
		}
		if got := escalator.emitted(); !slices.Equal(got, []string{"run-unres"}) {
			t.Fatalf("event not emitted despite unresolvable spawn: %v", got)
		}
		if got := escalator.attention(); !slices.Equal(got, []string{"run-unres"}) {
			t.Fatalf("needs_attention not reached despite unresolvable spawn: %v", got)
		}
		if row, ok := store.snapshot("run-unres"); ok {
			t.Fatalf("budget not cleared after needs_attention: %#v", row)
		}
	})

	t.Run("Should retry a failing emitter without aborting the cycle", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 5, 28, 17, 0, 0, 0, time.UTC)
		work := workSnapshot("task-emit-err", "run-emit-err", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		store := newFakeStarvationStore()
		escalator := &fakeEscalationActor{emitErr: errors.New("emit boom")}
		// Event emission must still surface its error even when the monotonic ladder also allows spawn.
		scheduler, ctx := build(t, base, source, store, escalator, ladder(1, 1, 1, 99))

		result, err := scheduler.RunOnce(ctx)
		if err == nil {
			t.Fatal("RunOnce() error = nil, want joined emit error")
		}
		if result.StarvedRuns != 1 {
			t.Fatalf("StarvedRuns = %d, want 1 (cycle must not abort on emit failure)", result.StarvedRuns)
		}
		row, ok := store.snapshot("run-emit-err")
		if !ok {
			t.Fatal("budget row missing after failed emit")
		}
		if row.StarvedEventAt != nil {
			t.Fatal("starved_event_at set despite emit failure (must retry next cycle)")
		}
	})

	t.Run("Should hold a paused run's clock and clear a departed run on re-read", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 5, 28, 18, 0, 0, 0, time.UTC)
		source := &fakeTaskSource{}
		source.setStatus("run-paused", taskpkg.TaskRunStatusQueued)
		source.setStatus("run-claimed", taskpkg.TaskRunStatusClaimed)
		store := newFakeStarvationStore()
		seed := func(runID string) taskpkg.RunStarvationMutation {
			return taskpkg.RunStarvationMutation{
				RunID:          runID,
				WakeCount:      3,
				FirstStarvedAt: base,
				LastWakeAt:     base,
				EscalationTier: 1,
				UpdatedAt:      base,
			}
		}
		if _, err := store.UpsertRunStarvation(testutil.Context(t), seed("run-paused")); err != nil {
			t.Fatalf("seed paused row: %v", err)
		}
		if _, err := store.UpsertRunStarvation(testutil.Context(t), seed("run-claimed")); err != nil {
			t.Fatalf("seed claimed row: %v", err)
		}
		escalator := &fakeEscalationActor{}
		scheduler, ctx := build(t, base, source, store, escalator, ladder(1, 2, 3, 4))

		if _, err := scheduler.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		paused, ok := store.snapshot("run-paused")
		if !ok {
			t.Fatal("paused run budget cleared; the clock must hold while queued")
		}
		if paused.WakeCount != 3 {
			t.Fatalf("paused wake_count = %d, want 3 (clock must not advance off-candidate)", paused.WakeCount)
		}
		if _, ok := store.snapshot("run-claimed"); ok {
			t.Fatal("claimed run budget not cleared on re-read")
		}
	})

	t.Run("Should re-read a current candidate before spawn or event side effects", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 5, 28, 18, 30, 0, 0, time.UTC)
		work := workSnapshot("task-stale", "run-stale", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		source.setStatus("run-stale", taskpkg.TaskRunStatusClaimed)
		store := newFakeStarvationStore()
		if _, err := store.UpsertRunStarvation(testutil.Context(t), taskpkg.RunStarvationMutation{
			RunID:          "run-stale",
			WakeCount:      2,
			FirstStarvedAt: base,
			LastWakeAt:     base,
			EscalationTier: 1,
			UpdatedAt:      base,
		}); err != nil {
			t.Fatalf("seed stale row: %v", err)
		}
		escalator := &fakeEscalationActor{}
		scheduler, ctx := build(t, base, source, store, escalator, ladder(1, 1, 1, 99))

		if _, err := scheduler.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if got := escalator.spawns(); len(got) != 0 {
			t.Fatalf("spawn side effects = %v, want none for a stale non-queued candidate", got)
		}
		if got := escalator.emitted(); len(got) != 0 {
			t.Fatalf("starved events = %v, want none for a stale non-queued candidate", got)
		}
		if _, ok := store.snapshot("run-stale"); ok {
			t.Fatal("stale non-queued candidate budget not cleared")
		}
	})

	t.Run("Should preserve the durable budget across Rebuild", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 5, 28, 19, 0, 0, 0, time.UTC)
		work := workSnapshot("task-rebuild", "run-rebuild", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		store := newFakeStarvationStore()
		escalator := &fakeEscalationActor{}
		scheduler, ctx := build(t, base, source, store, escalator, ladder(1, 2, 3, 4))

		if _, err := scheduler.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce(first) error = %v", err)
		}
		if _, err := scheduler.Rebuild(ctx); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}
		if _, err := scheduler.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce(after rebuild) error = %v", err)
		}
		row, ok := store.snapshot("run-rebuild")
		if !ok {
			t.Fatal("budget row missing after rebuild")
		}
		if row.WakeCount != 2 {
			t.Fatalf(
				"wake_count = %d, want 2 (rebuild wiped in-memory state but not the durable budget)",
				row.WakeCount,
			)
		}
	})
}

func TestRunOnceUsesMinQueuedAgeFromThresholds(t *testing.T) {
	t.Parallel()

	t.Run("Should honor MinQueuedAge when only starvation thresholds are configured", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 28, 15, 0, 0, 0, time.UTC)
		source := &fakeTaskSource{
			pending: []RunSnapshot{
				workSnapshot(
					"task-min-age",
					"run-min-age",
					taskpkg.ScopeWorkspace,
					"ws-1",
					[]string{"go"},
					base,
				),
			},
		}
		sessions := &fakeSessionSource{}
		scheduler := newTestScheduler(
			t,
			source,
			sessions,
			&fakeWaker{},
			WithClock(clockwork.NewFakeClockAt(base.Add(30*time.Minute))),
			WithStarvationThresholds(StarvationThresholds{
				FanOutAfter:         1,
				SpawnAfter:          2,
				EventAfter:          3,
				NeedsAttentionAfter: 4,
				MinQueuedAge:        time.Hour,
			}),
		)

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.StarvedRuns != 0 {
			t.Fatalf("StarvedRuns = %d, want 0 before the configured min queued age", result.StarvedRuns)
		}
		if result.NoMatchRuns != 1 {
			t.Fatalf("NoMatchRuns = %d, want 1 with no eligible sessions", result.NoMatchRuns)
		}
	})
}

func TestStarvationThresholdsNormalize(t *testing.T) {
	t.Parallel()

	t.Run("Should clamp non-monotonic thresholds into a monotonic ladder", func(t *testing.T) {
		t.Parallel()

		normalized := (StarvationThresholds{
			FanOutAfter:         4,
			SpawnAfter:          2,
			EventAfter:          1,
			NeedsAttentionAfter: 3,
			MinQueuedAge:        time.Minute,
		}).normalize()

		if got, want := normalized.FanOutAfter, 4; got != want {
			t.Fatalf("FanOutAfter = %d, want %d", got, want)
		}
		if got, want := normalized.SpawnAfter, 4; got != want {
			t.Fatalf("SpawnAfter = %d, want %d", got, want)
		}
		if got, want := normalized.EventAfter, 4; got != want {
			t.Fatalf("EventAfter = %d, want %d", got, want)
		}
		if got, want := normalized.NeedsAttentionAfter, 4; got != want {
			t.Fatalf("NeedsAttentionAfter = %d, want %d", got, want)
		}
		if got, want := normalized.MinQueuedAge, time.Minute; got != want {
			t.Fatalf("MinQueuedAge = %s, want %s", got, want)
		}
	})
}

func TestRunOnceRequiresTaskOwnerMatch(t *testing.T) {
	t.Parallel()

	t.Run("Should wake only the session matching a pool-owned task agent", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 6, 10, 15, 0, 0, time.UTC)
		work := workSnapshot("task-owner", "run-owner", taskpkg.ScopeWorkspace, "ws-1", nil, base)
		work.Task.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "frontend-engineer-agent"}
		setRunParticipationChannel(t, &work.Run, "ws-1", "design-review")
		wrongOwner := sessionSnapshot("sess-analytics", "ws-1", "active", false, nil, base)
		wrongOwner.AgentName = "analytics-engineer-agent"
		wrongOwner.Channel = "design-review"
		matchingOwner := sessionSnapshot("sess-frontend", "ws-1", "active", false, nil, base.Add(time.Second))
		matchingOwner.AgentName = "frontend-engineer-agent"
		matchingOwner.Channel = "design-review"
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{wrongOwner, matchingOwner}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.WakeSucceeded != 1 {
			t.Fatalf("WakeSucceeded = %d, want 1 (result %#v)", result.WakeSucceeded, result)
		}
		targets := waker.targetsSnapshot()
		if got, want := len(targets), 1; got != want {
			t.Fatalf("wake targets = %d, want %d", got, want)
		}
		if got, want := targets[0].Session.ID, "sess-frontend"; got != want {
			t.Fatalf("woken session = %q, want %q", got, want)
		}
	})
}

func TestRunOnceDelegatesExpiredLeaseRecovery(t *testing.T) {
	base := time.Date(2026, 4, 26, 11, 0, 0, 0, time.UTC)
	source := &fakeTaskSource{
		recovered: []taskpkg.ExpiredLeaseRecoveryResult{{
			Run:               taskpkg.Run{ID: "run-recovered", Status: taskpkg.TaskRunStatusQueued},
			PreviousSessionID: "sess-old",
			Reason:            "scheduler_sweep",
		}},
	}
	scheduler := newTestScheduler(
		t,
		source,
		&fakeSessionSource{},
		&fakeWaker{},
		WithClock(clockwork.NewFakeClockAt(base)),
		WithSweepReason("scheduler_sweep"),
		WithSweepLimit(7),
	)

	result, err := scheduler.RunOnce(testutil.Context(t))
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.RecoveredLeases != 1 {
		t.Fatalf("RecoveredLeases = %d, want 1", result.RecoveredLeases)
	}
	calls := source.recoveryCallsSnapshot()
	if got, want := len(calls), 1; got != want {
		t.Fatalf("recovery calls = %d, want %d", got, want)
	}
	if calls[0].Reason != "scheduler_sweep" || calls[0].Limit != 7 || !calls[0].Now.Equal(base) {
		t.Fatalf("recovery request = %#v, want configured reason, limit, and clock time", calls[0])
	}
	if got := source.actorsSnapshot()[0].Actor.Kind; got != taskpkg.ActorKindDaemon {
		t.Fatalf("recovery actor kind = %q, want daemon", got)
	}
}

func TestRunOnceRunsLoopCoordinatorBackstop(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 4, 17, 30, 0, 0, time.UTC)
	source := &fakeLoopBackstopTaskSource{fakeTaskSource: &fakeTaskSource{}}
	scheduler := newTestScheduler(
		t,
		source,
		&fakeSessionSource{},
		&fakeWaker{},
		WithClock(clockwork.NewFakeClockAt(base)),
	)

	if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if source.calls != 1 {
		t.Fatalf("backstop calls = %d, want 1", source.calls)
	}
	if !source.now.Equal(base) {
		t.Fatalf("backstop now = %s, want %s", source.now, base)
	}
	if source.actor.Actor.Kind != taskpkg.ActorKindDaemon {
		t.Fatalf("backstop actor kind = %q, want daemon", source.actor.Actor.Kind)
	}
}

func TestRunOnceDelegatesTransientTaskBlockExpiry(t *testing.T) {
	t.Parallel()

	t.Run("Should record expired transient task blocks during RunOnce", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 4, 26, 11, 30, 0, 0, time.UTC)
		source := &fakeTaskSource{
			expiredBlocks: taskpkg.ExpireTaskBlocksResult{
				Blocks: []taskpkg.TaskBlock{{
					ID:     "block-expired",
					TaskID: "task-expired",
					Kind:   taskpkg.BlockKindTransient,
				}},
			},
		}
		scheduler := newTestScheduler(
			t,
			source,
			&fakeSessionSource{},
			&fakeWaker{},
			WithClock(clockwork.NewFakeClockAt(base)),
		)

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if got, want := result.ExpiredBlocks, 1; got != want {
			t.Fatalf("ExpiredBlocks = %d, want %d", got, want)
		}
		if got, want := result.ExpiredBlockIDs, []string{"block-expired"}; !slices.Equal(got, want) {
			t.Fatalf("ExpiredBlockIDs = %v, want %v", got, want)
		}
		calls := source.expiryCallsSnapshot()
		if got, want := len(calls), 1; got != want {
			t.Fatalf("expiry calls = %d, want %d", got, want)
		}
		if !calls[0].Equal(base) {
			t.Fatalf("expiry call time = %v, want %v", calls[0], base)
		}
		if got := source.expiryActors[0].Actor.Kind; got != taskpkg.ActorKindDaemon {
			t.Fatalf("expiry actor kind = %q, want daemon", got)
		}
	})
}

func TestRunOncePausedSchedulerStillSweepsExpiredLeases(t *testing.T) {
	t.Run("Should stop new dispatch without blocking lease recovery", func(t *testing.T) {
		base := time.Date(2026, 5, 21, 9, 30, 0, 0, time.UTC)
		source := &fakeTaskSource{
			pending: []RunSnapshot{
				workSnapshot(
					"task-paused-scheduler",
					"run-paused-scheduler",
					taskpkg.ScopeWorkspace,
					"ws-1",
					nil,
					base,
				),
			},
			recovered: []taskpkg.ExpiredLeaseRecoveryResult{{
				Run:               taskpkg.Run{ID: "run-recovered", Status: taskpkg.TaskRunStatusQueued},
				PreviousSessionID: "sess-stale",
				Reason:            "scheduler_sweep",
			}},
		}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-idle", "ws-1", "active", false, nil, base),
		}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			source,
			sessions,
			waker,
			WithClock(clockwork.NewFakeClockAt(base)),
			WithPauseStore(fakePauseStore{state: taskpkg.SchedulerPauseState{Paused: true}}),
		)

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if !result.Paused {
			t.Fatalf("Paused = false, want true in result %#v", result)
		}
		if result.RecoveredLeases != 1 {
			t.Fatalf("RecoveredLeases = %d, want 1", result.RecoveredLeases)
		}
		if got := len(source.recoveryCallsSnapshot()); got != 1 {
			t.Fatalf("recovery calls = %d, want 1", got)
		}
		if got := len(waker.targetsSnapshot()); got != 0 {
			t.Fatalf("wake targets = %d, want 0 while scheduler is paused", got)
		}
	})
}

func TestRunOnceSkipsDirectlyPausedTaskSnapshots(t *testing.T) {
	t.Parallel()

	t.Run("Should leave paused task work undispatched while dispatching eligible work", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
		paused := workSnapshot("task-paused", "run-paused", taskpkg.ScopeWorkspace, "ws-1", nil, base)
		paused.Task.Paused = true
		needsAttention := workSnapshot(
			"task-needs-attention",
			"run-needs-attention",
			taskpkg.ScopeWorkspace,
			"ws-1",
			nil,
			base,
		)
		needsAttention.Task.Status = taskpkg.TaskStatusNeedsAttention
		source := &fakeTaskSource{pending: []RunSnapshot{
			paused,
			needsAttention,
			workSnapshot("task-open", "run-open", taskpkg.ScopeWorkspace, "ws-1", nil, base.Add(time.Second)),
		}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-a", "ws-1", "active", false, nil, base),
			sessionSnapshot("sess-b", "ws-1", "active", false, nil, base.Add(time.Second)),
		}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.WakeSucceeded != 1 {
			t.Fatalf("WakeSucceeded = %d, want 1", result.WakeSucceeded)
		}
		if result.UnclaimableRuns != 2 {
			t.Fatalf("UnclaimableRuns = %d, want 2", result.UnclaimableRuns)
		}
		targets := waker.targetsSnapshot()
		if got, want := len(targets), 1; got != want {
			t.Fatalf("wake targets = %d, want %d", got, want)
		}
		if got, want := targets[0].Work.Run.ID, "run-open"; got != want {
			t.Fatalf("woken run = %q, want %q", got, want)
		}
	})
}

func TestRunOnceSchedulerPause(t *testing.T) {
	t.Parallel()

	t.Run("Should sweep leases and skip wake dispatch while paused", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
		source := &fakeTaskSource{
			pending: []RunSnapshot{
				workSnapshot("task-paused", "run-paused", taskpkg.ScopeWorkspace, "ws-1", nil, base),
			},
			recovered: []taskpkg.ExpiredLeaseRecoveryResult{{
				Run:               taskpkg.Run{ID: "run-recovered", Status: taskpkg.TaskRunStatusQueued},
				PreviousSessionID: "sess-old",
				Reason:            "scheduler_sweep",
			}},
		}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-idle", "ws-1", "active", false, nil, base),
		}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			source,
			sessions,
			waker,
			WithClock(clockwork.NewFakeClockAt(base)),
			WithPauseStore(fakePauseStore{state: taskpkg.SchedulerPauseState{Paused: true}}),
		)

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if !result.Paused || result.RecoveredLeases != 1 || result.PendingRuns != 1 || result.SessionsScanned != 1 {
			t.Fatalf("RunOnce() result = %#v, want paused cycle with recovery and loaded snapshots", result)
		}
		if got := len(waker.targetsSnapshot()); got != 0 {
			t.Fatalf("wake targets = %d, want 0 while paused", got)
		}
		if got := len(source.recoveryCallsSnapshot()); got != 1 {
			t.Fatalf("recovery calls = %d, want sweep to continue while paused", got)
		}
	})

	t.Run("Should restart starvation age at the durable scheduler resume boundary", func(t *testing.T) {
		t.Parallel()

		queuedAt := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
		resumedAt := queuedAt.Add(6 * time.Minute)
		clock := clockwork.NewFakeClockAt(resumedAt.Add(2*time.Minute - time.Nanosecond))
		source := &fakeTaskSource{pending: []RunSnapshot{
			workSnapshot("task-resumed", "run-resumed", taskpkg.ScopeWorkspace, "ws-1", nil, queuedAt),
		}}
		starvation := newFakeStarvationStore()
		scheduler := newTestScheduler(
			t,
			source,
			&fakeSessionSource{},
			&fakeWaker{},
			WithClock(clock),
			WithPauseStore(fakePauseStore{state: taskpkg.SchedulerPauseState{UpdatedAt: resumedAt}}),
			WithEscalationActor(&fakeEscalationActor{}),
			WithStarvationStore(starvation),
			WithStarvationThresholds(StarvationThresholds{
				FanOutAfter:         1,
				SpawnAfter:          2,
				EventAfter:          3,
				NeedsAttentionAfter: 4,
				MinQueuedAge:        2 * time.Minute,
			}),
		)

		beforeThreshold, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce(before threshold) error = %v", err)
		}
		if beforeThreshold.StarvedRuns != 0 {
			t.Fatalf("StarvedRuns = %d, want 0 while resume grace is active", beforeThreshold.StarvedRuns)
		}
		if _, ok := starvation.snapshot("run-resumed"); ok {
			t.Fatal("starvation budget exists before the post-resume minimum age")
		}

		clock.Advance(time.Nanosecond)
		afterThreshold, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce(after threshold) error = %v", err)
		}
		if afterThreshold.StarvedRuns != 1 {
			t.Fatalf("StarvedRuns = %d, want 1 after post-resume minimum age", afterThreshold.StarvedRuns)
		}
		row, ok := starvation.snapshot("run-resumed")
		if !ok || row.WakeCount != 1 {
			t.Fatalf("starvation budget = %#v, exists = %t, want wake_count 1", row, ok)
		}
	})
}

func TestRunOnceReturnsRecoveryAndWakeErrors(t *testing.T) {
	base := time.Date(2026, 4, 26, 11, 30, 0, 0, time.UTC)
	source := &fakeTaskSource{
		pending:    []RunSnapshot{workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", nil, base)},
		recoverErr: errors.New("recover failed"),
	}
	waker := &fakeWaker{err: errors.New("wake failed")}
	scheduler := newTestScheduler(
		t,
		source,
		&fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-1", "ws-1", "active", false, nil, base),
		}},
		waker,
		WithClock(clockwork.NewFakeClockAt(base)),
		WithWakeReason("wake-test"),
	)

	result, err := scheduler.RunOnce(testutil.Context(t))
	if err == nil {
		t.Fatal("RunOnce() error = nil, want joined recovery and wake errors")
	}
	if result.WakeAttempts != 1 || result.WakeFailed != 1 {
		t.Fatalf("wake result = %#v, want one failed wake", result)
	}
	stats := scheduler.Stats()
	if stats.RecoveryErrors != 1 || stats.WakeFailed != 1 {
		t.Fatalf("stats = %#v, want one recovery error and one wake failure", stats)
	}
	if stats.LastRecoveryError == "" || stats.LastWakeError == "" {
		t.Fatalf("stats = %#v, want last error fields recorded", stats)
	}
}

func TestRebuildClearsEphemeralWakeState(t *testing.T) {
	base := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	source := &fakeTaskSource{
		pending: []RunSnapshot{workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", nil, base)},
	}
	sessions := &fakeSessionSource{sessions: []SessionSnapshot{
		sessionSnapshot("sess-1", "ws-1", "active", false, nil, base),
	}}
	waker := &fakeWaker{}
	scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

	if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
		t.Fatalf("first RunOnce() error = %v", err)
	}
	if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
		t.Fatalf("second RunOnce() error = %v", err)
	}
	if got, want := len(waker.targetsSnapshot()), 1; got != want {
		t.Fatalf("wakes before rebuild = %d, want %d", got, want)
	}

	rebuild, err := scheduler.Rebuild(testutil.Context(t))
	if err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	if rebuild.ClearedWakeKeys != 1 {
		t.Fatalf("ClearedWakeKeys = %d, want 1", rebuild.ClearedWakeKeys)
	}
	if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
		t.Fatalf("third RunOnce() error = %v", err)
	}
	if got, want := len(waker.targetsSnapshot()), 2; got != want {
		t.Fatalf("wakes after rebuild = %d, want %d", got, want)
	}
}

func TestWakeCooldownExpiresAndAllowsRepeatNotification(t *testing.T) {
	base := time.Date(2026, 4, 26, 12, 30, 0, 0, time.UTC)
	clock := clockwork.NewFakeClockAt(base)
	source := &fakeTaskSource{
		pending: []RunSnapshot{workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", nil, base)},
	}
	sessions := &fakeSessionSource{sessions: []SessionSnapshot{
		sessionSnapshot("sess-1", "ws-1", "active", false, nil, base),
	}}
	waker := &fakeWaker{}
	scheduler := newTestScheduler(
		t,
		source,
		sessions,
		waker,
		WithClock(clock),
		WithWakeCooldown(time.Minute),
	)

	if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
		t.Fatalf("first RunOnce() error = %v", err)
	}
	if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
		t.Fatalf("second RunOnce() error = %v", err)
	}
	if got, want := len(waker.targetsSnapshot()), 1; got != want {
		t.Fatalf("wakes before cooldown expiry = %d, want %d", got, want)
	}

	clock.Advance(time.Minute)
	if _, err := scheduler.RunOnce(testutil.Context(t)); err != nil {
		t.Fatalf("third RunOnce() error = %v", err)
	}
	if got, want := len(waker.targetsSnapshot()), 2; got != want {
		t.Fatalf("wakes after cooldown expiry = %d, want %d", got, want)
	}
}

func TestRunOnceOrdersGlobalWakeTargetsByPriorityAndSession(t *testing.T) {
	base := time.Date(2026, 4, 26, 12, 45, 0, 0, time.UTC)
	low := workSnapshot("task-low", "run-low", taskpkg.ScopeGlobal, "", nil, base)
	low.Task.Priority = taskpkg.PriorityLow
	urgent := workSnapshot("task-urgent", "run-urgent", taskpkg.ScopeGlobal, "", nil, base.Add(time.Second))
	urgent.Task.Priority = taskpkg.PriorityUrgent
	source := &fakeTaskSource{pending: []RunSnapshot{low, urgent}}
	sessions := &fakeSessionSource{sessions: []SessionSnapshot{
		sessionSnapshot("sess-b", "", "active", false, nil, base),
		sessionSnapshot("sess-a", "", "active", false, nil, base),
	}}
	waker := &fakeWaker{}
	scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

	result, err := scheduler.RunOnce(testutil.Context(t))
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.WakeSucceeded != 2 {
		t.Fatalf("WakeSucceeded = %d, want 2", result.WakeSucceeded)
	}
	targets := waker.targetsSnapshot()
	gotRuns := []string{targets[0].Work.Run.ID, targets[1].Work.Run.ID}
	if want := []string{"run-urgent", "run-low"}; !slices.Equal(gotRuns, want) {
		t.Fatalf("wake run order = %v, want %v", gotRuns, want)
	}
	gotSessions := []string{targets[0].Session.ID, targets[1].Session.ID}
	if want := []string{"sess-a", "sess-b"}; !slices.Equal(gotSessions, want) {
		t.Fatalf("wake session order = %v, want %v", gotSessions, want)
	}
}

func TestRunOnceDispatchesSelectedTargetsAsBatch(t *testing.T) {
	t.Run("Should dispatch selected targets through batch waker", func(t *testing.T) {
		base := time.Date(2026, 4, 26, 12, 50, 0, 0, time.UTC)
		source := &fakeTaskSource{pending: []RunSnapshot{
			workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", nil, base),
			workSnapshot("task-2", "run-2", taskpkg.ScopeWorkspace, "ws-1", nil, base.Add(time.Second)),
		}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-a", "ws-1", "active", false, nil, base),
			sessionSnapshot("sess-b", "ws-1", "active", false, nil, base.Add(time.Second)),
		}}
		waker := &fakeBatchWaker{}
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.WakeAttempts != 2 || result.WakeSucceeded != 2 {
			t.Fatalf("wake result = %#v, want two successful batch wakes", result)
		}
		if got, want := waker.batchCallCount(), 1; got != want {
			t.Fatalf("batch wake calls = %d, want %d", got, want)
		}
		targets := waker.targetsSnapshot()
		if got, want := len(targets), 2; got != want {
			t.Fatalf("batch wake targets = %d, want %d", got, want)
		}
		for _, target := range targets {
			if got, want := target.Reason, defaultWakeReason; got != want {
				t.Fatalf("batch wake reason = %q, want %q", got, want)
			}
		}
	})
}

func TestShutdownCancelsLoopAndPreventsRestart(t *testing.T) {
	base := time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC)
	clock := clockwork.NewFakeClockAt(base)
	tickSeen := make(chan struct{}, 1)
	source := &fakeTaskSource{recoverCh: tickSeen}
	scheduler := newTestScheduler(
		t,
		source,
		&fakeSessionSource{},
		&fakeWaker{},
		WithClock(clock),
		WithInterval(time.Minute),
	)

	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForClockTimers(t, clock, 1)
	clock.Advance(time.Minute)
	select {
	case <-tickSeen:
	case <-testutil.Context(t).Done():
		t.Fatal("timed out waiting for scheduler tick")
	}

	if err := scheduler.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := scheduler.Start(testutil.Context(t)); !errors.Is(err, ErrStopped) {
		t.Fatalf("Start() after Shutdown error = %v, want ErrStopped", err)
	}
}

func TestNewValidatesRequiredDependencies(t *testing.T) {
	validTasks := &fakeTaskSource{}
	validSessions := &fakeSessionSource{}
	validWaker := &fakeWaker{}

	if _, err := New(nil, validSessions, validWaker); err == nil {
		t.Fatal("New(nil tasks) error = nil, want validation error")
	}
	if _, err := New(validTasks, nil, validWaker); err == nil {
		t.Fatal("New(nil sessions) error = nil, want validation error")
	}
	if _, err := New(validTasks, validSessions, nil); err == nil {
		t.Fatal("New(nil waker) error = nil, want validation error")
	}
	if _, err := New(
		validTasks,
		validSessions,
		validWaker,
		WithActor(taskpkg.ActorContext{}),
	); err == nil {
		t.Fatal("New(invalid actor) error = nil, want validation error")
	}
}

func newTestScheduler(t *testing.T, tasks TaskSource, sessions SessionSource, waker Waker, opts ...Option) *Scheduler {
	t.Helper()

	options := append([]Option{
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	}, opts...)
	scheduler, err := New(tasks, sessions, waker, options...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := scheduler.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
	return scheduler
}

func waitForClockTimers(t *testing.T, clock *clockwork.FakeClock, count int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
	defer cancel()
	if err := clock.BlockUntilContext(ctx, count); err != nil {
		t.Fatalf("BlockUntilContext() error = %v", err)
	}
}

func workSnapshot(
	taskID string,
	runID string,
	scope taskpkg.Scope,
	workspaceID string,
	required []string,
	queuedAt time.Time,
) RunSnapshot {
	return RunSnapshot{
		Task: taskpkg.Task{
			ID:          taskID,
			Scope:       scope,
			WorkspaceID: workspaceID,
			Status:      taskpkg.TaskStatusReady,
			Priority:    taskpkg.PriorityMedium,
		},
		Run: taskpkg.Run{
			ID:                   runID,
			TaskID:               taskID,
			Status:               taskpkg.TaskRunStatusQueued,
			RequiredCapabilities: append([]string(nil), required...),
			QueuedAt:             queuedAt,
		},
	}
}

func sessionSnapshot(
	id string,
	workspaceID string,
	state string,
	prompting bool,
	capabilities []string,
	createdAt time.Time,
) SessionSnapshot {
	return SessionSnapshot{
		ID:              id,
		WorkspaceID:     workspaceID,
		State:           state,
		Prompting:       prompting,
		Capabilities:    append([]string(nil), capabilities...),
		CapabilityState: CapabilityStateKnown,
		CreatedAt:       createdAt,
	}
}

type fakeTaskSource struct {
	mu            sync.Mutex
	pending       []RunSnapshot
	active        []taskpkg.Run
	statuses      map[string]taskpkg.RunStatus
	recovered     []taskpkg.ExpiredLeaseRecoveryResult
	expiredBlocks taskpkg.ExpireTaskBlocksResult
	recoverErr    error
	expireErr     error
	recoverCh     chan<- struct{}
	recoveryCalls []taskpkg.ExpiredLeaseRecovery
	expiryCalls   []time.Time
	actors        []taskpkg.ActorContext
	expiryActors  []taskpkg.ActorContext
}

type fakeLoopBackstopTaskSource struct {
	*fakeTaskSource
	calls int
	now   time.Time
	actor taskpkg.ActorContext
}

func (f *fakeLoopBackstopTaskSource) RunLoopCoordinatorBackstop(
	_ context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (int, error) {
	f.calls++
	f.now = now
	f.actor = actor
	return 0, nil
}

type fakePauseStore struct {
	state taskpkg.SchedulerPauseState
	err   error
}

func (f fakePauseStore) GetSchedulerPause(context.Context) (taskpkg.SchedulerPauseState, error) {
	if f.err != nil {
		return taskpkg.SchedulerPauseState{}, f.err
	}
	return f.state, nil
}

func (f *fakeTaskSource) PendingRuns(context.Context) ([]RunSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]RunSnapshot(nil), f.pending...), nil
}

func (f *fakeTaskSource) setStatus(runID string, status taskpkg.RunStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statuses == nil {
		f.statuses = make(map[string]taskpkg.RunStatus)
	}
	f.statuses[runID] = status
}

func (f *fakeTaskSource) GetRunStatus(_ context.Context, runID string) (taskpkg.RunStatus, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if status, ok := f.statuses[runID]; ok {
		return status, true, nil
	}
	for idx := range f.pending {
		if f.pending[idx].Run.ID == runID {
			return f.pending[idx].Run.Status, true, nil
		}
	}
	return taskpkg.TaskRunStatusUnknown, false, nil
}

type fakeEscalationActor struct {
	mu            sync.Mutex
	starvedEmits  []string
	spawnRequests []string
	attentionRuns []string
	emitErr       error
	spawnErr      error
	attentionErr  error
}

func (f *fakeEscalationActor) EmitRunStarved(_ context.Context, work *RunSnapshot, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.emitErr != nil {
		return f.emitErr
	}
	f.starvedEmits = append(f.starvedEmits, work.Run.ID)
	return nil
}

func (f *fakeEscalationActor) RequestWorkerSpawn(_ context.Context, work *RunSnapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.spawnErr != nil {
		return f.spawnErr
	}
	f.spawnRequests = append(f.spawnRequests, work.Run.ID)
	return nil
}

func (f *fakeEscalationActor) MarkRunNeedsAttention(
	_ context.Context,
	runID string,
	_ string,
) (taskpkg.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.attentionErr != nil {
		return taskpkg.Run{}, f.attentionErr
	}
	f.attentionRuns = append(f.attentionRuns, runID)
	return taskpkg.Run{ID: runID, Status: taskpkg.TaskRunStatusNeedsAttention}, nil
}

func (f *fakeEscalationActor) emitted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.starvedEmits...)
}

func (f *fakeEscalationActor) spawns() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.spawnRequests...)
}

func (f *fakeEscalationActor) attention() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.attentionRuns...)
}

type fakeStarvationStore struct {
	mu   sync.Mutex
	rows map[string]taskpkg.RunStarvation
}

func newFakeStarvationStore() *fakeStarvationStore {
	return &fakeStarvationStore{rows: make(map[string]taskpkg.RunStarvation)}
}

func (f *fakeStarvationStore) LoadRunStarvation(
	_ context.Context,
	runID string,
) (taskpkg.RunStarvation, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[runID]
	return row, ok, nil
}

func (f *fakeStarvationStore) ListRunStarvation(_ context.Context) ([]taskpkg.RunStarvation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rows := make([]taskpkg.RunStarvation, 0, len(f.rows))
	for _, row := range f.rows {
		rows = append(rows, row)
	}
	return rows, nil
}

func (f *fakeStarvationStore) UpsertRunStarvation(
	_ context.Context,
	mutation taskpkg.RunStarvationMutation,
) (taskpkg.RunStarvation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := taskpkg.RunStarvation(mutation)
	f.rows[mutation.RunID] = row
	return row, nil
}

func (f *fakeStarvationStore) ClearRunStarvation(_ context.Context, runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rows, runID)
	return nil
}

func (f *fakeStarvationStore) snapshot(runID string) (taskpkg.RunStarvation, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[runID]
	return row, ok
}

func (f *fakeTaskSource) ActiveRuns(context.Context) ([]taskpkg.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]taskpkg.Run(nil), f.active...), nil
}

func (f *fakeTaskSource) RecoverExpiredRunLeases(
	_ context.Context,
	recovery taskpkg.ExpiredLeaseRecovery,
	actor taskpkg.ActorContext,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recoveryCalls = append(f.recoveryCalls, recovery)
	f.actors = append(f.actors, actor)
	if f.recoverCh != nil {
		select {
		case f.recoverCh <- struct{}{}:
		default:
		}
	}
	if f.recoverErr != nil {
		return nil, f.recoverErr
	}
	return append([]taskpkg.ExpiredLeaseRecoveryResult(nil), f.recovered...), nil
}

func (f *fakeTaskSource) ExpireTaskBlocks(
	_ context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (taskpkg.ExpireTaskBlocksResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.expiryCalls = append(f.expiryCalls, now)
	f.expiryActors = append(f.expiryActors, actor)
	if f.expireErr != nil {
		return taskpkg.ExpireTaskBlocksResult{}, f.expireErr
	}
	return taskpkg.ExpireTaskBlocksResult{
		Blocks: append([]taskpkg.TaskBlock(nil), f.expiredBlocks.Blocks...),
	}, nil
}

func (f *fakeTaskSource) recoveryCallsSnapshot() []taskpkg.ExpiredLeaseRecovery {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]taskpkg.ExpiredLeaseRecovery(nil), f.recoveryCalls...)
}

func (f *fakeTaskSource) expiryCallsSnapshot() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.expiryCalls...)
}

func (f *fakeTaskSource) actorsSnapshot() []taskpkg.ActorContext {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]taskpkg.ActorContext(nil), f.actors...)
}

type fakeSessionSource struct {
	mu       sync.Mutex
	sessions []SessionSnapshot
}

func (f *fakeSessionSource) setSessions(sessions []SessionSnapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = append([]SessionSnapshot(nil), sessions...)
}

func (f *fakeSessionSource) Sessions(context.Context) ([]SessionSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SessionSnapshot(nil), f.sessions...), nil
}

type fakeWaker struct {
	mu      sync.Mutex
	err     error
	targets []WakeTarget
}

func (f *fakeWaker) Wake(_ context.Context, target *WakeTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if target != nil {
		f.targets = append(f.targets, *target)
	}
	return nil
}

func (f *fakeWaker) targetsSnapshot() []WakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]WakeTarget(nil), f.targets...)
}

type fakeBatchWaker struct {
	mu      sync.Mutex
	targets []WakeTarget
	calls   int
}

func (f *fakeBatchWaker) Wake(context.Context, *WakeTarget) error {
	return errors.New("single wake should not be called")
}

func (f *fakeBatchWaker) WakeMany(_ context.Context, targets []WakeTarget) []error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.targets = append(f.targets, targets...)
	return make([]error, len(targets))
}

func (f *fakeBatchWaker) batchCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeBatchWaker) targetsSnapshot() []WakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]WakeTarget(nil), f.targets...)
}
