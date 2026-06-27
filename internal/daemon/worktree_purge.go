package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	taskMultiEventPrefix    = "task.multi."
	taskParallelEventPrefix = "task.parallel."
)

type runWorktreePurgePlan struct {
	taskWorktrees []taskWorktreePurgeTarget
	integration   *integrationWorktreePurgeTarget
}

type taskWorktreePurgeTarget struct {
	Path       string
	BaseCommit string
}

type integrationWorktreePurgeTarget struct {
	Path   string
	Branch string
}

type preparedRunWorktreePurge struct {
	allocator     *taskMultiWorktreeAllocator
	workspaceRoot string
	worktreesRoot string
	plan          runWorktreePurgePlan
}

func (m *RunManager) purgeRunWorktrees(
	ctx context.Context,
	run *globaldb.Run,
	settings RunLifecycleSettings,
) ([]string, error) {
	prepared, ok, err := m.prepareRunWorktreePurge(ctx, run, settings)
	if err != nil || !ok {
		return nil, err
	}
	return prepared.execute(ctx, run.RunID)
}

func (m *RunManager) prepareRunWorktreePurge(
	ctx context.Context,
	run *globaldb.Run,
	settings RunLifecycleSettings,
) (*preparedRunWorktreePurge, bool, error) {
	if run == nil {
		return nil, false, nil
	}
	allocator, worktreesRoot := m.purgeWorktreeAllocator(settings)
	if allocator == nil || strings.TrimSpace(worktreesRoot) == "" {
		return nil, false, nil
	}
	workspace, err := m.globalDB.Get(ctx, run.WorkspaceID)
	if err != nil {
		return nil, false, fmt.Errorf("load workspace for purge run %s: %w", run.RunID, err)
	}
	events, ok, err := m.listRunEventsForPurge(ctx, run.RunID)
	if err != nil || !ok {
		return nil, false, err
	}
	plan, err := buildRunWorktreePurgePlan(run.RunID, workspace.RootDir, worktreesRoot, events)
	if err != nil {
		return nil, false, err
	}
	if len(plan.taskWorktrees) == 0 && plan.integration == nil {
		return nil, false, nil
	}
	return &preparedRunWorktreePurge{
		allocator:     allocator,
		workspaceRoot: workspace.RootDir,
		worktreesRoot: worktreesRoot,
		plan:          plan,
	}, true, nil
}

func (p preparedRunWorktreePurge) execute(ctx context.Context, runID string) ([]string, error) {
	purged, err := p.removeTaskWorktrees(ctx, runID)
	if err != nil {
		return purged, err
	}
	integrationPath, err := p.removeIntegrationWorktree(ctx, runID)
	if err != nil {
		return purged, err
	}
	if integrationPath != "" {
		purged = append(purged, integrationPath)
	}
	return purged, nil
}

func (p preparedRunWorktreePurge) removeTaskWorktrees(ctx context.Context, runID string) ([]string, error) {
	removableTaskWorktrees, err := p.inspectTaskWorktrees(ctx, runID)
	if err != nil {
		return nil, err
	}
	purged := make([]string, 0, len(removableTaskWorktrees)+1)
	for _, target := range removableTaskWorktrees {
		if err := p.allocator.Remove(ctx, p.workspaceRoot, target.Path); err != nil {
			return purged, fmt.Errorf("purge worktree for run %s at %s: %w", runID, target.Path, err)
		}
		purged = append(purged, target.Path)
		removeEmptyWorktreeParents(p.worktreesRoot, target.Path)
	}
	return purged, nil
}

func (p preparedRunWorktreePurge) inspectTaskWorktrees(
	ctx context.Context,
	runID string,
) ([]taskWorktreePurgeTarget, error) {
	removable := make([]taskWorktreePurgeTarget, 0, len(p.plan.taskWorktrees))
	for _, target := range p.plan.taskWorktrees {
		exists, err := inspectTaskWorktreeForPurge(ctx, p.allocator, p.workspaceRoot, target)
		if err != nil {
			return nil, fmt.Errorf("inspect worktree for run %s at %s: %w", runID, target.Path, err)
		}
		if exists {
			removable = append(removable, target)
		}
	}
	return removable, nil
}

