package kernel

import (
	"context"
	"errors"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	"github.com/compozy/compozy/internal/core/run"
)

const (
	runStartStatusNoWork    = "no-work"
	runStartStatusSucceeded = "succeeded"
)

type operations interface {
	ValidateRuntimeConfig(*model.RuntimeConfig) error
	OpenRunScope(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error)
	Prepare(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error)
	Execute(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	ExecuteExec(context.Context, *model.RuntimeConfig, model.RunScope) error
	FetchReviews(context.Context, core.Config) (*model.FetchResult, error)
	Migrate(context.Context, model.MigrationConfig) (*model.MigrationResult, error)
	Sync(context.Context, model.SyncConfig) (*model.SyncResult, error)
	Archive(context.Context, model.ArchiveConfig) (*model.ArchiveResult, error)
}

type realOperations struct {
	agentRegistry agent.RuntimeRegistry
}

func (o realOperations) ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	return o.agentRegistry.ValidateRuntimeConfig(cfg)
}

func (realOperations) OpenRunScope(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	opts model.OpenRunScopeOptions,
) (model.RunScope, error) {
	return model.OpenRunScope(ctx, cfg, opts)
}

func (o realOperations) Prepare(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	scope model.RunScope,
) (*model.SolvePreparation, error) {
	return plan.Prepare(ctx, cfg, scope)
}

func (o realOperations) Execute(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
) error {
	if prep == nil {
		return errors.New("execute run: missing preparation")
	}

	return run.Execute(
		ctx,
		prep.Jobs,
		prep.RunArtifacts,
		prep.Journal(),
		prep.EventBus(),
		cfg,
		prep.RuntimeManager(),
	)
}

func (realOperations) ExecuteExec(ctx context.Context, cfg *model.RuntimeConfig, scope model.RunScope) error {
	return run.ExecuteExec(ctx, cfg, scope)
}

func (realOperations) FetchReviews(ctx context.Context, cfg core.Config) (*model.FetchResult, error) {
	return core.FetchReviewsDirect(ctx, cfg)
}

func (realOperations) Migrate(ctx context.Context, cfg model.MigrationConfig) (*model.MigrationResult, error) {
	return core.MigrateDirect(ctx, cfg)
}

func (realOperations) Sync(ctx context.Context, cfg model.SyncConfig) (*model.SyncResult, error) {
	return core.SyncDirect(ctx, cfg)
}

func (realOperations) Archive(ctx context.Context, cfg model.ArchiveConfig) (*model.ArchiveResult, error) {
	return core.ArchiveDirect(ctx, cfg)
}

type runStartHandler struct {
	deps KernelDeps
	ops  operations
}

var _ Handler[commands.RunStartCommand, commands.RunStartResult] = (*runStartHandler)(nil)

func newRunStartHandler(deps KernelDeps, ops operations) *runStartHandler {
	return &runStartHandler{deps: deps, ops: ops}
}

func (h *runStartHandler) Handle(
	ctx context.Context,
	cmd commands.RunStartCommand,
) (result commands.RunStartResult, retErr error) {
	runtimeCfg := cmd.RuntimeConfig()
	if err := h.ops.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return commands.RunStartResult{}, err
	}

	opts := h.runScopeOptions(runtimeCfg)
	if runtimeCfg.Mode == model.ExecutionModeExec && !opts.EnableExecutableExtensions {
		return h.handleFastExec(ctx, runtimeCfg)
	}

	scope, err := h.openStartedRunScope(ctx, runtimeCfg, opts)
	if err != nil {
		return commands.RunStartResult{}, err
	}

	if runtimeCfg.Mode == model.ExecutionModeExec {
		return h.handleScopedExec(ctx, runtimeCfg, scope)
	}

	return h.handlePreparedRun(ctx, runtimeCfg, scope)
}

func (h *runStartHandler) runScopeOptions(runtimeCfg *model.RuntimeConfig) model.OpenRunScopeOptions {
	opts := h.deps.OpenRunScopeOptions
	if runtimeCfg.EnableExecutableExtensions {
		opts.EnableExecutableExtensions = true
	}
	return opts
}

