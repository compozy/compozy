package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/worktree"

	sessionpkg "github.com/compozy/compozy/internal/session"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type terminalExecutionWorktrees interface {
	ResolveSessionWorktreeForProfile(context.Context, string, string, string) (string, string, error)
}

type terminalExecutionSessions interface {
	Status(context.Context, string) (*sessionpkg.Info, error)
	ActivePromptRun(context.Context, string) (sessionpkg.PromptRunIdentity, error)
}

func terminalExecutionRootResolver(state *bootState) terminalpkg.ExecutionRootResolver {
	return func(ctx context.Context, workspaceID string, actor terminalpkg.Actor) (string, error) {
		if state == nil || state.sessions == nil {
			return "", terminalpkg.ErrServiceUnavailable
		}
		return resolveTerminalExecutionRoot(
			ctx,
			state.sessions,
			daemonSessionWorktreeResolver{state: state},
			workspaceID,
			actor,
		)
	}
}

func resolveTerminalExecutionRoot(
	ctx context.Context,
	sessions terminalExecutionSessions,
	worktrees terminalExecutionWorktrees,
	workspaceID string,
	actor terminalpkg.Actor,
) (root string, err error) {
	defer func() { err = terminalExecutionError(err) }()
	if !completeTerminalActor(actor) {
		return "", terminalpkg.ErrRunIdentityIncomplete
	}
	info, err := sessions.Status(ctx, actor.SessionID)
	if err != nil {
		return "", err
	}
	if info == nil || info.ID != actor.SessionID || info.WorkspaceID != workspaceID ||
		info.ProfileID != actor.ProfileID {
		return "", terminalpkg.ErrNotFound
	}
	active, err := sessions.ActivePromptRun(ctx, actor.SessionID)
	if err != nil {
		return "", err
	}
	if active.WorkspaceID != workspaceID || active.ProfileID != actor.ProfileID ||
		active.SessionID != actor.SessionID ||
		active.RunID != actor.RunID {
		return "", terminalpkg.ErrNotFound
	}
	if active.Generation != actor.Generation || info.RuntimeGeneration != actor.Generation {
		return "", terminalpkg.ErrGenerationFenced
	}
	ref := strings.TrimSpace(info.WorktreeID)
	if ref == "" {
		return "", nil
	}
	id, root, err := worktrees.ResolveSessionWorktreeForProfile(ctx, workspaceID, actor.ProfileID, ref)
	if err != nil {
		return "", err
	}
	if id != ref || strings.TrimSpace(root) == "" {
		return "", terminalpkg.ErrNotFound
	}
	return root, nil
}

func completeTerminalActor(actor terminalpkg.Actor) bool {
	return actor.SessionID != "" && actor.RunID != "" && actor.ProfileID != "" && actor.Generation > 0
}

func terminalExecutionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, terminalpkg.ErrRunIdentityIncomplete):
		return err
	case errors.Is(err, sessionpkg.ErrSessionNotFound),
		errors.Is(err, worktree.ErrNotFound),
		errors.Is(err, terminalpkg.ErrNotFound):
		return &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeNotFound,
			Message: "terminal execution binding is unavailable",
			Err:     terminalpkg.ErrNotFound,
		}
	case errors.Is(err, sessionpkg.ErrPromptNotActive), errors.Is(err, terminalpkg.ErrGenerationFenced):
		return &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeGenerationFenced,
			Message: "terminal agent run is no longer active",
			Err:     terminalpkg.ErrGenerationFenced,
		}
	case errors.Is(err, worktree.ErrMissing), errors.Is(err, worktree.ErrNotReady):
		return &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeInvalidCwd,
			Message: "bound worktree directory is unavailable",
			Err:     errors.Join(terminalpkg.ErrInvalidCwd, err),
		}
	default:
		return terminalpkg.ErrServiceUnavailable
	}
}