func (p preparedRunWorktreePurge) removeIntegrationWorktree(ctx context.Context, runID string) (string, error) {
	if p.plan.integration == nil {
		return "", nil
	}
	removedPath, removed, err := p.allocator.DiscardIntegrationBranchIfExists(
		ctx,
		p.workspaceRoot,
		p.worktreesRoot,
		p.plan.integration.Path,
		p.plan.integration.Branch,
	)
	if err != nil {
		return "", fmt.Errorf("purge integration worktree for run %s: %w", runID, err)
	}
	if !removed {
		return "", nil
	}
	removeEmptyWorktreeParents(p.worktreesRoot, removedPath)
	return removedPath, nil
}

func (m *RunManager) purgeWorktreeAllocator(
	settings RunLifecycleSettings,
) (*taskMultiWorktreeAllocator, string) {
	worktreesRoot := strings.TrimSpace(settings.WorktreesRoot)
	if worktreesRoot == "" && m.worktreeAllocator != nil {
		worktreesRoot = strings.TrimSpace(m.worktreeAllocator.worktreesRoot)
	}
	if worktreesRoot == "" {
		return nil, ""
	}
	if m.worktreeAllocator != nil && strings.TrimSpace(m.worktreeAllocator.worktreesRoot) == worktreesRoot {
		return m.worktreeAllocator, worktreesRoot
	}
	return newTaskMultiWorktreeAllocator(worktreesRoot), worktreesRoot
}

func (m *RunManager) listRunEventsForPurge(
	ctx context.Context,
	runID string,
) ([]eventspkg.Event, bool, error) {
	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		return nil, false, err
	}
	if _, err := os.Stat(runArtifacts.RunDBPath); errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("stat run db for purge %s: %w", runID, err)
	}
	openRunDB := m.openRunDB
	if openRunDB == nil {
		openRunDB = openRunDBForRunID
	}
	runDB, err := openRunDB(ctx, runID)
	if err != nil {
		return nil, false, fmt.Errorf("open run db for purge %s: %w", runID, err)
	}
	defer func() {
		_ = runDB.Close()
	}()

	result, err := runDB.ListEvents(ctx, 0, 0)
	if err != nil {
		return nil, false, fmt.Errorf("list events for purge %s: %w", runID, err)
	}
	return result.Events, true, nil
}

func buildRunWorktreePurgePlan(
	runID string,
	workspaceRoot string,
	worktreesRoot string,
	events []eventspkg.Event,
) (runWorktreePurgePlan, error) {
	plan := runWorktreePurgePlan{taskWorktrees: make([]taskWorktreePurgeTarget, 0)}
	seenTaskPaths := make(map[string]int)
	sawParallelTaskRun := false
	for _, event := range events {
		eventKind := string(event.Kind)
		switch {
		case strings.HasPrefix(eventKind, taskMultiEventPrefix):
			target, err := taskMultiEventWorktreeTarget(event)
			if err != nil {
				return runWorktreePurgePlan{}, err
			}
			if err := addOwnedTaskWorktreePath(&plan, seenTaskPaths, worktreesRoot, target); err != nil {
				return runWorktreePurgePlan{}, err
			}
		case strings.HasPrefix(eventKind, taskParallelEventPrefix):
			sawParallelTaskRun = true
			target, err := taskParallelEventWorktreeTarget(event)
			if err != nil {
				return runWorktreePurgePlan{}, err
			}
			if err := addOwnedTaskWorktreePath(&plan, seenTaskPaths, worktreesRoot, target); err != nil {
				return runWorktreePurgePlan{}, err
			}
		}
	}
	if !sawParallelTaskRun {
		return plan, nil
	}
	integrationPath, err := planParallelIntegrationPath(worktreesRoot, workspaceRoot, runID)
	if err != nil {
		return runWorktreePurgePlan{}, err
	}
	ownedPath, ok, err := cleanOwnedWorktreePath(worktreesRoot, integrationPath)
	if err != nil {
		return runWorktreePurgePlan{}, err
	}
	if !ok {
		return runWorktreePurgePlan{}, fmt.Errorf(
			"derived integration worktree for run %s escapes worktree root",
			runID,
		)
	}
	plan.integration = &integrationWorktreePurgeTarget{
		Path:   ownedPath,
		Branch: parallelIntegrationBranch(runID),
	}
	return plan, nil
}

