package kernel

import (
	"context"
	"errors"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	"github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/pkg/compozy/events"
)

const (
	runStartStatusNoWork    = "no-work"
	runStartStatusSucceeded = "succeeded"
)

type operations interface {
	ValidateRuntimeConfig(*model.RuntimeConfig) error
	Prepare(context.Context, *model.RuntimeConfig) (*model.SolvePreparation, error)
	Execute(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	ExecuteExec(context.Context, *model.RuntimeConfig) error
	FetchReviews(context.Context, core.Config) (*model.FetchResult, error)
	Migrate(context.Context, model.MigrationConfig) (*model.MigrationResult, error)
	Sync(context.Context, model.SyncConfig) (*model.SyncResult, error)
	Archive(context.Context, model.ArchiveConfig) (*model.ArchiveResult, error)
}

type realOperations struct {
	agentRegistry agent.RuntimeRegistry
	eventBus      *events.Bus[events.Event]
}

func (o realOperations) ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	return o.agentRegistry.ValidateRuntimeConfig(cfg)
}

func (o realOperations) Prepare(ctx context.Context, cfg *model.RuntimeConfig) (*model.SolvePreparation, error) {
	return plan.Prepare(ctx, cfg, o.eventBus)
}

func (o realOperations) Execute(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
) error {
	if prep == nil {
		return errors.New("execute run: missing preparation")
	}
	return run.Execute(ctx, prep.Jobs, prep.RunArtifacts, prep.Journal(), o.eventBus, cfg)
}

func (realOperations) ExecuteExec(ctx context.Context, cfg *model.RuntimeConfig) error {
	return run.ExecuteExec(ctx, cfg)
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
) (commands.RunStartResult, error) {
	var zero commands.RunStartResult

	runtimeCfg := cmd.RuntimeConfig()
	if err := h.ops.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return zero, err
	}

	if runtimeCfg.Mode == model.ExecutionModeExec {
		if err := h.ops.ExecuteExec(ctx, runtimeCfg); err != nil {
			return zero, err
		}

		result := commands.RunStartResult{Status: runStartStatusSucceeded}
		if runtimeCfg.RunID != "" {
			runArtifacts := model.NewRunArtifacts(runtimeCfg.WorkspaceRoot, runtimeCfg.RunID)
			result.RunID = runArtifacts.RunID
			result.ArtifactsDir = runArtifacts.RunDir
		}
		return result, nil
	}

	prep, err := h.ops.Prepare(ctx, runtimeCfg)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return commands.RunStartResult{Status: runStartStatusNoWork}, nil
		}
		return zero, err
	}

	if err := h.ops.Execute(ctx, prep, runtimeCfg); err != nil {
		return zero, err
	}

	return commands.RunStartResult{
		RunID:        prep.RunArtifacts.RunID,
		ArtifactsDir: prep.RunArtifacts.RunDir,
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

func (h *workflowPrepareHandler) Handle(
	ctx context.Context,
	cmd commands.WorkflowPrepareCommand,
) (commands.WorkflowPrepareResult, error) {
	var zero commands.WorkflowPrepareResult

	runtimeCfg := cmd.RuntimeConfig()
	if err := h.ops.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return zero, err
	}

	prep, err := h.ops.Prepare(ctx, runtimeCfg)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return zero, core.ErrNoWork
		}
		return zero, err
	}
	defer plan.ClosePreparationJournal(ctx, prep)

	return commands.WorkflowPrepareResult{
		Preparation:  core.NewPreparation(prep),
		RunID:        prep.RunArtifacts.RunID,
		ArtifactsDir: prep.RunArtifacts.RunDir,
	}, nil
}

type workflowSyncHandler struct {
	deps KernelDeps
	ops  operations
}

var _ Handler[commands.WorkflowSyncCommand, commands.WorkflowSyncResult] = (*workflowSyncHandler)(nil)

func newWorkflowSyncHandler(deps KernelDeps, ops operations) *workflowSyncHandler {
	return &workflowSyncHandler{deps: deps, ops: ops}
}

func (h *workflowSyncHandler) Handle(
	ctx context.Context,
	cmd commands.WorkflowSyncCommand,
) (commands.WorkflowSyncResult, error) {
	result, err := h.ops.Sync(ctx, cmd.CoreConfig())
	if err != nil {
		return commands.WorkflowSyncResult{}, err
	}
	return commands.WorkflowSyncResult{Result: result}, nil
}

type workflowArchiveHandler struct {
	deps KernelDeps
	ops  operations
}

var _ Handler[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult] = (*workflowArchiveHandler)(nil)

func newWorkflowArchiveHandler(deps KernelDeps, ops operations) *workflowArchiveHandler {
	return &workflowArchiveHandler{deps: deps, ops: ops}
}

func (h *workflowArchiveHandler) Handle(
	ctx context.Context,
	cmd commands.WorkflowArchiveCommand,
) (commands.WorkflowArchiveResult, error) {
	result, err := h.ops.Archive(ctx, cmd.CoreConfig())
	if err != nil {
		return commands.WorkflowArchiveResult{}, err
	}
	return commands.WorkflowArchiveResult{Result: result}, nil
}

type workspaceMigrateHandler struct {
	deps KernelDeps
	ops  operations
}

var _ Handler[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult] = (*workspaceMigrateHandler)(nil)

func newWorkspaceMigrateHandler(deps KernelDeps, ops operations) *workspaceMigrateHandler {
	return &workspaceMigrateHandler{deps: deps, ops: ops}
}

func (h *workspaceMigrateHandler) Handle(
	ctx context.Context,
	cmd commands.WorkspaceMigrateCommand,
) (commands.WorkspaceMigrateResult, error) {
	result, err := h.ops.Migrate(ctx, cmd.CoreConfig())
	if err != nil {
		return commands.WorkspaceMigrateResult{}, err
	}
	return commands.WorkspaceMigrateResult{Result: result}, nil
}

type reviewsFetchHandler struct {
	deps KernelDeps
	ops  operations
}

var _ Handler[commands.ReviewsFetchCommand, commands.ReviewsFetchResult] = (*reviewsFetchHandler)(nil)

func newReviewsFetchHandler(deps KernelDeps, ops operations) *reviewsFetchHandler {
	return &reviewsFetchHandler{deps: deps, ops: ops}
}

func (h *reviewsFetchHandler) Handle(
	ctx context.Context,
	cmd commands.ReviewsFetchCommand,
) (commands.ReviewsFetchResult, error) {
	result, err := h.ops.FetchReviews(ctx, cmd.CoreConfig())
	if err != nil {
		return commands.ReviewsFetchResult{}, err
	}
	return commands.ReviewsFetchResult{Result: result}, nil
}
