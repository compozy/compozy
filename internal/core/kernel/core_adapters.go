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

func dispatchAdapter[C any, R any](ctx context.Context, label string, cmd C) (R, error) {
	var zero R

	dispatcher, err := coreAdapterDispatcherFn()
	if err != nil {
		return zero, fmt.Errorf("%s: %w", label, err)
	}

	result, err := Dispatch[C, R](ctx, dispatcher, cmd)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", label, err)
	}
	return result, nil
}

func dispatchPrepareAdapter(ctx context.Context, cfg core.Config) (*core.Preparation, error) {
	result, err := dispatchAdapter[commands.WorkflowPrepareCommand, commands.WorkflowPrepareResult](
		ctx,
		"prepare",
		commands.WorkflowPrepareFromConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return result.Preparation, nil
}

func dispatchRunAdapter(ctx context.Context, cfg core.Config) error {
	_, err := dispatchAdapter[commands.RunStartCommand, commands.RunStartResult](
		ctx,
		"run",
		commands.RunStartFromConfig(cfg),
	)
	return err
}

func dispatchFetchReviewsAdapter(ctx context.Context, cfg core.Config) (*core.FetchResult, error) {
	result, err := dispatchAdapter[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
		ctx,
		"fetch reviews",
		commands.ReviewsFetchFromConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

func dispatchMigrateAdapter(ctx context.Context, cfg core.MigrationConfig) (*core.MigrationResult, error) {
	result, err := dispatchAdapter[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult](
		ctx,
		"migrate",
		commands.WorkspaceMigrateFromMigrationConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

func dispatchSyncAdapter(ctx context.Context, cfg core.SyncConfig) (*core.SyncResult, error) {
	result, err := dispatchAdapter[commands.WorkflowSyncCommand, commands.WorkflowSyncResult](
		ctx,
		"sync",
		commands.WorkflowSyncFromSyncConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

func dispatchArchiveAdapter(ctx context.Context, cfg core.ArchiveConfig) (*core.ArchiveResult, error) {
	result, err := dispatchAdapter[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult](
		ctx,
		"archive",
		commands.WorkflowArchiveFromArchiveConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}