func (h *runStartHandler) handleFastExec(
	ctx context.Context,
	runtimeCfg *model.RuntimeConfig,
) (commands.RunStartResult, error) {
	if err := h.ops.ExecuteExec(ctx, runtimeCfg, nil); err != nil {
		return commands.RunStartResult{}, err
	}

	result := commands.RunStartResult{Status: runStartStatusSucceeded}
	if runtimeCfg.RunID != "" {
		runArtifacts := model.NewRunArtifacts(runtimeCfg.WorkspaceRoot, runtimeCfg.RunID)
		result.RunID = runArtifacts.RunID
		result.ArtifactsDir = runArtifacts.RunDir
	}

	return result, nil
}

func (h *runStartHandler) openStartedRunScope(
	ctx context.Context,
	runtimeCfg *model.RuntimeConfig,
	opts model.OpenRunScopeOptions,
) (model.RunScope, error) {
	scope, err := h.ops.OpenRunScope(ctx, runtimeCfg, opts)
	if err != nil {
		return nil, err
	}

	if err := startRunManager(ctx, scope); err != nil {
		return nil, errors.Join(err, scope.Close(ctx))
	}

	return scope, nil
}

func (h *runStartHandler) handleScopedExec(
	ctx context.Context,
	runtimeCfg *model.RuntimeConfig,
	scope model.RunScope,
) (result commands.RunStartResult, retErr error) {
	defer func() {
		retErr = errors.Join(retErr, scope.Close(ctx))
	}()

	if err := h.ops.ExecuteExec(ctx, runtimeCfg, scope); err != nil {
		return commands.RunStartResult{}, err
	}

	artifacts := scope.RunArtifacts()
	return commands.RunStartResult{
		RunID:        artifacts.RunID,
		ArtifactsDir: artifacts.RunDir,
		Status:       runStartStatusSucceeded,
	}, nil
}

type workflowPrepareHandler struct {
	deps KernelDeps
	ops  operations
}

var _ Handler[commands.WorkflowPrepareCommand, commands.WorkflowPrepareResult] = (*workflowPrepareHandler)(nil)

func newWorkflowPrepareHandler(deps KernelDeps, ops operations) *workflowPrepareHandler {
	return &workflowPrepareHandler{deps: deps, ops: ops}
}

func (h *runStartHandler) handlePreparedRun(
	ctx context.Context,
	runtimeCfg *model.RuntimeConfig,
	scope model.RunScope,
) (result commands.RunStartResult, retErr error) {
	scopeOwned := true
	defer func() {
		if !scopeOwned || scope == nil {
			return
		}
		retErr = errors.Join(retErr, scope.Close(ctx))
	}()

	prep, err := h.ops.Prepare(ctx, runtimeCfg, scope)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return commands.RunStartResult{Status: runStartStatusNoWork}, nil
		}
		return commands.RunStartResult{}, err
	}
	prep.SetRunScope(scope)
	scopeOwned = false
	defer func() {
		retErr = errors.Join(retErr, closePreparationRunScope(ctx, prep))
	}()

	if err := h.ops.Execute(ctx, prep, runtimeCfg); err != nil {
		return commands.RunStartResult{}, err
	}

	return commands.RunStartResult{
		RunID:        prep.RunArtifacts.RunID,
		ArtifactsDir: prep.RunArtifacts.RunDir,
		Status:       runStartStatusSucceeded,
	}, nil
}

