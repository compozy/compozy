package usage

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	repositoryMeterName          = "compozy.llm.usage"
	repositoryOutcomeLabel       = "outcome"
	repositoryOutcomeSuccess     = "success"
	repositoryOutcomeError       = "error"
	repositoryErrorLabel         = "error_type"
	repositoryErrorValidation    = "validation"
	repositoryErrorTimeout       = "timeout"
	repositoryErrorUnknown       = "unknown"
	repositoryErrorDatabase      = "db_error"
	defaultRepositoryQueueSize   = 128
	defaultRepositoryWorkerCount = 1
)

var (
	defaultPersistLatencyBuckets = []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1}
	// Note: Go represents send-on-closed-channel panics with this exact text. Update this value and
	// associated tests if the runtime wording changes in future Go releases.
	errSendClosedChannel = "send on closed channel"
)

// ErrRepositoryClosed indicates the repository no longer accepts new work.
var ErrRepositoryClosed = errors.New("usage repository closed")

// PersistFunc persists the finalized usage payload downstream.
type PersistFunc func(ctx context.Context, finalized *Finalized) error

// RepositoryOptions configure queue behavior.
type RepositoryOptions struct {
	QueueCapacity int
	WorkerCount   int
}

// Repository asynchronously persists usage summaries and records telemetry.
type Repository struct {
	persist   PersistFunc
	queue     chan *persistRequest
	metrics   *repositoryMetrics
	wg        sync.WaitGroup
	stopOnce  sync.Once
	closed    atomic.Bool
	workers   int
	queueSize int
}

type persistRequest struct {
	ctx       context.Context
	finalized *Finalized
}

// NewRepository constructs a Repository with the provided callback.
func NewRepository(persist PersistFunc, opts *RepositoryOptions) (*Repository, error) {
	if persist == nil {
		return nil, fmt.Errorf("persist function is required")
	}
	queueSize := defaultRepositoryQueueSize
	workerCount := defaultRepositoryWorkerCount
	if opts != nil {
		if opts.QueueCapacity > 0 {
			queueSize = opts.QueueCapacity
		}
		if opts.WorkerCount > 0 {
			workerCount = opts.WorkerCount
		}
	}
	repo := &Repository{
		persist:   persist,
		queue:     make(chan *persistRequest, queueSize),
		metrics:   &repositoryMetrics{},
		workers:   workerCount,
		queueSize: queueSize,
	}
	repo.startWorkers()
	return repo, nil
}

// Persist enqueues the finalized usage payload for asynchronous persistence.
func (r *Repository) Persist(ctx context.Context, finalized *Finalized) (err error) {
	if ctx == nil {
		return fmt.Errorf("nil context provided")
	}
	if finalized == nil || finalized.Summary == nil || len(finalized.Summary.Entries) == 0 {
		return nil
	}
	if r.closed.Load() {
		return ErrRepositoryClosed
	}
	req := cloneRequest(ctx, finalized)
	if req == nil {
		return nil
	}
	defer func() {
		if rec := recover(); rec != nil {
			if recErr := classifyRecover(rec); recErr != nil {
				err = recErr
				return
			}
			panic(rec)
		}
	}()
	select {
	case r.queue <- req:
		r.metrics.AddQueueDelta(req.ctx, 1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop drains the queue and releases worker resources.
func (r *Repository) Stop() {
	r.stopOnce.Do(func() {
		r.closed.Store(true)
		close(r.queue)
	})
	r.wg.Wait()
}

func (r *Repository) startWorkers() {
	r.wg.Add(r.workers)
	for i := 0; i < r.workers; i++ {
		go r.runWorker()
	}
}

func (r *Repository) runWorker() {
	defer r.wg.Done()
	for req := range r.queue {
		if req == nil {
			continue
		}
		r.metrics.AddQueueDelta(req.ctx, -1)
		r.handleRequest(req)
	}
}

func (r *Repository) handleRequest(req *persistRequest) {
	start := time.Now()
	err := r.safePersist(req)
	duration := time.Since(start)
	r.metrics.RecordPersist(req.ctx, duration, err)
	if err == nil {
		return
	}
	log := logger.FromContext(req.ctx)
	fields := []any{
		"component", string(req.finalized.Metadata.Component),
	}
	if !req.finalized.Metadata.TaskExecID.IsZero() {
		fields = append(fields, "task_exec_id", req.finalized.Metadata.TaskExecID.String())
	}
	if !req.finalized.Metadata.WorkflowExecID.IsZero() {
		fields = append(fields, "workflow_exec_id", req.finalized.Metadata.WorkflowExecID.String())
	}
	if req.finalized.Metadata.AgentID != nil && *req.finalized.Metadata.AgentID != "" {
		fields = append(fields, "agent_id", *req.finalized.Metadata.AgentID)
	}
	fields = append(fields, "error", err)
	log.Warn("Failed to persist usage summary", fields...)
}

// safePersist executes the persistence callback while recovering panics.
func (r *Repository) safePersist(req *persistRequest) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			stack := string(debug.Stack())
			if recErr := classifyRecover(rec); recErr != nil {
				err = fmt.Errorf("persist panic: %w; stack=%s", recErr, stack)
				return
			}
			err = fmt.Errorf("persist panic: %v; stack=%s", rec, stack)
		}
	}()
	return r.persist(req.ctx, req.finalized)
}

