package globaldb

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBTaskDependencyRoundTripAndDelete(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	rootTask := taskRecordForTest("task-dependency-root")
	middleTask := taskRecordForTest("task-dependency-middle")
	leafTask := taskRecordForTest("task-dependency-leaf")

	for _, record := range []taskpkg.Task{rootTask, middleTask, leafTask} {
		if err := globalDB.CreateTask(testutil.Context(t), record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}

	rootDependsOnMiddle := taskDependencyForTest(rootTask.ID, middleTask.ID)
	middleDependsOnLeaf := taskDependencyForTest(middleTask.ID, leafTask.ID)
	for _, dependency := range []taskpkg.Dependency{rootDependsOnMiddle, middleDependsOnLeaf} {
		if err := globalDB.CreateDependency(testutil.Context(t), dependency); err != nil {
			t.Fatalf("CreateDependency(%q -> %q) error = %v", dependency.TaskID, dependency.DependsOnTaskID, err)
		}
	}

	dependencies, err := globalDB.ListDependencies(testutil.Context(t), rootTask.ID)
	if err != nil {
		t.Fatalf("ListDependencies() error = %v", err)
	}
	if got, want := len(dependencies), 1; got != want {
		t.Fatalf("len(ListDependencies()) = %d, want %d", got, want)
	}
	assertTaskDependencyEqual(t, dependencies[0], rootDependsOnMiddle)

	dependents, err := globalDB.ListDependents(testutil.Context(t), middleTask.ID)
	if err != nil {
		t.Fatalf("ListDependents() error = %v", err)
	}
	if got, want := len(dependents), 1; got != want {
		t.Fatalf("len(ListDependents()) = %d, want %d", got, want)
	}
	assertTaskDependencyEqual(t, dependents[0], rootDependsOnMiddle)

	count, err := globalDB.CountDependencies(testutil.Context(t), rootTask.ID)
	if err != nil {
		t.Fatalf("CountDependencies() error = %v", err)
	}
	if got, want := count, 1; got != want {
		t.Fatalf("CountDependencies() = %d, want %d", got, want)
	}

	hasPath, err := globalDB.HasDependencyPath(testutil.Context(t), rootTask.ID, leafTask.ID)
	if err != nil {
		t.Fatalf("HasDependencyPath(root, leaf) error = %v", err)
	}
	if !hasPath {
		t.Fatal("HasDependencyPath(root, leaf) = false, want true")
	}

	if err := globalDB.DeleteDependency(testutil.Context(t), rootTask.ID, middleTask.ID); err != nil {
		t.Fatalf("DeleteDependency() error = %v", err)
	}

	dependencies, err = globalDB.ListDependencies(testutil.Context(t), rootTask.ID)
	if err != nil {
		t.Fatalf("ListDependencies(after delete) error = %v", err)
	}
	if got := len(dependencies); got != 0 {
		t.Fatalf("len(ListDependencies(after delete)) = %d, want 0", got)
	}

	hasPath, err = globalDB.HasDependencyPath(testutil.Context(t), rootTask.ID, leafTask.ID)
	if err != nil {
		t.Fatalf("HasDependencyPath(after delete) error = %v", err)
	}
	if hasPath {
		t.Fatal("HasDependencyPath(after delete) = true, want false")
	}
}

func TestGlobalDBCreateDependencyRejectsInvalidEdges(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	rootTask := taskRecordForTest("task-dependency-guard-root")
	if err := globalDB.CreateTask(testutil.Context(t), rootTask); err != nil {
		t.Fatalf("CreateTask(root) error = %v", err)
	}

	t.Run("Should reject self dependency", func(t *testing.T) {
		t.Parallel()

		err := globalDB.CreateDependency(testutil.Context(t), taskDependencyForTest(rootTask.ID, rootTask.ID))
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("CreateDependency(self) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reject duplicate dependency", func(t *testing.T) {
		t.Parallel()

		dependencyTask := taskRecordForTest("task-dependency-guard-duplicate")
		if err := globalDB.CreateTask(testutil.Context(t), dependencyTask); err != nil {
			t.Fatalf("CreateTask(duplicate target) error = %v", err)
		}

		dependency := taskDependencyForTest(rootTask.ID, dependencyTask.ID)
		if err := globalDB.CreateDependency(testutil.Context(t), dependency); err != nil {
			t.Fatalf("CreateDependency(first) error = %v", err)
		}

		err := globalDB.CreateDependency(testutil.Context(t), dependency)
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("CreateDependency(duplicate) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should enforce dependency limit", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		limitRoot := taskRecordForTest("task-dependency-limit-root")
		if err := globalDB.CreateTask(testutil.Context(t), limitRoot); err != nil {
			t.Fatalf("CreateTask(limit root) error = %v", err)
		}

		for idx := range taskpkg.MaxDependencyCount {
			dependencyTask := taskRecordForTest("task-dependency-limit-" + strconv.Itoa(idx))
			dependencyTask.ID = "task-dependency-limit-" + strconv.Itoa(idx)
			dependencyTask.Identifier = "identifier-task-dependency-limit-" + strconv.Itoa(idx)
			dependencyTask.Title = "Task dependency limit " + strconv.Itoa(idx)
			if err := globalDB.CreateTask(testutil.Context(t), dependencyTask); err != nil {
				t.Fatalf("CreateTask(limit target %d) error = %v", idx, err)
			}
			if err := globalDB.CreateDependency(
				testutil.Context(t),
				taskDependencyForTest(limitRoot.ID, dependencyTask.ID),
			); err != nil {
				t.Fatalf("CreateDependency(limit %d) error = %v", idx, err)
			}
		}

		overflowTask := taskRecordForTest("task-dependency-limit-overflow")
		if err := globalDB.CreateTask(testutil.Context(t), overflowTask); err != nil {
			t.Fatalf("CreateTask(limit overflow) error = %v", err)
		}

		err := globalDB.CreateDependency(testutil.Context(t), taskDependencyForTest(limitRoot.ID, overflowTask.ID))
		if !errors.Is(err, taskpkg.ErrGraphLimitExceeded) {
			t.Fatalf("CreateDependency(limit overflow) error = %v, want ErrGraphLimitExceeded", err)
		}
	})
}

func TestGlobalDBTaskEventRoundTripRejectsOversizePayloadAndPreservesOrigin(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-event-roundtrip")
	if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run := taskRunForTest("run-event-roundtrip", taskRecord.ID)
	run.Status = taskpkg.TaskRunStatusRunning
	run.SessionID = "sess-task-event-roundtrip"
	run.ClaimedBy = actorForTest(taskpkg.ActorKindDaemon, "scheduler")
	run.ClaimedAt = run.QueuedAt.Add(10 * time.Second)
	run.StartedAt = run.QueuedAt.Add(20 * time.Second)
	if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	event := taskEventForTest("evt-roundtrip", taskRecord.ID, run.ID)
	if err := globalDB.CreateTaskEvent(testutil.Context(t), event); err != nil {
		t.Fatalf("CreateTaskEvent() error = %v", err)
	}

	events, err := globalDB.ListTaskEvents(testutil.Context(t), taskpkg.EventQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(ListTaskEvents()) = %d, want %d", got, want)
	}
	assertTaskEventEqual(t, events[0], event)

	oversize := taskEventForTest("evt-oversize", taskRecord.ID, run.ID)
	oversize.Payload = taskJSONBlob(taskpkg.MaxPayloadBytes + 1)
	err = globalDB.CreateTaskEvent(testutil.Context(t), oversize)
	if !errors.Is(err, taskpkg.ErrPayloadTooLarge) {
		t.Fatalf("CreateTaskEvent(oversize) error = %v, want ErrPayloadTooLarge", err)
	}
}

func TestGlobalDBTaskEventRejectsRunTaskMismatch(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	firstTask := taskRecordForTest("task-event-first")
	secondTask := taskRecordForTest("task-event-second")
	for _, record := range []taskpkg.Task{firstTask, secondTask} {
		if err := globalDB.CreateTask(testutil.Context(t), record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}

	run := taskRunForTest("run-event-mismatch", firstTask.ID)
	if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	event := taskEventForTest("evt-mismatch", secondTask.ID, run.ID)
	err := globalDB.CreateTaskEvent(testutil.Context(t), event)
	if !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("CreateTaskEvent(run mismatch) error = %v, want ErrValidation", err)
	}
}

func TestGlobalDBTaskRunIdempotencyLookupUsesOriginScope(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-idempotency-scope")
	if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	runA := taskRunForTest("run-idempotency-a", taskRecord.ID)
	runA.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "rule:nightly"}
	runA.IdempotencyKey = "idem-shared"
	runB := taskRunForTest("run-idempotency-b", taskRecord.ID)
	runB.QueuedAt = runB.QueuedAt.Add(time.Minute)
	runB.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindNetwork, Ref: "peer:finance"}
	runB.IdempotencyKey = "idem-shared"
	for _, run := range []taskpkg.Run{runA, runB} {
		if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
			t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
		}
	}

	recordA := taskRunIdempotencyForTest("idem-shared", runA.ID, runA.Origin)
	recordB := taskRunIdempotencyForTest("idem-shared", runB.ID, runB.Origin)
	if err := globalDB.SaveTaskRunIdempotency(testutil.Context(t), recordA); err != nil {
		t.Fatalf("SaveTaskRunIdempotency(recordA) error = %v", err)
	}
	if err := globalDB.SaveTaskRunIdempotency(testutil.Context(t), recordA); err != nil {
		t.Fatalf("SaveTaskRunIdempotency(recordA duplicate same run) error = %v", err)
	}
	if err := globalDB.SaveTaskRunIdempotency(testutil.Context(t), recordB); err != nil {
		t.Fatalf("SaveTaskRunIdempotency(recordB) error = %v", err)
	}

	gotA, err := globalDB.GetTaskRunByIdempotencyKey(testutil.Context(t), recordA.IdempotencyKey, recordA.Origin)
	if err != nil {
		t.Fatalf("GetTaskRunByIdempotencyKey(recordA) error = %v", err)
	}
	assertTaskRunEqual(t, gotA, runA)

	gotB, err := globalDB.GetTaskRunByIdempotencyKey(testutil.Context(t), recordB.IdempotencyKey, recordB.Origin)
	if err != nil {
		t.Fatalf("GetTaskRunByIdempotencyKey(recordB) error = %v", err)
	}
	assertTaskRunEqual(t, gotB, runB)

	_, err = globalDB.GetTaskRunByIdempotencyKey(
		testutil.Context(t),
		recordA.IdempotencyKey,
		taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "http"},
	)
	if !errors.Is(err, taskpkg.ErrTaskRunIdempotencyNotFound) {
		t.Fatalf("GetTaskRunByIdempotencyKey(missing origin scope) error = %v, want ErrTaskRunIdempotencyNotFound", err)
	}
}

