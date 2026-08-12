package task

import (
	"context"
	"fmt"
	"strings"
)

const (
	// WorktreeModeInherit resolves through the configured task default at enqueue time.
	WorktreeModeInherit WorktreeMode = "inherit"
	// WorktreeModeNone runs from the parent workspace root.
	WorktreeModeNone WorktreeMode = "none"
	// WorktreeModeRef binds the run to one attached worktree.
	WorktreeModeRef WorktreeMode = "ref"
	// WorktreeModePerRun materializes one fresh worktree for each run.
	WorktreeModePerRun WorktreeMode = "per_run"
)

// WorktreeMode identifies task-level worktree selection behavior.
type WorktreeMode string

// WorktreePolicy selects task-level worktree behavior at enqueue time.
type WorktreePolicy struct {
	Mode        WorktreeMode `json:"mode"`
	WorktreeRef string       `json:"worktree_ref,omitempty"`
}

// WorktreeRefValidator checks that a profile reference resolves to an attached
// worktree owned by the task's workspace.
type WorktreeRefValidator interface {
	ValidateTaskWorktreeRef(ctx context.Context, workspaceID string, ref string) error
}

// Normalize returns the normalized worktree mode.
func (m WorktreeMode) Normalize() WorktreeMode {
	return WorktreeMode(strings.ToLower(strings.TrimSpace(string(m))))
}

func normalizeWorktreePolicy(policy WorktreePolicy) WorktreePolicy {
	policy.Mode = policy.Mode.Normalize()
	if policy.Mode == "" {
		policy.Mode = WorktreeModeInherit
	}
	policy.WorktreeRef = strings.TrimSpace(policy.WorktreeRef)
	return policy
}

func validateWorktreePolicy(policy WorktreePolicy) error {
	switch policy.Mode.Normalize() {
	case WorktreeModeInherit, WorktreeModeNone, WorktreeModePerRun:
		if policy.WorktreeRef != "" {
			return fmt.Errorf(
				"%w: task_execution_profile.worktree.worktree_ref must be empty for %s",
				ErrValidation,
				policy.Mode,
			)
		}
	case WorktreeModeRef:
		if policy.WorktreeRef == "" {
			return fmt.Errorf(
				"%w: task_execution_profile.worktree.worktree_ref is required for ref",
				ErrValidation,
			)
		}
	default:
		return fmt.Errorf(
			"%w: task_execution_profile.worktree.mode must be %q, %q, %q, or %q: %q",
			ErrValidation,
			WorktreeModeInherit,
			WorktreeModeNone,
			WorktreeModeRef,
			WorktreeModePerRun,
			policy.Mode,
		)
	}
	return validateSelectorAtom(
		policy.WorktreeRef,
		"task_execution_profile.worktree.worktree_ref",
		true,
	)
}

func resolveWorktreePolicySnapshot(
	policy WorktreePolicy,
	defaultMode WorktreeMode,
) (WorktreePolicy, error) {
	resolved := normalizeWorktreePolicy(policy)
	if resolved.Mode == WorktreeModeInherit {
		resolved.Mode = defaultMode.Normalize()
		if resolved.Mode == "" || resolved.Mode == WorktreeModeInherit {
			resolved.Mode = WorktreeModeNone
		}
	}
	if err := validateResolvedWorktreePolicy(resolved); err != nil {
		return WorktreePolicy{}, err
	}
	return resolved, nil
}

func validateResolvedWorktreePolicy(policy WorktreePolicy) error {
	policy.Mode = policy.Mode.Normalize()
	if policy.Mode == "" {
		policy.Mode = WorktreeModeNone
	}
	policy.WorktreeRef = strings.TrimSpace(policy.WorktreeRef)
	if policy.Mode == WorktreeModeInherit {
		return fmt.Errorf("%w: resolved worktree mode cannot be inherit", ErrValidation)
	}
	return validateWorktreePolicy(policy)
}
