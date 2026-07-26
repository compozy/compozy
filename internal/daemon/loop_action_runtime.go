package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	loopActionRuntimeActorRef       = "loop-action"
	loopActionRuntimeOriginRef      = "daemon.loop-action"
	loopActionHeartbeatMaxInterval  = 5 * time.Minute
	loopActionRuntimeReasonEnqueued = "task_run_enqueued"
	loopActionRuntimeReasonRecover  = "recovery"
	loopActionRuntimeReasonKey      = "reason"
)

type loopActionTaskManager interface {
	ClaimNextRun(context.Context, taskpkg.ClaimCriteria, taskpkg.ActorContext) (*taskpkg.ClaimResult, error)
	HeartbeatRunLease(context.Context, taskpkg.LeaseHeartbeat, taskpkg.ActorContext) (*taskpkg.Run, error)
	CompleteRunLease(context.Context, taskpkg.LeaseCompletion, taskpkg.ActorContext) (*taskpkg.Run, error)
	FailRunLease(context.Context, taskpkg.LeaseFailure, taskpkg.ActorContext) (*taskpkg.Run, error)
}

type loopActionRunner interface {
	ExecuteActionRun(context.Context, taskpkg.Run, taskpkg.ActorContext) (taskpkg.RunResult, error)
	ActionRunTimeout(context.Context, taskpkg.Run) (time.Duration, bool, error)
}

type loopActionRuntime struct {
	manager          loopActionTaskManager
	store            taskStore
	runner           loopActionRunner
	sessions         loopActionSessionStatus
	logger           *slog.Logger
	now              func() time.Time
	actionRunTimeout time.Duration

	root                 context.Context
	cancel               context.CancelFunc
	sem                  chan struct{}
	spawnMu              sync.Mutex
	wg                   sync.WaitGroup
	stopping             atomic.Bool
	heartbeatInterval    func(time.Duration) time.Duration
	livenessPollInterval func(time.Duration) time.Duration
	claimRetryInterval   time.Duration
}

var _ taskRunEnqueuedObserver = (*loopActionRuntime)(nil)

func newLoopActionRuntime(
	manager loopActionTaskManager,
	store taskStore,
	runner loopActionRunner,
	sessions loopActionSessionStatus,
	logger *slog.Logger,
	now func() time.Time,
	actionRunTimeout time.Duration,
) (*loopActionRuntime, error) {
	if manager == nil {
		return nil, errors.New("daemon: loop action runtime requires task manager")
	}
	if store == nil {
		return nil, errors.New("daemon: loop action runtime requires task store")
	}
	if runner == nil {
		return nil, errors.New("daemon: loop action runtime requires coordinator runner")
	}
	if actionRunTimeout <= 0 || actionRunTimeout > taskpkg.MaxRunLeaseDuration {
		return nil, fmt.Errorf("daemon: loop action timeout must be between 1ns and %s", taskpkg.MaxRunLeaseDuration)
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	root, cancel := context.WithCancel(context.Background())
	return &loopActionRuntime{
		manager:              manager,
		store:                store,
		runner:               runner,
		sessions:             sessions,
		logger:               logger,
		now:                  now,
		actionRunTimeout:     actionRunTimeout,
		root:                 root,
		cancel:               cancel,
		sem:                  make(chan struct{}, looppkg.LoopMaxFanoutWidth),
		heartbeatInterval:    loopActionHeartbeatInterval,
		livenessPollInterval: loopActionLivenessPollInterval,
		claimRetryInterval:   loopActionClaimRetryInterval,
	}, nil
}

func (r *loopActionRuntime) OnTaskRunEnqueued(
	ctx context.Context,
	payload hookspkg.TaskRunEnqueuedPayload,
) {
	if r == nil {
		return
	}
	ctx, cancel := taskRunActivationContext(ctx)
	defer cancel()
	runID := strings.TrimSpace(payload.RunID)
	if runID == "" {
		r.logError("daemon: loop action enqueue payload missing run id", nil, payload)
		return
	}
	run, err := r.store.GetTaskRun(ctx, runID)
	if err != nil {
		r.logError("daemon: load loop action run", err, payload)
		return
	}
	taskRecord, err := r.store.GetTask(ctx, run.TaskID)
	if err != nil {
		r.logError("daemon: load loop action task", err, payload)
		return
	}
	r.startQueuedRun(taskRecord, run, loopActionRuntimeReasonEnqueued, payload)
}

func (r *loopActionRuntime) Recover(ctx context.Context) {
	if r == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runs, err := r.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
	if err != nil {
		r.logError("daemon: list queued loop action runs", err, hookspkg.TaskRunEnqueuedPayload{})
		return
	}
	for _, run := range runs {
		if !isQueuedLoopActionRun(run) {
			continue
		}
		taskRecord, err := r.store.GetTask(ctx, run.TaskID)
		if err != nil {
			r.logError("daemon: load loop action recovery task", err, loopActionPayload(run))
			continue
		}
		r.startQueuedRun(taskRecord, run, loopActionRuntimeReasonRecover, loopActionPayload(run))
	}
}

func (r *loopActionRuntime) startQueuedRun(
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	reason string,
	payload hookspkg.TaskRunEnqueuedPayload,
) {
	if r == nil {
		return
	}
	r.spawnMu.Lock()
	defer r.spawnMu.Unlock()
	if r.stopping.Load() {
		return
	}
	r.wg.Go(func() {
		if err := r.executeStartedRun(r.root, taskRecord, run, reason); err != nil &&
			!errors.Is(err, context.Canceled) {
			r.logError("daemon: execute loop action run", err, payload)
		}
	})
}

func (r *loopActionRuntime) executeQueuedRun(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	reason string,
) error {
	if !isQueuedLoopActionRun(run) {
		return nil
	}
	actor, err := taskpkg.DeriveDaemonActorContext(
		loopActionRuntimeActorRef,
		loopActionRuntimeOriginRef,
	)
	if err != nil {
		return err
	}
	actionTimeout, err := r.actionTimeoutForRun(ctx, run)
	if err != nil {
		return err
	}
	leaseDuration := leaseDurationForActionTimeout(actionTimeout)
	claim, err := r.manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            strings.TrimSpace(run.ID),
		Scope:            taskRecord.Scope.Normalize(),
		WorkspaceID:      strings.TrimSpace(taskRecord.WorkspaceID),
		RunKind:          taskpkg.RunKindWorker,
		ClaimerSessionID: loopActionClaimerSessionID(run.ID),
		ClaimedBy: &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon,
			Ref:  loopActionRuntimeActorRef,
		},
		LeaseDuration: leaseDuration,
		Now:           r.now().UTC(),
	}, actor)
	if err != nil {
		if errors.Is(err, taskpkg.ErrNoClaimableRun) {
			return nil
		}
		return err
	}
	result, err := r.executeClaimedRun(ctx, claim, actor, leaseDuration, actionTimeout)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}
		return r.failClaimedRun(ctx, claim, actor, reason, result.TokensUsed, err)
	}
	_, err = r.manager.CompleteRunLease(context.WithoutCancel(ctx), taskpkg.LeaseCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Result:     result,
		TokensUsed: result.TokensUsed,
		Now:        r.now().UTC(),
	}, actor)
	return err
}