func TestGlobalDBTaskRunIdempotencyRejectsOriginMismatch(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-idempotency-mismatch")
	if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run := taskRunForTest("run-idempotency-mismatch", taskRecord.ID)
	run.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "rule:nightly"}
	if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	record := taskRunIdempotencyForTest(
		"idem-mismatch",
		run.ID,
		taskpkg.Origin{Kind: taskpkg.OriginKindNetwork, Ref: "peer:other"},
	)
	err := globalDB.SaveTaskRunIdempotency(testutil.Context(t), record)
	if !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("SaveTaskRunIdempotency(origin mismatch) error = %v, want ErrValidation", err)
	}
}

func TestGlobalDBTaskDependencyAndAuditErrorPaths(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	rootTask := taskRecordForTest("task-error-root")
	if err := globalDB.CreateTask(testutil.Context(t), rootTask); err != nil {
		t.Fatalf("CreateTask(root) error = %v", err)
	}

	err := globalDB.DeleteDependency(testutil.Context(t), rootTask.ID, "missing-dependency")
	if !errors.Is(err, taskpkg.ErrTaskDependencyNotFound) {
		t.Fatalf("DeleteDependency(missing) error = %v, want ErrTaskDependencyNotFound", err)
	}

	err = globalDB.CreateDependency(testutil.Context(t), taskDependencyForTest(rootTask.ID, "missing-task"))
	if !errors.Is(err, taskpkg.ErrTaskNotFound) {
		t.Fatalf("CreateDependency(missing target) error = %v, want ErrTaskNotFound", err)
	}

	if _, err := globalDB.ListDependencies(testutil.Context(t), " "); err == nil {
		t.Fatal("ListDependencies(empty) error = nil, want non-nil")
	}
	if _, err := globalDB.CountDependencies(testutil.Context(t), " "); err == nil {
		t.Fatal("CountDependencies(empty) error = nil, want non-nil")
	}
	if _, err := globalDB.HasDependencyPath(testutil.Context(t), "", rootTask.ID); err == nil {
		t.Fatal("HasDependencyPath(empty from) error = nil, want non-nil")
	}

	event := taskEventForTest("evt-missing-run", rootTask.ID, "missing-run")
	err = globalDB.CreateTaskEvent(testutil.Context(t), event)
	if !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
		t.Fatalf("CreateTaskEvent(missing run) error = %v, want ErrTaskRunNotFound", err)
	}
}

