package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/jonboulle/clockwork"
)

const (
	defaultInterval      = 15 * time.Second
	defaultWakeCooldown  = time.Minute
	defaultSweepLimit    = 100
	defaultSweepReason   = "scheduler_sweep"
	defaultWakeReason    = "pending_task_run"
	defaultStarvationAge = 2 * time.Minute

	defaultFanOutAfter         = 2
	defaultSpawnAfter          = 4
	defaultEventAfter          = 6
	defaultNeedsAttentionAfter = 10
)

var (
	// ErrStopped reports that a stopped scheduler cannot be restarted.
	ErrStopped = errors.New("scheduler: stopped")
	// ErrSpawnUnresolvable reports that no agent covers a starved run's required
	// capabilities, so Tier2 spawn is skipped without failing the cycle; Tier3/Tier4
	// escalation still proceed as the wake count climbs.
	ErrSpawnUnresolvable = errors.New("scheduler: starvation spawn has no capable agent")
)

// CapabilityState records whether a session's capability projection is authoritative.
type CapabilityState string

const (
	// CapabilityStateKnown means Capabilities is authoritative, including an empty slice.
	CapabilityStateKnown CapabilityState = "known"
	// CapabilityStateUnknown means capability projection failed for this cycle.
	CapabilityStateUnknown CapabilityState = "unknown"
)

// CapacityKind separates structural compatibility from momentary availability.
type CapacityKind string

const (
	CapacityAvailable     CapacityKind = "available"
	CapacityWaiting       CapacityKind = "capacity_waiting"
	CapacityUnmatched     CapacityKind = "unmatched"
	CapacityIndeterminate CapacityKind = "indeterminate"
)

// CapacityDisposition is one run's mutually exclusive scheduler-capacity classification.
type CapacityDisposition struct {
	Kind      CapacityKind
	Available []SessionSnapshot
	Reason    string
}

// TaskSource provides durable task-run snapshots and lease recovery.
type TaskSource interface {
	PendingRuns(ctx context.Context) ([]RunSnapshot, error)
	ActiveRuns(ctx context.Context) ([]taskpkg.Run, error)
	GetRunStatus(ctx context.Context, runID string) (taskpkg.RunStatus, bool, error)
	RecoverExpiredRunLeases(
		ctx context.Context,
		recovery taskpkg.ExpiredLeaseRecovery,
		actor taskpkg.ActorContext,
	) ([]taskpkg.ExpiredLeaseRecoveryResult, error)
	ExpireTaskBlocks(
		ctx context.Context,
		now time.Time,
		actor taskpkg.ActorContext,
	) (taskpkg.ExpireTaskBlocksResult, error)
}

// LoopCoordinatorBackstop lets the daemon-owned scheduler wake in-daemon loop coordinators
// without routing coordinator task_runs to ACP sessions.
type LoopCoordinatorBackstop interface {
	RunLoopCoordinatorBackstop(ctx context.Context, now time.Time, actor taskpkg.ActorContext) (int, error)
}

// StarvationStore persists the durable per-run escalation budget so the convergence tier
// ladder survives daemon restart (in-memory wake state is wiped on Rebuild; the budget is not).
type StarvationStore interface {
	LoadRunStarvation(ctx context.Context, runID string) (taskpkg.RunStarvation, bool, error)
	ListRunStarvation(ctx context.Context) ([]taskpkg.RunStarvation, error)
	UpsertRunStarvation(ctx context.Context, mutation taskpkg.RunStarvationMutation) (taskpkg.RunStarvation, error)
	ClearRunStarvation(ctx context.Context, runID string) error
}

// StarvationThresholds bounds the convergence escalation ladder. The counts are wake cycles a
// claimable run must remain queued before each tier fires; they must be monotonic and positive.
type StarvationThresholds struct {
	FanOutAfter         int
	SpawnAfter          int
	EventAfter          int
	NeedsAttentionAfter int
	MinQueuedAge        time.Duration
}

