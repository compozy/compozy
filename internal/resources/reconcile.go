package resources

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultReconcileCoalesceWindow   = 50 * time.Millisecond
	defaultReconcileTimeout          = 5 * time.Second
	defaultReconcileFailureThreshold = 3
	defaultReconcileDegradedBackoff  = 500 * time.Millisecond
)

// ReconcileReason identifies why one kind was scheduled.
type ReconcileReason string

const (
	ReconcileReasonBoot       ReconcileReason = "boot"
	ReconcileReasonWrite      ReconcileReason = "write"
	ReconcileReasonDependency ReconcileReason = "dependency"
)

// Normalize returns the canonical trimmed reason.
func (r ReconcileReason) Normalize() ReconcileReason {
	return ReconcileReason(strings.TrimSpace(string(r)))
}

// Validate reports whether the reconcile reason is supported.
func (r ReconcileReason) Validate(path string) error {
	switch r.Normalize() {
	case ReconcileReasonBoot, ReconcileReasonWrite, ReconcileReasonDependency:
		return nil
	default:
		return fmt.Errorf(
			"%w: %s must be %q, %q, or %q: %q",
			ErrValidation,
			path,
			ReconcileReasonBoot,
			ReconcileReasonWrite,
			ReconcileReasonDependency,
			r,
		)
	}
}

// ReconcileDriver drives boot-time and post-commit resource projection.
type ReconcileDriver interface {
	Trigger(ctx context.Context, kind ResourceKind, reason ReconcileReason) error
	RunBoot(ctx context.Context) error
	Close(ctx context.Context) error
}

// ReconcileEventType identifies one emitted reconcile event.
type ReconcileEventType string

const (
	ReconcileEventRequested ReconcileEventType = "requested"
	ReconcileEventCoalesced ReconcileEventType = "coalesced"
	ReconcileEventFailed    ReconcileEventType = "failed"
	ReconcileEventDegraded  ReconcileEventType = "degraded"
	ReconcileEventApplied   ReconcileEventType = "applied"
)

// ReconcileEvent carries one metric-friendly reconcile observation.
type ReconcileEvent struct {
	Type                ReconcileEventType
	Kind                ResourceKind
	Reason              ReconcileReason
	Duration            time.Duration
	Revision            int64
	Operations          int
	ConsecutiveFailures int
	DegradedUntil       time.Time
	Err                 error
}

// ReconcileEventSink receives reconcile lifecycle events for metrics and observability wiring.
type ReconcileEventSink interface {
	ObserveReconcileEvent(ctx context.Context, event ReconcileEvent)
}

// ReconcileHealthStatus captures the scheduler health state for one kind.
type ReconcileHealthStatus string

const (
	ReconcileHealthStatusHealthy  ReconcileHealthStatus = "healthy"
	ReconcileHealthStatusFailing  ReconcileHealthStatus = "failing"
	ReconcileHealthStatusDegraded ReconcileHealthStatus = "degraded"
)

// ReconcileHealth captures the current health state for one projected kind.
type ReconcileHealth struct {
	Kind                ResourceKind
	Status              ReconcileHealthStatus
	ConsecutiveFailures int
	DegradedUntil       time.Time
	LastError           error
}

// ReconcileHealthSink receives kind-health updates from the driver.
type ReconcileHealthSink interface {
	ReportReconcileHealth(ctx context.Context, health ReconcileHealth)
}

// ReconcileOption configures a reconcile driver instance.
type ReconcileOption func(*reconcileDriver)

// WithReconcileLogger overrides the driver logger.
func WithReconcileLogger(logger *slog.Logger) ReconcileOption {
	return func(d *reconcileDriver) {
		if logger != nil {
			d.logger = logger
		}
	}
}

// WithReconcileNow overrides the clock used by the driver.
func WithReconcileNow(now func() time.Time) ReconcileOption {
	return func(d *reconcileDriver) {
		if now != nil {
			d.now = now
		}
	}
}

// WithReconcileCoalesceWindow overrides the per-kind rerun coalescing window.
func WithReconcileCoalesceWindow(window time.Duration) ReconcileOption {
	return func(d *reconcileDriver) {
		if window > 0 {
			d.coalesceWindow = window
		}
	}
}

