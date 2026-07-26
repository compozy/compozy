package globaldb

import (
	"testing"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBRunStarvation(t *testing.T) {
	t.Parallel()

	t.Run("Should count only queued runs with an active convergence episode", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-starvation-count")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		ageOnly := taskRunForTest("run-age-only", taskRecord.ID)
		escalating := taskRunForTest("run-escalating", taskRecord.ID)
		for _, run := range []taskpkg.Run{ageOnly, escalating} {
			if err := globalDB.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
			}
		}
		now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
		if _, err := globalDB.UpsertRunStarvation(ctx, taskpkg.RunStarvationMutation{
			RunID:          escalating.ID,
			WakeCount:      1,
			FirstStarvedAt: now,
			LastWakeAt:     now,
			UpdatedAt:      now,
		}); err != nil {
			t.Fatalf("UpsertRunStarvation() error = %v", err)
		}

		count, err := globalDB.CountEscalatingQueuedTaskRuns(ctx)
		if err != nil {
			t.Fatalf("CountEscalatingQueuedTaskRuns() error = %v", err)
		}
		if got, want := count, 1; got != want {
			t.Fatalf("CountEscalatingQueuedTaskRuns() = %d, want %d active episode", got, want)
		}
	})

	for _, testCase := range []struct {
		name           string
		runStatus      taskpkg.RunStatus
		taskStatus     taskpkg.Status
		directPaused   bool
		ancestorPaused bool
	}{
		{
			name:       "Should exclude a claimed run episode from scheduler pressure",
			runStatus:  taskpkg.TaskRunStatusClaimed,
			taskStatus: taskpkg.TaskStatusReady,
		},
		{
			name:       "Should exclude a running run episode from scheduler pressure",
			runStatus:  taskpkg.TaskRunStatusRunning,
			taskStatus: taskpkg.TaskStatusReady,
		},
		{
			name:       "Should exclude a terminal run episode from scheduler pressure",
			runStatus:  taskpkg.TaskRunStatusCompleted,
			taskStatus: taskpkg.TaskStatusCompleted,
		},
		{
			name:         "Should exclude a directly paused task episode from scheduler pressure",
			runStatus:    taskpkg.TaskRunStatusQueued,
			taskStatus:   taskpkg.TaskStatusReady,
			directPaused: true,
		},
		{
			name:           "Should exclude an ancestor-paused task episode from scheduler pressure",
			runStatus:      taskpkg.TaskRunStatusQueued,
			taskStatus:     taskpkg.TaskStatusReady,
			ancestorPaused: true,
		},
		{
			name:       "Should exclude a blocked task episode from scheduler pressure",
			runStatus:  taskpkg.TaskRunStatusQueued,
			taskStatus: taskpkg.TaskStatusBlocked,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			if testCase.ancestorPaused {
				parent := taskRecordForTest("task-starvation-parent-paused")
				parent.Status = taskpkg.TaskStatusReady
				parent.Paused = true
				parent.PausedBy = "scheduler-test"
				parent.PausedReason = "hold descendants"
				parent.PausedAt = parent.CreatedAt
				if err := globalDB.CreateTask(ctx, parent); err != nil {
					t.Fatalf("CreateTask(parent) error = %v", err)
				}
			}
			taskRecord := taskRecordForTest("task-starvation-excluded")
			taskRecord.Status = testCase.taskStatus
			taskRecord.Paused = testCase.directPaused
			if testCase.directPaused {
				taskRecord.PausedBy = "scheduler-test"
				taskRecord.PausedReason = "hold task"
				taskRecord.PausedAt = taskRecord.CreatedAt
			}
			if testCase.ancestorPaused {
				taskRecord.ParentTaskID = "task-starvation-parent-paused"
			}
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run := taskRunForTest("run-starvation-excluded", taskRecord.ID)
			if err := globalDB.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun() error = %v", err)
			}
			if testCase.runStatus.Normalize() != taskpkg.TaskRunStatusQueued {
				if _, err := globalDB.db.ExecContext(
					ctx,
					`UPDATE task_runs SET status = ? WHERE id = ?`,
					testCase.runStatus.String(),
					run.ID,
				); err != nil {
					t.Fatalf("update run status error = %v", err)
				}
			}
			now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
			if _, err := globalDB.UpsertRunStarvation(ctx, taskpkg.RunStarvationMutation{
				RunID:          run.ID,
				WakeCount:      2,
				FirstStarvedAt: now,
				LastWakeAt:     now,
				UpdatedAt:      now,
			}); err != nil {
				t.Fatalf("UpsertRunStarvation() error = %v", err)
			}

			count, err := globalDB.CountEscalatingQueuedTaskRuns(ctx)
			if err != nil {
				t.Fatalf("CountEscalatingQueuedTaskRuns() error = %v", err)
			}
			if count != 0 {
				t.Fatalf("CountEscalatingQueuedTaskRuns() = %d, want 0", count)
			}
		})
	}

	t.Run("Should round-trip the escalation budget and clear it", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-starvation")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-starvation", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		if _, ok, err := globalDB.LoadRunStarvation(ctx, run.ID); err != nil || ok {
			t.Fatalf("LoadRunStarvation(absent) = (ok %v, err %v), want (false, nil)", ok, err)
		}

		firstStarved := time.Date(2026, 5, 28, 12, 0, 0, 123456789, time.UTC)
		lastWake := time.Date(2026, 5, 28, 12, 6, 0, 500000000, time.UTC)
		spawnAt := time.Date(2026, 5, 28, 12, 4, 0, 250000000, time.UTC)
		eventAt := time.Date(2026, 5, 28, 12, 6, 0, 750000000, time.UTC)
		mutation := taskpkg.RunStarvationMutation{
			RunID:            run.ID,
			WakeCount:        6,
			FirstStarvedAt:   firstStarved,
			LastWakeAt:       lastWake,
			EscalationTier:   3,
			SpawnRequestedAt: &spawnAt,
			StarvedEventAt:   &eventAt,
			UpdatedAt:        lastWake,
		}
		if _, err := globalDB.UpsertRunStarvation(ctx, mutation); err != nil {
			t.Fatalf("UpsertRunStarvation() error = %v", err)
		}

		loaded, ok, err := globalDB.LoadRunStarvation(ctx, run.ID)
		if err != nil || !ok {
			t.Fatalf("LoadRunStarvation() = (ok %v, err %v), want (true, nil)", ok, err)
		}
		if loaded.WakeCount != 6 || loaded.EscalationTier != 3 {
			t.Fatalf("loaded counters = wake %d tier %d, want 6/3", loaded.WakeCount, loaded.EscalationTier)
		}
		if !loaded.FirstStarvedAt.Equal(firstStarved) || !loaded.LastWakeAt.Equal(lastWake) {
			t.Fatalf("timestamps did not round-trip: first %v last %v", loaded.FirstStarvedAt, loaded.LastWakeAt)
		}
		if loaded.SpawnRequestedAt == nil || !loaded.SpawnRequestedAt.Equal(spawnAt) {
			t.Fatalf("spawn_requested_at = %v, want %v", loaded.SpawnRequestedAt, spawnAt)
		}
		if loaded.StarvedEventAt == nil || !loaded.StarvedEventAt.Equal(eventAt) {
			t.Fatalf("starved_event_at = %v, want %v", loaded.StarvedEventAt, eventAt)
		}

		rows, err := globalDB.ListRunStarvation(ctx)
		if err != nil {
			t.Fatalf("ListRunStarvation() error = %v", err)
		}
		if len(rows) != 1 || rows[0].RunID != run.ID {
			t.Fatalf("ListRunStarvation() = %#v, want one row for %q", rows, run.ID)
		}

		if err := globalDB.ClearRunStarvation(ctx, run.ID); err != nil {
			t.Fatalf("ClearRunStarvation() error = %v", err)
		}
		if _, ok, err := globalDB.LoadRunStarvation(ctx, run.ID); err != nil || ok {
			t.Fatalf("LoadRunStarvation(after clear) = (ok %v, err %v), want (false, nil)", ok, err)
		}
	})

	t.Run("Should cascade-delete the budget when its run is deleted", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-starvation-cascade")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-starvation-cascade", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
		if _, err := globalDB.UpsertRunStarvation(ctx, taskpkg.RunStarvationMutation{
			RunID: run.ID, WakeCount: 1, FirstStarvedAt: now, LastWakeAt: now, EscalationTier: 0, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("UpsertRunStarvation() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM task_runs WHERE id = ?`, run.ID); err != nil {
			t.Fatalf("delete task run error = %v", err)
		}
		if _, ok, err := globalDB.LoadRunStarvation(ctx, run.ID); err != nil || ok {
			t.Fatalf("budget survived run deletion: (ok %v, err %v), want cascade delete", ok, err)
		}
	})
}
