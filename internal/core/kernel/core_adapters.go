package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
)

var (
	coreAdapterDispatcherOnce sync.Once
	coreAdapterDispatcher     *Dispatcher
	coreAdapterDispatcherErr  error
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

func sharedCoreAdapterDispatcher() (*Dispatcher, error) {
	coreAdapterDispatcherOnce.Do(func() {
		dispatcher := BuildDefault(KernelDeps{
			Logger:        slog.Default(),
			AgentRegistry: agent.DefaultRegistry(),
		})
		if err := ValidateDefaultRegistry(dispatcher); err != nil {
			coreAdapterDispatcherErr = err
			return
		}
		coreAdapterDispatcher = dispatcher
	})
	return coreAdapterDispatcher, coreAdapterDispatcherErr
}

func dispatchPrepareAdapter(ctx context.Context, cfg core.Config) (*core.Preparation, error) {
	dispatcher, err := coreAdapterDispatcherFn()
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	result, err := Dispatch[commands.WorkflowPrepareCommand, commands.WorkflowPrepareResult](
		ctx,
		dispatcher,
		commands.WorkflowPrepareFromConfig(cfg),
	)
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	return result.Preparation, nil
}

func dispatchRunAdapter(ctx context.Context, cfg core.Config) error {
	dispatcher, err := coreAdapterDispatcherFn()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	_, err = Dispatch[commands.RunStartCommand, commands.RunStartResult](
		ctx,
		dispatcher,
		commands.RunStartFromConfig(cfg),
	)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func dispatchFetchReviewsAdapter(ctx context.Context, cfg core.Config) (*core.FetchResult, error) {
	dispatcher, err := coreAdapterDispatcherFn()
	if err != nil {
		return nil, fmt.Errorf("fetch reviews: %w", err)
	}
	result, err := Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
		ctx,
		dispatcher,
		commands.ReviewsFetchFromConfig(cfg),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch reviews: %w", err)
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

	dispatcher, err := coreAdapterDispatcherFn()
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	result, err := Dispatch[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult](
		ctx,
		dispatcher,
		command,
	)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
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

	dispatcher, err := coreAdapterDispatcherFn()
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	result, err := Dispatch[commands.WorkflowSyncCommand, commands.WorkflowSyncResult](
		ctx,
		dispatcher,
		command,
	)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
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

	dispatcher, err := coreAdapterDispatcherFn()
	if err != nil {
		return nil, fmt.Errorf("archive: %w", err)
	}
	result, err := Dispatch[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult](
		ctx,
		dispatcher,
		command,
	)
	if err != nil {
		return nil, fmt.Errorf("archive: %w", err)
	}
	return result.Result, nil
}
