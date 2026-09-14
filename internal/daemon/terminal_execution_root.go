package daemon

import (
	"context"
	"strings"

	sessionpkg "github.com/compozy/compozy/internal/session"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

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
	worktrees sessionpkg.WorktreeResolver,
	workspaceID string,
	actor terminalpkg.Actor,
) (string, error) {
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
	id, root, err := worktrees.ResolveSessionWorktree(ctx, workspaceID, ref)
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
