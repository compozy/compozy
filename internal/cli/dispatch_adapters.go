package cli

import (
	"context"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/kernel/commands"
)

func newRunWorkflow(dispatcher *kernel.Dispatcher) func(context.Context, core.Config) error {
	if dispatcher == nil {
		return core.Run
	}

	return func(ctx context.Context, cfg core.Config) error {
		_, err := kernel.Dispatch[commands.RunStartCommand, commands.RunStartResult](
			ctx,
			dispatcher,
			commands.RunStartFromConfig(cfg),
		)
		return err
	}
}

func newFetchReviewsRunner(
	dispatcher *kernel.Dispatcher,
) func(context.Context, core.Config) (*core.FetchResult, error) {
	if dispatcher == nil {
		return core.FetchReviews
	}

	return func(ctx context.Context, cfg core.Config) (*core.FetchResult, error) {
		result, err := kernel.Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
			ctx,
			dispatcher,
			commands.ReviewsFetchFromConfig(cfg),
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}

func newMigrateRunner(
	dispatcher *kernel.Dispatcher,
) func(context.Context, core.MigrationConfig) (*core.MigrationResult, error) {
	if dispatcher == nil {
		return core.Migrate
	}

	return func(ctx context.Context, cfg core.MigrationConfig) (*core.MigrationResult, error) {
		typedCommand := commands.WorkspaceMigrateFromConfig(core.Config{
			WorkspaceRoot: cfg.WorkspaceRoot,
			Name:          cfg.Name,
			TasksDir:      cfg.TasksDir,
			ReviewsDir:    cfg.ReviewsDir,
			DryRun:        cfg.DryRun,
		})
		typedCommand.RootDir = cfg.RootDir

		result, err := kernel.Dispatch[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult](
			ctx,
			dispatcher,
			typedCommand,
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}

func newSyncRunner(dispatcher *kernel.Dispatcher) func(context.Context, core.SyncConfig) (*core.SyncResult, error) {
	if dispatcher == nil {
		return core.Sync
	}

	return func(ctx context.Context, cfg core.SyncConfig) (*core.SyncResult, error) {
		typedCommand := commands.WorkflowSyncFromConfig(core.Config{
			WorkspaceRoot: cfg.WorkspaceRoot,
			Name:          cfg.Name,
			TasksDir:      cfg.TasksDir,
		})
		typedCommand.RootDir = cfg.RootDir

		result, err := kernel.Dispatch[commands.WorkflowSyncCommand, commands.WorkflowSyncResult](
			ctx,
			dispatcher,
			typedCommand,
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}

func newArchiveRunner(
	dispatcher *kernel.Dispatcher,
) func(context.Context, core.ArchiveConfig) (*core.ArchiveResult, error) {
	if dispatcher == nil {
		return core.Archive
	}

	return func(ctx context.Context, cfg core.ArchiveConfig) (*core.ArchiveResult, error) {
		typedCommand := commands.WorkflowArchiveFromConfig(core.Config{
			WorkspaceRoot: cfg.WorkspaceRoot,
			Name:          cfg.Name,
			TasksDir:      cfg.TasksDir,
		})
		typedCommand.RootDir = cfg.RootDir

		result, err := kernel.Dispatch[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult](
			ctx,
			dispatcher,
			typedCommand,
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}