func cloneRequest(ctx context.Context, finalized *Finalized) *persistRequest {
	if finalized == nil || finalized.Summary == nil || len(finalized.Summary.Entries) == 0 {
		return nil
	}
	summary := finalized.Summary.Clone()
	if summary == nil || len(summary.Entries) == 0 {
		return nil
	}
	meta := cloneMetadata(finalized.Metadata)
	reqCtx := context.WithoutCancel(ctx)
	return &persistRequest{
		ctx: reqCtx,
		finalized: &Finalized{
			Metadata: meta,
			Summary:  summary,
		},
	}
}

func cloneMetadata(meta Metadata) Metadata {
	clone := meta
	if meta.AgentID != nil {
		id := *meta.AgentID
		clone.AgentID = &id
	}
	return clone
}

func classifyRecover(rec any) error {
	if rec == nil {
		return nil
	}
	if err, ok := rec.(error); ok {
		if strings.Contains(err.Error(), errSendClosedChannel) {
			return ErrRepositoryClosed
		}
		return err
	}
	msg := fmt.Sprint(rec)
	if strings.Contains(msg, errSendClosedChannel) {
		return ErrRepositoryClosed
	}
	return errors.New(msg)
}

type repositoryMetrics struct {
	once    sync.Once
	latency metric.Float64Histogram
	errors  metric.Int64Counter
	queue   metric.Int64UpDownCounter
}

func (m *repositoryMetrics) AddQueueDelta(ctx context.Context, delta int64) {
	m.ensureInstruments(ctx)
	if ctx == nil || m.queue == nil || delta == 0 {
		return
	}
	m.queue.Add(ctx, delta)
}

func (m *repositoryMetrics) RecordPersist(ctx context.Context, duration time.Duration, err error) {
	m.ensureInstruments(ctx)
	if ctx == nil {
		return
	}
	if m.latency != nil {
		outcome := repositoryOutcomeSuccess
		if err != nil {
			outcome = repositoryOutcomeError
		}
		m.latency.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String(repositoryOutcomeLabel, outcome),
		))
	}
	if err != nil && m.errors != nil {
		m.errors.Add(ctx, 1, metric.WithAttributes(
			attribute.String(repositoryErrorLabel, categorizePersistenceError(err)),
		))
	}
}

// ensureInstruments lazily initializes OpenTelemetry instruments using the first
// context provided. Configuration such as histogram buckets is captured from that
// initial context and remains fixed for the lifetime of this repository instance.
func (m *repositoryMetrics) ensureInstruments(ctx context.Context) {
	if ctx == nil {
		return
	}
	m.once.Do(func() {
		log := logger.FromContext(ctx)
		buckets, bucketErr := resolvePersistBuckets(ctx)
		if bucketErr != nil {
			log.Warn("Invalid persist latency buckets provided, falling back to defaults", "error", bucketErr)
		}
		meter := otel.GetMeterProvider().Meter(repositoryMeterName)
		latency, err := meter.Float64Histogram(
			monitoringmetrics.MetricNameWithSubsystem("llm_usage", "persist_seconds"),
			metric.WithDescription("Usage persistence latency"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(buckets...),
		)
		if err == nil {
			m.latency = latency
		} else {
			log.Error("Failed to create usage persist latency histogram", "error", err)
		}
		errorsCounter, err := meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem("llm_usage", "persist_errors_total"),
			metric.WithDescription("Usage persistence failures"),
		)
		if err == nil {
			m.errors = errorsCounter
		} else {
			log.Error("Failed to create usage persist error counter", "error", err)
		}
		queueGauge, err := meter.Int64UpDownCounter(
			monitoringmetrics.MetricNameWithSubsystem("llm_usage", "persist_queue_size"),
			metric.WithDescription("Pending usage records waiting to be persisted"),
		)
		if err == nil {
			m.queue = queueGauge
		} else {
			log.Error("Failed to create usage persist queue gauge", "error", err)
		}
	})
}

func resolvePersistBuckets(ctx context.Context) ([]float64, error) {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return cloneBuckets(defaultPersistLatencyBuckets), nil
	}
	candidate := cfg.LLM.UsageMetrics.PersistBuckets
	if len(candidate) == 0 {
		return cloneBuckets(defaultPersistLatencyBuckets), nil
	}
	sanitized, err := sanitizePersistBuckets(candidate)
	if err != nil {
		return cloneBuckets(defaultPersistLatencyBuckets), err
	}
	return sanitized, nil
}

func sanitizePersistBuckets(candidate []float64) ([]float64, error) {
	if len(candidate) == 0 {
		return nil, fmt.Errorf("no histogram buckets configured")
	}
	out := make([]float64, len(candidate))
	copy(out, candidate)
	for i, value := range out {
		if value < 0 {
			return nil, fmt.Errorf("bucket[%d]=%f must be non-negative", i, value)
		}
		if i > 0 && value <= out[i-1] {
			return nil, fmt.Errorf("bucket[%d]=%f must be greater than bucket[%d]=%f", i, value, i-1, out[i-1])
		}
	}
	return out, nil
}

func cloneBuckets(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]float64, len(values))
	copy(out, values)
	return out
}

func categorizePersistenceError(err error) string {
	if err == nil {
		return repositoryErrorUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return repositoryErrorTimeout
	}
	lowered := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lowered, "invalid"),
		strings.Contains(lowered, "not found"),
		strings.Contains(lowered, "validation"):
		return repositoryErrorValidation
	case strings.Contains(lowered, "timeout"):
		return repositoryErrorTimeout
	default:
		return repositoryErrorDatabase
	}
}
