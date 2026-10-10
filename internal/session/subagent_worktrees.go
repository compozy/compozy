package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

type SubagentWorktrees interface {
	Provision(context.Context, SubagentWorktreeRequest) (SubagentWorktree, error)
	Rollback(ctx context.Context, workspaceID, worktreeID, subagentID string) error
	FindByRun(ctx context.Context, workspaceID, subagentID string) (*SubagentWorktree, error)
	SafeRollback(ctx context.Context, workspaceID, worktreeID, subagentID, baseSHA string) (bool, error)
	Observe(ctx context.Context, workspaceID, worktreeID, baseSHA string) SubagentWorktreeFacts
}

type SubagentWorktreeRequest struct {
	WorkspaceID, ProfileID, SubagentID, Title string
	BaseRef                                   string
	// CallerPath is the already-authorized checkout, used only to resolve an omitted base.
	CallerPath       string
	CallerWorktreeID string
}
type SubagentWorktree struct{ ID, Name, Branch, Path, BaseRef, BaseSHA string }
type SubagentWorktreeFacts = store.SubagentWorktreeFacts

type ErrSubagentIsolationFailed struct {
	Cause  string
	Detail string
	Err    error
}

func (e *ErrSubagentIsolationFailed) Error() string {
	message := fmt.Sprintf("Could not create the subagent worktree: %s.", e.Cause)
	if e.Detail != "" {
		message += " " + e.Detail
	}
	return message
}
func (e *ErrSubagentIsolationFailed) Unwrap() error { return e.Err }
func WithSubagentWorktrees(w SubagentWorktrees) SubagentOption {
	return func(s *subagentService) { s.worktrees = w }
}

func associateSubagentWorktree(row *store.SessionSubagent, wt SubagentWorktree) {
	previous := row.WorktreeState()
	row.Worktree = &store.SubagentWorktreeState{ID: wt.ID, Name: wt.Name, Branch: wt.Branch,
		Path: wt.Path, BaseRef: wt.BaseRef, BaseSHA: wt.BaseSHA, Cleanup: previous.Cleanup, Facts: previous.Facts}
}

func subagentFirstPrompt(row store.SessionSubagent, task string) string {
	text := subagentPrompt(row.Role, task)
	if row.Isolation != SubagentIsolationWorktree {
		return text
	}
	return fmt.Sprintf(
		"[You are working in an isolated worktree on branch %s, based on %s. Commit your changes on this branch. To open a pull request, run `compozy worktree deliver`.]\n\n%s",
		row.WorktreeState().Branch,
		row.WorktreeState().BaseRef,
		text,
	)
}
