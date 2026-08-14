package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// WorktreeResolver resolves one ready worktree within its parent workspace.
type WorktreeResolver interface {
	ResolveSessionWorktree(ctx context.Context, workspaceRegistryID, ref string) (id, root string, err error)
}

func (m *Manager) resolveSessionWorktree(
	ctx context.Context,
	workspaceID string,
	ref string,
) (id string, root string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", nil
	}
	if m == nil || m.worktreeResolver == nil {
		return "", "", errors.New("session: worktree resolver is required")
	}
	id, root, err = m.worktreeResolver.ResolveSessionWorktree(ctx, strings.TrimSpace(workspaceID), ref)
	if err != nil {
		return "", "", err
	}
	id = strings.TrimSpace(id)
	root = strings.TrimSpace(root)
	if id == "" || root == "" {
		return "", "", errors.New("session: worktree resolver returned an incomplete binding")
	}
	return id, root, nil
}

func (m *Manager) revalidateSessionWorktree(ctx context.Context, spec *sessionStartSpec) error {
	if spec == nil || strings.TrimSpace(spec.worktreeID) == "" {
		return nil
	}
	id, root, err := m.resolveSessionWorktree(ctx, spec.workspace.ID, spec.worktreeID)
	if err != nil {
		return err
	}
	originalRoot, err := canonicalDirectory(spec.worktreeRoot)
	if err != nil {
		return fmt.Errorf("session: canonicalize selected worktree root: %w", err)
	}
	resolvedRoot, err := canonicalDirectory(root)
	if err != nil {
		return fmt.Errorf("session: canonicalize current worktree root: %w", err)
	}
	if id != spec.worktreeID || resolvedRoot != originalRoot {
		return fmt.Errorf("%w: worktree binding changed before runtime start", ErrValidation)
	}
	return nil
}

func (s *sessionStartSpec) executionRoot() string {
	if s == nil {
		return ""
	}
	if root := strings.TrimSpace(s.worktreeRoot); root != "" {
		return root
	}
	return strings.TrimSpace(s.workspace.RootDir)
}

func (s *sessionStartSpec) executionAdditionalDirs() []string {
	if s == nil || strings.TrimSpace(s.worktreeID) != "" {
		return nil
	}
	return append([]string(nil), s.workspace.AdditionalDirs...)
}