func (r *loopActionRuntime) failClaimedRun(
	ctx context.Context,
	claim *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	reason string,
	tokensUsed int64,
	cause error,
) error {
	if claim == nil {
		return cause
	}
	metadata, err := marshalLoopActionFailureMetadata(reason, cause)
	if err != nil {
		return errors.Join(cause, err)
	}
	_, failErr := r.manager.FailRunLease(context.WithoutCancel(ctx), taskpkg.LeaseFailure{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Failure: taskpkg.RunFailure{
			Error:    cause.Error(),
			Metadata: metadata,
		},
		TokensUsed: tokensUsed,
		Now:        r.now().UTC(),
	}, actor)
	return errors.Join(cause, failErr)
}

func (r *loopActionRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.spawnMu.Lock()
	if r.stopping.CompareAndSwap(false, true) && r.cancel != nil {
		r.cancel()
	}
	r.spawnMu.Unlock()
	return r.wait(ctx)
}

func (r *loopActionRuntime) wait(ctx context.Context) error {
	if r == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for loop action runtime: %w", ctx.Err())
	}
}

func isQueuedLoopActionRun(run taskpkg.Run) bool {
	return run.Status.Normalize() == taskpkg.TaskRunStatusQueued &&
		run.RunKind.Normalize() == taskpkg.RunKindWorker &&
		strings.TrimSpace(run.LoopRunID) != ""
}

func loopActionClaimerSessionID(runID string) string {
	return loopActionRuntimeActorRef + ":" + strings.TrimSpace(runID)
}

func loopActionPayload(run taskpkg.Run) hookspkg.TaskRunEnqueuedPayload {
	return hookspkg.TaskRunEnqueuedPayload{
		TaskRunContext: hookspkg.TaskRunContext{
			TaskID:                       run.TaskID,
			RunID:                        run.ID,
			ResolvedNetworkParticipation: new(run.NetworkSpecSnapshot()),
		},
	}
}

func (r *loopActionRuntime) logError(
	msg string,
	err error,
	payload hookspkg.TaskRunEnqueuedPayload,
) {
	if r == nil || r.logger == nil {
		return
	}
	args := []any{
		coordinatorRuntimeTaskIDKey, strings.TrimSpace(payload.TaskID),
		daemonLogRunIDKey, strings.TrimSpace(payload.RunID),
	}
	if err != nil {
		args = append(args, "error", err)
	}
	r.logger.Error(msg, args...)
}

func (r *loopActionRuntime) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("%s:%p", loopActionRuntimeActorRef, r)
}
