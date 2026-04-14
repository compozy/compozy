package executor

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type executorState string

const (
	executorStateInitializing executorState = "initializing"
	executorStateRunning      executorState = "running"
	executorStateDraining     executorState = "draining"
	executorStateForcing      executorState = "forcing"
	executorStateShutdown     executorState = "shutdown"
	executorStateTerminated   executorState = "terminated"
)

type shutdownRequest struct {
	force  bool
	source shutdownSource
}

type normalCompletionHook func(int32, []failInfo, int) error

type executorController struct {
	ctx              context.Context
	execCtx          *jobExecutionContext
	state            executorState
	shutdownState    shutdownState
	cancelJobs       context.CancelCauseFunc
	done             <-chan struct{}
	shutdownRequests chan shutdownRequest
	onNormalDone     normalCompletionHook
}

func executeJobsWithGracefulShutdown(
	ctx context.Context,
	jobs []job,
	cfg *config,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
	onNormalDone normalCompletionHook,
) (int32, []failInfo, int, error) {
	execCtx, err := newJobExecutionContext(ctx, jobs, cfg, runJournal, bus)
	if err != nil {
		total := len(jobs)
		return 0, []failInfo{{Err: err}}, total, nil
	}
	defer execCtx.cleanup()

	jobCtx, cancelJobs := context.WithCancelCause(ctx)
	controller := &executorController{
		ctx:              ctx,
		execCtx:          execCtx,
		state:            executorStateInitializing,
		cancelJobs:       cancelJobs,
		shutdownRequests: make(chan shutdownRequest, 4),
		onNormalDone:     onNormalDone,
	}
	if execCtx.ui != nil {
		execCtx.ui.SetQuitHandler(controller.requestShutdown)
	}
	execCtx.launchWorkers(jobCtx)
	controller.done = execCtx.waitChannel()
	return controller.awaitCompletion()
}

func (c *executorController) awaitCompletion() (int32, []failInfo, int, error) {
	c.state = executorStateRunning
	ctxDone := c.ctx.Done()
	var shutdownTimer *time.Timer
	var shutdownTimerCh <-chan time.Time

	for {
		select {
		case <-c.done:
			return c.handleDone(shutdownTimer)
		case req := <-c.shutdownRequests:
			if req.force {
				c.beginForce(req.source)
				continue
			}
			if c.beginDrain(req.source) {
				shutdownTimer, shutdownTimerCh = resetShutdownTimer(shutdownTimer)
			}
		case <-ctxDone:
			ctxDone = nil
			if c.beginDrain(shutdownSourceSignal) {
				shutdownTimer, shutdownTimerCh = resetShutdownTimer(shutdownTimer)
			}
		case <-shutdownTimerCh:
			shutdownTimerCh = nil
			c.beginForce(shutdownSourceTimer)
		}
	}
}

func (c *executorController) handleDone(shutdownTimer *time.Timer) (int32, []failInfo, int, error) {
	if shutdownTimer != nil {
		shutdownTimer.Stop()
	}
	if c.state == executorStateRunning {
		c.state = executorStateShutdown
		if c.onNormalDone != nil {
			failed := atomic.LoadInt32(&c.execCtx.failed)
			failures := c.execCtx.failures
			total := c.execCtx.total
			if err := c.onNormalDone(failed, failures, total); err != nil {
				c.state = executorStateTerminated
				return c.result(err)
			}
		}
		if err := c.execCtx.awaitUIAfterCompletion(); err != nil {
			c.state = executorStateTerminated
			return c.result(err)
		}
		c.execCtx.reportAggregateUsage()
		c.state = executorStateTerminated
		return c.result(nil)
	}
	c.emitShutdownFallback(
		"Controller shutdown complete after shutdown grace period (%v)\n",
		gracefulShutdownTimeout,
	)
	forced := c.state == executorStateForcing
	c.state = executorStateShutdown
	if err := c.execCtx.shutdownUI(); err != nil {
		c.state = executorStateTerminated
		return c.result(err)
	}
	c.execCtx.reportAggregateUsage()
	c.execCtx.emitShutdownTerminated(c.shutdownState, forced)
	c.state = executorStateTerminated
	return c.result(nil)
}

func resetShutdownTimer(current *time.Timer) (*time.Timer, <-chan time.Time) {
	if current != nil {
		current.Stop()
	}
	next := time.NewTimer(gracefulShutdownTimeout)
	return next, next.C
}

func (c *executorController) result(err error) (int32, []failInfo, int, error) {
	failed := atomic.LoadInt32(&c.execCtx.failed)
	return failed, c.execCtx.failures, c.execCtx.total, err
}

func (c *executorController) emitShutdownFallback(format string, args ...any) {
	if c == nil || c.execCtx == nil || c.execCtx.cfg == nil {
		return
	}
	if !c.execCtx.cfg.HumanOutputEnabled() || c.execCtx.ui != nil {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

func (c *executorController) requestShutdown(req uiQuitRequest) {
	force := req == uiQuitRequestForce
	select {
	case c.shutdownRequests <- shutdownRequest{force: force, source: shutdownSourceUI}:
	default:
	}
}

func (c *executorController) beginDrain(source shutdownSource) bool {
	if c.state != executorStateRunning && c.state != executorStateInitializing {
		return false
	}
	c.emitShutdownFallback(
		"\nReceived shutdown request (%s) while executor in %s state; requesting drain...\n",
		source,
		c.state,
	)
	c.state = executorStateDraining
	if c.cancelJobs != nil {
		c.cancelJobs(context.Canceled)
	}
	now := time.Now()
	state := shutdownState{
		Phase:       shutdownPhaseDraining,
		Source:      source,
		RequestedAt: now,
		DeadlineAt:  now.Add(gracefulShutdownTimeout),
	}
	c.shutdownState = state
	c.execCtx.emitShutdownRequested(state)
	c.execCtx.publishShutdownStatus(state)
	return true
}

func (c *executorController) beginForce(source shutdownSource) {
	if c.state == executorStateForcing || c.state == executorStateShutdown || c.state == executorStateTerminated {
		return
	}
	if c.state == executorStateRunning || c.state == executorStateInitializing {
		if !c.beginDrain(source) {
			return
		}
	}
	c.emitShutdownFallback("Escalating shutdown via %s; forcing exit\n", source)
	c.state = executorStateForcing
	if c.cancelJobs != nil {
		c.cancelJobs(context.Canceled)
	}
	c.execCtx.forceActiveClients()
	requestedAt := c.shutdownState.RequestedAt
	if requestedAt.IsZero() {
		requestedAt = time.Now()
	}
	c.shutdownState = shutdownState{
		Phase:       shutdownPhaseForcing,
		Source:      source,
		RequestedAt: requestedAt,
		DeadlineAt:  c.shutdownState.DeadlineAt,
	}
}
