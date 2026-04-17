package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const authoredTaskListHeader = "| # | Title | Status | Complexity | Dependencies |"

func syncTaskMetadata(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	target, singleWorkflow, err := resolveSyncTarget(cfg)
	result := &SyncResult{Target: target}
	if err != nil {
		return result, err
	}

	db, workspace, err := openSyncGlobalDB(ctx, target)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = db.Close()
	}()

	if singleWorkflow {
		if err := syncWorkflow(ctx, db, workspace.ID, target, result); err != nil {
			return result, err
		}
		sortSyncResult(result)
		return result, nil
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return result, fmt.Errorf("read sync target: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !entry.IsDir() || !model.IsActiveWorkflowDirName(entry.Name()) {
			continue
		}
		if err := syncWorkflow(ctx, db, workspace.ID, filepath.Join(target, entry.Name()), result); err != nil {
			return result, err
		}
	}

	sortSyncResult(result)
	return result, nil
}

func openSyncGlobalDB(
	ctx context.Context,
	targetPath string,
) (*globaldb.GlobalDB, globaldb.Workspace, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return nil, globaldb.Workspace{}, fmt.Errorf("resolve compozy home paths: %w", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		return nil, globaldb.Workspace{}, fmt.Errorf("ensure compozy home layout: %w", err)
	}

	db, err := globaldb.Open(ctx, homePaths.GlobalDBPath)
	if err != nil {
		return nil, globaldb.Workspace{}, fmt.Errorf("open global sync database: %w", err)
	}

	workspace, err := db.ResolveOrRegister(ctx, targetPath)
	if err != nil {
		_ = db.Close()
		return nil, globaldb.Workspace{}, fmt.Errorf("resolve workspace for sync target: %w", err)
	}
	return db, workspace, nil
}

func resolveSyncTarget(cfg SyncConfig) (string, bool, error) {
	resolved, err := resolveWorkflowTarget(workflowTargetOptions{
		command:       "sync",
		workspaceRoot: cfg.WorkspaceRoot,
		rootDir:       cfg.RootDir,
		name:          cfg.Name,
		tasksDir:      cfg.TasksDir,
		selectorFlags: "--name or --tasks-dir",
	})
	if err != nil {
		return "", false, err
	}
	return resolved.target, resolved.specificTarget, nil
}

func syncWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	tasksDir string,
	result *SyncResult,
) error {
	if db == nil {
		return errors.New("sync database is required")
	}
	if result == nil {
		return errors.New("sync result is required")
	}

	removedLegacyArtifacts, err := cleanupLegacyWorkflowMetadata(tasksDir)
	if err != nil {
		return err
	}

	artifactSnapshots, checkpointChecksum, err := collectArtifactSnapshots(tasksDir)
	if err != nil {
		return err
	}
	taskItems, err := collectTaskItems(tasksDir)
	if err != nil {
		return err
	}
	reviewRounds, err := collectReviewRounds(tasksDir)
	if err != nil {
		return err
	}

	syncedAt := time.Now().UTC()
	syncState, err := db.ReconcileWorkflowSync(ctx, globaldb.WorkflowSyncInput{
		WorkspaceID:        workspaceID,
		WorkflowSlug:       filepath.Base(tasksDir),
		SyncedAt:           syncedAt,
		CheckpointScope:    "workflow",
		CheckpointChecksum: checkpointChecksum,
		ArtifactSnapshots:  artifactSnapshots,
		TaskItems:          taskItems,
		ReviewRounds:       reviewRounds,
	})
	if err != nil {
		return fmt.Errorf("sync workflow %s: %w", tasksDir, err)
	}

	result.WorkflowsScanned++
	result.SnapshotsUpserted += syncState.SnapshotsUpserted
	result.TaskItemsUpserted += syncState.TaskItemsUpserted
	result.ReviewRoundsUpserted += syncState.ReviewRoundsUpserted
	result.ReviewIssuesUpserted += syncState.ReviewIssuesUpserted
	result.CheckpointsUpdated += syncState.CheckpointsUpdated
	result.LegacyArtifactsRemoved += len(removedLegacyArtifacts)
	result.SyncedPaths = append(result.SyncedPaths, tasksDir)
	if len(removedLegacyArtifacts) > 0 {
		sort.Strings(removedLegacyArtifacts)
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf(
				"%s: removed legacy generated artifacts %s",
				filepath.Base(tasksDir),
				strings.Join(removedLegacyArtifacts, ", "),
			),
		)
	}
	return nil
}