func taskMultiEventWorktreeTarget(event eventspkg.Event) (taskWorktreePurgeTarget, error) {
	var payload kinds.TaskRunMultiplePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return taskWorktreePurgeTarget{}, fmt.Errorf("decode %s purge payload: %w", event.Kind, err)
	}
	return taskWorktreePurgeTarget{
		Path:       payload.WorktreePath,
		BaseCommit: payload.BaseCommit,
	}, nil
}

func taskParallelEventWorktreeTarget(event eventspkg.Event) (taskWorktreePurgeTarget, error) {
	var payload kinds.TaskParallelPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return taskWorktreePurgeTarget{}, fmt.Errorf("decode %s purge payload: %w", event.Kind, err)
	}
	return taskWorktreePurgeTarget{Path: payload.WorktreePath}, nil
}

func addOwnedTaskWorktreePath(
	plan *runWorktreePurgePlan,
	seen map[string]int,
	worktreesRoot string,
	target taskWorktreePurgeTarget,
) error {
	path, ok, err := cleanOwnedWorktreePath(worktreesRoot, target.Path)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if existingIndex, exists := seen[path]; exists {
		if strings.TrimSpace(plan.taskWorktrees[existingIndex].BaseCommit) == "" {
			plan.taskWorktrees[existingIndex].BaseCommit = strings.TrimSpace(target.BaseCommit)
		}
		return nil
	}
	seen[path] = len(plan.taskWorktrees)
	plan.taskWorktrees = append(plan.taskWorktrees, taskWorktreePurgeTarget{
		Path:       path,
		BaseCommit: strings.TrimSpace(target.BaseCommit),
	})
	return nil
}

func inspectTaskWorktreeForPurge(
	ctx context.Context,
	allocator *taskMultiWorktreeAllocator,
	workspaceRoot string,
	target taskWorktreePurgeTarget,
) (bool, error) {
	if allocator == nil {
		return false, errors.New("daemon: worktree allocator is required")
	}
	run, err := allocator.requireGitRunner()
	if err != nil {
		return false, err
	}
	if _, statErr := os.Stat(target.Path); errors.Is(statErr, os.ErrNotExist) {
		return false, nil
	} else if statErr != nil {
		return false, fmt.Errorf("stat worktree %s: %w", target.Path, statErr)
	}
	status, err := run(ctx, target.Path, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("inspect worktree status in %s: %w", target.Path, err)
	}
	if strings.TrimSpace(status) != "" {
		return false, fmt.Errorf("worktree %s has uncommitted changes", target.Path)
	}
	preserved, err := taskWorktreeHasPreservedCommits(ctx, run, workspaceRoot, target)
	if err != nil {
		return false, err
	}
	if preserved {
		return false, fmt.Errorf("worktree %s has committed changes not retained by a branch", target.Path)
	}
	return true, nil
}

func taskWorktreeHasPreservedCommits(
	ctx context.Context,
	run taskMultiWorktreeGitRunner,
	workspaceRoot string,
	target taskWorktreePurgeTarget,
) (bool, error) {
	head, err := run(ctx, target.Path, "rev-parse", taskMultiWorktreeHeadRef)
	if err != nil {
		return false, fmt.Errorf("resolve worktree head in %s: %w", target.Path, err)
	}
	head = strings.TrimSpace(head)
	base := strings.TrimSpace(target.BaseCommit)
	if base != "" {
		return taskWorktreeHasCommitsAfterBase(ctx, run, target.Path, base, head)
	}
	branches, err := run(ctx, workspaceRoot, "branch", "--contains", head, "--format=%(refname:short)")
	if err != nil {
		return false, fmt.Errorf("inspect branches containing worktree head %s: %w", head, err)
	}
	return strings.TrimSpace(branches) == "", nil
}

