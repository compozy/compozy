package daemon

import (
	"context"
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
	return workspacefiles.New(sessionFileGitRunner{runner: runner}, workspacefiles.WithLogger(logger))
}

// sessionFileGitRunner adapts the hardened worktree runner to the index's
// streaming capability so listings are not bounded by the buffered output cap.
type sessionFileGitRunner struct {
	runner *worktree.RealGitRunner
}

var _ workspacefiles.StreamingGitRunner = sessionFileGitRunner{}

func (r sessionFileGitRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	return r.runner.Run(ctx, dir, args...)
}

func (r sessionFileGitRunner) RunStreaming(
	ctx context.Context,
	dir string,
	onStdout func(chunk []byte),
	args ...string,
) ([]byte, error) {
	_, stderr, err := r.runner.RunStreaming(ctx, dir, func(output worktree.GitOutput) {
		if output.Stream == worktree.GitOutputStdout {
			onStdout(output.Chunk)
		}
	}, args...)
	return stderr, err
}