// DefaultStarvationThresholds returns the built-in convergence ladder.
func DefaultStarvationThresholds() StarvationThresholds {
	return StarvationThresholds{
		FanOutAfter:         defaultFanOutAfter,
		SpawnAfter:          defaultSpawnAfter,
		EventAfter:          defaultEventAfter,
		NeedsAttentionAfter: defaultNeedsAttentionAfter,
		MinQueuedAge:        defaultStarvationAge,
	}
}

func (t StarvationThresholds) normalize() StarvationThresholds {
	defaults := DefaultStarvationThresholds()
	if t.FanOutAfter <= 0 {
		t.FanOutAfter = defaults.FanOutAfter
	}
	if t.SpawnAfter <= 0 {
		t.SpawnAfter = defaults.SpawnAfter
	}
	if t.EventAfter <= 0 {
		t.EventAfter = defaults.EventAfter
	}
	if t.NeedsAttentionAfter <= 0 {
		t.NeedsAttentionAfter = defaults.NeedsAttentionAfter
	}
	if t.MinQueuedAge <= 0 {
		t.MinQueuedAge = defaults.MinQueuedAge
	}
	if t.SpawnAfter < t.FanOutAfter {
		t.SpawnAfter = t.FanOutAfter
	}
	if t.EventAfter < t.SpawnAfter {
		t.EventAfter = t.SpawnAfter
	}
	if t.NeedsAttentionAfter < t.EventAfter {
		t.NeedsAttentionAfter = t.EventAfter
	}
	return t
}

// PauseStore supplies the persisted scheduler-wide pause flag.
type PauseStore interface {
	GetSchedulerPause(ctx context.Context) (taskpkg.SchedulerPauseState, error)
}

// SessionSource provides live runtime sessions that may be notified.
type SessionSource interface {
	Sessions(ctx context.Context) ([]SessionSnapshot, error)
}

// Waker sends an advisory notification to one selected idle session.
type Waker interface {
	Wake(ctx context.Context, target *WakeTarget) error
}

// EscalationActor is the single seam through which the scheduler drives convergence
// escalation. It never claims work: it emits observability events, requests a
// capability-matched worker spawn (the spawned session self-claims), and marks a run
// needs_attention. Implemented by the daemon over the task service.
type EscalationActor interface {
	EmitRunStarved(ctx context.Context, work *RunSnapshot, age time.Duration) error
	RequestWorkerSpawn(ctx context.Context, work *RunSnapshot) error
	MarkRunNeedsAttention(ctx context.Context, runID string, diagnostic string) (taskpkg.Run, error)
}

// BatchWaker handles every selected wake target in one scheduler cycle.
type BatchWaker interface {
	WakeMany(ctx context.Context, targets []WakeTarget) []error
}

// RunSnapshot joins a durable run with its owning task.
type RunSnapshot struct {
	Task taskpkg.Task
	Run  taskpkg.Run
}

// SessionSnapshot is the scheduler's rebuildable view of one live session.
type SessionSnapshot struct {
	ID              string
	AgentName       string
	WorkspaceID     string
	Channel         string
	Type            string
	State           string
	Prompting       bool
	Capabilities    []string
	CapabilityState CapabilityState
	CreatedAt       time.Time
}

// WakeTarget records the exact run/session pair selected for notification.
type WakeTarget struct {
	Work    RunSnapshot
	Session SessionSnapshot
	Reason  string
}

// CycleResult reports one mechanical scheduler pass.
type CycleResult struct {
	PendingRuns           int
	ActiveRuns            int
	SessionsScanned       int
	RecoveredLeases       int
	ExpiredBlocks         int
	WakeAttempts          int
	WakeSucceeded         int
	WakeFailed            int
	NoMatchRuns           int
	RecentlyNotified      int
	UnclaimableRuns       int
	Paused                bool
	StarvedRuns           int
	CapacityWaitingRuns   int
	SpawnRequested        int
	NeedsAttention        int
	SelectedRunIDs        []string
	NoMatchRunIDs         []string
	RecoveredRunIDs       []string
	ExpiredBlockIDs       []string
	StarvedRunIDs         []string
	CapacityWaitingRunIDs []string
	SpawnRequestedRunIDs  []string
	NeedsAttentionRunIDs  []string
}

