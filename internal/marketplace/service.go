package marketplace

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

const maxStoredErrorBytes = 1024

const (
	errorClassCanceled = "canceled"
	errorClassNetwork  = "network"
	errorClassTimeout  = "timeout"
)

// CatalogService coordinates TTL freshness and durable projections.
type CatalogService struct {
	store              Store
	ttl                time.Duration
	refreshTimeout     time.Duration
	now                func() time.Time
	notifier           Notifier
	packageCache       *pluginsource.PackageCache
	installedPackages  func(context.Context) ([]InstalledPackage, error)
	logger             *slog.Logger
	sourceMu           sync.RWMutex
	sources            []*registeredSource
	byName             map[string]*registeredSource
	generation         int64
	flightMu           sync.Mutex
	completedRefreshes uint64
	lifecycleCtx       context.Context
	lifecycleStop      context.CancelFunc
	flightWG           sync.WaitGroup
	closeOnce          sync.Once
	closeDone          chan struct{}
	closed             bool
}

var _ Service = (*CatalogService)(nil)

// ServiceOption customizes service time and observability seams.
type ServiceOption func(*CatalogService)

// WithNow injects the UTC clock used for freshness decisions.
func WithNow(now func() time.Time) ServiceOption {
	return func(service *CatalogService) {
		if service != nil && now != nil {
			service.now = now
		}
	}
}

// WithNotifier attaches the failure-isolated canonical refresh notifier.
func WithNotifier(notifier Notifier) ServiceOption {
	return func(service *CatalogService) {
		if service != nil {
			service.notifier = notifier
		}
	}
}

// NewService owns configured source projections and independently bounded refresh flights.
func NewService(
	ctx context.Context, store Store, sources []SourceBinding,
	ttl, refreshTimeout time.Duration, options ...ServiceOption,
) (*CatalogService, error) {
	if ctx == nil {
		return nil, errors.New("marketplace catalog: context is required")
	}
	if store == nil {
		return nil, errors.New("marketplace catalog: store is required")
	}
	if ttl <= 0 {
		return nil, errors.New("marketplace catalog: TTL must be positive")
	}
	if refreshTimeout <= 0 {
		return nil, errors.New("marketplace catalog: refresh timeout must be positive")
	}
	lifecycleCtx, lifecycleStop := context.WithCancel(context.Background())
	service := &CatalogService{store: store, ttl: ttl, refreshTimeout: refreshTimeout,
		now: func() time.Time { return time.Now().UTC() }, lifecycleCtx: lifecycleCtx,
		lifecycleStop: lifecycleStop, closeDone: make(chan struct{}),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	if err := service.SetSources(ctx, sources); err != nil {
		lifecycleStop()
		return nil, err
	}
	return service, nil
}

// Browse reads a complete source snapshot before scheduling any stale refreshes.
func (s *CatalogService) Browse(ctx context.Context, query string, offset, limit int) (BrowseResult, error) {
	if err := s.checkReady(ctx); err != nil {
		return BrowseResult{}, err
	}
	s.sourceMu.RLock()
	defer s.sourceMu.RUnlock()
	names := make([]string, 0, len(s.sources))
	for _, source := range s.sources {
		names = append(names, source.binding.Config.Name)
	}
	s.flightMu.Lock()
	completed := s.completedRefreshes
	s.flightMu.Unlock()
	page, err := s.store.BrowseSources(ctx, names, query, offset, limit)
	if err != nil {
		return BrowseResult{}, err
	}
	for i := range page.Sources {
		state := &page.Sources[i]
		if !state.Enabled {
			continue
		}
		state.Stale = s.sourceStale(*state)
		if state.Stale {
			page.Stale = true
			if page.ErrorClass == "" {
				page.ErrorClass, page.LastError = state.ErrorClass, state.LastError
			}
			if _, err := s.startRefreshFlight(s.byName[state.Source], false); err != nil {
				return BrowseResult{}, err
			}
		}
	}
	for index := range page.Entries {
		if err := s.projectPackageAvailability(ctx, &page.Entries[index]); err != nil {
			return BrowseResult{}, err
		}
	}
	page.Refreshing = s.refreshingSince(completed)
	return page, nil
}

// Detail resolves an entry from its currently enabled source without remote I/O.
func (s *CatalogService) Detail(ctx context.Context, source, entryID string) (*Entry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	s.sourceMu.RLock()
	defer s.sourceMu.RUnlock()
	for _, registered := range s.sources {
		config := registered.binding.Config
		if !config.Enabled || (source != "" && config.Name != source) {
			continue
		}
		entry, err := s.store.GetEntry(ctx, config.Name, entryID)
		if errors.Is(err, ErrEntryNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := s.projectPackageAvailability(ctx, entry); err != nil {
			return nil, err
		}
		return entry, nil
	}
	return nil, ErrEntryNotFound
}

// Entry joins installed provenance to the current source name by immutable origin.
func (s *CatalogService) Entry(ctx context.Context, origin Origin) (*Entry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	s.sourceMu.RLock()
	defer s.sourceMu.RUnlock()
	for _, source := range s.sources {
		if source.binding.Config.Enabled && source.binding.Config.Ref == origin.SourceRef {
			entry, err := s.store.GetEntry(ctx, source.binding.Config.Name, origin.EntryID)
			if errors.Is(err, ErrEntryNotFound) {
				continue
			}
			return entry, err
		}
	}
	return nil, ErrEntryNotFound
}

func (s *CatalogService) Status(ctx context.Context) ([]SourceState, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	s.sourceMu.RLock()
	defer s.sourceMu.RUnlock()
	states := make([]SourceState, 0, len(s.sources))
	for _, source := range s.sources {
		state, err := s.store.SourceState(ctx, source.binding.Config.Name)
		if err != nil {
			return nil, err
		}
		state.Stale = s.sourceStale(*state)
		states = append(states, *state)
	}
	return states, nil
}

func (s *CatalogService) sourceStale(state SourceState) bool {
	return state.Stale || state.FetchedAt.IsZero() || !state.FetchedAt.Add(s.ttl).After(s.now().UTC())
}

func (s *CatalogService) checkReady(ctx context.Context) error {
	if ctx == nil {
		return errors.New("marketplace catalog: service context is required")
	}
	if s == nil || s.store == nil {
		return errors.New("marketplace catalog: service is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.lifecycleError()
}
