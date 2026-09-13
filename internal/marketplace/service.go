package marketplace

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const maxStoredErrorBytes = 1024

const (
	errorClassCanceled = "canceled"
	errorClassNetwork  = "network"
	errorClassTimeout  = "timeout"
)

// CatalogService coordinates TTL freshness and durable projections.
type CatalogService struct {
	store          Store
	source         Source
	ttl            time.Duration
	refreshTimeout time.Duration
	now            func() time.Time
	notifier       Notifier
	flightMu       sync.Mutex
	flight         *refreshFlight
	lifecycleCtx   context.Context
	lifecycleStop  context.CancelFunc
	flightWG       sync.WaitGroup
	closeOnce      sync.Once
	closeDone      chan struct{}
	closed         bool
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

// NewService creates the internal curated catalog service.
func NewService(
	store Store,
	source Source,
	ttl time.Duration,
	refreshTimeout time.Duration,
	options ...ServiceOption,
) (*CatalogService, error) {
	if store == nil {
		return nil, errors.New("marketplace catalog: store is required")
	}
	if ttl <= 0 {
		return nil, errors.New("marketplace catalog: TTL must be positive")
	}
	if refreshTimeout <= 0 {
		return nil, errors.New("marketplace catalog: refresh timeout must be positive")
	}
	if source == nil {
		return nil, errors.New("marketplace catalog: source is required")
	}
	lifecycleCtx, lifecycleStop := context.WithCancel(context.Background())
	service := &CatalogService{
		store:          store,
		source:         source,
		ttl:            ttl,
		refreshTimeout: refreshTimeout,
		now: func() time.Time {
			return time.Now().UTC()
		},
		lifecycleCtx:  lifecycleCtx,
		lifecycleStop: lifecycleStop,
		closeDone:     make(chan struct{}),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

// Browse refreshes stale state on demand, then returns one atomic projection snapshot.
func (s *CatalogService) Browse(ctx context.Context, query string, offset, limit int) (BrowseResult, error) {
	if err := s.checkReady(ctx); err != nil {
		return BrowseResult{}, err
	}
	refreshErr := s.ensureFresh(ctx)
	result, err := s.store.BrowseSource(ctx, CompozyCatalogSource, query, offset, limit)
	if err != nil {
		return BrowseResult{}, errors.Join(refreshErr, err)
	}
	if len(result.Entries) == 0 {
		return result, refreshErr
	}
	return result, nil
}

// Detail refreshes stale state on demand and resolves by immutable entry id.
func (s *CatalogService) Detail(ctx context.Context, entryID string) (*Entry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	refreshErr := s.ensureFresh(ctx)
	entry, getErr := s.store.GetEntry(ctx, CompozyCatalogSource, entryID)
	if getErr != nil {
		return nil, errors.Join(refreshErr, getErr)
	}
	return entry, nil
}

// Refresh force-fetches the curated source even while its TTL is fresh.
func (s *CatalogService) Refresh(ctx context.Context) (RefreshReport, error) {
	if ctx == nil {
		return RefreshReport{}, errors.New("marketplace catalog: refresh context is required")
	}
	if err := s.checkReady(ctx); err != nil {
		return RefreshReport{}, err
	}
	outcome, err := s.withRefreshFlight(ctx)
	if err != nil {
		err = fmt.Errorf("refresh %s: %w", CompozyCatalogSource, err)
	}
	return RefreshReport{Outcomes: []RefreshOutcome{outcome}}, err
}

// Status returns the curated source's persisted freshness state.
func (s *CatalogService) Status(ctx context.Context) ([]SourceState, error) {
	if ctx == nil {
		return nil, errors.New("marketplace catalog: status context is required")
	}
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	state, err := s.store.SourceState(ctx, CompozyCatalogSource)
	if errors.Is(err, ErrSourceStateMissing) {
		return []SourceState{{Source: CompozyCatalogSource}}, nil
	}
	if err != nil {
		return nil, err
	}
	return []SourceState{*state}, nil
}

func (s *CatalogService) checkReady(ctx context.Context) error {
	if ctx == nil {
		return errors.New("marketplace catalog: service context is required")
	}
	if s == nil || s.store == nil || s.source == nil {
		return errors.New("marketplace catalog: service is required")
	}
	return s.lifecycleError()
}
