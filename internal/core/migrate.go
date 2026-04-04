package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/reviews"
)

type pendingFileMigration struct {
	path    string
	content string
}

type migrationScanState struct {
	result  *MigrationResult
	pending []pendingFileMigration
}

func migrateArtifacts(ctx context.Context, cfg MigrationConfig) (*MigrationResult, error) {
	target, err := resolveMigrationTarget(cfg)
	result := &MigrationResult{
		Target: target,
		DryRun: cfg.DryRun,
	}
	if err != nil {
		return result, err
	}

	pending, err := scanMigrationTarget(ctx, target, result)
	if err != nil {
		return result, err
	}

	sort.Strings(result.MigratedPaths)
	sort.Strings(result.InvalidPaths)
	result.FilesMigrated = len(pending)

	if len(result.InvalidPaths) > 0 {
		return result, fmt.Errorf("migration aborted: %d invalid artifact(s) found", len(result.InvalidPaths))
	}
	if cfg.DryRun {
		return result, nil
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].path < pending[j].path
	})
	for _, file := range pending {
		if err := os.WriteFile(file.path, []byte(file.content), 0o600); err != nil {
			return result, fmt.Errorf("write migrated artifact %s: %w", file.path, err)
		}
	}

	return result, nil
}

func resolveMigrationTarget(cfg MigrationConfig) (string, error) {
	specificTargets := 0
	if strings.TrimSpace(cfg.Name) != "" {
		specificTargets++
	}
	if strings.TrimSpace(cfg.TasksDir) != "" {
		specificTargets++
	}
	if strings.TrimSpace(cfg.ReviewsDir) != "" {
		specificTargets++
	}
	if specificTargets > 1 {
		return "", errors.New("migrate accepts only one of --name, --tasks-dir, or --reviews-dir")
	}

	rootDir := strings.TrimSpace(cfg.RootDir)
	if rootDir == "" {
		rootDir = model.TasksBaseDirForWorkspace(cfg.WorkspaceRoot)
	}

	target := rootDir
	switch {
	case strings.TrimSpace(cfg.ReviewsDir) != "":
		target = strings.TrimSpace(cfg.ReviewsDir)
	case strings.TrimSpace(cfg.TasksDir) != "":
		target = strings.TrimSpace(cfg.TasksDir)
	case strings.TrimSpace(cfg.Name) != "":
		target = filepath.Join(rootDir, strings.TrimSpace(cfg.Name))
	}

	resolved, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve migration target: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat migration target: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("migration target is not a directory: %s", resolved)
	}
	return resolved, nil
}

func scanMigrationTarget(
	ctx context.Context,
	target string,
	result *MigrationResult,
) ([]pendingFileMigration, error) {
	state := migrationScanState{
		result:  result,
		pending: make([]pendingFileMigration, 0),
	}

	err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return state.handlePath(path, entry)
	})
	if err != nil {
		return nil, err
	}
	return state.pending, nil
}

func (s *migrationScanState) handlePath(path string, entry fs.DirEntry) error {
	if entry.IsDir() {
		if entry.Name() == "grouped" {
			s.result.FilesSkipped++
			return filepath.SkipDir
		}
		return nil
	}

	base := filepath.Base(path)
	switch {
	case prompt.ExtractTaskNumber(base) > 0:
		s.result.FilesScanned++
		return s.appendMigration(path, inspectTaskArtifact)
	case prompt.ExtractIssueNumber(base) > 0:
		s.result.FilesScanned++
		return s.appendReviewMigration(path)
	case base == "_meta.md":
		s.result.FilesScanned++
		return s.recordRoundMeta(path)
	default:
		s.result.FilesSkipped++
		return nil
	}
}

func (s *migrationScanState) appendMigration(
	path string,
	inspect func(string, *MigrationResult) (*pendingFileMigration, error),
) error {
	fileMigration, err := inspect(path, s.result)
	if err != nil {
		s.recordInvalid(path)
		return nil
	}
	if fileMigration != nil {
		s.pending = append(s.pending, *fileMigration)
	}
	return nil
}

func (s *migrationScanState) appendReviewMigration(path string) error {
	fileMigration, err := inspectReviewArtifact(path, s.result)
	if err != nil {
		s.recordInvalid(path)
		return nil
	}
	if fileMigration != nil {
		s.pending = append(s.pending, *fileMigration)
	}
	return nil
}

func (s *migrationScanState) recordRoundMeta(path string) error {
	if err := inspectRoundMeta(path, s.result); err != nil {
		s.recordInvalid(path)
	}
	return nil
}

func (s *migrationScanState) recordInvalid(path string) {
	s.result.FilesInvalid++
	s.result.InvalidPaths = append(s.result.InvalidPaths, path)
}

func inspectTaskArtifact(path string, result *MigrationResult) (*pendingFileMigration, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task artifact: %w", err)
	}

	if _, err := prompt.ParseTaskFile(string(content)); err == nil {
		result.FilesAlreadyFrontmatter++
		return nil, nil
	} else if errors.Is(err, prompt.ErrV1TaskMetadata) {
		result.FilesAlreadyFrontmatter++
		return nil, nil
	} else if !errors.Is(err, prompt.ErrLegacyTaskMetadata) {
		return nil, err
	}

	legacyTask, err := prompt.ParseLegacyTaskFile(string(content))
	if err != nil {
		return nil, err
	}
	body, err := prompt.ExtractLegacyTaskBody(string(content))
	if err != nil {
		return nil, err
	}
	migrated, err := frontmatter.Format(model.TaskFileMeta{
		Status:       legacyTask.Status,
		TaskType:     legacyTask.TaskType,
		Complexity:   legacyTask.Complexity,
		Dependencies: legacyTask.Dependencies,
	}, body)
	if err != nil {
		return nil, err
	}

	result.MigratedPaths = append(result.MigratedPaths, path)
	return &pendingFileMigration{path: path, content: migrated}, nil
}

func inspectReviewArtifact(path string, result *MigrationResult) (*pendingFileMigration, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read review artifact: %w", err)
	}

	if _, err := prompt.ParseReviewContext(string(content)); err == nil {
		result.FilesAlreadyFrontmatter++
		return nil, nil
	} else if !errors.Is(err, prompt.ErrLegacyReviewMetadata) {
		return nil, err
	}

	legacyReview, err := prompt.ParseLegacyReviewContext(string(content))
	if err != nil {
		return nil, err
	}
	body, err := prompt.ExtractLegacyReviewBody(string(content))
	if err != nil {
		return nil, err
	}
	fileName := legacyReview.File
	if strings.TrimSpace(fileName) == "" {
		fileName = model.UnknownFileName
	}
	migrated, err := frontmatter.Format(model.ReviewFileMeta{
		Status:      legacyReview.Status,
		File:        fileName,
		Line:        legacyReview.Line,
		Severity:    legacyReview.Severity,
		Author:      legacyReview.Author,
		ProviderRef: legacyReview.ProviderRef,
	}, body)
	if err != nil {
		return nil, err
	}

	result.MigratedPaths = append(result.MigratedPaths, path)
	return &pendingFileMigration{path: path, content: migrated}, nil
}

func inspectRoundMeta(path string, result *MigrationResult) error {
	if _, err := reviews.ReadRoundMeta(filepath.Dir(path)); err != nil {
		return err
	}
	result.FilesAlreadyFrontmatter++
	return nil
}
