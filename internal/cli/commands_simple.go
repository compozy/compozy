package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/spf13/cobra"
)

type simpleCommandBase struct {
	workspaceRoot string
	projectConfig workspace.ProjectConfig
	rootDir       string
	name          string
	tasksDir      string
}

type migrateCommandState struct {
	simpleCommandBase
	reviewsDir string
	dryRun     bool
	migrateFn  func(context.Context, core.MigrationConfig) (*core.MigrationResult, error)
}

type syncCommandState struct {
	simpleCommandBase
	syncFn func(context.Context, core.SyncConfig) (*core.SyncResult, error)
}

type archiveCommandState struct {
	simpleCommandBase
	archiveFn func(context.Context, core.ArchiveConfig) (*core.ArchiveResult, error)
}

func newMigrateCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	state := &migrateCommandState{
		migrateFn: newMigrateRunner(dispatcher),
	}
	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "Migrate legacy workflow artifacts to frontmatter",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Convert legacy XML-tagged workflow artifacts under .compozy/tasks into Markdown frontmatter.

By default, the command scans the whole project workflow root recursively.`,
		Example: `  compozy migrate
  compozy migrate --dry-run
  compozy migrate --name my-feature
  compozy migrate --reviews-dir .compozy/tasks/my-feature/reviews-001`,
		RunE: state.run,
	}

	cmd.Flags().StringVar(&state.rootDir, "root-dir", "", "Workflow root to scan (default: .compozy/tasks)")
	cmd.Flags().StringVar(&state.name, "name", "", "Restrict migration to one workflow name under the workflow root")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Restrict migration to one task workflow directory")
	cmd.Flags().StringVar(&state.reviewsDir, "reviews-dir", "", "Restrict migration to one review round directory")
	cmd.Flags().BoolVar(&state.dryRun, "dry-run", false, "Plan migrations without writing files")
	return cmd
}

func newSyncCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	state := &syncCommandState{
		syncFn: newSyncRunner(dispatcher),
	}
	cmd := &cobra.Command{
		Use:          "sync",
		Short:        "Reconcile workflow artifacts into global.db",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Parse workflow artifacts under .compozy/tasks and reconcile their
structured task, review, and snapshot state into the daemon global.db catalog.

By default, the command scans the whole workflow root and syncs every active workflow.`,
		Example: `  compozy sync
  compozy sync --name my-feature
  compozy sync --tasks-dir .compozy/tasks/my-feature`,
		RunE: state.run,
	}

	cmd.Flags().StringVar(&state.rootDir, "root-dir", "", "Workflow root to scan (default: .compozy/tasks)")
	cmd.Flags().StringVar(&state.name, "name", "", "Restrict sync to one workflow name under the workflow root")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Restrict sync to one task workflow directory")
	return cmd
}

func newArchiveCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	state := &archiveCommandState{
		archiveFn: newArchiveRunner(dispatcher),
	}
	cmd := &cobra.Command{
		Use:          "archive",
		Short:        "Move fully completed workflows into the archive root",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Archive fully completed workflows under .compozy/tasks by moving them into
.compozy/tasks/_archived/<timestamp-ms>-<shortid>-<slug>.

Archive eligibility is determined from synced global.db task and review state rather than
filesystem metadata files. Single-workflow archive requests reject active runs and incomplete
workflow state; workspace-wide archive requests skip ineligible workflows deterministically.`,
		Example: `  compozy archive
  compozy archive --name my-feature
  compozy archive --tasks-dir .compozy/tasks/my-feature`,
		RunE: state.run,
	}

	cmd.Flags().StringVar(&state.rootDir, "root-dir", "", "Workflow root to scan (default: .compozy/tasks)")
	cmd.Flags().StringVar(&state.name, "name", "", "Restrict archiving to one workflow name under the workflow root")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Restrict archiving to one task workflow directory")
	return cmd
}

func (s *migrateCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	migrateFn := s.migrateFn
	if migrateFn == nil {
		migrateFn = core.Migrate
	}

	result, err := migrateFn(ctx, core.MigrationConfig{
		WorkspaceRoot: s.workspaceRoot,
		RootDir:       s.rootDir,
		Name:          s.name,
		TasksDir:      s.tasksDir,
		ReviewsDir:    s.reviewsDir,
		DryRun:        s.dryRun,
	})
	if result != nil {
		const summaryFormat = "Migrate target: %s\n" +
			"Dry run: %t\n" +
			"Scanned: %d\n" +
			"Migrated: %d\n" +
			"V1->V2 migrated: %d\n" +
			"Already frontmatter: %d\n" +
			"Skipped: %d\n" +
			"Invalid: %d\n"
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			summaryFormat,
			result.Target,
			result.DryRun,
			result.FilesScanned,
			result.FilesMigrated,
			result.V1ToV2Migrated,
			result.FilesAlreadyFrontmatter,
			result.FilesSkipped,
			result.FilesInvalid,
		)
		if len(result.UnmappedTypeFiles) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Unmapped type files: %d\n", len(result.UnmappedTypeFiles))
			for _, path := range result.UnmappedTypeFiles {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", path)
			}

			registry, regErr := taskTypeRegistryFromConfig(s.projectConfig)
			if regErr == nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nFix prompt:\n%s\n", migrationFixPrompt(result, registry))
			}
		}
	}
	return err
}

func migrationFixPrompt(result *core.MigrationResult, registry *tasks.TypeRegistry) string {
	report := tasks.Report{
		TasksDir: migrationTasksDir(result),
		Issues:   make([]tasks.Issue, 0, len(result.UnmappedTypeFiles)),
	}
	for _, path := range result.UnmappedTypeFiles {
		report.Issues = append(report.Issues, tasks.Issue{
			Path:    path,
			Field:   "type",
			Message: fmt.Sprintf(`type value is unmapped; must be one of: %s`, strings.Join(registry.Values(), ", ")),
		})
	}
	return tasks.FixPrompt(report, registry)
}

func migrationTasksDir(result *core.MigrationResult) string {
	if result == nil {
		return ""
	}
	if len(result.UnmappedTypeFiles) == 0 {
		return result.Target
	}
	return filepath.Dir(result.UnmappedTypeFiles[0])
}

func (s *syncCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	syncFn := s.syncFn
	if syncFn == nil {
		syncFn = core.Sync
	}

	result, err := syncFn(ctx, core.SyncConfig{
		WorkspaceRoot: s.workspaceRoot,
		RootDir:       s.rootDir,
		Name:          s.name,
		TasksDir:      s.tasksDir,
	})
	if result != nil {
		const summaryFormat = "Sync target: %s\n" +
			"Workflows scanned: %d\n" +
			"Artifact snapshots upserted: %d\n" +
			"Task items upserted: %d\n" +
			"Review rounds upserted: %d\n" +
			"Review issues upserted: %d\n" +
			"Checkpoints updated: %d\n" +
			"Legacy artifacts removed: %d\n"
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			summaryFormat,
			result.Target,
			result.WorkflowsScanned,
			result.SnapshotsUpserted,
			result.TaskItemsUpserted,
			result.ReviewRoundsUpserted,
			result.ReviewIssuesUpserted,
			result.CheckpointsUpdated,
			result.LegacyArtifactsRemoved,
		)
		for _, warning := range result.Warnings {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning)
		}
	}
	return err
}

func (s *archiveCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	archiveFn := s.archiveFn
	if archiveFn == nil {
		archiveFn = core.Archive
	}

	result, err := archiveFn(ctx, core.ArchiveConfig{
		WorkspaceRoot: s.workspaceRoot,
		RootDir:       s.rootDir,
		Name:          s.name,
		TasksDir:      s.tasksDir,
	})
	if result != nil {
		const summaryFormat = "Archive target: %s\n" +
			"Archive root: %s\n" +
			"Workflows scanned: %d\n" +
			"Archived: %d\n" +
			"Skipped: %d\n"
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			summaryFormat,
			result.Target,
			result.ArchiveRoot,
			result.WorkflowsScanned,
			result.Archived,
			result.Skipped,
		)
		for _, path := range result.ArchivedPaths {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Archived path: %s\n", path)
		}
		for _, path := range result.SkippedPaths {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Skipped workflow: %s (%s)\n", path, result.SkippedReasons[path])
		}
	}
	return err
}
