package recall

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const defaultSignalRecorderCapacity = 256

// SignalRecorderConfig controls the bounded asynchronous recall-signal worker.
type SignalRecorderConfig struct {
	QueueCapacity  int
	WorkerRetryMax int
	MetricsEnabled bool
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

	ctx      context.Context
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	closed   atomic.Bool
	acceptMu sync.Mutex

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

// NewSignalRecorder starts a bounded worker for one recall-signal authority.
func NewSignalRecorder(
	ctx context.Context,
	source SignalRecorderSource,
	cfg SignalRecorderConfig,
	logger *slog.Logger,
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
	recorder := &SignalRecorder{
		source:   source,
		queue:    make(chan signalRecordJob, capacity),
		logger:   logger,
		retryMax: max(cfg.WorkerRetryMax, 0),
		ctx:      ctx,
		stop:     make(chan struct{}),
	}
	recorder.wg.Add(1)
	go recorder.run()
	return recorder, nil
}

// Submit enqueues recall signals without waiting for catalog writes.
func (r *SignalRecorder) Submit(
	_ context.Context,
	query memcontract.Query,
	signals []Signal,
) SignalRecorderSubmitResult {
	if r == nil || len(signals) == 0 {
		return SignalRecorderSubmitResult{}
	}
	r.acceptMu.Lock()
	defer r.acceptMu.Unlock()
	if r.closed.Load() || r.ctx.Err() != nil {
		return SignalRecorderSubmitResult{}
	}
	job := signalRecordJob{
		query:   query,
		signals: cloneSignals(signals),
	}
	select {
	case r.queue <- job:
		r.submitted.Add(uint64(len(job.signals)))
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

	select {
	case r.queue <- job:
		r.submitted.Add(uint64(len(job.signals)))
		return SignalRecorderSubmitResult{Submitted: true, Dropped: len(job.dropped) > 0}
	default:
		r.dropped.Add(uint64(len(job.signals)))
		r.recordDroppedSignals(
			append(job.dropped, signalDroppedJob{query: job.query, signals: cloneSignals(job.signals)}),
		)
		return SignalRecorderSubmitResult{Dropped: true}
	}
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
	r.acceptMu.Lock()
	if r.closed.CompareAndSwap(false, true) {
		r.stopOnce.Do(func() {
			close(r.stop)
		})
	}
	r.acceptMu.Unlock()
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("memory recall: close signal recorder: %w", ctx.Err())
	}
}

func (r *SignalRecorder) run() {
	defer r.wg.Done()
	for {
		select {
		case <-r.stop:
			r.drain()
			return
		case <-r.ctx.Done():
			r.drain()
			return
		case job := <-r.queue:
			r.process(job)
		}
	}
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
	if len(job.dropped) > 0 {
		r.recordDroppedSignals(job.dropped)
	}
	if len(job.signals) == 0 {
		return
	}
	var err error
	for attempt := 0; attempt <= r.retryMax; attempt++ {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			err = ctxErr
			break
		}
		if err = r.source.RecordRecall(r.ctx, job.signals); err == nil {
			r.recorded.Add(uint64(len(job.signals)))
			return
		}
	}
	r.failed.Add(uint64(len(job.signals)))
	r.warn("memory recall: record recall signal failed", "error", err)
	if eventErr := r.source.RecordRecallSignalFailed(r.ctx, job.query, err); eventErr != nil {
		r.warn("memory recall: record signal failure event failed", "error", eventErr)
	}
}

func (r *SignalRecorder) recordDroppedSignals(dropped []signalDroppedJob) {
	for _, drop := range dropped {
		if len(drop.signals) == 0 {
			continue
		}
		if err := r.source.RecordRecallSignalDropped(r.ctx, drop.query, drop.signals, len(r.queue)); err != nil {
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
