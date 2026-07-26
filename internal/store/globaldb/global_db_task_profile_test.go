package globaldb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBExecutionProfileStore(t *testing.T) {
	t.Parallel()

	t.Run("Should upsert load replace and delete typed execution profile selectors", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		createdAt := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(time.Hour)
		globalDB.now = func() time.Time { return createdAt }
		if err := globalDB.CreateTask(ctx, taskRecordForTest("task-profile-1")); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		if _, err := globalDB.GetExecutionProfile(ctx, "task-profile-1"); !errors.Is(
			err,
			taskpkg.ErrExecutionProfileNotFound,
		) {
			t.Fatalf("GetExecutionProfile(missing) error = %v, want profile not found", err)
		}
		liveMode := participation.ModeLive
		runStrategy := participation.StrategyRun
		maxWakes := 3
		coalesceWindow := "750ms"

		created, err := globalDB.UpsertExecutionProfile(ctx, &taskpkg.ExecutionProfile{
			TaskID: "task-profile-1",
			Coordinator: taskpkg.CoordinatorProfile{
				Mode:     taskpkg.CoordinatorModeGuided,
				Provider: "claude",
				Model:    "opus",
				Guidance: "Keep the next worker on the failing acceptance criterion.",
			},
			Worker: taskpkg.WorkerProfile{
				Mode:                  taskpkg.WorkerModeSelect,
				AgentName:             "coder",
				AllowedAgentNames:     []string{"coder"},
				PreferredAgentNames:   []string{"coder"},
				RequiredCapabilities:  []string{"go"},
				PreferredCapabilities: []string{"tests"},
			},
			Review: taskpkg.ReviewProfile{
				AgentName:             "reviewer",
				AllowedAgentNames:     []string{"reviewer"},
				PreferredAgentNames:   []string{"reviewer"},
				AllowedChannelIDs:     []string{"review-channel"},
				PreferredChannelIDs:   []string{"review-channel"},
				AllowedPeerIDs:        []string{"peer-a"},
				PreferredPeerIDs:      []string{"peer-a"},
				RequiredCapabilities:  []string{"review"},
				PreferredCapabilities: []string{"go"},
			},
			Participants: taskpkg.ParticipantPolicy{
				AllowedAgentNames:     []string{"coder", "reviewer"},
				PreferredAgentNames:   []string{"reviewer"},
				AllowedChannelIDs:     []string{"work-channel"},
				PreferredChannelIDs:   []string{"review-channel"},
				AllowedPeerIDs:        []string{"peer-a"},
				PreferredPeerIDs:      []string{"peer-a"},
				RequiredCapabilities:  []string{"go"},
				PreferredCapabilities: []string{"review"},
			},
			Sandbox: taskpkg.SandboxPolicy{Mode: taskpkg.SandboxModeRef, SandboxRef: "workspace"},
			Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeModeEvidence},
			NetworkParticipation: &participation.Request{
				Mode:            &liveMode,
				ChannelStrategy: &runStrategy,
				Bounds: &participation.BoundsRequest{
					MaxWakes:       &maxWakes,
					CoalesceWindow: &coalesceWindow,
				},
			},
		})
		if err != nil {
			t.Fatalf("UpsertExecutionProfile(create) error = %v", err)
		}
		assertStoredProfileShape(t, &created)

		loaded, err := globalDB.GetExecutionProfile(ctx, "task-profile-1")
		if err != nil {
			t.Fatalf("GetExecutionProfile() error = %v", err)
		}
		assertStoredProfileShape(t, &loaded)
		if got, want := profileAgentSelectorCount(ctx, t, globalDB), 7; got != want {
			t.Fatalf("task_profile_agents row count = %d, want %d", got, want)
		}

		globalDB.now = func() time.Time { return updatedAt }
		updated, err := globalDB.UpsertExecutionProfile(ctx, &taskpkg.ExecutionProfile{
			TaskID: "task-profile-1",
			Worker: taskpkg.WorkerProfile{
				Mode:              taskpkg.WorkerModeSelect,
				AgentName:         "runner",
				AllowedAgentNames: []string{"runner"},
			},
			Sandbox: taskpkg.SandboxPolicy{Mode: taskpkg.SandboxModeInherit},
		})
		if err != nil {
			t.Fatalf("UpsertExecutionProfile(update) error = %v", err)
		}
		if got, want := updated.CreatedAt, created.CreatedAt; !got.Equal(want) {
			t.Fatalf("updated.CreatedAt = %v, want %v", got, want)
		}
		if !updated.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("updated.UpdatedAt = %v, want %v", updated.UpdatedAt, updatedAt)
		}
		if got, want := updated.Runtime.Mode, taskpkg.RuntimeModeDefault; got != want {
			t.Fatalf("updated.Runtime.Mode = %q, want %q", got, want)
		}
		if updated.NetworkParticipation != nil {
			t.Fatalf("updated.NetworkParticipation = %#v, want nil after replacement", updated.NetworkParticipation)
		}
		assertStringSliceGlobal(t, updated.Worker.AllowedAgentNames, []string{"runner"})
		assertStringSliceGlobal(t, updated.Review.AllowedAgentNames, nil)

		globalDB.now = func() time.Time { return updatedAt.Add(time.Hour) }
		reloaded, err := globalDB.GetExecutionProfile(ctx, "task-profile-1")
		if err != nil {
			t.Fatalf("GetExecutionProfile(reload) error = %v", err)
		}
		reloaded.Worker.AgentName = "runner-reloaded"
		reloaded.Worker.AllowedAgentNames = []string{"runner-reloaded"}
		staleUpdatedAt := reloaded.UpdatedAt
		replayed, err := globalDB.UpsertExecutionProfile(ctx, &reloaded)
		if err != nil {
			t.Fatalf("UpsertExecutionProfile(replay) error = %v", err)
		}
		if !replayed.UpdatedAt.After(staleUpdatedAt) {
			t.Fatalf("replayed.UpdatedAt = %v, want later than stale %v", replayed.UpdatedAt, staleUpdatedAt)
		}

		if err := globalDB.DeleteExecutionProfile(ctx, "task-profile-1"); err != nil {
			t.Fatalf("DeleteExecutionProfile() error = %v", err)
		}
		if got, want := profileAgentSelectorCount(ctx, t, globalDB), 0; got != want {
			t.Fatalf("task_profile_agents row count after delete = %d, want %d", got, want)
		}
		if _, err := globalDB.GetExecutionProfile(ctx, "task-profile-1"); !errors.Is(
			err,
			taskpkg.ErrExecutionProfileNotFound,
		) {
			t.Fatalf("GetExecutionProfile(after delete) error = %v, want profile not found", err)
		}
	})

	t.Run("Should reject profile rows for missing tasks", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		_, err := globalDB.UpsertExecutionProfile(testutil.Context(t), &taskpkg.ExecutionProfile{
			TaskID: "missing-task",
			Worker: taskpkg.WorkerProfile{
				Mode:              taskpkg.WorkerModeSelect,
				AgentName:         "coder",
				AllowedAgentNames: []string{"coder"},
			},
			Sandbox: taskpkg.SandboxPolicy{Mode: taskpkg.SandboxModeInherit},
		})
		if !errors.Is(err, taskpkg.ErrTaskNotFound) {
			t.Fatalf("UpsertExecutionProfile(missing task) error = %v, want %v", err, taskpkg.ErrTaskNotFound)
		}
	})
}

