package globaldb

import (
	"errors"
	"testing"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBTerminalRunHistoryImport(t *testing.T) {
	t.Parallel()

	t.Run("Should atomically import one completed run projection and audit event", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := completedHistoryTaskForTest("success")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := completedHistoryRunForTest("success", taskRecord)
		actor := operatorActorContextForTest("operator:history-success")
		command, err := taskpkg.NewTerminalRunHistoryImport(run, actor)
		if err != nil {
			t.Fatalf("NewTerminalRunHistoryImport() error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)

		if err := globalDB.ImportTerminalRunHistory(ctx, &command); err != nil {
			t.Fatalf("ImportTerminalRunHistory() error = %v", err)
		}
		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if storedRun.ID != run.ID ||
			storedRun.TaskID != run.TaskID ||
			storedRun.Status.Normalize() != taskpkg.TaskRunStatusCompleted ||
			storedRun.SessionID != run.SessionID ||
			!storedRun.StartedAt.Equal(run.StartedAt) ||
			!storedRun.EndedAt.Equal(run.EndedAt) ||
			string(storedRun.Result) != string(run.Result) {
			t.Fatalf("stored run = %#v, want completed history %#v", storedRun, run)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if storedTask.CurrentRunID != "" {
			t.Fatalf("CurrentRunID = %q, want no active projection for completed history", storedTask.CurrentRunID)
		}
		events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID:    taskRecord.ID,
			RunID:     run.ID,
			EventType: eventspkg.TaskRunCompleted,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("completed event count = %d, want %d", got, want)
		}
		if got, want := len(observer.records), 1; got != want {
			t.Fatalf("published event count = %d, want %d", got, want)
		}
	})

	t.Run("Should roll back run and projection when completed event append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := completedHistoryTaskForTest("rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := completedHistoryRunForTest("rollback", taskRecord)
		command, err := taskpkg.NewTerminalRunHistoryImport(
			run,
			operatorActorContextForTest("operator:history-rollback"),
		)
		if err != nil {
			t.Fatalf("NewTerminalRunHistoryImport() error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunCompleted)

		err = globalDB.ImportTerminalRunHistory(ctx, &command)
		assertForcedTaskEventInsertError(t, err, "ImportTerminalRunHistory()")
		if _, err := globalDB.GetTaskRun(ctx, run.ID); !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
			t.Fatalf("GetTaskRun(after rollback) error = %v, want %v", err, taskpkg.ErrTaskRunNotFound)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if storedTask.CurrentRunID != "" {
			t.Fatalf("CurrentRunID after rollback = %q, want empty", storedTask.CurrentRunID)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("published event count after rollback = %d, want 0", got)
		}
	})

	t.Run("Should preserve failed and canceled outcomes in their task projections", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name       string
			runStatus  taskpkg.RunStatus
			taskStatus taskpkg.Status
			eventType  string
		}{
			{name: "failed", runStatus: taskpkg.TaskRunStatusFailed, taskStatus: taskpkg.TaskStatusFailed, eventType: eventspkg.TaskRunFailed},
			{name: "canceled", runStatus: taskpkg.TaskRunStatusCanceled, taskStatus: taskpkg.TaskStatusCanceled, eventType: eventspkg.TaskRunCanceled},
		} {
			t.Run("Should import "+testCase.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				globalDB := openTestGlobalDB(t)
				taskRecord := completedHistoryTaskForTest(testCase.name)
				taskRecord.Status = testCase.taskStatus
				if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
					t.Fatalf("CreateTask() error = %v", err)
				}
				run := completedHistoryRunForTest(testCase.name, taskRecord)
				run.Status = testCase.runStatus
				command, err := taskpkg.NewTerminalRunHistoryImport(
					run,
					operatorActorContextForTest("operator:history-"+testCase.name),
				)
				if err != nil {
					t.Fatalf("NewTerminalRunHistoryImport() error = %v", err)
				}
				if err := globalDB.ImportTerminalRunHistory(ctx, &command); err != nil {
					t.Fatalf("ImportTerminalRunHistory() error = %v", err)
				}
				storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
				if err != nil {
					t.Fatalf("GetTaskRun() error = %v", err)
				}
				if storedRun.Status.Normalize() != testCase.runStatus {
					t.Fatalf("stored run status = %q, want %q", storedRun.Status, testCase.runStatus)
				}
				events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
					TaskID: taskRecord.ID, RunID: run.ID, EventType: testCase.eventType,
				})
				if err != nil {
					t.Fatalf("ListTaskEvents() error = %v", err)
				}
				if len(events) != 1 {
					t.Fatalf("terminal event count = %d, want 1", len(events))
				}
			})
		}
	})

	t.Run("Should reject a missing import command", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		err := globalDB.ImportTerminalRunHistory(testutil.Context(t), nil)
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("ImportTerminalRunHistory(nil) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reject every non-terminal source before persistence", func(t *testing.T) {
		t.Parallel()

		run := completedHistoryRunForTest("reject", completedHistoryTaskForTest("reject"))
		run.Status = taskpkg.TaskRunStatusQueued
		run.EndedAt = time.Time{}
		_, err := taskpkg.NewTerminalRunHistoryImport(
			run,
			operatorActorContextForTest("operator:history-reject"),
		)
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf(
				"NewTerminalRunHistoryImport(queued) error = %v, want %v",
				err,
				taskpkg.ErrInvalidStatusTransition,
			)
		}
	})
}

func completedHistoryTaskForTest(suffix string) taskpkg.Task {
	taskRecord := taskRecordForTest("task-history-" + suffix)
	taskRecord.Status = taskpkg.TaskStatusCompleted
	taskRecord.ClosedAt = taskRecord.UpdatedAt.Add(10 * time.Minute)
	return taskRecord
}

func completedHistoryRunForTest(suffix string, taskRecord taskpkg.Task) taskpkg.Run {
	run := taskRunForTest("run-history-"+suffix, taskRecord.ID)
	run.Status = taskpkg.TaskRunStatusCompleted
	run.StartedAt = run.QueuedAt.Add(time.Minute)
	run.EndedAt = run.StartedAt.Add(5 * time.Minute)
	run.Result = []byte(`{"ok":true}`)
	return run
}
