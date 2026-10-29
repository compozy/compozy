package session

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) prepareWorktreeBindingDestination(
	ctx context.Context,
	meta *store.SessionMeta,
	ref string,
) (cwdOut, idOut string, releaseOut func(), errOut error) {
	var releases []func()
	releaseAll := func() {
		for _, release := range slices.Backward(releases) {
			release()
		}
	}
	defer func() {
		if errOut != nil {
			releaseAll()
		}
	}()
	workspace, err := m.resolveResumeWorkspace(ctx, meta)
	if err != nil {
		return "", "", nil, err
	}
	root, worktreeID := workspace.RootDir, ""
	if strings.TrimSpace(ref) != "" {
		resolver, ok := m.worktreeResolver.(WorktreeBindingResolver)
		if !ok {
			return "", "", nil, errors.New("session: worktree binding resolver is unavailable")
		}
		var release func()
		worktreeID, root, release, err = resolver.AcquireSessionWorktreeBinding(
			ctx,
			meta.WorkspaceID,
			meta.ProfileID,
			ref,
		)
		if err != nil {
			return "", "", nil, err
		}
		releases = append(releases, release)
	}
	cwd, err := ResolveSessionCWD(root, root)
	if err != nil {
		return "", "", nil, err
	}
	releaseFence, err := m.reserveWorktreeBindingCheckouts(meta.WorkspaceID, meta.WorktreeIDValue(), worktreeID)
	if err != nil {
		return "", "", nil, err
	}
	releases = append(releases, releaseFence)
	if meta.WorktreeIDValue() != "" {
		resolver, ok := m.worktreeResolver.(WorktreeBindingResolver)
		if !ok {
			return "", "", nil, errors.New("session: worktree binding resolver is unavailable")
		}
		release, err := resolver.AcquireSessionWorktreeUnbinding(
			ctx,
			meta.WorkspaceID,
			meta.ProfileID,
			meta.WorktreeIDValue(),
		)
		if err != nil {
			return "", "", nil, err
		}
		releases = append(releases, release)
	}

	return cwd, worktreeID, releaseAll, nil
}