func taskWorktreeHasCommitsAfterBase(
	ctx context.Context,
	run taskMultiWorktreeGitRunner,
	worktreePath string,
	base string,
	head string,
) (bool, error) {
	if head == base {
		return false, nil
	}
	countText, err := run(ctx, worktreePath, "rev-list", "--count", base+".."+head)
	if err != nil {
		return false, fmt.Errorf("inspect commits after base %s in %s: %w", base, worktreePath, err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(countText))
	if err != nil {
		return false, fmt.Errorf("parse commits after base %s in %s: %w", base, worktreePath, err)
	}
	return count > 0, nil
}

func cleanOwnedWorktreePath(worktreesRoot string, rawPath string) (string, bool, error) {
	root := strings.TrimSpace(worktreesRoot)
	path := strings.TrimSpace(rawPath)
	if root == "" || path == "" || !filepath.IsAbs(path) {
		return "", false, nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false, fmt.Errorf("resolve worktree root %s: %w", root, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("resolve worktree path %s: %w", path, err)
	}
	cleanRoot := filepath.Clean(absRoot)
	cleanPath := filepath.Clean(absPath)
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return "", false, fmt.Errorf("relativize worktree path %s to %s: %w", cleanPath, cleanRoot, err)
	}
	if isRelativeChildPath(rel) {
		insideResolvedRoot, err := isLexicallyOwnedWorktreeChild(cleanRoot, cleanPath)
		if err != nil {
			return "", false, err
		}
		if !insideResolvedRoot {
			return "", false, nil
		}
		return cleanPath, true, nil
	}
	insideSymlinkRoot, err := isSymlinkEquivalentWorktreeChild(cleanRoot, cleanPath)
	if err != nil {
		return "", false, err
	}
	if !insideSymlinkRoot {
		return "", false, nil
	}
	return cleanPath, true, nil
}

func isLexicallyOwnedWorktreeChild(cleanRoot string, cleanPath string) (bool, error) {
	evalRoot, rootErr := filepath.EvalSymlinks(cleanRoot)
	if rootErr != nil {
		return false, nil
	}
	cleanEvalRoot := filepath.Clean(evalRoot)
	pathToEvaluate := cleanPath
	for {
		evalPath, pathErr := filepath.EvalSymlinks(pathToEvaluate)
		if pathErr == nil {
			evalRel, err := filepath.Rel(cleanEvalRoot, filepath.Clean(evalPath))
			if err != nil {
				return false, fmt.Errorf("relativize worktree path %s to %s: %w", evalPath, evalRoot, err)
			}
			if pathToEvaluate == cleanPath {
				return isRelativeChildPath(evalRel), nil
			}
			return evalRel == "." || isRelativeChildPath(evalRel), nil
		}
		if !errors.Is(pathErr, os.ErrNotExist) {
			return false, fmt.Errorf("resolve worktree path %s: %w", pathToEvaluate, pathErr)
		}
		parent := filepath.Dir(pathToEvaluate)
		if parent == pathToEvaluate {
			return false, nil
		}
		relToRoot, err := filepath.Rel(cleanRoot, parent)
		if err != nil {
			return false, fmt.Errorf("relativize worktree path %s to %s: %w", parent, cleanRoot, err)
		}
		if relToRoot != "." && !isRelativeChildPath(relToRoot) {
			return false, nil
		}
		pathToEvaluate = parent
	}
}

func isSymlinkEquivalentWorktreeChild(cleanRoot string, cleanPath string) (bool, error) {
	evalRoot, rootErr := filepath.EvalSymlinks(cleanRoot)
	evalPath, pathErr := filepath.EvalSymlinks(cleanPath)
	if rootErr != nil || pathErr != nil {
		return false, nil
	}
	evalRel, err := filepath.Rel(filepath.Clean(evalRoot), filepath.Clean(evalPath))
	if err != nil {
		return false, fmt.Errorf("relativize worktree path %s to %s: %w", evalPath, evalRoot, err)
	}
	return isRelativeChildPath(evalRel), nil
}

func isRelativeChildPath(rel string) bool {
	return rel != "." && rel != "" && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func removeEmptyWorktreeParents(worktreesRoot string, worktreePath string) {
	root, ok, err := cleanOwnedWorktreePath(filepath.Dir(worktreesRoot), worktreesRoot)
	if err != nil || !ok {
		return
	}
	path, ok, err := cleanOwnedWorktreePath(root, worktreePath)
	if err != nil || !ok {
		return
	}
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == "." || rel == "" || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}
