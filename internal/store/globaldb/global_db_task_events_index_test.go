package globaldb

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestTaskEventsTypeSeqIndexFreshDB(t *testing.T) {
	t.Parallel()

	t.Run("Should install the type-sequence index on a fresh database", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		assertTaskEventIndexesReady(t, globalDB.db)
	})
}

func TestTaskEventsTypeSeqIndexReopenAfterRestart(t *testing.T) {
	t.Parallel()

	t.Run("Should retain the type-sequence index after reopening", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		assertTaskEventIndexesReady(t, first.db)
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(ctx); err != nil {
				t.Errorf("Close(second) error = %v", err)
			}
		})
		assertTaskEventIndexesReady(t, second.db)
	})
}

func TestTaskWakeEventLookupUsesTaskScopedAuditIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject decoys and find the exact task wake identity", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskA := taskRecordForTest("task-wake-lookup-a")
		taskB := taskRecordForTest("task-wake-lookup-b")
		for _, record := range []taskpkg.Task{taskA, taskB} {
			if err := globalDB.CreateTask(ctx, record); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
			}
		}
		appendWakeLookupEventForTest(
			t, globalDB, "evt-wake-wrong-type", taskA.ID, "task.updated", "wake-target",
		)
		appendWakeLookupEventForTest(
			t, globalDB, "evt-wake-other-task", taskB.ID, eventspkg.TaskWakeDelivered, "wake-target",
		)
		appendWakeLookupEventForTest(
			t, globalDB, "evt-wake-other-id", taskA.ID, eventspkg.TaskWakeSuppressed, "wake-other",
		)

		recorded, err := globalDB.TaskWakeEventExists(ctx, taskA.ID, "wake-target")
		if err != nil {
			t.Fatalf("TaskWakeEventExists(decoys) error = %v", err)
		}
		if recorded {
			t.Fatal("TaskWakeEventExists(decoys) = true, want false")
		}
		appendWakeLookupEventForTest(
			t, globalDB, "evt-wake-target", taskA.ID, eventspkg.TaskWakeDelivered, "wake-target",
		)
		recorded, err = globalDB.TaskWakeEventExists(ctx, taskA.ID, "wake-target")
		if err != nil {
			t.Fatalf("TaskWakeEventExists(target) error = %v", err)
		}
		if !recorded {
			t.Fatal("TaskWakeEventExists(target) = false, want true")
		}
	})
}

func appendWakeLookupEventForTest(
	t *testing.T,
	globalDB *GlobalDB,
	eventID string,
	taskID string,
	eventType string,
	wakeEventID string,
) {
	t.Helper()

	event := taskEventForTest(eventID, taskID, "")
	event.EventType = eventType
	payload, err := json.Marshal(map[string]string{"wake_event_id": wakeEventID})
	if err != nil {
		t.Fatalf("json.Marshal(wake payload) error = %v", err)
	}
	event.Payload = payload
	if err := globalDB.CreateTaskEvent(testutil.Context(t), event); err != nil {
		t.Fatalf("CreateTaskEvent(%q) error = %v", eventID, err)
	}
}

func assertTaskEventIndexesReady(t *testing.T, db *sql.DB) {
	t.Helper()

	assertIndexSQLContains(t, db, "idx_task_events_type_seq", "task_events(event_type, event_seq)")
	assertQueryPlanUsesIndex(
		t,
		db,
		`SELECT MAX(event_seq) FROM task_events WHERE event_type = ?`,
		"idx_task_events_type_seq",
		string(hookspkg.HookTaskStatusChanged),
	)
	assertIndexSQLContains(
		t,
		db,
		"idx_task_events_wake_event",
		"task_events(task_id, event_type, json_extract(payload_json, '$.wake_event_id'))",
	)
	assertQueryPlanUsesIndex(
		t,
		db,
		`SELECT EXISTS(
			SELECT 1 FROM task_events
			WHERE task_id = ?
			  AND event_type IN ('task.wake.delivered', 'task.wake.suppressed')
			  AND json_extract(payload_json, '$.wake_event_id') = ?
		)`,
		"idx_task_events_wake_event",
		"task-id",
		"wake-id",
	)
}
