package recall

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	memcontract "github.com/compozy/compozy/internal/memory/contract"
)

const (
	defaultSignalRecorderCapacity  = 256
	defaultSignalRecorderIOTimeout = 5 * time.Second
)

// SignalRecorderConfig controls the bounded asynchronous recall-signal worker.
type SignalRecorderConfig struct {
	QueueCapacity  int
	WorkerRetryMax int
}

// SignalRecorderStats is a point-in-time snapshot of recorder counters.
type SignalRecorderStats struct {
	Submitted  uint64
	Recorded   uint64
	Dropped    uint64
	Failed     uint64
	QueueDepth int
}

// SignalRecorderSource persists recall signal side effects for the worker.
type SignalRecorderSource interface {
	RecordRecall(ctx context.Context, signals []Signal) error
	RecordRecallSignalFailed(ctx context.Context, query memcontract.Query, cause error) error
	RecordRecallSignalDropped(ctx context.Context, query memcontract.Query, signals []Signal, queueDepth int) error
}

// SignalRecorderSubmitResult describes whether Submit accepted the batch and
// whether it had to drop an older queued batch first.
type SignalRecorderSubmitResult struct {
	Submitted bool
	Dropped   bool
}

// SignalRecorder owns the bounded async write path for recall signals.
type SignalRecorder struct {
	source   SignalRecorderSource
	queue    chan signalRecordJob
	logger   *slog.Logger
	retryMax int

	lifecycleCtx context.Context
	cancel       context.CancelFunc
	stop         chan struct{}
	done         chan struct{}
	idleWake     chan struct{}
	wg           sync.WaitGroup
	closed       atomic.Bool
	acceptMu     sync.Mutex
	shouldStop   func() bool
	onStopped    func()

	submitted atomic.Uint64
	recorded  atomic.Uint64
	dropped   atomic.Uint64
	failed    atomic.Uint64
}

type signalRecordJob struct {
	query   memcontract.Query
	signals []Signal
	dropped []signalDroppedJob
}

type signalDroppedJob struct {
	query   memcontract.Query
	signals []Signal
}

var _ SignalRecorderSource = Source(nil)

// SignalRecorderOption configures recorder lifecycle hooks.
type SignalRecorderOption func(*SignalRecorder)

// WithIdleStop stops a recorder after it becomes quiescent when shouldStop
// confirms that no owner still needs it.
func WithIdleStop(shouldStop func() bool) SignalRecorderOption {
	return func(recorder *SignalRecorder) {
		if shouldStop != nil {
			recorder.shouldStop = shouldStop
		}
	}
}

// WithStopped invokes onStopped after the recorder's worker has terminated.
func WithStopped(onStopped func()) SignalRecorderOption {
	return func(recorder *SignalRecorder) {
		if onStopped != nil {
			recorder.onStopped = onStopped
		}
	}
}

// NewSignalRecorder starts a bounded worker for one recall-signal authority.
func NewSignalRecorder(
	ctx context.Context,
	source SignalRecorderSource,
	cfg SignalRecorderConfig,
	logger *slog.Logger,
	opts ...SignalRecorderOption,
) (*SignalRecorder, error) {
	if ctx == nil {
		return nil, errors.New("memory recall: signal recorder context is required")
	}
	if source == nil {
		return nil, errors.New("memory recall: signal recorder source is required")
	}
	capacity := cfg.QueueCapacity
	if capacity <= 0 {
		capacity = defaultSignalRecorderCapacity
	}
	if logger == nil {
		logger = slog.Default()
	}
	workerCtx, cancel := context.WithCancel(ctx)
	recorder := &SignalRecorder{
		source:       source,
		queue:        make(chan signalRecordJob, capacity),
		logger:       logger,
		retryMax:     max(cfg.WorkerRetryMax, 0),
		lifecycleCtx: workerCtx,
		cancel:       cancel,
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
		idleWake:     make(chan struct{}, 1),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(recorder)
		}
	}
	recorder.wg.Go(recorder.run)
	return recorder, nil
}

// Submit enqueues recall signals without waiting for catalog writes.
func (r *SignalRecorder) Submit(
	ctx context.Context,
	query memcontract.Query,
	signals []Signal,
) SignalRecorderSubmitResult {
	if r == nil || ctx == nil || ctx.Err() != nil || len(signals) == 0 {
		return SignalRecorderSubmitResult{}
	}
	r.acceptMu.Lock()
	if r.closed.Load() || r.lifecycleCtx.Err() != nil {
		r.acceptMu.Unlock()
		return SignalRecorderSubmitResult{}
	}
	job := signalRecordJob{
		query:   query,
		signals: cloneSignals(signals),
	}
	select {
	case r.queue <- job:
		r.submitted.Add(uint64(len(job.signals)))
		r.acceptMu.Unlock()
		return SignalRecorderSubmitResult{Submitted: true}
	default:
	}

	select {
	case dropped := <-r.queue:
		job.dropped = append(cloneDroppedJobs(dropped.dropped), signalDroppedJob{
			query:   dropped.query,
			signals: cloneSignals(dropped.signals),
		})
		r.dropped.Add(uint64(len(dropped.signals)))
	default:
	}
	r.queue <- job
	r.submitted.Add(uint64(len(job.signals)))
	r.acceptMu.Unlock()
	return SignalRecorderSubmitResult{Submitted: true, Dropped: len(job.dropped) > 0}
}

