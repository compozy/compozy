package memory

import (
	"context"

	"errors"

	"log/slog"

	"path/filepath"
	"strings"
	"sync"
	"time"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	defaultMinHours    = 24
	defaultMinSessions = 3
	defaultGoal        = "memory-consolidation"
)

var (
	// ErrLockUnavailable reports that a consolidation run could not obtain the lock.
	ErrLockUnavailable = errors.New("memory: consolidation lock is unavailable")
	// ErrDreamRoleDisabled reports that no effective workspace route permits a dream run.
	ErrDreamRoleDisabled = errors.New("memory: dream role is disabled")
)

// SessionSpawner starts a one-shot consolidation session with the provided
// goal, prompt, and normalized workspace ID. A blank workspace ID lets the
// spawner derive the eligible workspaces itself using the supplied timestamp
// from the most recent successful consolidation.
type SessionSpawner func(ctx context.Context, goal, prompt, workspaceID string, lastConsolidatedAt time.Time) error

// Option configures a consolidation Service.
type Option func(*Service)

type consolidationLocker interface {
	LastConsolidatedAt() (time.Time, error)
	TryAcquire() (time.Time, bool, error)
	Release() error
	Rollback(priorMtime time.Time) error
}

// Service evaluates consolidation gates and runs the dream worker when the lock and thresholds allow it.
type Service struct {
	memStore    *Store
	sessionsDir string
	lockPath    string
	minHours    float64
	minSessions int
	logger      *slog.Logger
	goal        string
	prompt      string
	dreamGate   DreamGateConfig

	lock               consolidationLocker
	now                func() time.Time
	countSessionsSince func(time.Time) (int, error)
	workspaceResolver  workspacepkg.RuntimeResolver

	mu         sync.Mutex
	runMu      sync.Mutex
	pending    bool
	priorMtime time.Time
}

type persistedSessionMetadata struct {
	State       string     `json:"state,omitempty"`
	SessionType string     `json:"session_type,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StoppedAt   *time.Time `json:"stopped_at,omitempty"`
}

// NewService constructs a Service with default gate thresholds and prompt.
func NewService(opts ...Option) *Service {
	service := &Service{
		minHours:    defaultMinHours,
		minSessions: defaultMinSessions,
		logger:      slog.Default(),
		goal:        defaultGoal,
		prompt:      ConsolidationPrompt(),
		dreamGate:   defaultDreamGateConfig(),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}

	if service.lock == nil {
		service.lock = NewConsolidationLock(service.lockPath)
	}
	if service.countSessionsSince == nil {
		service.countSessionsSince = service.scanCompletedSessionsSince
	}

	return service
}

// WithMemoryStore wires the memory store used for consolidation-path setup.
func WithMemoryStore(store *Store) Option {
	return func(service *Service) {
		service.memStore = store
		if service.lockPath == "" && store != nil && store.globalDir != "" {
			service.lockPath = filepath.Join(store.globalDir, consolidationLockName)
		}
	}
}

// WithWorkspaceResolver wires workspace resolution for consolidation runs that
// need a workspace root path.
func WithWorkspaceResolver(resolver workspacepkg.RuntimeResolver) Option {
	return func(service *Service) {
		service.workspaceResolver = resolver
	}
}

// WithSessionsDir configures the root directory containing persisted sessions.
func WithSessionsDir(path string) Option {
	return func(service *Service) {
		service.sessionsDir = cleanDirPath(path)
	}
}

// WithLockPath configures the path to the consolidation lock file.
func WithLockPath(path string) Option {
	return func(service *Service) {
		service.lockPath = strings.TrimSpace(path)
	}
}

// WithMinHours overrides the minimum age threshold for the time gate.
func WithMinHours(hours float64) Option {
	return func(service *Service) {
		if hours < 0 {
			hours = 0
		}
		service.minHours = hours
	}
}

// WithMinSessions overrides the completed-session threshold for the session gate.
func WithMinSessions(count int) Option {
	return func(service *Service) {
		if count < 0 {
			count = 0
		}
		service.minSessions = count
	}
}

// WithLogger injects the logger used for gate evaluation and run lifecycle logs.
func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) {
		if logger != nil {
			service.logger = logger
		}
	}
}

// WithDreamGateConfig overrides recall-signal promotion thresholds for dreaming.
func WithDreamGateConfig(config DreamGateConfig) Option {
	return func(service *Service) {
		service.dreamGate = normalizeDreamGateConfig(config)
	}
}

func withGoal(goal string) Option {
	return func(service *Service) {
		if trimmed := strings.TrimSpace(goal); trimmed != "" {
			service.goal = trimmed
		}
	}
}

// ShouldRun evaluates the time and session gates in that order.
func (s *Service) ShouldRun() (bool, error) {
	if err := s.validate(); err != nil {
		return false, err
	}

	lastConsolidatedAt, err := s.lock.LastConsolidatedAt()
	if err != nil {
		return false, err
	}
	if !s.timeGatePasses(lastConsolidatedAt) {
		s.logger.Debug(
			"memory: time gate blocked consolidation",
			"last_consolidated_at", lastConsolidatedAt,
			"min_hours", s.minHours,
		)
		return false, nil
	}

	completedSessions := 0
	if s.minSessions > 0 {
		completedSessions, err = s.countSessionsSince(lastConsolidatedAt)
		if err != nil {
			return false, err
		}
	}
	if completedSessions < s.minSessions {
		s.logger.Debug(
			"memory: session gate blocked consolidation",
			"completed_sessions", completedSessions,
			"min_sessions", s.minSessions,
			"last_consolidated_at", lastConsolidatedAt,
		)
		return false, nil
	}

	s.logger.Debug(
		"memory: consolidation gates passed",
		"completed_sessions", completedSessions,
	)

	return true, nil
}