// RebuildResult reports the durable state discovered while rebuilding
// scheduler-owned ephemeral state.
type RebuildResult struct {
	PendingRuns     int
	ActiveRuns      int
	SessionsScanned int
	ClearedWakeKeys int
	RebuiltAt       time.Time
}

// Stats is a lock-protected snapshot of scheduler observability counters.
type Stats struct {
	Cycles              int
	Rebuilds            int
	RecoveredLeases     int
	ExpiredBlocks       int
	RecoveryErrors      int
	ExpiryErrors        int
	WakeAttempts        int
	WakeSucceeded       int
	WakeFailed          int
	NoMatchRuns         int
	RecentlyNotified    int
	UnclaimableRuns     int
	StarvedRuns         int
	CapacityWaitingRuns int
	SpawnRequested      int
	NeedsAttention      int
	LastCycleAt         time.Time
	LastRebuildAt       time.Time
	LastRecoveryError   string
	LastExpiryError     string
	LastWakeError       string
}

// Option customizes scheduler runtime behavior.
type Option func(*Scheduler)

// WithLogger overrides the scheduler logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Scheduler) {
		s.logger = logger
	}
}

// WithClock overrides the scheduler clock, mainly for deterministic tests.
func WithClock(clock clockwork.Clock) Option {
	return func(s *Scheduler) {
		s.clock = clock
	}
}

// WithInterval overrides the background sweep/notify interval.
func WithInterval(interval time.Duration) Option {
	return func(s *Scheduler) {
		s.interval = interval
	}
}

// WithWakeCooldown overrides how long a run/session wake key is suppressed
// before the scheduler may notify that same session again.
func WithWakeCooldown(cooldown time.Duration) Option {
	return func(s *Scheduler) {
		s.wakeCooldown = cooldown
	}
}

// WithSweepReason overrides the task-service recovery reason.
func WithSweepReason(reason string) Option {
	return func(s *Scheduler) {
		s.sweepReason = reason
	}
}

// WithWakeReason overrides the synthetic wake metadata reason.
func WithWakeReason(reason string) Option {
	return func(s *Scheduler) {
		s.wakeReason = reason
	}
}

// WithSweepLimit overrides the maximum expired leases recovered per pass.
func WithSweepLimit(limit int) Option {
	return func(s *Scheduler) {
		s.sweepLimit = limit
	}
}

// WithActor overrides the daemon actor used for recovery writes.
func WithActor(actor taskpkg.ActorContext) Option {
	return func(s *Scheduler) {
		s.actor = actor
	}
}

// WithPauseStore lets the scheduler skip wake dispatch while preserving lease sweep.
func WithPauseStore(store PauseStore) Option {
	return func(s *Scheduler) {
		s.pauseStore = store
	}
}

// WithStarvationAge overrides how long a claimable run may sit queued before the
// scheduler escalates it (fan the advisory wake to every eligible session and emit
// a starvation signal). Zero disables starvation escalation.
func WithStarvationAge(age time.Duration) Option {
	return func(s *Scheduler) {
		s.starvationAge = age
		s.starveThresholds.MinQueuedAge = age
	}
}

// WithEscalationActor injects the seam used to emit starvation signals, request worker spawns,
// and mark runs needs_attention. The scheduler never claims.
func WithEscalationActor(actor EscalationActor) Option {
	return func(s *Scheduler) {
		s.escalator = actor
	}
}

// WithStarvationStore injects the durable escalation budget the convergence ladder advances.
// Without it the tier ladder is disabled (the scheduler still fans out starved runs).
func WithStarvationStore(store StarvationStore) Option {
	return func(s *Scheduler) {
		s.starvation = store
	}
}

// WithStarvationThresholds overrides the convergence tier ladder bounds.
func WithStarvationThresholds(thresholds StarvationThresholds) Option {
	return func(s *Scheduler) {
		s.starveThresholds = thresholds.normalize()
		s.starvationAge = s.starveThresholds.MinQueuedAge
	}
}
