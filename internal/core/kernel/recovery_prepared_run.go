package kernel

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/recovery"
)

var (
	_ recovery.PreparedRun = (*kernelWorkflowPreparedRun)(nil)
	_ recovery.PreparedRun = (*kernelExecPreparedRun)(nil)
)

type kernelWorkflowPreparedRun struct {
	ops        operations
	runtimeCfg *model.RuntimeConfig
	scope      model.RunScope
}

func newKernelWorkflowPreparedRun(
	ops operations,
	runtimeCfg *model.RuntimeConfig,
	scope model.RunScope,
) *kernelWorkflowPreparedRun {
	return &kernelWorkflowPreparedRun{ops: ops, runtimeCfg: runtimeCfg, scope: scope}
}

func (r *kernelWorkflowPreparedRun) Execute(ctx context.Context) (recovery.RunOutcome, error) {
	return r.executePrepared(ctx, nil)
}

func (r *kernelWorkflowPreparedRun) RestartFailed(
	ctx context.Context,
	failedJobIDs []string,
) (recovery.RunOutcome, error) {
	return r.executePrepared(ctx, failedJobIDs)
}

func (r *kernelWorkflowPreparedRun) executePrepared(
	ctx context.Context,
	failedJobIDs []string,
) (recovery.RunOutcome, error) {
	if r == nil || r.ops == nil {
		return recovery.RunOutcome{}, errors.New("kernel prepared run: missing operations")
	}
	if err := recovery.RefreshRunScopeJournal(ctx, r.scope); err != nil {
		return recovery.RunOutcome{}, err
	}
	prep, err := r.ops.Prepare(ctx, r.runtimeCfg, r.scope)
	if err != nil {
		return recovery.RunOutcome{}, err
	}
	if prep == nil {
		return recovery.RunOutcome{}, errors.New("kernel prepared run: prepare returned nil preparation")
	}
	prep.SetRunScope(r.scope)
	if failedJobIDs != nil {
		filtered, err := recovery.FilterJobsBySafeName(prep.Jobs, failedJobIDs)
		if err != nil {
			return recovery.RunOutcome{}, err
		}
		prep.Jobs = filtered
	}
	executeErr := r.ops.Execute(ctx, prep, r.runtimeCfg)
	return readWorkflowRunOutcome(prep.RunArtifacts, executeErr)
}

func readWorkflowRunOutcome(artifacts model.RunArtifacts, executeErr error) (recovery.RunOutcome, error) {
	outcome, readErr := recovery.ReadRunOutcome(artifacts)
	if readErr != nil {
		if executeErr != nil {
			return recovery.RunOutcome{}, executeErr
		}
		return recovery.RunOutcome{}, readErr
	}
	return *outcome, executeErr
}

type kernelExecPreparedRun struct {
	ops        operations
	runtimeCfg *model.RuntimeConfig
	scope      model.RunScope
}

func newKernelExecPreparedRun(
	ops operations,
	runtimeCfg *model.RuntimeConfig,
	scope model.RunScope,
) *kernelExecPreparedRun {
	return &kernelExecPreparedRun{ops: ops, runtimeCfg: runtimeCfg, scope: scope}
}

func (r *kernelExecPreparedRun) Execute(ctx context.Context) (recovery.RunOutcome, error) {
	return r.executeExec(ctx)
}

func (r *kernelExecPreparedRun) RestartFailed(
	ctx context.Context,
	failedJobIDs []string,
) (recovery.RunOutcome, error) {
	if len(failedJobIDs) == 0 {
		return recovery.RunOutcome{}, errors.New("kernel exec recovery: no failed job IDs supplied")
	}
	return r.executeExec(ctx)
}

func (r *kernelExecPreparedRun) executeExec(ctx context.Context) (recovery.RunOutcome, error) {
	if r == nil || r.ops == nil {
		return recovery.RunOutcome{}, errors.New("kernel exec recovery: missing operations")
	}
	if r.runtimeCfg != nil {
		r.runtimeCfg.Persist = true
	}
	if err := recovery.RefreshRunScopeJournal(ctx, r.scope); err != nil {
		return recovery.RunOutcome{}, err
	}
	executeErr := r.ops.ExecuteExec(ctx, r.runtimeCfg, r.scope)
	if r.scope == nil {
		return recovery.RunOutcome{}, executeErr
	}
	return recovery.ReadExecRunOutcome(ctx, r.runtimeCfg, r.scope.RunArtifacts(), executeErr)
}
