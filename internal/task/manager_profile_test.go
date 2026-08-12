package task

import (
	"context"
	"errors"
	"reflect"
	"strings"
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
		if got, want := profile.Worktree.Mode, WorktreeModeInherit; got != want {
			t.Fatalf("Worktree.Mode = %q, want %q", got, want)
		}
	})

	t.Run("Should patch only the worktree policy block", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		store.tasks["task-1"] = validProfileTask("task-1")
		before := ExecutionProfile{
			TaskID:      "task-1",
			Coordinator: CoordinatorProfile{Mode: CoordinatorModeGuided, Guidance: "Keep scope."},
			Worker: WorkerProfile{
				Mode: WorkerModeSelect, AgentName: "coder", AllowedAgentNames: []string{"coder"},
			},
			Review:       ReviewProfile{AgentName: "reviewer", AllowedAgentNames: []string{"reviewer"}},
			Participants: ParticipantPolicy{AllowedPeerIDs: []string{"peer-1"}},
			Sandbox:      SandboxPolicy{Mode: SandboxModeRef, SandboxRef: "workspace"},
			Worktree:     WorktreePolicy{Mode: WorktreeModeNone},
			Runtime:      RuntimePolicy{Mode: RuntimeModeEvidence},
		}
		store.profiles["task-1"] = before
		manager := newTaskManagerForTest(t, store)

		after, err := manager.SetWorktreePolicy(
			context.Background(),
			"task-1",
			WorktreePolicy{Mode: WorktreeModePerRun},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("SetWorktreePolicy() error = %v", err)
		}
		if got, want := after.Worktree.Mode, WorktreeModePerRun; got != want {
			t.Fatalf("Worktree.Mode = %q, want %q", got, want)
		}
		if !reflect.DeepEqual(after.Coordinator, before.Coordinator) ||
			!reflect.DeepEqual(after.Worker, before.Worker) ||
			!reflect.DeepEqual(after.Review, before.Review) ||
			!reflect.DeepEqual(after.Participants, before.Participants) ||
			!reflect.DeepEqual(after.Sandbox, before.Sandbox) ||
			!reflect.DeepEqual(after.Runtime, before.Runtime) {
			t.Fatalf("SetWorktreePolicy() changed unrelated profile blocks: before=%#v after=%#v", before, after)
		}
	})

	t.Run("Should reject unavailable worktree references in the task workspace", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		taskRecord := validProfileTask("task-1")
		taskRecord.WorkspaceID = "ws-profile"
		store.tasks[taskRecord.ID] = taskRecord
		wantErr := errors.New("worktree_ref_invalid")
		validator := &recordingTaskWorktreeRefValidator{err: wantErr}
		manager := newTaskManagerForTestWithOptions(t, store, WithWorktreeRefValidator(validator))

		for _, ref := range []string{"missing", "foreign-workspace-ref"} {
			_, err := manager.SetWorktreePolicy(
				context.Background(),
				taskRecord.ID,
				WorktreePolicy{Mode: WorktreeModeRef, WorktreeRef: " " + ref + " "},
				validActorContext(),
			)
			if !errors.Is(err, wantErr) {
				t.Fatalf("SetWorktreePolicy(%s) error = %v, want %v", ref, err, wantErr)
			}
		}
		if got, want := validator.calls, []taskWorktreeRefValidationCall{
			{workspaceID: "ws-profile", ref: "missing"},
			{workspaceID: "ws-profile", ref: "foreign-workspace-ref"},
		}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ValidateTaskWorktreeRef() calls = %#v, want %#v", got, want)
		}
		if _, ok := store.profiles[taskRecord.ID]; ok {
			t.Fatalf("profile persisted after rejected worktree refs: %#v", store.profiles[taskRecord.ID])
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

	t.Run("Should reject worktree policy patch while current run is active", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		taskRecord := validProfileTask("task-1")
		taskRecord.CurrentRunID = "run-active"
		store.tasks[taskRecord.ID] = taskRecord
		manager := newTaskManagerForTest(t, store)

		_, err := manager.SetWorktreePolicy(
			context.Background(),
			"task-1",
			WorktreePolicy{Mode: WorktreeModePerRun},
			validActorContext(),
		)
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("SetWorktreePolicy(active) error = %v, want %v", err, ErrInvalidStatusTransition)
		}
	})

	t.Run("Should snapshot the resolved worktree policy at enqueue", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		store.tasks["task-1"] = validProfileTask("task-1")
		store.profiles["task-1"] = ExecutionProfile{
			TaskID:   "task-1",
			Worktree: WorktreePolicy{Mode: WorktreeModePerRun},
		}
		manager := newTaskManagerForTest(t, store)

		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: "task-1"},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		store.profiles["task-1"] = ExecutionProfile{
			TaskID:   "task-1",
			Worktree: WorktreePolicy{Mode: WorktreeModeNone},
		}
		stored, err := store.GetTaskRun(context.Background(), run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := stored.ResolvedWorktreeMode, WorktreeModePerRun; got != want {
			t.Fatalf("ResolvedWorktreeMode = %q, want immutable %q", got, want)
		}
	})

	t.Run("Should let fan-out request dedicated per-run worktrees without mutating the profile", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		store.tasks["task-1"] = validProfileTask("task-1")
		store.profiles["task-1"] = ExecutionProfile{
			TaskID:   "task-1",
			Worktree: WorktreePolicy{Mode: WorktreeModeNone},
		}
		manager := newTaskManagerForTest(t, store)

		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: "task-1", WorktreePerRun: true},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		if got, want := run.ResolvedWorktreeMode, WorktreeModePerRun; got != want {
			t.Fatalf("ResolvedWorktreeMode = %q, want fan-out override %q", got, want)
		}
		if got, want := store.profiles["task-1"].Worktree.Mode, WorktreeModeNone; got != want {
			t.Fatalf("stored profile Worktree.Mode = %q, want unchanged %q", got, want)
		}
	})

	t.Run("Should resolve inherit from config without importing config", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		store.tasks["task-1"] = validProfileTask("task-1")
		options := DefaultExecutionProfileValidationOptions()
		options.DefaultWorktreeMode = WorktreeModePerRun
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithExecutionProfileValidationOptions(options),
		)

		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: "task-1"},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		if got, want := run.ResolvedWorktreeMode, WorktreeModePerRun; got != want {
			t.Fatalf("ResolvedWorktreeMode = %q, want config default %q", got, want)
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
		profile, err := manager.GetExecutionProfile(context.Background(), "task-1", validActorContext())
		if err != nil {
			t.Fatalf("GetExecutionProfile(after delete) error = %v", err)
		}
		if got, want := profile.Worktree.Mode, WorktreeModeInherit; got != want {
			t.Fatalf("Worktree.Mode after delete = %q, want %q", got, want)
		}
	})
}

func validProfileTask(id string) Task {
	record := validTask()
	record.ID = id
	record.Identifier = "TASK-" + id
	return record
}

type taskWorktreeRefValidationCall struct {
	workspaceID string
	ref         string
}

type recordingTaskWorktreeRefValidator struct {
	calls []taskWorktreeRefValidationCall
	err   error
}

func (v *recordingTaskWorktreeRefValidator) ValidateTaskWorktreeRef(
	_ context.Context,
	workspaceID string,
	ref string,
) error {
	v.calls = append(v.calls, taskWorktreeRefValidationCall{
		workspaceID: strings.TrimSpace(workspaceID),
		ref:         strings.TrimSpace(ref),
	})
	return v.err
}
