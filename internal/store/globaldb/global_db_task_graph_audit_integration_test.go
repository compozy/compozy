//go:build integration

package globaldb

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBCreateDependencyCycleFailsTransactionally(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)

	globalDB, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	taskA := taskRecordForTest("task-cycle-a")
	taskB := taskRecordForTest("task-cycle-b")
	taskC := taskRecordForTest("task-cycle-c")
	for _, record := range []taskpkg.Task{taskA, taskB, taskC} {
		if err := globalDB.CreateTask(ctx, record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}

	for _, dependency := range []taskpkg.Dependency{
		taskDependencyForTest(taskA.ID, taskB.ID),
		taskDependencyForTest(taskB.ID, taskC.ID),
	} {
		if err := globalDB.CreateDependency(ctx, dependency); err != nil {
			t.Fatalf("CreateDependency(%q -> %q) error = %v", dependency.TaskID, dependency.DependsOnTaskID, err)
		}
	}

	err = globalDB.CreateDependency(ctx, taskDependencyForTest(taskC.ID, taskA.ID))
	if !errors.Is(err, taskpkg.ErrCycleDetected) {
		t.Fatalf("CreateDependency(cycle) error = %v, want ErrCycleDetected", err)
	}

	dependencies, err := globalDB.ListDependencies(ctx, taskC.ID)
	if err != nil {
		t.Fatalf("ListDependencies(taskC) error = %v", err)
	}
	if got := len(dependencies); got != 0 {
		t.Fatalf("len(ListDependencies(taskC)) = %d, want 0", got)
	}
}

func TestGlobalDBTaskRunIdempotencyDeduplicatesDuplicateWrites(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)

	first, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	taskRecord := taskRecordForTest("task-idempotency-integration")
	if err := first.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	runOne := taskRunForTest("run-idempotency-integration-one", taskRecord.ID)
	runOne.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "rule:nightly"}
	runOne.IdempotencyKey = "idem-duplicate"
	runTwo := taskRunForTest("run-idempotency-integration-two", taskRecord.ID)
	runTwo.QueuedAt = runTwo.QueuedAt.Add(time.Minute)
	runTwo.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "rule:nightly"}
	runTwo.IdempotencyKey = "idem-duplicate"
	for _, run := range []taskpkg.Run{runOne, runTwo} {
		if err := first.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
		}
	}

	recordOne := taskRunIdempotencyForTest("idem-duplicate", runOne.ID, runOne.Origin)
	recordTwo := taskRunIdempotencyForTest("idem-duplicate", runTwo.ID, runTwo.Origin)
	if err := first.SaveTaskRunIdempotency(ctx, recordOne); err != nil {
		t.Fatalf("SaveTaskRunIdempotency(first) error = %v", err)
	}
	err = first.SaveTaskRunIdempotency(ctx, recordTwo)
	if !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("SaveTaskRunIdempotency(duplicate) error = %v, want ErrValidation", err)
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	gotRun, err := second.GetTaskRunByIdempotencyKey(ctx, recordOne.IdempotencyKey, recordOne.Origin)
	if err != nil {
		t.Fatalf("GetTaskRunByIdempotencyKey() error = %v", err)
	}
	assertTaskRunEqual(t, gotRun, runOne)
}
