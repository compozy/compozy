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
		if got, want := profile.Worktree.Mode, WorktreeModeInherit; got != want {
			t.Fatalf("Worktree.Mode = %q, want %q", got, want)
		}
		assertStringSlice(t, profile.Worker.AllowedAgentNames, []string{"coder", "reviewer"})
		assertStringSlice(t, profile.Worker.RequiredCapabilities, []string{"go", "test"})
		assertStringSlice(t, profile.Review.AllowedChannelIDs, []string{"chan-a", "chan-b"})
		assertStringSlice(t, profile.Participants.PreferredPeerIDs, []string{"peer-1", "peer-2"})
	})

	t.Run("Should trim and lowercase a referenced worktree policy", func(t *testing.T) {
		t.Parallel()

		profile, err := (&ExecutionProfile{
			TaskID: "task-1",
			Worktree: WorktreePolicy{
				Mode:        WorktreeMode("REF "),
				WorktreeRef: " feature-docs ",
			},
		}).Normalize(DefaultExecutionProfileValidationOptions())
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		if got, want := profile.Worktree, (WorktreePolicy{
			Mode: WorktreeModeRef, WorktreeRef: "feature-docs",
		}); got != want {
			t.Fatalf("Worktree = %#v, want %#v", got, want)
		}
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

	for _, test := range []struct {
		name   string
		policy WorktreePolicy
	}{
		{name: "Should require a worktree ref for ref mode", policy: WorktreePolicy{Mode: WorktreeModeRef}},
		{name: "Should reject a worktree ref for inherit mode", policy: WorktreePolicy{
			Mode: WorktreeModeInherit, WorktreeRef: "feature",
		}},
		{name: "Should reject a worktree ref for none mode", policy: WorktreePolicy{
			Mode: WorktreeModeNone, WorktreeRef: "feature",
		}},
		{name: "Should reject a worktree ref for per run mode", policy: WorktreePolicy{
			Mode: WorktreeModePerRun, WorktreeRef: "feature",
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := (&ExecutionProfile{
				TaskID:   "task-1",
				Worktree: test.policy,
			}).Normalize(DefaultExecutionProfileValidationOptions())
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("Normalize() error = %v, want %v", err, ErrValidation)
			}
		})
	}

	t.Run("Should resolve inherit to the configured per run snapshot", func(t *testing.T) {
		t.Parallel()

		resolved, err := resolveWorktreePolicySnapshot(
			WorktreePolicy{Mode: WorktreeModeInherit},
			WorktreeModePerRun,
		)
		if err != nil {
			t.Fatalf("resolveWorktreePolicySnapshot() error = %v", err)
		}
		if got, want := resolved.Mode, WorktreeModePerRun; got != want {
			t.Fatalf("resolved.Mode = %q, want %q", got, want)
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
