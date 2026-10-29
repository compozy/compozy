package daemon

import (
	"context"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/worktree"
)

type turnEndNotifierRegistrar interface {
	AddTurnEndNotifier(session.TurnEndNotifier)
}

type worktreeTurnRefresher interface {
	Status(context.Context, string, string, bool) (*worktree.Status, error)
}

func registerWorktreeTurnRefresh(
	sessions SessionManager,
	refresher worktreeTurnRefresher,
	logger *slog.Logger,
) {
	registrar, ok := sessions.(turnEndNotifierRegistrar)
	if !ok || refresher == nil {
		return
	}
	registrar.AddTurnEndNotifier(worktreeTurnRefreshNotifier(sessions, refresher, logger))
	if bindings, ok := sessions.(interface {
		AddWorktreeBindingNotifier(session.WorktreeBindingNotifier)
	}); ok {
		bindings.AddWorktreeBindingNotifier(func(ctx context.Context, info *session.Info, previous string) {
			for _, id := range []string{previous, info.WorktreeID} {
				if id == "" {
					continue
				}
				if _, err := refresher.Status(ctx, info.WorkspaceID, id, true); err != nil && logger != nil {
					logger.WarnContext(ctx, "refresh worktree after session binding", "worktree_id", id, "error", err)
				}
			}
		})
	}
}

func worktreeTurnRefreshNotifier(
	sessions interface {
		Status(context.Context, string) (*session.Info, error)
	},
	refresher worktreeTurnRefresher,
	logger *slog.Logger,
) session.TurnEndNotifier {
	return func(ctx context.Context, identity session.PromptRunIdentity) {
		info, err := sessions.Status(ctx, identity.SessionID)
		if err != nil || info == nil || strings.TrimSpace(info.WorktreeID) == "" {
			return
		}
		if _, err := refresher.Status(
			ctx,
			info.WorkspaceID,
			info.WorktreeID,
			true,
		); err != nil && logger != nil {
			logger.Warn(
				"daemon: refresh bound worktree after session turn",
				"session_id", identity.SessionID,
				"worktree_id", info.WorktreeID,
				"error", err,
			)
		}
	}
}
