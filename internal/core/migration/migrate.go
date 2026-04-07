package migration

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workspace"
	"gopkg.in/yaml.v3"
)

type pendingFileMigration struct {
	path    string
	content string
}

type Config = model.MigrationConfig
type Result = model.MigrationResult

type migrationOutcome int

const (
	migrationOutcomeSkipped migrationOutcome = iota
	migrationOutcomeV1ToV2
)

type migrationScanState struct {
	result   *Result
	registry *tasks.TypeRegistry
	pending  []pendingFileMigration
}

var reviewRoundDirPattern = regexp.MustCompile(`^reviews-\d+$`)

func Migrate(ctx context.Context, cfg Config) (*Result, error) {
	return migrateArtifacts(ctx, cfg)
}

func migrateArtifacts(ctx context.Context, cfg Config) (*Result, error) {
	target, err := resolveMigrationTarget(cfg)
	result := &Result{
		Target: target,
		DryRun: cfg.DryRun,
	}
	if err != nil {
		return result, err
	}

	registry, err := migrationTaskTypeRegistry(ctx, cfg.WorkspaceRoot)
	if err != nil {
		return result, err
	}

	pending, err := scanMigrationTarget(ctx, target, result, registry)
	if err != nil {
		return result, err
	}

	sort.Strings(result.MigratedPaths)
	sort.Strings(result.InvalidPaths)
	sort.Strings(result.UnmappedTypeFiles)
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

func resolveMigrationTarget(cfg Config) (string, error) {
	resolved, err := resolveWorkflowTarget(workflowTargetOptions{
		command:       "migrate",
		workspaceRoot: cfg.WorkspaceRoot,
		rootDir:       cfg.RootDir,
		name:          cfg.Name,
		tasksDir:      cfg.TasksDir,
		reviewsDir:    cfg.ReviewsDir,
		selectorFlags: "--name, --tasks-dir, or --reviews-dir",
	})
	if err != nil {
		return "", err
	}
	return resolved.target, nil
}

func scanMigrationTarget(
	ctx context.Context,
	target string,
	result *Result,
	registry *tasks.TypeRegistry,
) ([]pendingFileMigration, error) {
	state := migrationScanState{
		result:   result,
		registry: registry,
		pending:  make([]pendingFileMigration, 0),
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
		if entry.Name() == "grouped" || entry.Name() == "memory" {
			s.result.FilesSkipped++
			return filepath.SkipDir
		}
		return nil
	}

	base := filepath.Base(path)
	switch {
	case tasks.ExtractTaskNumber(base) > 0:
		s.result.FilesScanned++
		return s.appendTaskMigration(path)
	case reviews.ExtractIssueNumber(base) > 0:
		s.result.FilesScanned++
		return s.appendReviewMigration(path)
	case base == "_meta.md":
		if reviewRoundDirPattern.MatchString(filepath.Base(filepath.Dir(path))) {
			s.result.FilesScanned++
			return s.recordRoundMeta(path)
		}
		s.result.FilesSkipped++
		return nil
	default:
		s.result.FilesSkipped++
		return nil
	}
}

func (s *migrationScanState) appendTaskMigration(path string) error {
	fileMigration, err := inspectTaskArtifact(path, s.result, s.registry)
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

func inspectTaskArtifact(
	path string,
	result *Result,
	registry *tasks.TypeRegistry,
) (*pendingFileMigration, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task artifact: %w", err)
	}

	var node yaml.Node
	if _, err := frontmatter.Parse(string(content), &node); err == nil {
		if !taskFrontMatterNeedsV1ToV2(&node) {
			if _, parseErr := tasks.ParseTaskFile(string(content)); parseErr != nil {
				return nil, parseErr
			}
			result.FilesAlreadyFrontmatter++
			return nil, nil
		}

		fileMigration, outcome, migrateErr := migrateV1ToV2(path, string(content), registry)
		if migrateErr != nil {
			return nil, migrateErr
		}
		if outcome == migrationOutcomeV1ToV2 {
			result.MigratedPaths = append(result.MigratedPaths, path)
			result.V1ToV2Migrated++
			if migratedTypeIsUnmapped(fileMigration.content) {
				result.UnmappedTypeFiles = append(result.UnmappedTypeFiles, path)
			}
		}
		return fileMigration, nil
	} else if !tasks.LooksLikeLegacyTaskFile(string(content)) {
		return nil, fmt.Errorf("parse task artifact: %w", err)
	}

	legacyV1, err := migrateLegacyTaskToV1(string(content))
	if err != nil {
		return nil, err
	}

	fileMigration, outcome, err := migrateV1ToV2(path, legacyV1, registry)
	if err != nil {
		return nil, err
	}
	if outcome == migrationOutcomeV1ToV2 {
		result.MigratedPaths = append(result.MigratedPaths, path)
		result.V1ToV2Migrated++
		if migratedTypeIsUnmapped(fileMigration.content) {
			result.UnmappedTypeFiles = append(result.UnmappedTypeFiles, path)
		}
	}
	return fileMigration, nil
}

func inspectReviewArtifact(path string, result *Result) (*pendingFileMigration, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read review artifact: %w", err)
	}

	if _, err := reviews.ParseReviewContext(string(content)); err == nil {
		result.FilesAlreadyFrontmatter++
		return nil, nil
	} else if !errors.Is(err, reviews.ErrLegacyReviewMetadata) {
		return nil, err
	}

	legacyReview, err := reviews.ParseLegacyReviewContext(string(content))
	if err != nil {
		return nil, err
	}
	body, err := reviews.ExtractLegacyReviewBody(string(content))
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

func inspectRoundMeta(path string, result *Result) error {
	if _, err := reviews.ReadRoundMeta(filepath.Dir(path)); err != nil {
		return err
	}
	result.FilesAlreadyFrontmatter++
	return nil
}

func migrationTaskTypeRegistry(ctx context.Context, workspaceRoot string) (*tasks.TypeRegistry, error) {
	cfg, _, err := workspace.LoadConfig(ctx, workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("load workspace config for migrate: %w", err)
	}
	if cfg.Tasks.Types == nil {
		return tasks.NewRegistry(nil)
	}
	return tasks.NewRegistry(*cfg.Tasks.Types)
}

func migrateLegacyTaskToV1(content string) (string, error) {
	legacyTask, err := tasks.ParseLegacyTaskFile(content)
	if err != nil {
		return "", err
	}

	body, err := tasks.ExtractLegacyTaskBody(content)
	if err != nil {
		return "", err
	}

	migrated, err := frontmatter.Format(model.TaskFileMeta{
		Status:       legacyTask.Status,
		TaskType:     legacyTask.TaskType,
		Complexity:   legacyTask.Complexity,
		Dependencies: legacyTask.Dependencies,
	}, body)
	if err != nil {
		return "", err
	}
	return migrated, nil
}

func migrateV1ToV2(
	path string,
	content string,
	registry *tasks.TypeRegistry,
) (*pendingFileMigration, migrationOutcome, error) {
	var meta model.TaskFileMeta
	body, err := frontmatter.Parse(content, &meta)
	if err != nil {
		return nil, migrationOutcomeSkipped, fmt.Errorf("parse v1 task front matter: %w", err)
	}
	if strings.TrimSpace(meta.Status) == "" {
		return nil, migrationOutcomeSkipped, errors.New("task front matter missing status")
	}

	migrated, err := frontmatter.Format(model.TaskFileMeta{
		Status:       strings.TrimSpace(meta.Status),
		Title:        tasks.ExtractTaskBodyTitle(body),
		TaskType:     tasks.RemapLegacyTaskType(meta.TaskType, registry),
		Complexity:   strings.TrimSpace(meta.Complexity),
		Dependencies: meta.Dependencies,
	}, body)
	if err != nil {
		return nil, migrationOutcomeSkipped, err
	}

	return &pendingFileMigration{path: path, content: migrated}, migrationOutcomeV1ToV2, nil
}

func taskFrontMatterNeedsV1ToV2(node *yaml.Node) bool {
	mapping := node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return false
		}
		mapping = node.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return false
	}

	hasTitle := false
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(keyNode.Value)) {
		case "domain", "scope":
			return true
		case "title":
			hasTitle = true
		}
	}

	return !hasTitle
}

func migratedTypeIsUnmapped(content string) bool {
	var meta model.TaskFileMeta
	if _, err := frontmatter.Parse(content, &meta); err != nil {
		return false
	}
	return strings.TrimSpace(meta.TaskType) == ""
}
