package daemon

import (
	"log/slog"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspacefiles"
	"github.com/compozy/compozy/internal/worktree"
)

// The file-search handler reaches the working directory through this optional
// session-manager capability.
var _ core.SessionWorkingDirResolver = (*session.Manager)(nil)

// newSessionFileIndex builds the composer file-search index over the hardened
// git runner. Without git the index serves the bounded walk only.
func newSessionFileIndex(logger *slog.Logger) workspacefiles.Index {
	runner, err := worktree.NewRealGitRunner(workspacefiles.DefaultGitTimeout)
	if err != nil {
		logger.Warn("daemon: Git unavailable for session file search; using directory walk", "error", err)
		return workspacefiles.New(nil, workspacefiles.WithLogger(logger))
	}
	return workspacefiles.New(runner, workspacefiles.WithLogger(logger))
}
