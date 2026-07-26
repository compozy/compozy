package scheduler

import (
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/jonboulle/clockwork"
)

func TestRunOnceIgnoresCoordinationChannel(t *testing.T) {
	t.Parallel()

	t.Run("Should choose the oldest eligible session regardless of channel", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 4, 26, 13, 30, 0, 0, time.UTC)
		work := workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		setRunParticipationChannel(t, &work.Run, "ws-1", "finance")
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			{
				ID:           "sess-marketing",
				WorkspaceID:  "ws-1",
				Channel:      "marketing",
				State:        "active",
				Capabilities: []string{"go"},
				CreatedAt:    base,
			},
			{
				ID:           "sess-finance",
				WorkspaceID:  "ws-1",
				Channel:      "finance",
				State:        "active",
				Capabilities: []string{"go"},
				CreatedAt:    base.Add(time.Second),
			},
		}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.WakeAttempts != 1 || result.WakeSucceeded != 1 {
			t.Fatalf("result = %#v, want one successful wake", result)
		}

		targets := waker.targetsSnapshot()
		if got, want := len(targets), 1; got != want {
			t.Fatalf("wake targets = %d, want %d", got, want)
		}
		if got, want := targets[0].Session.ID, "sess-marketing"; got != want {
			t.Fatalf("woken session = %q, want %q", got, want)
		}
	})

	t.Run("Should wake a differently channeled eligible session", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 4, 26, 13, 45, 0, 0, time.UTC)
		work := workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
		setRunParticipationChannel(t, &work.Run, "ws-1", "finance")
		source := &fakeTaskSource{pending: []RunSnapshot{work}}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			{
				ID:           "sess-marketing",
				WorkspaceID:  "ws-1",
				Channel:      "marketing",
				State:        "active",
				Capabilities: []string{"go"},
				CreatedAt:    base,
			},
		}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

		result, err := scheduler.RunOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.WakeAttempts != 1 || result.WakeSucceeded != 1 {
			t.Fatalf("result = %#v, want one successful wake", result)
		}
		if got := len(waker.targetsSnapshot()); got != 1 {
			t.Fatalf("wake targets = %d, want 1", got)
		}
	})

	t.Run(
		"Should wake an unchanneled eligible session",
		func(t *testing.T) {
			t.Parallel()

			base := time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)
			work := workSnapshot("task-1", "run-1", taskpkg.ScopeWorkspace, "ws-1", []string{"go"}, base)
			setRunParticipationChannel(t, &work.Run, "ws-1", "finance")
			source := &fakeTaskSource{pending: []RunSnapshot{work}}
			sessions := &fakeSessionSource{sessions: []SessionSnapshot{
				{
					ID:           "sess-unscoped",
					WorkspaceID:  "ws-1",
					Channel:      "",
					State:        "active",
					Capabilities: []string{"go"},
					CreatedAt:    base,
				},
			}}
			waker := &fakeWaker{}
			scheduler := newTestScheduler(t, source, sessions, waker, WithClock(clockwork.NewFakeClockAt(base)))

			result, err := scheduler.RunOnce(testutil.Context(t))
			if err != nil {
				t.Fatalf("RunOnce() error = %v", err)
			}
			if result.WakeAttempts != 1 || result.WakeSucceeded != 1 {
				t.Fatalf("result = %#v, want one successful wake", result)
			}
			if got := len(waker.targetsSnapshot()); got != 1 {
				t.Fatalf("wake targets = %d, want 1", got)
			}
		},
	)
}

func setRunParticipationChannel(t *testing.T, run *taskpkg.Run, workspaceID string, channelID string) {
	t.Helper()

	spec := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channelID,
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         1,
			MaxWakeWallTime:  "1m",
			MaxTotalWallTime: "5m",
			MaxInputTokens:   1024,
			MaxOutputTokens:  512,
			MaxWakeDepth:     1,
			CoalesceWindow:   "500ms",
		},
	}
	if err := participation.ValidateSpec(spec); err != nil {
		t.Fatalf("ValidateSpec() error = %v", err)
	}
	run.SetNetworkState(spec, "", "", "")
}