// WithReconcileTimeout overrides the default per-kind reconcile timeout.
func WithReconcileTimeout(timeout time.Duration) ReconcileOption {
	return func(d *reconcileDriver) {
		if timeout > 0 {
			d.defaultTimeout = timeout
		}
	}
}

// WithReconcileKindTimeout overrides the timeout for one kind.
func WithReconcileKindTimeout(kind ResourceKind, timeout time.Duration) ReconcileOption {
	return func(d *reconcileDriver) {
		normalizedKind := kind.Normalize()
		if normalizedKind == "" || timeout <= 0 {
			return
		}
		if d.kindTimeouts == nil {
			d.kindTimeouts = make(map[ResourceKind]time.Duration)
		}
		d.kindTimeouts[normalizedKind] = timeout
	}
}

// WithReconcileKindTimeouts overrides timeouts for multiple kinds.
func WithReconcileKindTimeouts(timeouts map[ResourceKind]time.Duration) ReconcileOption {
	return func(d *reconcileDriver) {
		for kind, timeout := range timeouts {
			WithReconcileKindTimeout(kind, timeout)(d)
		}
	}
}

// WithReconcileFailureThreshold overrides the failure count that opens the degraded circuit.
func WithReconcileFailureThreshold(threshold int) ReconcileOption {
	return func(d *reconcileDriver) {
		if threshold > 0 {
			d.failureThreshold = threshold
		}
	}
}

// WithReconcileDegradedBackoff overrides the degraded-circuit backoff.
func WithReconcileDegradedBackoff(backoff time.Duration) ReconcileOption {
	return func(d *reconcileDriver) {
		if backoff > 0 {
			d.degradedBackoff = backoff
		}
	}
}

// WithReconcileEventSink wires a metric-friendly event sink into the driver.
func WithReconcileEventSink(sink ReconcileEventSink) ReconcileOption {
	return func(d *reconcileDriver) {
		d.eventSink = sink
	}
}

// WithReconcileHealthSink wires a health sink into the driver.
func WithReconcileHealthSink(sink ReconcileHealthSink) ReconcileOption {
	return func(d *reconcileDriver) {
		d.healthSink = sink
	}
}

type reconcileDriver struct {
	raw   RawStore
	actor MutationActor

	logger           *slog.Logger
	now              func() time.Time
	coalesceWindow   time.Duration
	defaultTimeout   time.Duration
	kindTimeouts     map[ResourceKind]time.Duration
	failureThreshold int
	degradedBackoff  time.Duration
	eventSink        ReconcileEventSink
	healthSink       ReconcileHealthSink

	mu         sync.Mutex
	closed     bool
	queue      []ResourceKind
	kindStates map[ResourceKind]*reconcileKindState

	projectors   map[ResourceKind]projector
	dependents   map[ResourceKind][]ResourceKind
	bootOrder    []ResourceKind
	topoErr      error
	topoRank     map[ResourceKind]int
	workerCtx    context.Context
	workerCancel context.CancelFunc
	notifyCh     chan struct{}
	doneCh       chan struct{}
}

type reconcileKindState struct {
	pending             bool
	running             bool
	dirty               bool
	pendingReason       ReconcileReason
	dirtyReason         ReconcileReason
	readyAt             time.Time
	degradedUntil       time.Time
	consecutiveFailures int
}

type reconcilePassResult struct {
	reason     ReconcileReason
	revision   int64
	operations int
	duration   time.Duration
	err        error
}

var _ ReconcileDriver = (*reconcileDriver)(nil)