// Stats returns the current queue depth and cumulative worker counters.
func (r *SignalRecorder) Stats() SignalRecorderStats {
	if r == nil {
		return SignalRecorderStats{}
	}
	return SignalRecorderStats{
		Submitted:  r.submitted.Load(),
		Recorded:   r.recorded.Load(),
		Dropped:    r.dropped.Load(),
		Failed:     r.failed.Load(),
		QueueDepth: len(r.queue),
	}
}

// Close stops the worker after draining already-queued batches.
func (r *SignalRecorder) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("memory recall: signal recorder close context is required")
	}
	r.RequestStop()
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		r.Cancel()
		return fmt.Errorf("memory recall: close signal recorder: %w", ctx.Err())
	}
}

// Done closes exactly once when the owned worker exits.
func (r *SignalRecorder) Done() <-chan struct{} {
	if r == nil {
		return nil
	}
	return r.done
}

// RequestStop rejects future submissions and drains already accepted work.
func (r *SignalRecorder) RequestStop() {
	if r == nil {
		return
	}
	r.acceptMu.Lock()
	r.requestStopLocked()
	r.acceptMu.Unlock()
}

// Cancel stops admission and cancels any active recorder I/O.
func (r *SignalRecorder) Cancel() {
	if r == nil {
		return
	}
	r.RequestStop()
	r.cancel()
}

func (r *SignalRecorder) run() {
	defer r.finish()
	for {
		if r.stopIfIdle() {
			return
		}
		select {
		case <-r.stop:
			r.drain()
			return
		case <-r.lifecycleCtx.Done():
			r.drain()
			return
		case <-r.idleWake:
		case job := <-r.queue:
			r.process(job)
		}
	}
}

// NotifyIdle asks the worker to re-evaluate quiescence after its owner releases work.
func (r *SignalRecorder) NotifyIdle() {
	if r == nil {
		return
	}
	select {
	case r.idleWake <- struct{}{}:
	default:
	}
}

func (r *SignalRecorder) stopIfIdle() bool {
	if r == nil || r.shouldStop == nil {
		return false
	}
	r.acceptMu.Lock()
	defer r.acceptMu.Unlock()
	if r.closed.Load() || len(r.queue) > 0 || !r.shouldStop() {
		return false
	}
	r.requestStopLocked()
	return true
}

func (r *SignalRecorder) requestStopLocked() {
	if r.closed.CompareAndSwap(false, true) {
		close(r.stop)
	}
}

func (r *SignalRecorder) finish() {
	r.cancel()
	if r.onStopped != nil {
		r.onStopped()
	}
	close(r.done)
}

func (r *SignalRecorder) drain() {
	for {
		select {
		case job := <-r.queue:
			r.process(job)
		default:
			return
		}
	}
}

func (r *SignalRecorder) process(job signalRecordJob) {
	ctx, cancel := context.WithTimeout(r.lifecycleCtx, defaultSignalRecorderIOTimeout)
	defer cancel()
	if len(job.dropped) > 0 {
		r.recordDroppedSignals(ctx, job.dropped)
	}
	if len(job.signals) == 0 {
		return
	}
	var err error
	for attempt := 0; attempt <= r.retryMax; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
			break
		}
		if err = r.source.RecordRecall(ctx, job.signals); err == nil {
			r.recorded.Add(uint64(len(job.signals)))
			return
		}
	}
	r.failed.Add(uint64(len(job.signals)))
	r.warn("memory recall: record recall signal failed", "error", err)
	if eventErr := r.source.RecordRecallSignalFailed(ctx, job.query, err); eventErr != nil {
		r.warn("memory recall: record signal failure event failed", "error", eventErr)
	}
}

func (r *SignalRecorder) recordDroppedSignals(ctx context.Context, dropped []signalDroppedJob) {
	for _, drop := range dropped {
		if len(drop.signals) == 0 {
			continue
		}
		if err := r.source.RecordRecallSignalDropped(ctx, drop.query, drop.signals, len(r.queue)); err != nil {
			r.warn("memory recall: record dropped signal event failed", "error", err)
		}
	}
}

func (r *SignalRecorder) warn(msg string, args ...any) {
	if r != nil && r.logger != nil {
		r.logger.Warn(msg, args...)
	}
}

func cloneSignals(signals []Signal) []Signal {
	cloned := make([]Signal, len(signals))
	copy(cloned, signals)
	for idx := range cloned {
		if cloned[idx].SurfacedAt.IsZero() {
			cloned[idx].SurfacedAt = time.Now().UTC()
		}
	}
	return cloned
}

func cloneDroppedJobs(dropped []signalDroppedJob) []signalDroppedJob {
	cloned := make([]signalDroppedJob, 0, len(dropped))
	for _, drop := range dropped {
		cloned = append(cloned, signalDroppedJob{
			query:   drop.query,
			signals: cloneSignals(drop.signals),
		})
	}
	return cloned
}
