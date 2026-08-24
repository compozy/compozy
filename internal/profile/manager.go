package profile

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/vault"
)

const defaultProfileID = "00000000000000000000000000"

// Event is the profile lifecycle payload persisted as an event summary and
// streamed to clients over the logs stream. The tags are load-bearing: the
// marshaled form lands in the public `content` field of every log event, so it
// follows the same snake_case shape as the rest of that wire.
type Event struct {
	Name                string `json:"name"`
	ProfileID           string `json:"profile_id"`
	ProfileName         string `json:"profile_name"`
	PreviousProfileName string `json:"previous_profile_name,omitempty"`
	OperationID         string `json:"operation_id,omitempty"`
	Error               string `json:"error,omitempty"`
}

type EventRecorder interface{ RecordProfileEvent(Event) }

// PlacementCatalog reports extension resources that bind to one profile name.
type PlacementCatalog interface {
	PlacementsForProfile(context.Context, string) ([]PlacementRef, error)
}

// DesktopPartitionCatalog reports and removes the window arrangements one profile
// owns across every workspace. Desktops live outside the catalog database, so the
// count feeds the delete preview and the removal runs as a journaled finalize step.
type DesktopPartitionCatalog interface {
	CountDesktopPartitions(ctx context.Context, profileID string) (int, error)
	PurgeDesktopPartitions(ctx context.Context, profileID string) error
}

type Manager struct {
	store      *globaldb.GlobalDB
	home       compozyconfig.HomePaths
	now        func() time.Time
	entropy    io.Reader
	logger     *slog.Logger
	events     EventRecorder
	placements PlacementCatalog
	desktops   DesktopPartitionCatalog
	selections *selectionStore
	vaultRefs  *vault.ProfileRefRewriter
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

func WithPlacementCatalog(catalog PlacementCatalog) Option {
	return func(m *Manager) error {
		m.placements = catalog
		return nil
	}
}

func WithDesktopPartitionCatalog(catalog DesktopPartitionCatalog) Option {
	return func(m *Manager) error {
		m.desktops = catalog
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
	vaultRefs, err := vault.NewProfileRefRewriter(vault.NewFileKeyProvider(m.home.HomeDir, nil))
	if err != nil {
		return nil, fmt.Errorf("profile: create vault ref rewriter: %w", err)
	}
	m.vaultRefs = vaultRefs
	m.selections = &selectionStore{manager: m}
	return m, nil
}

func (m *Manager) SelectionStore() SelectionStore { return m.selections }
