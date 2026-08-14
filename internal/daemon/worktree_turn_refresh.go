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
}

func worktreeTurnRefreshNotifier(
	sessions interface {
		Status(context.Context, string) (*session.Info, error)
	},
	refresher worktreeTurnRefresher,
	logger *slog.Logger,
) session.TurnEndNotifier {
	return func(sessionID string) {
		info, err := sessions.Status(context.Background(), sessionID)
		if err != nil || info == nil || strings.TrimSpace(info.WorktreeID) == "" {
			return
		}
		if _, err := refresher.Status(
			context.Background(),
			info.WorkspaceID,
			info.WorktreeID,
			true,
		); err != nil && logger != nil {
			logger.Warn(
				"daemon: refresh bound worktree after session turn",
				"session_id", sessionID,
				"worktree_id", info.WorktreeID,
				"error", err,
			)
		}
	}
}
