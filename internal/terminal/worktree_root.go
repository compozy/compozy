package terminal

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// WorktreeRootResolver returns the checkout root of a ready managed worktree owned by
// the workspace and profile, plus a release for the usage lease that keeps the
// checkout from being removed while the terminal runs. Failures are terminal
// errors with a worktree error code.
type WorktreeRootResolver func(ctx context.Context, workspaceID, profileID, ref string) (string, func(), error)

// WithWorktreeRootResolver lets terminal opens start in a managed worktree of their workspace.
func WithWorktreeRootResolver(resolver WorktreeRootResolver) Option {
	return func(service *Service) error {
		if resolver == nil {
			return errors.New("terminal: worktree root resolver is required")
		}
		service.worktreeRoot = resolver
		return nil
	}
}

func (m *Service) applyWorktreeRoot(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	workspaceID string,
	profileID string,
	ref string,
) (func(), error) {
	if resolved.Scope == workspacepkg.ScopeUser {
		return nil, worktreeNotFoundError(ref)
	}
	if m.worktreeRoot == nil {
		return nil, errors.Join(ErrUnsupported, errors.New("terminal: worktree terminals are unavailable"))
	}
	root, release, err := m.worktreeRoot(ctx, workspaceID, profileID, ref)
	if err != nil {
		return nil, err
	}
	if release == nil {
		release = func() {}
	}
	release = sync.OnceFunc(release)
	if strings.TrimSpace(root) == "" {
		release()
		return nil, worktreeNotFoundError(ref)
	}
	resolved.RootDir = root
	resolved.AdditionalDirs = nil
	return release, nil
}

func (s *session) releaseWorktree() {
	releaseWorktreeNow(s.worktreeRelease)
}

func worktreeNotFoundError(ref string) *Error {
	return &Error{
		Code:    ErrorCodeWorktreeNotFound,
		Message: "worktree " + ref + " was not found in this project for the acting profile",
		Err:     ErrWorktreeNotFound,
	}
}

func boundRootConflict(boundRoot, worktreeRoot string) bool {
	return canonicalDir(boundRoot) != canonicalDir(worktreeRoot)
}

func canonicalDir(path string) string {
	clean := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		return resolved
	}
	return clean
}

func releaseWorktreeNow(release func()) {
	if release != nil {
		release()
	}
}

// releaseWorktreeAfterExit keeps a rolled-back launch's worktree lease until its
// process has exited, so a shell that outlived cleanup still fences removal.
func (m *Service) releaseWorktreeAfterExit(proc Proc, release func()) {
	if release == nil {
		return
	}
	if proc == nil {
		release()
		return
	}
	go func() {
		defer release()
		if _, err := proc.Wait(context.Background()); err != nil {
			m.logger.Warn("terminal: wait for rolled-back process before releasing worktree", "error", err)
		}
	}()
}