func collectArtifactSnapshots(tasksDir string) ([]globaldb.ArtifactSnapshotInput, string, error) {
	snapshots := make([]globaldb.ArtifactSnapshotInput, 0)
	checksumParts := make([]string, 0)
	root, err := os.OpenRoot(strings.TrimSpace(tasksDir))
	if err != nil {
		return nil, "", fmt.Errorf("open workflow root for artifact scan: %w", err)
	}
	defer root.Close()

	err = filepath.WalkDir(tasksDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if path != tasksDir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return nil
		}

		relativePath, err := filepath.Rel(tasksDir, path)
		if err != nil {
			return fmt.Errorf("resolve relative artifact path for %s: %w", path, err)
		}
		relativePath = filepath.ToSlash(relativePath)
		if relativePath == "_meta.md" {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat artifact %s: %w", path, err)
		}
		content, err := root.ReadFile(filepath.FromSlash(relativePath))
		if err != nil {
			return fmt.Errorf("read artifact %s: %w", path, err)
		}

		frontmatterJSON, bodyText, err := snapshotArtifactContent(string(content))
		if err != nil {
			return fmt.Errorf("parse artifact %s: %w", path, err)
		}

		checksum := checksumHex(content)
		artifactKind := classifyArtifactKind(relativePath)
		snapshots = append(snapshots, globaldb.ArtifactSnapshotInput{
			ArtifactKind:    artifactKind,
			RelativePath:    relativePath,
			Checksum:        checksum,
			FrontmatterJSON: frontmatterJSON,
			BodyText:        bodyText,
			SourceMTime:     info.ModTime().UTC(),
		})
		checksumParts = append(checksumParts, artifactKind+"\x00"+relativePath+"\x00"+checksum)
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("walk workflow artifacts: %w", err)
	}

	sort.SliceStable(snapshots, func(i, j int) bool {
		left := snapshots[i].ArtifactKind + "\x00" + snapshots[i].RelativePath
		right := snapshots[j].ArtifactKind + "\x00" + snapshots[j].RelativePath
		return left < right
	})
	sort.Strings(checksumParts)
	return snapshots, checksumHex([]byte(strings.Join(checksumParts, "\n"))), nil
}

func snapshotArtifactContent(content string) (string, string, error) {
	metadata := make(map[string]any)
	body, err := frontmatter.Parse(content, &metadata)
	if err == nil {
		encoded, marshalErr := json.Marshal(metadata)
		if marshalErr != nil {
			return "", "", fmt.Errorf("marshal artifact front matter: %w", marshalErr)
		}
		return string(encoded), body, nil
	}
	if errors.Is(err, frontmatter.ErrHeaderNotFound) {
		return "{}", content, nil
	}
	return "", "", err
}

func classifyArtifactKind(relativePath string) string {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	base := filepath.Base(clean)

	switch {
	case clean == "_prd.md":
		return "prd"
	case clean == "_techspec.md":
		return "techspec"
	case clean == "_tasks.md":
		return "tasks_index"
	case tasks.ExtractTaskNumber(base) > 0 && !strings.Contains(clean, "/"):
		return "task"
	case strings.HasPrefix(clean, "adrs/"):
		return "adr"
	case strings.HasPrefix(clean, "memory/"):
		return "memory"
	case isReviewRoundMetaPath(clean):
		return "review_round_meta"
	case isReviewIssuePath(clean):
		return "review_issue"
	case strings.HasPrefix(clean, "qa/"):
		return "qa"
	case strings.HasPrefix(clean, "prompt/"), strings.HasPrefix(clean, "prompts/"):
		return "prompt"
	case strings.HasPrefix(clean, "protocol/"), strings.HasPrefix(clean, "protocols/"):
		return "protocol"
	default:
		return "artifact"
	}
}

func isReviewRoundMetaPath(relativePath string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	if filepath.Base(clean) != "_meta.md" {
		return false
	}
	dir := filepath.Dir(clean)
	return strings.HasPrefix(dir, "reviews-")
}

func isReviewIssuePath(relativePath string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	dir := filepath.Dir(clean)
	if !strings.HasPrefix(dir, "reviews-") {
		return false
	}
	return reviews.ExtractIssueNumber(filepath.Base(clean)) > 0
}

