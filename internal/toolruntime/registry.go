package toolruntime

import (
	"context"

	"errors"
	"fmt"
	"log/slog"
	"os"

	"sync"
	"time"

	"github.com/compozy/agh/internal/procutil"
)

const (
	defaultInterruptGrace = 250 * time.Millisecond
	defaultKillGrace      = time.Second
)

// InterruptFunc interrupts a live process record owned by the current daemon.
type InterruptFunc func(context.Context, ProcessRecord) error

// Interrupter signals a recovered process after ownership validation.
type Interrupter interface {
	InterruptProcess(ctx context.Context, record ProcessRecord) error
}

// Verifier validates that a PID still belongs to the stored start-time evidence.
type Verifier func(pid int, startedAt time.Time) bool

// Option customizes a Registry.
type Option func(*Registry)

// Registry owns in-memory process handles and durable checkpointing.
type Registry struct {
	store       Store
	verifier    Verifier
	interrupter Interrupter
	now         func() time.Time
	daemonPID   int
	logger      *slog.Logger

	mu     sync.RWMutex
	active map[string]activeProcess
}

type activeProcess struct {
	record    ProcessRecord
	interrupt InterruptFunc
}

// RegisterConfig describes one process registration.
type RegisterConfig struct {
	ID             string
	Source         ProcessSource
	Owner          ProcessOwner
	PID            int
	ProcessGroupID int
	Command        string
	Args           []string
	Cwd            string
	StartedAt      time.Time
	Interrupt      InterruptFunc
}

// BootReconcileReport summarizes restart reconciliation.
type BootReconcileReport struct {
	Checked   int
	Recovered int
	Stale     int
}

// InterruptReport summarizes one scoped interrupt request.
type InterruptReport struct {
	Matched     int
	Signaled    int
	Stale       int
	Unavailable int
}

// Handle represents a registered process checkpoint handle.
type Handle struct {
	registry *Registry
	id       string
	mu       sync.Mutex
	complete bool
}

// WithVerifier overrides PID/start-time validation.
func WithVerifier(verifier Verifier) Option {
	return func(registry *Registry) {
		registry.verifier = verifier
	}
}

// WithInterrupter overrides recovered-process signaling.
func WithInterrupter(interrupter Interrupter) Option {
	return func(registry *Registry) {
		registry.interrupter = interrupter
	}
}

// WithNow overrides the registry clock.
func WithNow(now func() time.Time) Option {
	return func(registry *Registry) {
		registry.now = now
	}
}

// WithDaemonPID records the owning daemon PID in new checkpoints.
func WithDaemonPID(pid int) Option {
	return func(registry *Registry) {
		registry.daemonPID = pid
	}
}

// WithLogger injects a diagnostic logger.
func WithLogger(logger *slog.Logger) Option {
	return func(registry *Registry) {
		registry.logger = logger
	}
}

// NewRegistry constructs a process registry. A nil store keeps live scoped
// interrupts working but skips durable checkpoints.
func NewRegistry(store Store, opts ...Option) *Registry {
	registry := &Registry{
		store:       store,
		verifier:    procutil.MatchesStartTime,
		interrupter: defaultInterrupter{},
		now:         func() time.Time { return time.Now().UTC() },
		daemonPID:   os.Getpid(),
		logger:      slog.Default(),
		active:      make(map[string]activeProcess),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(registry)
		}
	}
	if registry.verifier == nil {
		registry.verifier = procutil.MatchesStartTime
	}
	if registry.interrupter == nil {
		registry.interrupter = defaultInterrupter{}
	}
	if registry.now == nil {
		registry.now = func() time.Time { return time.Now().UTC() }
	}
	if registry.daemonPID <= 0 {
		registry.daemonPID = os.Getpid()
	}
	if registry.logger == nil {
		registry.logger = slog.Default()
	}
	if registry.active == nil {
		registry.active = make(map[string]activeProcess)
	}
	return registry
}

// Register checkpoints a running process and returns a handle for later updates.
func (r *Registry) Register(ctx context.Context, cfg RegisterConfig) (*Handle, error) {
	if r == nil {
		return nil, errors.New("toolruntime: registry is required")
	}
	if ctx == nil {
		return nil, errors.New("toolruntime: register context is required")
	}
	id := cfg.ID
	if id == "" {
		generated, err := newProcessID()
		if err != nil {
			return nil, err
		}
		id = generated
	}
	startedAt := cfg.StartedAt
	if startedAt.IsZero() && cfg.PID > 0 {
		observed, err := procutil.StartedAt(cfg.PID)
		if err != nil {
			return nil, fmt.Errorf("toolruntime: observe process %d start time: %w", cfg.PID, err)
		}
		if observed.IsZero() {
			return nil, fmt.Errorf(
				"%w: observed empty start time for process %d",
				ErrOwnershipValidationFailed,
				cfg.PID,
			)
		}
		startedAt = observed
	}
	now := r.now().UTC()
	record := normalizeRecord(ProcessRecord{
		ID:             id,
		Source:         cfg.Source,
		Owner:          cfg.Owner,
		PID:            cfg.PID,
		ProcessGroupID: cfg.ProcessGroupID,
		Command:        cfg.Command,
		Args:           cfg.Args,
		Cwd:            cfg.Cwd,
		StartedAt:      startedAt,
		State:          ProcessStateRunning,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, now, r.daemonPID)
	if err := validateRecord(record); err != nil {
		return nil, err
	}
	if err := r.upsert(ctx, record); err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.active[record.ID] = activeProcess{record: record, interrupt: cfg.Interrupt}
	r.mu.Unlock()

	return &Handle{registry: r, id: record.ID}, nil
}