func TestGlobalDBTaskRunIdempotencyErrorPaths(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	if _, err := globalDB.GetTaskRunByIdempotencyKey(
		testutil.Context(t),
		"idem-missing",
		taskpkg.Origin{Kind: taskpkg.OriginKindHTTP},
	); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("GetTaskRunByIdempotencyKey(invalid origin) error = %v, want ErrValidation", err)
	}

	record := taskRunIdempotencyForTest(
		"idem-missing-run",
		"missing-run",
		taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "rule:nightly"},
	)
	err := globalDB.SaveTaskRunIdempotency(testutil.Context(t), record)
	if !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
		t.Fatalf("SaveTaskRunIdempotency(missing run) error = %v, want ErrTaskRunNotFound", err)
	}
}

func taskDependencyForTest(taskID string, dependsOnTaskID string) taskpkg.Dependency {
	return taskpkg.Dependency{
		TaskID:          taskID,
		DependsOnTaskID: dependsOnTaskID,
		Kind:            taskpkg.DependencyKindBlocks,
		CreatedAt:       time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC),
	}
}

func taskEventForTest(id string, taskID string, runID string) taskpkg.Event {
	return taskpkg.Event{
		ID:        id,
		TaskID:    taskID,
		RunID:     runID,
		EventType: "task.run_started",
		Actor: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon,
			Ref:  "scheduler",
		},
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKindAutomation,
			Ref:  "rule:nightly",
		},
		Payload:   json.RawMessage(`{"forced_stop":false}`),
		Timestamp: time.Date(2026, 4, 14, 14, 30, 0, 0, time.UTC),
	}
}

