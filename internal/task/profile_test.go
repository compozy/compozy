package task

import (
	"errors"
	"testing"
)

func TestExecutionProfileValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize default modes and selector sets", func(t *testing.T) {
		t.Parallel()

		profile, err := (&ExecutionProfile{
			TaskID: " task-1 ",
			Worker: WorkerProfile{
				AllowedAgentNames:    []string{" coder ", "reviewer", "coder"},
				RequiredCapabilities: []string{"go", "test", "go"},
			},
			Review: ReviewProfile{
				AllowedChannelIDs: []string{"chan-b", "chan-a", "chan-b"},
			},
			Participants: ParticipantPolicy{
				PreferredPeerIDs: []string{"peer-2", "peer-1", "peer-2"},
			},
		}).Normalize(DefaultExecutionProfileValidationOptions())
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
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
		assertStringSlice(t, profile.Worker.AllowedAgentNames, []string{"coder", "reviewer"})
		assertStringSlice(t, profile.Worker.RequiredCapabilities, []string{"go", "test"})
		assertStringSlice(t, profile.Review.AllowedChannelIDs, []string{"chan-a", "chan-b"})
		assertStringSlice(t, profile.Participants.PreferredPeerIDs, []string{"peer-1", "peer-2"})
	})

	t.Run("Should reject provider overrides when config gate is disabled", func(t *testing.T) {
		t.Parallel()

		_, err := (&ExecutionProfile{
			TaskID: "task-1",
			Worker: WorkerProfile{
				Provider: "claude",
			},
		}).Normalize(ExecutionProfileValidationOptions{
			AllowProviderOverride: false,
			AllowSandboxNone:      true,
			AllowSandboxRef:       true,
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Normalize() error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject exact worker outside allowed agents", func(t *testing.T) {
		t.Parallel()

		_, err := (&ExecutionProfile{
			TaskID: "task-1",
			Worker: WorkerProfile{
				AgentName:         "coder-a",
				AllowedAgentNames: []string{"coder-b"},
			},
		}).Normalize(DefaultExecutionProfileValidationOptions())
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Normalize() error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject sandbox none when config gate is disabled", func(t *testing.T) {
		t.Parallel()

		_, err := (&ExecutionProfile{
			TaskID:  "task-1",
			Sandbox: SandboxPolicy{Mode: SandboxModeNone},
		}).Normalize(ExecutionProfileValidationOptions{
			AllowProviderOverride: true,
			AllowSandboxNone:      false,
			AllowSandboxRef:       true,
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Normalize() error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should require sandbox ref for ref mode", func(t *testing.T) {
		t.Parallel()

		_, err := (&ExecutionProfile{
			TaskID:  "task-1",
			Sandbox: SandboxPolicy{Mode: SandboxModeRef},
		}).Normalize(DefaultExecutionProfileValidationOptions())
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Normalize() error = %v, want %v", err, ErrValidation)
		}
	})
}

func assertStringSlice(t *testing.T, got []string, want []string) {
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