// NewReconcileDriver constructs the topology-aware reconcile scheduler.
func NewReconcileDriver(
	raw RawStore,
	actor MutationActor,
	registrations []ProjectorRegistration,
	opts ...ReconcileOption,
) (ReconcileDriver, error) {
	projectors, err := buildProjectorSet(registrations)
	if err != nil {
		return nil, err
	}
	if raw == nil && len(projectors) > 0 {
		return nil, errors.New("resources: raw store is required when projectors are registered")
	}

	normalizedActor := actor
	if raw != nil || len(projectors) > 0 {
		normalizedActor, err = normalizeActor(actor)
		if err != nil {
			return nil, err
		}
	}

	driver := &reconcileDriver{
		raw:              raw,
		actor:            normalizedActor,
		logger:           slog.Default(),
		now:              func() time.Time { return time.Now().UTC() },
		coalesceWindow:   defaultReconcileCoalesceWindow,
		defaultTimeout:   defaultReconcileTimeout,
		kindTimeouts:     make(map[ResourceKind]time.Duration),
		failureThreshold: defaultReconcileFailureThreshold,
		degradedBackoff:  defaultReconcileDegradedBackoff,
		kindStates:       make(map[ResourceKind]*reconcileKindState, len(projectors)),
		projectors:       projectors,
		dependents:       make(map[ResourceKind][]ResourceKind),
		topoRank:         make(map[ResourceKind]int, len(projectors)),
		notifyCh:         make(chan struct{}, 1),
		doneCh:           make(chan struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(driver)
		}
	}
	if driver.logger == nil {
		driver.logger = slog.Default()
	}
	if driver.now == nil {
		return nil, errors.New("resources: reconcile clock is required")
	}
	if driver.coalesceWindow <= 0 {
		return nil, errors.New("resources: reconcile coalesce window must be positive")
	}
	if driver.defaultTimeout <= 0 {
		return nil, errors.New("resources: reconcile timeout must be positive")
	}
	if driver.failureThreshold <= 0 {
		return nil, errors.New("resources: reconcile failure threshold must be positive")
	}
	if driver.degradedBackoff <= 0 {
		return nil, errors.New("resources: reconcile degraded backoff must be positive")
	}

	driver.bootOrder, driver.dependents, driver.topoRank, driver.topoErr = buildReconcileTopology(projectors)
	for kind := range projectors {
		driver.kindStates[kind] = &reconcileKindState{}
	}

	driver.workerCtx, driver.workerCancel = context.WithCancel(context.Background())
	go driver.run()
	return driver, nil
}

func buildProjectorSet(registrations []ProjectorRegistration) (map[ResourceKind]projector, error) {
	projectors := make(map[ResourceKind]projector, len(registrations))
	for _, registration := range registrations {
		projector, err := unwrapProjectorRegistration(registration)
		if err != nil {
			return nil, err
		}
		kind := projector.Kind().Normalize()
		if err := kind.Validate("projector.kind"); err != nil {
			return nil, err
		}
		if _, exists := projectors[kind]; exists {
			return nil, fmt.Errorf("%w: duplicate projector registration for kind %q", ErrConflict, kind)
		}
		projectors[kind] = projector
	}
	return projectors, nil
}

func buildReconcileTopology(
	projectors map[ResourceKind]projector,
) ([]ResourceKind, map[ResourceKind][]ResourceKind, map[ResourceKind]int, error) {
	dependents := make(map[ResourceKind][]ResourceKind, len(projectors))
	indegree := make(map[ResourceKind]int, len(projectors))
	for kind := range projectors {
		indegree[kind] = 0
	}

	for kind, projector := range projectors {
		for _, dependency := range normalizeKinds(projector.DependsOn()) {
			if dependency == "" {
				continue
			}
			if _, ok := projectors[dependency]; ok {
				indegree[kind]++
			}
			dependents[dependency] = append(dependents[dependency], kind)
		}
	}

	for dependencyKind := range dependents {
		sort.Slice(dependents[dependencyKind], func(i int, j int) bool {
			return string(dependents[dependencyKind][i]) < string(dependents[dependencyKind][j])
		})
	}

	ready := make([]ResourceKind, 0, len(projectors))
	for kind, count := range indegree {
		if count == 0 {
			ready = append(ready, kind)
		}
	}
	sort.Slice(ready, func(i int, j int) bool {
		return string(ready[i]) < string(ready[j])
	})

	order := make([]ResourceKind, 0, len(projectors))
	for len(ready) > 0 {
		kind := ready[0]
		ready = ready[1:]
		order = append(order, kind)

		for _, dependent := range dependents[kind] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Slice(ready, func(i int, j int) bool {
			return string(ready[i]) < string(ready[j])
		})
	}

	rank := make(map[ResourceKind]int, len(projectors))
	for idx, kind := range order {
		rank[kind] = idx
	}

	if len(order) != len(projectors) {
		return nil, dependents, rank, fmt.Errorf("%w: reconcile projector dependency cycle detected", ErrValidation)
	}

	for dependencyKind := range dependents {
		sort.Slice(dependents[dependencyKind], func(i int, j int) bool {
			left := dependents[dependencyKind][i]
			right := dependents[dependencyKind][j]
			return rank[left] < rank[right]
		})
	}

	return order, dependents, rank, nil
}