func assertStoredProfileShape(t *testing.T, profile *taskpkg.ExecutionProfile) {
	t.Helper()

	if got, want := profile.TaskID, "task-profile-1"; got != want {
		t.Fatalf("TaskID = %q, want %q", got, want)
	}
	if got, want := profile.Coordinator.Mode, taskpkg.CoordinatorModeGuided; got != want {
		t.Fatalf("Coordinator.Mode = %q, want %q", got, want)
	}
	if got, want := profile.Worker.AgentName, "coder"; got != want {
		t.Fatalf("Worker.AgentName = %q, want %q", got, want)
	}
	if got, want := profile.Review.AgentName, "reviewer"; got != want {
		t.Fatalf("Review.AgentName = %q, want %q", got, want)
	}
	if got, want := profile.Sandbox.Mode, taskpkg.SandboxModeRef; got != want {
		t.Fatalf("Sandbox.Mode = %q, want %q", got, want)
	}
	if got, want := profile.Runtime.Mode, taskpkg.RuntimeModeEvidence; got != want {
		t.Fatalf("Runtime.Mode = %q, want %q", got, want)
	}
	if profile.NetworkParticipation == nil || profile.NetworkParticipation.Mode == nil ||
		*profile.NetworkParticipation.Mode != participation.ModeLive ||
		profile.NetworkParticipation.ChannelStrategy == nil ||
		*profile.NetworkParticipation.ChannelStrategy != participation.StrategyRun ||
		profile.NetworkParticipation.Bounds == nil ||
		profile.NetworkParticipation.Bounds.MaxWakes == nil ||
		*profile.NetworkParticipation.Bounds.MaxWakes != 3 {
		t.Fatalf("NetworkParticipation = %#v, want live run strategy with max_wakes 3", profile.NetworkParticipation)
	}
	assertStringSliceGlobal(t, profile.Participants.AllowedAgentNames, []string{"coder", "reviewer"})
	assertStringSliceGlobal(t, profile.Review.AllowedChannelIDs, []string{"review-channel"})
	assertStringSliceGlobal(t, profile.Review.RequiredCapabilities, []string{"review"})
}

func profileAgentSelectorCount(ctx context.Context, t *testing.T, globalDB *GlobalDB) int {
	t.Helper()

	var count int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_profile_agents`).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext(task_profile_agents count) error = %v", err)
	}
	return count
}

func assertStringSliceGlobal(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("slice = %#v, want %#v", got, want)
	}
	for idx := range got {
		if got[idx] != want[idx] {
			t.Fatalf("slice = %#v, want %#v", got, want)
		}
	}
}