func (h *workflowPrepareHandler) Handle(
	ctx context.Context,
	cmd commands.WorkflowPrepareCommand,
) (result commands.WorkflowPrepareResult, retErr error) {
	runtimeCfg := cmd.RuntimeConfig()
	if err := h.ops.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return commands.WorkflowPrepareResult{}, err
	}

	scope, err := h.ops.OpenRunScope(ctx, runtimeCfg, h.deps.OpenRunScopeOptions)
	if err != nil {
		return commands.WorkflowPrepareResult{}, err
	}
	scopeOwned := true
	defer func() {
		if !scopeOwned || scope == nil {
			return
		}
		retErr = errors.Join(retErr, scope.Close(ctx))
	}()

	if err := startRunManager(ctx, scope); err != nil {
		return commands.WorkflowPrepareResult{}, err
	}

	prep, err := h.ops.Prepare(ctx, runtimeCfg, scope)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return commands.WorkflowPrepareResult{}, core.ErrNoWork
		}
		return commands.WorkflowPrepareResult{}, err
	}
	prep.SetRunScope(scope)
	scopeOwned = false
	defer func() {
		retErr = errors.Join(retErr, closePreparationRunScope(ctx, prep))
	}()

	return commands.WorkflowPrepareResult{
		Preparation:  core.NewPreparation(prep),
		RunID:        prep.RunArtifacts.RunID,
		ArtifactsDir: prep.RunArtifacts.RunDir,
	}, nil
}

func startRunManager(ctx context.Context, scope model.RunScope) error {
	if scope == nil {
		return nil
	}

	if manager := scope.RunManager(); manager != nil {
		if err := manager.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

func closePreparationRunScope(ctx context.Context, prep *model.SolvePreparation) error {
	if prep == nil || prep.RunScope == nil {
		return nil
	}

	closeCtx := ctx
	if closeCtx == nil {
		closeCtx = context.Background()
	}
	if _, hasDeadline := closeCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		closeCtx, cancel = context.WithTimeout(closeCtx, time.Second)
		defer cancel()
	}

	return prep.CloseJournal(closeCtx)
}

type delegatingHandler[C any, R any, M any] struct {
	execute func(context.Context, C) (M, error)
	wrap    func(M) R
}

func (h delegatingHandler[C, R, M]) Handle(ctx context.Context, cmd C) (R, error) {
	var zero R
	result, err := h.execute(ctx, cmd)
	if err != nil {
		return zero, err
	}
	return h.wrap(result), nil
}

func newDelegatingHandler[C any, R any, M any](
	execute func(context.Context, C) (M, error),
	wrap func(M) R,
) Handler[C, R] {
	return delegatingHandler[C, R, M]{
		execute: execute,
		wrap:    wrap,
	}
}

func newWorkflowSyncHandler(
	_ KernelDeps,
	ops operations,
) Handler[commands.WorkflowSyncCommand, commands.WorkflowSyncResult] {
	return newDelegatingHandler(
		func(ctx context.Context, cmd commands.WorkflowSyncCommand) (*model.SyncResult, error) {
			return ops.Sync(ctx, cmd.CoreConfig())
		},
		func(result *model.SyncResult) commands.WorkflowSyncResult {
			return commands.WorkflowSyncResult{Result: result}
		},
	)
}

func newWorkflowArchiveHandler(
	_ KernelDeps,
	ops operations,
) Handler[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult] {
	return newDelegatingHandler(
		func(ctx context.Context, cmd commands.WorkflowArchiveCommand) (*model.ArchiveResult, error) {
			return ops.Archive(ctx, cmd.CoreConfig())
		},
		func(result *model.ArchiveResult) commands.WorkflowArchiveResult {
			return commands.WorkflowArchiveResult{Result: result}
		},
	)
}

func newWorkspaceMigrateHandler(
	_ KernelDeps,
	ops operations,
) Handler[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult] {
	return newDelegatingHandler(
		func(ctx context.Context, cmd commands.WorkspaceMigrateCommand) (*model.MigrationResult, error) {
			return ops.Migrate(ctx, cmd.CoreConfig())
		},
		func(result *model.MigrationResult) commands.WorkspaceMigrateResult {
			return commands.WorkspaceMigrateResult{Result: result}
		},
	)
}

func newReviewsFetchHandler(
	_ KernelDeps,
	ops operations,
) Handler[commands.ReviewsFetchCommand, commands.ReviewsFetchResult] {
	return newDelegatingHandler(
		func(ctx context.Context, cmd commands.ReviewsFetchCommand) (*model.FetchResult, error) {
			return ops.FetchReviews(ctx, cmd.CoreConfig())
		},
		func(result *model.FetchResult) commands.ReviewsFetchResult {
			return commands.ReviewsFetchResult{Result: result}
		},
	)
}
