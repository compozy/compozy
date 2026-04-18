package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const workflowStateNotSyncedReason = "workflow state not synced"

func archiveTaskWorkflows(ctx context.Context, cfg ArchiveConfig) (*ArchiveResult, error) {
	target, rootDir, singleWorkflow, err := resolveArchiveTarget(ctx, cfg)
	result := &ArchiveResult{
		Target:         target,
		ArchiveRoot:    model.ArchivedTasksDir(rootDir),
		SkippedReasons: make(map[string]string),
	}
	if err != nil {
		return result, err
	}

	db, workspace, err := openWorkflowGlobalDB(ctx, target)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = db.Close()
	}()

	if singleWorkflow {
		if err := archiveWorkflow(ctx, db, workspace.ID, target, result, true); err != nil {
			return result, err
		}
		sortArchiveResult(result)
		return result, nil
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return result, fmt.Errorf("read archive target: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !entry.IsDir() || !model.IsActiveWorkflowDirName(entry.Name()) {
			continue
		}
		if err := archiveWorkflow(
			ctx,
			db,
			workspace.ID,
			filepath.Join(target, entry.Name()),
			result,
			false,
		); err != nil {
			return result, err
		}
	}

	sortArchiveResult(result)
	return result, nil
}

func resolveArchiveTarget(ctx context.Context, cfg ArchiveConfig) (string, string, bool, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == model.ArchivedWorkflowDirName {
		return "", "", false, fmt.Errorf("archive target cannot be %s", model.ArchivedWorkflowDirName)
	}

	resolvedTarget, rootDir, specificTarget, slug, err := resolveArchiveSelection(cfg, name)
	if err != nil {
		return "", "", false, err
	}
	if err := validateArchiveTarget(ctx, resolvedTarget, rootDir, slug, specificTarget); err != nil {
		return "", "", false, err
	}
	return resolvedTarget, rootDir, specificTarget, nil
}

func archiveSlugForTarget(name string, target string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return filepath.Base(strings.TrimSpace(target))
}

func resolveArchiveSelection(
	cfg ArchiveConfig,
	name string,
) (target string, rootDir string, specificTarget bool, slug string, err error) {
	if countArchiveSelectors(cfg) > 1 {
		return "", "", false, "", fmt.Errorf("archive accepts only one of --name or --tasks-dir")
	}

	rootDir = strings.TrimSpace(cfg.RootDir)
	if rootDir == "" {
		rootDir = model.TasksBaseDirForWorkspace(cfg.WorkspaceRoot)
	}
	rootDir, err = filepath.Abs(rootDir)
	if err != nil {
		return "", "", false, "", fmt.Errorf("resolve archive root: %w", err)
	}

	target = rootDir
	switch {
	case strings.TrimSpace(cfg.TasksDir) != "":
		target = strings.TrimSpace(cfg.TasksDir)
		specificTarget = true
	case name != "":
		target = filepath.Join(rootDir, name)
		specificTarget = true
	}

	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", false, "", fmt.Errorf("resolve archive target: %w", err)
	}
	if specificTarget {
		rootDir = filepath.Dir(target)
	}
	return target, rootDir, specificTarget, archiveSlugForTarget(name, target), nil
}

func countArchiveSelectors(cfg ArchiveConfig) int {
	selectors := 0
	if strings.TrimSpace(cfg.Name) != "" {
		selectors++
	}
	if strings.TrimSpace(cfg.TasksDir) != "" {
		selectors++
	}
	return selectors
}

func validateArchiveTarget(
	ctx context.Context,
	target string,
	rootDir string,
	slug string,
	specificTarget bool,
) error {
	if pathContainsArchivedComponent(target) {
		return fmt.Errorf("archive target cannot be inside %s", model.ArchivedWorkflowDirName)
	}

	info, err := os.Stat(target)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("archive target is not a directory: %s", target)
		}
		return nil
	}

	if specificTarget && errors.Is(err, os.ErrNotExist) {
		archiveRoot := model.ArchivedTasksDir(rootDir)
		if archivedWorkflowExists(archiveRoot, slug) || archivedWorkflowIdentityExists(ctx, rootDir, slug) {
			return globaldb.WorkflowArchivedError{Slug: slug}
		}
	}
	return fmt.Errorf("stat archive target: %w", err)
}

