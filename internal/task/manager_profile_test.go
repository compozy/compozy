package task

import (
	"context"
	"errors"
	"testing"
)

func TestTaskManagerExecutionProfiles(t *testing.T) {
	t.Parallel()

	t.Run("Should return default inherit profile when no row is persisted", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		store.tasks["task-1"] = validProfileTask("task-1")
		manager := newTaskManagerForTest(t, store)

		profile, err := manager.GetExecutionProfile(context.Background(), "task-1", validActorContext())
		if err != nil {
			t.Fatalf("GetExecutionProfile() error = %v", err)
		}

		if got, want := profile.Coordinator.Mode, CoordinatorModeInherit; got != want {
			t.Fatalf("Coordinator.Mode = %q, want %q", got, want)
		}
		if got, want := profile.Worker.Mode, WorkerModeInherit; got != want {
			t.Fatalf("Worker.Mode = %q, want %q", got, want)
		}
		if got, want := profile.Sandbox.Mode, SandboxModeInherit; got != want {
			t.Fatalf("Sandbox.Mode = %q, want %q", got, want)
		}
	})

	t.Run("Should persist validated profile and emit audit event", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		store.tasks["task-1"] = validProfileTask("task-1")
		manager := newTaskManagerForTest(t, store)

		profile, err := manager.SetExecutionProfile(context.Background(), "task-1", &ExecutionProfile{
			Coordinator: CoordinatorProfile{Mode: CoordinatorModeGuided, Guidance: "Keep the worker focused."},
			Worker: WorkerProfile{
				Mode:                  WorkerModeSelect,
				AgentName:             "coder",
				AllowedAgentNames:     []string{"coder"},
				RequiredCapabilities:  []string{"go"},
				PreferredCapabilities: []string{"tests"},
			},
			Review: ReviewProfile{
				AgentName:         "reviewer",
				AllowedAgentNames: []string{"reviewer"},
			},
			Sandbox: SandboxPolicy{Mode: SandboxModeRef, SandboxRef: "workspace"},
		}, validActorContext())
		if err != nil {
			t.Fatalf("SetExecutionProfile() error = %v", err)
		}

		if got, want := profile.TaskID, "task-1"; got != want {
			t.Fatalf("TaskID = %q, want %q", got, want)
		}
		if got, want := store.profiles["task-1"].Worker.AgentName, "coder"; got != want {
			t.Fatalf("stored Worker.AgentName = %q, want %q", got, want)
		}
		if !containsEventType(store.events, taskEventProfileUpdated) {
			t.Fatalf("events = %#v, want %q", sortedEventTypes(store.events), taskEventProfileUpdated)
		}
	})

	t.Run("Should reject profile mutation while current run is active", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		taskRecord := validProfileTask("task-1")
		taskRecord.CurrentRunID = "run-active"
		store.tasks[taskRecord.ID] = taskRecord
		manager := newTaskManagerForTest(t, store)

		_, err := manager.SetExecutionProfile(
			context.Background(),
			"task-1",
			&ExecutionProfile{},
			validActorContext(),
		)
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("SetExecutionProfile(active) error = %v, want %v", err, ErrInvalidStatusTransition)
		}
	})

	t.Run("Should delete persisted profile and emit audit event", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		store.tasks["task-1"] = validProfileTask("task-1")
		store.profiles["task-1"] = ExecutionProfile{
			TaskID:      "task-1",
			Coordinator: CoordinatorProfile{Mode: CoordinatorModeGuided},
			Worker:      WorkerProfile{Mode: WorkerModeInherit},
			Sandbox:     SandboxPolicy{Mode: SandboxModeInherit},
		}
		manager := newTaskManagerForTest(t, store)

		if err := manager.DeleteExecutionProfile(context.Background(), "task-1", validActorContext()); err != nil {
			t.Fatalf("DeleteExecutionProfile() error = %v", err)
		}
		if _, ok := store.profiles["task-1"]; ok {
			t.Fatalf("profile still present: %#v", store.profiles["task-1"])
		}
		if !containsEventType(store.events, taskEventProfileDeleted) {
			t.Fatalf("events = %#v, want %q", sortedEventTypes(store.events), taskEventProfileDeleted)
		}
	})
}

func validProfileTask(id string) Task {
	record := validTask()
	record.ID = id
	record.Identifier = "TASK-" + id
	return record
}
