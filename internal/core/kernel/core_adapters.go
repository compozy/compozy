package kernel

import (
	"context"
	"log/slog"
	"sync"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
)

var (
	coreAdapterDispatcherOnce sync.Once
	coreAdapterDispatcher     *Dispatcher
	coreAdapterDispatcherFn   = sharedCoreAdapterDispatcher
)

func init() {
	core.RegisterDispatcherAdapters(core.DispatcherAdapters{
		Prepare:      dispatchPrepareAdapter,
		Run:          dispatchRunAdapter,
		FetchReviews: dispatchFetchReviewsAdapter,
		Migrate:      dispatchMigrateAdapter,
		Sync:         dispatchSyncAdapter,
		Archive:      dispatchArchiveAdapter,
	})
}

func sharedCoreAdapterDispatcher() *Dispatcher {
	coreAdapterDispatcherOnce.Do(func() {
		dispatcher := BuildDefault(KernelDeps{
			Logger:        slog.Default(),
			AgentRegistry: agent.DefaultRegistry(),
		})
		if err := ValidateDefaultRegistry(dispatcher); err != nil {
			panic(err)
		}
		coreAdapterDispatcher = dispatcher
	})
	return coreAdapterDispatcher
}

func dispatchPrepareAdapter(ctx context.Context, cfg core.Config) (*core.Preparation, error) {
	result, err := Dispatch[commands.WorkflowPrepareCommand, commands.WorkflowPrepareResult](
		ctx,
		coreAdapterDispatcherFn(),
		commands.WorkflowPrepareFromConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return result.Preparation, nil
}

func dispatchRunAdapter(ctx context.Context, cfg core.Config) error {
	_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		ctx,
		coreAdapterDispatcherFn(),
		commands.RunStartFromConfig(cfg),
	)
	return err
}

func dispatchFetchReviewsAdapter(ctx context.Context, cfg core.Config) (*core.FetchResult, error) {
	result, err := Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
		ctx,
		coreAdapterDispatcherFn(),
		commands.ReviewsFetchFromConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

func dispatchMigrateAdapter(ctx context.Context, cfg core.MigrationConfig) (*core.MigrationResult, error) {
	command := commands.WorkspaceMigrateFromConfig(core.Config{
		WorkspaceRoot: cfg.WorkspaceRoot,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
		ReviewsDir:    cfg.ReviewsDir,
		DryRun:        cfg.DryRun,
	})
	command.RootDir = cfg.RootDir

	result, err := Dispatch[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult](
		ctx,
		coreAdapterDispatcherFn(),
		command,
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

func dispatchSyncAdapter(ctx context.Context, cfg core.SyncConfig) (*core.SyncResult, error) {
	command := commands.WorkflowSyncFromConfig(core.Config{
		WorkspaceRoot: cfg.WorkspaceRoot,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
	})
	command.RootDir = cfg.RootDir

	result, err := Dispatch[commands.WorkflowSyncCommand, commands.WorkflowSyncResult](
		ctx,
		coreAdapterDispatcherFn(),
		command,
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

func dispatchArchiveAdapter(ctx context.Context, cfg core.ArchiveConfig) (*core.ArchiveResult, error) {
	command := commands.WorkflowArchiveFromConfig(core.Config{
		WorkspaceRoot: cfg.WorkspaceRoot,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
	})
	command.RootDir = cfg.RootDir

	result, err := Dispatch[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult](
		ctx,
		coreAdapterDispatcherFn(),
		command,
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}