func collectTaskItems(tasksDir string) ([]globaldb.TaskItemInput, error) {
	entries, err := tasks.ReadTaskEntries(tasksDir, true)
	if err != nil {
		return nil, fmt.Errorf("read task entries: %w", err)
	}

	taskItems := make([]globaldb.TaskItemInput, 0, len(entries))
	for _, entry := range entries {
		task, err := tasks.ParseTaskFile(entry.Content)
		if err != nil {
			return nil, tasks.WrapParseError(entry.AbsPath, err)
		}

		taskNumber := tasks.ExtractTaskNumber(entry.Name)
		if taskNumber == 0 {
			return nil, fmt.Errorf("invalid task file name %q", entry.Name)
		}
		taskItems = append(taskItems, globaldb.TaskItemInput{
			TaskNumber: taskNumber,
			TaskID:     fmt.Sprintf("task_%d", taskNumber),
			Title:      task.Title,
			Status:     strings.ToLower(strings.TrimSpace(task.Status)),
			Kind:       task.TaskType,
			DependsOn:  append([]string(nil), task.Dependencies...),
			SourcePath: filepath.ToSlash(entry.Name),
		})
	}
	return taskItems, nil
}

func collectReviewRounds(tasksDir string) ([]globaldb.ReviewRoundInput, error) {
	roundNumbers, err := reviews.DiscoverRounds(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("discover review rounds: %w", err)
	}

	rounds := make([]globaldb.ReviewRoundInput, 0, len(roundNumbers))
	for _, roundNumber := range roundNumbers {
		reviewDir := reviews.ReviewDirectory(tasksDir, roundNumber)
		roundMeta, err := reviews.ReadRoundMeta(reviewDir)
		if err != nil {
			return nil, fmt.Errorf("read review round metadata %s: %w", reviewDir, err)
		}
		if roundMeta.Round != 0 && roundMeta.Round != roundNumber {
			return nil, fmt.Errorf(
				"review round metadata mismatch in %s: front matter round=%d directory round=%d",
				reviewDir,
				roundMeta.Round,
				roundNumber,
			)
		}

		reviewEntries, err := reviews.ReadReviewEntries(reviewDir)
		if err != nil {
			return nil, fmt.Errorf("read review entries %s: %w", reviewDir, err)
		}

		resolvedCount := 0
		issues := make([]globaldb.ReviewIssueInput, 0, len(reviewEntries))
		for _, entry := range reviewEntries {
			reviewCtx, err := reviews.ParseReviewContext(entry.Content)
			if err != nil {
				return nil, reviews.WrapParseError(entry.AbsPath, err)
			}
			if strings.EqualFold(strings.TrimSpace(reviewCtx.Status), "resolved") {
				resolvedCount++
			}

			relativePath, err := filepath.Rel(tasksDir, entry.AbsPath)
			if err != nil {
				return nil, fmt.Errorf("resolve review issue path %s: %w", entry.AbsPath, err)
			}
			issues = append(issues, globaldb.ReviewIssueInput{
				IssueNumber: reviews.ExtractIssueNumber(entry.Name),
				Severity:    strings.TrimSpace(reviewCtx.Severity),
				Status:      strings.ToLower(strings.TrimSpace(reviewCtx.Status)),
				SourcePath:  filepath.ToSlash(relativePath),
			})
		}

		rounds = append(rounds, globaldb.ReviewRoundInput{
			RoundNumber:     roundNumber,
			Provider:        strings.TrimSpace(roundMeta.Provider),
			PRRef:           strings.TrimSpace(roundMeta.PR),
			ResolvedCount:   resolvedCount,
			UnresolvedCount: len(issues) - resolvedCount,
			Issues:          issues,
		})
	}

	return rounds, nil
}

func cleanupLegacyWorkflowMetadata(tasksDir string) ([]string, error) {
	removed := make([]string, 0, 2)

	if deleted, err := removeFileIfPresent(filepath.Join(tasksDir, "_meta.md")); err != nil {
		return nil, fmt.Errorf("remove legacy workflow metadata: %w", err)
	} else if deleted {
		removed = append(removed, "_meta.md")
	}

	taskListPath := filepath.Join(tasksDir, "_tasks.md")
	taskListBody, err := os.ReadFile(taskListPath)
	switch {
	case err == nil:
		if shouldRemoveLegacyTaskList(string(taskListBody)) {
			if err := os.Remove(taskListPath); err != nil {
				return nil, fmt.Errorf("remove legacy task list %s: %w", taskListPath, err)
			}
			removed = append(removed, "_tasks.md")
		}
	case errors.Is(err, os.ErrNotExist):
		// Nothing to clean.
	default:
		return nil, fmt.Errorf("read task list %s: %w", taskListPath, err)
	}

	return removed, nil
}

func shouldRemoveLegacyTaskList(content string) bool {
	return !strings.Contains(content, authoredTaskListHeader)
}

func removeFileIfPresent(path string) (bool, error) {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func checksumHex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func sortSyncResult(result *SyncResult) {
	if result == nil {
		return
	}
	sort.Strings(result.SyncedPaths)
	sort.Strings(result.Warnings)
}
