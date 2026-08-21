package profile

import (
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const defaultProfileID = "00000000000000000000000000"

type Event struct {
	Name, ProfileID, ProfileName, OperationID, Error string
}

type EventRecorder interface{ RecordProfileEvent(Event) }

type Manager struct {
	store      *globaldb.GlobalDB
	home       compozyconfig.HomePaths
	now        func() time.Time
	entropy    io.Reader
	logger     *slog.Logger
	events     EventRecorder
	selections *selectionStore
	opMu       sync.Mutex
}

type Option func(*Manager) error

func WithStore(store *globaldb.GlobalDB) Option {
	return func(m *Manager) error {
		if store == nil || store.DB() == nil {
			return errors.New("profile: global database is required")
		}
		m.store = store
		return nil
	}
}

func WithHomePaths(paths compozyconfig.HomePaths) Option {
	return func(m *Manager) error {
		if strings.TrimSpace(paths.ProfilesDir) == "" {
			return errors.New("profile: profiles directory is required")
		}
		m.home = paths
		return nil
	}
}

func WithClock(now func() time.Time) Option {
	return func(m *Manager) error {
		if now == nil {
			return errors.New("profile: clock is required")
		}
		m.now = now
		return nil
	}
}

func WithEntropy(entropy io.Reader) Option {
	return func(m *Manager) error {
		if entropy == nil {
			return errors.New("profile: entropy source is required")
		}
		m.entropy = entropy
		return nil
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) error {
		if logger == nil {
			return errors.New("profile: logger is required")
		}
		m.logger = logger
		return nil
	}
}

func WithEventRecorder(recorder EventRecorder) Option {
	return func(m *Manager) error {
		m.events = recorder
		return nil
	}
}

func NewManager(opts ...Option) (*Manager, error) {
	m := &Manager{
		now:     func() time.Time { return time.Now().UTC() },
		entropy: rand.Reader,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(m); err != nil {
				return nil, err
			}
		}
	}
	if m.store == nil || m.store.DB() == nil {
		return nil, errors.New("profile: global database is required")
	}
	if strings.TrimSpace(m.home.ProfilesDir) == "" {
		return nil, errors.New("profile: profiles directory is required")
	}
	m.selections = &selectionStore{manager: m}
	return m, nil
}

func (m *Manager) SelectionStore() SelectionStore { return m.selections }
