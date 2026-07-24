package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/gitenv"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// siblingSkipReason enumerates the diagnosable causes for dropping a sibling
// worktree from the completion union. A skipped sibling only ever under-reports,
// so these surface at slog.Debug (never warn/error).
const (
	siblingSkipNotRegistered     = "not_registered"
	siblingSkipListWorkflowError = "list_workflows_error"
	siblingSkipParseError        = "parse_error"
)

// siblingWorktreeRoots enumerates the sibling git worktrees of workspaceRoot for
// the completion read. It runs `git worktree list --porcelain`, parses it with
// the shared pure gitenv.ParseWorktreeList primitive, drops the primary root,
// drops any worktree whose directory no longer exists on disk, de-duplicates,
// and sorts. It applies NO ownership filter (ADR-002): the read must reach a
// user's arbitrary manual checkout, unlike the daemon write fan-out.
//
// Enumeration is best-effort at the caller: any git failure returns a non-nil
// error the union swallows, degrading to the single-workspace result.
func siblingWorktreeRoots(ctx context.Context, workspaceRoot string) ([]string, error) {
	output, err := gitenv.Run(ctx, workspaceRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list sibling worktrees: %w", err)
	}
	// The primary root is resolved for comparison. A removed worktree that git
	// still lists as prunable resolves to ok=false and is dropped here so the
	// stale row is never unioned (globaldb.Get resolves it by string match).
	primary, _ := canonicalExistingDir(workspaceRoot)
	seen := make(map[string]struct{})
	roots := make([]string, 0)
	for _, raw := range gitenv.ParseWorktreeList(output) {
		resolved, ok := canonicalExistingDir(raw)
		if !ok {
			continue
		}
		if resolved == primary {
			continue
		}
		if _, dup := seen[resolved]; dup {
			continue
		}
		seen[resolved] = struct{}{}
		roots = append(roots, resolved)
	}
	sort.Strings(roots)
	return roots, nil
}

// canonicalExistingDir resolves a worktree path to its real, cleaned form and
// reports whether it currently exists as a directory. A worktree git still lists
// but whose directory was removed resolves to ok=false so the read drops it.
func canonicalExistingDir(path string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(strings.TrimSpace(path))
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return filepath.Clean(resolved), true
}

// completedIDsForWorkspaceRoot performs the single-workspace completion read and
// additionally returns the resolved workspace ID for authoritative union dedup.
// It preserves the original CompletedTaskGroupIDsWithDB behavior exactly: read
// via db.Get and, only when the workspace is not yet registered, fall back to
// db.ResolveOrRegister for the querying workspace.
func completedIDsForWorkspaceRoot(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceRoot, initiative string,
) (workspaceID string, ids []string, err error) {
	workspace, err := db.Get(ctx, workspaceRoot)
	if errors.Is(err, globaldb.ErrWorkspaceNotFound) {
		workspace, err = db.ResolveOrRegister(ctx, workspaceRoot)
	}
	if err != nil {
		return "", nil, fmt.Errorf("resolve completion hydration workspace: %w", err)
	}
	workflows, err := db.ListWorkflows(ctx, globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID})
	if err != nil {
		return "", nil, fmt.Errorf("list completion hydration workflows: %w", err)
	}
	completed, err := completedTaskGroupIDsForInitiative(workflows, initiative)
	if err != nil {
		return "", nil, err
	}
	return workspace.ID, completed, nil
}

// unionSiblingCompletions folds each sibling worktree's completion into completed
// in a read-only, best-effort pass. seenWorkspace (resolved workspace IDs) and
// seenID (task-group IDs in first-seen order) drive dedup. Every sibling-side
// failure is swallowed and diagnosed at slog.Debug, so the result can only ever
// shrink toward — never grow past — the union of live siblings.
func unionSiblingCompletions(
	ctx context.Context,
	db *globaldb.GlobalDB,
	roots []string,
	initiative string,
	completed []string,
	seenWorkspace map[string]struct{},
	seenID map[string]struct{},
) []string {
	for _, root := range roots {
		// Read-only: db.Get never registers a sibling (AUTH-003, AUTH-004).
		workspace, err := db.Get(ctx, root)
		if err != nil {
			debugSkipSibling(root, siblingSkipNotRegistered, err)
			continue
		}
		if _, dup := seenWorkspace[workspace.ID]; dup {
			continue
		}
		seenWorkspace[workspace.ID] = struct{}{}
		workflows, err := db.ListWorkflows(ctx, globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID})
		if err != nil {
			debugSkipSibling(root, siblingSkipListWorkflowError, err)
			continue
		}
		ids, err := completedTaskGroupIDsForInitiative(workflows, initiative)
		if err != nil {
			debugSkipSibling(root, siblingSkipParseError, err)
			continue
		}
		completed = appendUnseenTaskGroupIDs(completed, seenID, ids)
	}
	return completed
}

// newTaskGroupIDSet indexes task-group IDs into a first-seen membership set.
func newTaskGroupIDSet(ids []string) map[string]struct{} {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return seen
}

// appendUnseenTaskGroupIDs appends only task-group IDs not already in seen,
// preserving first-seen order and keeping the result deduplicated.
func appendUnseenTaskGroupIDs(completed []string, seen map[string]struct{}, ids []string) []string {
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		completed = append(completed, id)
	}
	return completed
}

// debugSkipSibling records a skipped sibling at Debug level. A skipped sibling
// only under-reports completion, so no warning/error level is warranted.
func debugSkipSibling(siblingRoot, reason string, err error) {
	slog.Debug(
		"skip sibling worktree completion",
		"sibling_root", siblingRoot,
		"reason", reason,
		"error", err,
	)
}
