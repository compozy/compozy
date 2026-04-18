package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
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
	outputFormat  string
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

func newMigrateCommand(dispatcher dispatcherProvider) *cobra.Command {
	state := &migrateCommandState{
		migrateFn: newMigrateRunnerWithProvider(dispatcher),
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

func newSyncCommand(dispatcher dispatcherProvider) *cobra.Command {
	state := &syncCommandState{
		syncFn: newSyncRunnerWithProvider(dispatcher),
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
	cmd.Flags().StringVar(&state.outputFormat, "format", operatorOutputFormatText, "Output format: text or json")
	return cmd
}

func newArchiveCommand(dispatcher dispatcherProvider) *cobra.Command {
	state := &archiveCommandState{
		archiveFn: newArchiveRunnerWithProvider(dispatcher),
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
	cmd.Flags().StringVar(&state.outputFormat, "format", operatorOutputFormatText, "Output format: text or json")
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

	format, err := normalizeOperatorOutputFormat(s.outputFormat)
	if err != nil {
		return withExitCode(1, err)
	}

	if err := s.loadWorkspaceRootForTarget(ctx); err != nil {
		return withExitCode(2, fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err))
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	request, err := s.syncRequest()
	if err != nil {
		return withExitCode(1, err)
	}
	result, err := client.SyncWorkflow(ctx, request)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	return writeSyncOutput(cmd, format, result)
}

func (s *archiveCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	format, err := normalizeOperatorOutputFormat(s.outputFormat)
	if err != nil {
		return withExitCode(1, err)
	}

	if err := s.loadWorkspaceRootForTarget(ctx); err != nil {
		return withExitCode(2, fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err))
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	result, err := s.archiveViaDaemon(ctx, client)
	if err != nil {
		return err
	}
	return writeArchiveOutput(cmd, format, result)
}

func (s *simpleCommandBase) loadWorkspaceRootForTarget(ctx context.Context) error {
	startDir := strings.TrimSpace(s.tasksDir)
	if startDir == "" {
		startDir = strings.TrimSpace(s.rootDir)
	}
	root, err := discoverWorkspaceRootFrom(ctx, startDir)
	if err != nil {
		return err
	}
	cfg, err := loadWorkspaceProjectConfig(ctx, root)
	if err != nil {
		return err
	}
	s.workspaceRoot = root
	s.projectConfig = cfg
	return nil
}

func (s *syncCommandState) syncRequest() (apicore.SyncRequest, error) {
	if strings.TrimSpace(s.name) != "" && strings.TrimSpace(s.tasksDir) != "" {
		return apicore.SyncRequest{}, errors.New("sync accepts only one of --name or --tasks-dir")
	}

	if strings.TrimSpace(s.tasksDir) != "" {
		tasksDir, err := resolveTaskWorkflowDir(s.workspaceRoot, s.name, s.tasksDir)
		if err != nil {
			return apicore.SyncRequest{}, err
		}
		return apicore.SyncRequest{Path: tasksDir}, nil
	}

	if strings.TrimSpace(s.rootDir) != "" {
		rootDir, err := absoluteWorkflowPath(s.workspaceRoot, s.rootDir)
		if err != nil {
			return apicore.SyncRequest{}, err
		}
		return apicore.SyncRequest{
			Path:         rootDir,
			WorkflowSlug: strings.TrimSpace(s.name),
		}, nil
	}

	return apicore.SyncRequest{
		Workspace:    s.workspaceRoot,
		WorkflowSlug: strings.TrimSpace(s.name),
	}, nil
}

func (s *archiveCommandState) archiveViaDaemon(
	ctx context.Context,
	client daemonCommandClient,
) (*core.ArchiveResult, error) {
	archiveRootBase := model.TasksBaseDirForWorkspace(s.workspaceRoot)
	if strings.TrimSpace(s.rootDir) != "" {
		resolvedRoot, err := absoluteWorkflowPath(s.workspaceRoot, s.rootDir)
		if err == nil {
			archiveRootBase = resolvedRoot
		}
	}
	result := &core.ArchiveResult{
		Target:         s.archiveTarget(),
		ArchiveRoot:    model.ArchivedTasksDir(archiveRootBase),
		SkippedReasons: make(map[string]string),
	}

	slugs, err := s.archiveWorkflowSlugs(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, slug := range slugs {
		result.WorkflowsScanned++
		archiveResult, archiveErr := client.ArchiveTaskWorkflow(ctx, s.workspaceRoot, slug)
		if archiveErr == nil {
			if archiveResult.Archived {
				result.Archived++
				result.ArchivedPaths = append(result.ArchivedPaths, slug)
			}
			continue
		}

		var remoteErr *apiclient.RemoteError
		if errors.As(archiveErr, &remoteErr) && remoteErr.StatusCode == 409 && len(slugs) > 1 {
			result.Skipped++
			result.SkippedPaths = append(result.SkippedPaths, slug)
			result.SkippedReasons[slug] = remoteErr.Error()
			continue
		}
		return nil, mapDaemonCommandError(archiveErr)
	}

	sort.Strings(result.ArchivedPaths)
	sort.Strings(result.SkippedPaths)
	return result, nil
}

func (s *archiveCommandState) archiveWorkflowSlugs(
	ctx context.Context,
	client daemonCommandClient,
) ([]string, error) {
	if strings.TrimSpace(s.name) != "" {
		return []string{strings.TrimSpace(s.name)}, nil
	}
	if strings.TrimSpace(s.tasksDir) != "" {
		tasksDir, err := resolveTaskWorkflowDir(s.workspaceRoot, s.name, s.tasksDir)
		if err != nil {
			return nil, err
		}
		return []string{filepath.Base(tasksDir)}, nil
	}
	if strings.TrimSpace(s.rootDir) != "" {
		rootDir, err := absoluteWorkflowPath(s.workspaceRoot, s.rootDir)
		if err != nil {
			return nil, err
		}
		return listWorkflowSlugsFromRoot(rootDir)
	}

	workflows, err := client.ListTaskWorkflows(ctx, s.workspaceRoot)
	if err != nil {
		return nil, mapDaemonCommandError(err)
	}
	slugs := make([]string, 0, len(workflows))
	for _, workflow := range workflows {
		if workflow.ArchivedAt != nil {
			continue
		}
		slugs = append(slugs, workflow.Slug)
	}
	sort.Strings(slugs)
	return slugs, nil
}

func (s *archiveCommandState) archiveTarget() string {
	if strings.TrimSpace(s.tasksDir) != "" {
		return strings.TrimSpace(s.tasksDir)
	}
	if strings.TrimSpace(s.rootDir) != "" {
		return strings.TrimSpace(s.rootDir)
	}
	if strings.TrimSpace(s.name) != "" {
		return strings.TrimSpace(s.name)
	}
	return model.TasksBaseDirForWorkspace(s.workspaceRoot)
}

func absoluteWorkflowPath(workspaceRoot string, value string) (string, error) {
	resolved := strings.TrimSpace(value)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(workspaceRoot, resolved)
	}
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve workflow path: %w", err)
	}
	return absPath, nil
}

func listWorkflowSlugsFromRoot(rootDir string) ([]string, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("read archive root %s: %w", rootDir, err)
	}
	slugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && model.IsActiveWorkflowDirName(entry.Name()) {
			slugs = append(slugs, entry.Name())
		}
	}
	sort.Strings(slugs)
	return slugs, nil
}

func writeSyncOutput(cmd *cobra.Command, format string, result apicore.SyncResult) error {
	if format == operatorOutputFormatJSON {
		if err := writeOperatorJSON(cmd.OutOrStdout(), result); err != nil {
			return withExitCode(2, fmt.Errorf("write sync json: %w", err))
		}
		return nil
	}

	const summaryFormat = "Sync target: %s\n" +
		"Workflows scanned: %d\n" +
		"Artifact snapshots upserted: %d\n" +
		"Task items upserted: %d\n" +
		"Review rounds upserted: %d\n" +
		"Review issues upserted: %d\n" +
		"Checkpoints updated: %d\n" +
		"Legacy artifacts removed: %d\n"
	if _, err := fmt.Fprintf(
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
	); err != nil {
		return withExitCode(2, fmt.Errorf("write sync output: %w", err))
	}
	for _, warning := range result.Warnings {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning); err != nil {
			return withExitCode(2, fmt.Errorf("write sync warning: %w", err))
		}
	}
	return nil
}

func writeArchiveOutput(cmd *cobra.Command, format string, result *core.ArchiveResult) error {
	if format == operatorOutputFormatJSON {
		if err := writeOperatorJSON(cmd.OutOrStdout(), result); err != nil {
			return withExitCode(2, fmt.Errorf("write archive json: %w", err))
		}
		return nil
	}

	const summaryFormat = "Archive target: %s\n" +
		"Archive root: %s\n" +
		"Workflows scanned: %d\n" +
		"Archived: %d\n" +
		"Skipped: %d\n"
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		summaryFormat,
		result.Target,
		result.ArchiveRoot,
		result.WorkflowsScanned,
		result.Archived,
		result.Skipped,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write archive output: %w", err))
	}
	for _, slug := range result.ArchivedPaths {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Archived workflow: %s\n", slug); err != nil {
			return withExitCode(2, fmt.Errorf("write archive success: %w", err))
		}
	}
	for _, slug := range result.SkippedPaths {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"Skipped workflow: %s (%s)\n",
			slug,
			result.SkippedReasons[slug],
		); err != nil {
			return withExitCode(2, fmt.Errorf("write archive skip: %w", err))
		}
	}
	return nil
}
