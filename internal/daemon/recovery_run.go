package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/recovery"
)

type daemonWorkflowPreparedRun struct {
	manager    *RunManager
	runtimeCfg *model.RuntimeConfig
	scope      model.RunScope
}

func newDaemonWorkflowPreparedRun(
	manager *RunManager,
	runtimeCfg *model.RuntimeConfig,
	scope model.RunScope,
) *daemonWorkflowPreparedRun {
	return &daemonWorkflowPreparedRun{
		manager:    manager,
		runtimeCfg: runtimeCfg,
		scope:      scope,
	}
}

func (r *daemonWorkflowPreparedRun) Execute(ctx context.Context) (recovery.RunOutcome, error) {
	return r.executePrepared(ctx, nil)
}

func (r *daemonWorkflowPreparedRun) RestartFailed(
	ctx context.Context,
	failedJobIDs []string,
) (recovery.RunOutcome, error) {
	return r.executePrepared(ctx, failedJobIDs)
}

func (r *daemonWorkflowPreparedRun) executePrepared(
	ctx context.Context,
	failedJobIDs []string,
) (recovery.RunOutcome, error) {
	if r == nil || r.manager == nil {
		return recovery.RunOutcome{}, errors.New("daemon recovery: missing run manager")
	}
	if err := recovery.RefreshRunScopeJournal(ctx, r.scope); err != nil {
		return recovery.RunOutcome{}, err
	}
	prep, err := r.manager.prepare(ctx, r.runtimeCfg, r.scope)
	if err != nil {
		return recovery.RunOutcome{}, err
	}
	prep.SetRunScope(r.scope)
	if len(failedJobIDs) > 0 {
		filtered, err := recovery.FilterJobsBySafeName(prep.Jobs, failedJobIDs)
		if err != nil {
			return recovery.RunOutcome{}, err
		}
		prep.Jobs = filtered
	}
	if err := emitPreparedJobQueuedEvents(
		ctx,
		prep.Journal(),
		r.runtimeCfg.RunID,
		prep.Jobs,
		r.runtimeCfg.AccessMode,
	); err != nil {
		return recovery.RunOutcome{}, err
	}
	executeErr := r.manager.execute(ctx, prep, r.runtimeCfg)
	return readDaemonWorkflowRunOutcome(prep.RunArtifacts, executeErr)
}

func readDaemonWorkflowRunOutcome(artifacts model.RunArtifacts, executeErr error) (recovery.RunOutcome, error) {
	outcome, readErr := recovery.ReadRunOutcome(artifacts)
	if readErr != nil {
		if executeErr != nil {
			return recovery.RunOutcome{}, executeErr
		}
		return recovery.RunOutcome{}, readErr
	}
	return *outcome, executeErr
}

type daemonExecPreparedRun struct {
	manager    *RunManager
	runtimeCfg *model.RuntimeConfig
	scope      model.RunScope
}

func newDaemonExecPreparedRun(
	manager *RunManager,
	runtimeCfg *model.RuntimeConfig,
	scope model.RunScope,
) *daemonExecPreparedRun {
	return &daemonExecPreparedRun{manager: manager, runtimeCfg: runtimeCfg, scope: scope}
}

func (r *daemonExecPreparedRun) Execute(ctx context.Context) (recovery.RunOutcome, error) {
	return r.executeExec(ctx)
}

func (r *daemonExecPreparedRun) RestartFailed(
	ctx context.Context,
	failedJobIDs []string,
) (recovery.RunOutcome, error) {
	if len(failedJobIDs) == 0 {
		return recovery.RunOutcome{}, errors.New("daemon exec recovery: no failed job IDs supplied")
	}
	return r.executeExec(ctx)
}

func (r *daemonExecPreparedRun) executeExec(ctx context.Context) (recovery.RunOutcome, error) {
	if r == nil || r.manager == nil {
		return recovery.RunOutcome{}, errors.New("daemon exec recovery: missing run manager")
	}
	if r.runtimeCfg != nil {
		r.runtimeCfg.Persist = true
	}
	if err := recovery.RefreshRunScopeJournal(ctx, r.scope); err != nil {
		return recovery.RunOutcome{}, err
	}
	executeErr := r.manager.executeExec(ctx, r.runtimeCfg, r.scope)
	if r.scope == nil {
		return recovery.RunOutcome{}, executeErr
	}
	return recovery.ReadExecRunOutcome(ctx, r.runtimeCfg, r.scope.RunArtifacts(), executeErr)
}
