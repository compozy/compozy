package bridgesdk

import (
	"context"
	"errors"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

const defaultProgressAsyncTimeout = 2 * time.Minute

// ProgressCancel stops one scheduled callback and reports whether it was canceled before running.
type ProgressCancel func() bool

// ProgressSchedule schedules one callback after a delay and returns its cancellation function.
type ProgressSchedule func(delay time.Duration, callback func()) ProgressCancel

// ProgressDispatcherOption customizes scheduled progress delivery.
type ProgressDispatcherOption func(*progressDispatcherOptions)

type progressDispatcherOptions struct {
	schedule     ProgressSchedule
	asyncTimeout time.Duration
	onAsyncError func(error)
}

type progressDispatchRun struct {
	cancelTimer  ProgressCancel
	cancelActive context.CancelFunc
}

// WithProgressSchedule replaces the real timer scheduler, primarily for deterministic tests.
func WithProgressSchedule(schedule ProgressSchedule) ProgressDispatcherOption {
	return func(options *progressDispatcherOptions) {
		if schedule != nil {
			options.schedule = schedule
		}
	}
}

// WithProgressAsyncTimeout bounds detached scheduled provider calls.
func WithProgressAsyncTimeout(timeout time.Duration) ProgressDispatcherOption {
	return func(options *progressDispatcherOptions) {
		if timeout > 0 {
			options.asyncTimeout = timeout
		}
	}
}

// WithProgressAsyncErrorHandler observes scheduled flush failures after they are retained locally.
func WithProgressAsyncErrorHandler(handler func(error)) ProgressDispatcherOption {
	return func(options *progressDispatcherOptions) {
		options.onAsyncError = handler
	}
}

// ProgressDispatcher schedules accumulator edits without tying them to a short delivery RPC context.
type ProgressDispatcher struct {
	mu sync.Mutex
	wg sync.WaitGroup

	accumulator  *ProgressAccumulator
	schedule     ProgressSchedule
	asyncTimeout time.Duration
	onAsyncError func(error)
	runs         map[uint64]*progressDispatchRun
	nextRunID    uint64
	lastAsyncErr error
	closed       bool
}

// NewProgressDispatcher wraps one accumulator with a cancelable throttle scheduler.
func NewProgressDispatcher(
	accumulator *ProgressAccumulator,
	options ...ProgressDispatcherOption,
) *ProgressDispatcher {
	settings := progressDispatcherOptions{
		schedule:     scheduleProgressAfter,
		asyncTimeout: defaultProgressAsyncTimeout,
	}
	for _, option := range options {
		if option != nil {
			option(&settings)
		}
	}
	return &ProgressDispatcher{
		accumulator:  accumulator,
		schedule:     settings.schedule,
		asyncTimeout: settings.asyncTimeout,
		onAsyncError: settings.onAsyncError,
		runs:         make(map[uint64]*progressDispatchRun),
	}
}

// OnProgress records one event and schedules any edit held inside the throttle window.
func (d *ProgressDispatcher) OnProgress(ctx context.Context, event bridgepkg.ToolProgress) error {
	if err := d.validate(); err != nil {
		return err
	}
	if err := d.accumulator.OnProgress(ctx, event); err != nil {
		return err
	}
	d.schedulePending(ctx)
	return nil
}

// Flush cancels the pending timer and writes accumulated progress immediately.
func (d *ProgressDispatcher) Flush(ctx context.Context) error {
	if err := d.validate(); err != nil {
		return err
	}
	d.cancelPending()
	return d.accumulator.Flush(ctx)
}

// OnContent cancels the pending timer, flushes progress, clears typing, and resets the bubble.
func (d *ProgressDispatcher) OnContent(ctx context.Context) error {
	if err := d.validate(); err != nil {
		return err
	}
	d.cancelPending()
	return d.accumulator.OnContent(ctx)
}

// LastAsyncError returns the most recent scheduled flush failure, if any.
func (d *ProgressDispatcher) LastAsyncError() error {
	if d == nil {
		return errors.New("bridgesdk: progress dispatcher is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastAsyncErr
}

// Close cancels pending work and waits for a callback that is already running.
func (d *ProgressDispatcher) Close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.closed = true
	cancelTimers := make([]ProgressCancel, 0, len(d.runs))
	cancelActive := make([]context.CancelFunc, 0, len(d.runs))
	for _, run := range d.runs {
		if run.cancelTimer != nil {
			cancelTimers = append(cancelTimers, run.cancelTimer)
		}
		if run.cancelActive != nil {
			cancelActive = append(cancelActive, run.cancelActive)
		}
	}
	d.mu.Unlock()
	for _, cancel := range cancelTimers {
		cancel()
	}
	for _, cancel := range cancelActive {
		cancel()
	}
	d.wg.Wait()
}

func (d *ProgressDispatcher) validate() error {
	if d == nil {
		return errors.New("bridgesdk: progress dispatcher is required")
	}
	if d.accumulator == nil {
		return errors.New("bridgesdk: progress dispatcher accumulator is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return errors.New("bridgesdk: progress dispatcher is closed")
	}
	return nil
}

func (d *ProgressDispatcher) schedulePending(ctx context.Context) {
	delay, pending := d.accumulator.PendingFlushDelay()
	if !pending {
		return
	}

	d.mu.Lock()
	if d.closed || len(d.runs) > 0 {
		d.mu.Unlock()
		return
	}
	baseCtx := context.WithoutCancel(ctx)
	d.nextRunID++
	runID := d.nextRunID
	run := &progressDispatchRun{}
	d.runs[runID] = run
	d.wg.Add(1)
	var finish sync.Once
	callback := func() {
		defer finish.Do(d.wg.Done)
		d.runScheduledFlush(baseCtx, runID)
	}
	cancelTimer := d.schedule(delay, callback)
	run.cancelTimer = func() bool {
		if cancelTimer == nil || !cancelTimer() {
			return false
		}
		d.mu.Lock()
		delete(d.runs, runID)
		d.mu.Unlock()
		finish.Do(d.wg.Done)
		return true
	}
	d.mu.Unlock()
}

func (d *ProgressDispatcher) runScheduledFlush(baseCtx context.Context, runID uint64) {
	d.mu.Lock()
	run, ok := d.runs[runID]
	if !ok {
		d.mu.Unlock()
		return
	}
	if d.closed {
		delete(d.runs, runID)
		d.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(baseCtx, d.asyncTimeout)
	defer cancel()
	run.cancelActive = cancel
	d.mu.Unlock()

	err := d.accumulator.flushDue(ctx)

	d.mu.Lock()
	delete(d.runs, runID)
	handler := d.onAsyncError
	closed := d.closed
	if !closed {
		d.lastAsyncErr = err
	}
	d.mu.Unlock()
	if closed {
		return
	}
	if err != nil {
		if handler != nil {
			handler(err)
		}
		return
	}
	if !closed {
		d.schedulePending(baseCtx)
	}
}

func (d *ProgressDispatcher) cancelPending() {
	d.mu.Lock()
	cancelTimers := make([]ProgressCancel, 0, len(d.runs))
	for _, run := range d.runs {
		if run.cancelTimer != nil {
			cancelTimers = append(cancelTimers, run.cancelTimer)
		}
	}
	d.mu.Unlock()
	for _, cancel := range cancelTimers {
		cancel()
	}
}

func scheduleProgressAfter(delay time.Duration, callback func()) ProgressCancel {
	timer := time.AfterFunc(delay, callback)
	return timer.Stop
}