func archivedWorkflowIdentityExists(ctx context.Context, rootDir string, slug string) bool {
	if strings.TrimSpace(rootDir) == "" || strings.TrimSpace(slug) == "" {
		return false
	}

	db, workspace, err := openWorkflowGlobalDB(ctx, rootDir)
	if err != nil {
		return false
	}
	defer func() {
		_ = db.Close()
	}()

	_, err = db.GetLatestArchivedWorkflowBySlug(ctx, workspace.ID, slug)
	return err == nil
}

func archivedWorkflowExists(archiveRoot string, slug string) bool {
	entries, err := os.ReadDir(strings.TrimSpace(archiveRoot))
	if err != nil {
		return false
	}

	suffix := "-" + strings.TrimSpace(slug)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) {
			return true
		}
	}
	return false
}

func archiveWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	tasksDir string,
	result *ArchiveResult,
	conflictOnSkip bool,
) error {
	if result == nil {
		return errors.New("archive result is required")
	}

	result.WorkflowsScanned++

	slug := filepath.Base(tasksDir)
	eligibility, err := db.GetWorkflowArchiveEligibility(ctx, workspaceID, slug)
	if err != nil {
		if errors.Is(err, globaldb.ErrWorkflowNotFound) {
			reason := workflowStateNotSyncedReason
			if conflictOnSkip {
				return globaldb.WorkflowNotArchivableError{
					WorkspaceID: workspaceID,
					Slug:        slug,
					Reason:      reason,
				}
			}
			recordArchiveSkip(result, tasksDir, reason)
			return nil
		}
		return err
	}

	if reason := eligibility.SkipReason(); reason != "" {
		if conflictOnSkip {
			return eligibility.ConflictError()
		}
		recordArchiveSkip(result, tasksDir, reason)
		return nil
	}

	if err := os.MkdirAll(result.ArchiveRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir archive root: %w", err)
	}

	archivedAt := time.Now().UTC()
	archivedDir := filepath.Join(
		result.ArchiveRoot,
		model.ArchivedWorkflowName(slug, eligibility.Workflow.ID, archivedAt),
	)
	if err := os.Rename(tasksDir, archivedDir); err != nil {
		return fmt.Errorf("archive workflow %s: %w", tasksDir, err)
	}

	if _, err := db.MarkWorkflowArchived(ctx, eligibility.Workflow.ID, archivedAt); err != nil {
		if rollbackErr := os.Rename(archivedDir, tasksDir); rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("persist archived workflow state %s: %w", eligibility.Workflow.ID, err),
				fmt.Errorf("rollback archived workflow rename %s: %w", archivedDir, rollbackErr),
			)
		}
		return fmt.Errorf("persist archived workflow state %s: %w", eligibility.Workflow.ID, err)
	}

	result.Archived++
	result.ArchivedPaths = append(result.ArchivedPaths, archivedDir)
	return nil
}

func recordArchiveSkip(result *ArchiveResult, tasksDir string, reason string) {
	if result == nil {
		return
	}
	result.Skipped++
	result.SkippedPaths = append(result.SkippedPaths, tasksDir)
	result.SkippedReasons[tasksDir] = reason
}

func pathContainsArchivedComponent(path string) bool {
	cleaned := filepath.Clean(path)
	for {
		if filepath.Base(cleaned) == model.ArchivedWorkflowDirName {
			return true
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			return false
		}
		cleaned = parent
	}
}

func sortArchiveResult(result *ArchiveResult) {
	if result == nil {
		return
	}
	sort.Strings(result.ArchivedPaths)
	sort.Strings(result.SkippedPaths)
}