func taskRunIdempotencyForTest(key string, runID string, origin taskpkg.Origin) taskpkg.RunIdempotency {
	return taskpkg.RunIdempotency{
		IdempotencyKey: key,
		RunID:          runID,
		Origin:         origin,
		CreatedAt:      time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC),
	}
}

func taskJSONBlob(targetSize int) json.RawMessage {
	if targetSize <= 2 {
		return json.RawMessage(`""`)
	}
	return json.RawMessage(`"` + strings.Repeat("a", targetSize-2) + `"`)
}

func assertTaskDependencyEqual(t *testing.T, got taskpkg.Dependency, want taskpkg.Dependency) {
	t.Helper()

	if got.TaskID != want.TaskID ||
		got.DependsOnTaskID != want.DependsOnTaskID ||
		got.Kind != want.Kind ||
		!got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("task dependency = %#v, want %#v", got, want)
	}
}

func assertTaskEventEqual(t *testing.T, got taskpkg.Event, want taskpkg.Event) {
	t.Helper()

	if got.ID != want.ID ||
		got.TaskID != want.TaskID ||
		got.RunID != want.RunID ||
		got.EventType != want.EventType ||
		got.Actor != want.Actor ||
		got.Origin != want.Origin ||
		string(got.Payload) != string(want.Payload) ||
		!got.Timestamp.Equal(want.Timestamp) {
		t.Fatalf("task event = %#v, want %#v", got, want)
	}
}
