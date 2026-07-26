package extractor

import (
	"context"
	"errors"

	"log/slog"

	"strings"
	"sync"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	defaultCoalesceMax       = 16
	defaultQueueCapacity     = 1
	defaultThrottleTurns     = 1
	defaultThrottleFlushWait = 500 * time.Millisecond
	actorKindSubagent        = "agent_subagent"
	actorKindRoot            = "agent_root"
	messageRoleAgent         = "assistant"
	sessionTypeDream         = "dream"
	sessionTypeSystem        = "system"
)

var errRuntimeDraining = errors.New("memory extractor: runtime is draining")

// Runtime owns asynchronous transcript extraction and daemon inbox production.
type Runtime struct {
	root              string
	extractor         memcontract.Extractor
	producer          *Producer
	events            EventSink
	logger            *slog.Logger
	now               func() time.Time
	coalesceMax       int
	queueCapacity     int
	throttleTurns     int
	throttleFlushWait time.Duration
	inboxPath         string
	workerCtx         context.Context
	cancelWorker      context.CancelFunc
	providerSlots     chan struct{}

	mu                    sync.Mutex
	sessions              map[string]*sessionState
	toolWrites            map[string]toolWriteMarkers
	skippedTurns          int64
	droppedTurns          int64
	coalescedTurns        int64
	backpressuredSessions int64
	closed                bool
	draining              bool
	wg                    sync.WaitGroup
}

type sessionState struct {
	inFlight   bool
	queued     *request
	flushTimer *time.Timer
}

type request struct {
	turn          memcontract.TurnRecord
	coalesceCount int
}

type toolWriteMarkers struct {
	consumeNext bool
	sequences   map[int64]struct{}
}

func (m toolWriteMarkers) empty() bool {
	return !m.consumeNext && len(m.sequences) == 0
}

// RuntimeStats exposes bounded operational state for daemon status APIs.
type RuntimeStats struct {
	QueuedSessions         int
	InFlightSessions       int
	ActiveProviderSessions int
	SkippedTurns           int64
	DroppedTurns           int64
	CoalescedTurns         int64
	BackpressuredSessions  int64
	Closed                 bool
}

// Option customizes the extractor runtime.
type Option func(*Runtime)

// WithEventSink records extractor telemetry.
func WithEventSink(sink EventSink) Option {
	return func(r *Runtime) {
		r.events = sink
	}
}

// WithLogger configures warning output.
func WithLogger(logger *slog.Logger) Option {
	return func(r *Runtime) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// WithClock injects deterministic time for tests.
func WithClock(now func() time.Time) Option {
	return func(r *Runtime) {
		if now != nil {
			r.now = now
		}
	}
}

// WithCoalesceMax configures the hard queue merge limit.
func WithCoalesceMax(limit int) Option {
	return func(r *Runtime) {
		if limit > 0 {
			r.coalesceMax = limit
		}
	}
}

// WithQueueCapacity bounds concurrent provider-backed extractor sessions.
func WithQueueCapacity(limit int) Option {
	return func(r *Runtime) {
		if limit > 0 {
			r.queueCapacity = limit
		}
	}
}

// WithThrottleTurns coalesces same-session bursts before launching another extraction.
func WithThrottleTurns(limit int) Option {
	return func(r *Runtime) {
		if limit > 0 {
			r.throttleTurns = limit
		}
	}
}

// WithThrottleFlushWait overrides the idle flush wait for tests and tightly controlled runtimes.
func WithThrottleFlushWait(wait time.Duration) Option {
	return func(r *Runtime) {
		if wait > 0 {
			r.throttleFlushWait = wait
		}
	}
}

// WithInboxPath overrides the default <root>/_inbox directory.
func WithInboxPath(path string) Option {
	return func(r *Runtime) {
		if strings.TrimSpace(path) != "" {
			r.inboxPath = path
		}
	}
}

// NewRuntime constructs a daemon-owned extractor runtime.
func NewRuntime(
	ctx context.Context,
	root string,
	extractor memcontract.Extractor,
	opts ...Option,
) (*Runtime, error) {
	if ctx == nil {
		return nil, errors.New("memory extractor: runtime context is required")
	}
	if extractor == nil {
		return nil, errors.New("memory extractor: extractor is required")
	}
	clean, err := cleanRoot(root)
	if err != nil {
		return nil, err
	}
	now := func() time.Time {
		return time.Now().UTC()
	}
	workerCtx, cancel := context.WithCancel(ctx)
	runtime := &Runtime{
		root:              clean,
		extractor:         extractor,
		logger:            slog.Default(),
		now:               now,
		coalesceMax:       defaultCoalesceMax,
		queueCapacity:     defaultQueueCapacity,
		throttleTurns:     defaultThrottleTurns,
		throttleFlushWait: defaultThrottleFlushWait,
		workerCtx:         workerCtx,
		cancelWorker:      cancel,
		sessions:          make(map[string]*sessionState),
		toolWrites:        make(map[string]toolWriteMarkers),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(runtime)
		}
	}
	if runtime.queueCapacity > 0 {
		runtime.providerSlots = make(chan struct{}, runtime.queueCapacity)
	}
	producer, err := NewProducer(clean, runtime.now, WithProducerInboxPath(runtime.inboxPath))
	if err != nil {
		cancel()
		return nil, err
	}
	runtime.producer = producer
	return runtime, nil
}
