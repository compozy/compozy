package marketplace

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	sources        map[Kind]Source
	ttl            time.Duration
	refreshTimeout time.Duration
	now            func() time.Time
	notifier       Notifier
	flightMu       sync.Mutex
	flights        map[Kind]*refreshFlight
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
	sources []Source,
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
	sourceByKind := make(map[Kind]Source, len(sources))
	for _, source := range sources {
		if source == nil {
			return nil, errors.New("marketplace catalog: source is required")
		}
		kind := source.Kind()
		if _, err := kindFilename(kind); err != nil {
			return nil, err
		}
		if _, exists := sourceByKind[kind]; exists {
			return nil, fmt.Errorf("marketplace catalog: source for %q is registered more than once", kind)
		}
		sourceByKind[kind] = source
	}
	if len(sourceByKind) == 0 {
		return nil, errors.New("marketplace catalog: at least one source is required")
	}
	lifecycleCtx, lifecycleStop := context.WithCancel(context.Background())
	service := &CatalogService{
		store:          store,
		sources:        sourceByKind,
		ttl:            ttl,
		refreshTimeout: refreshTimeout,
		now: func() time.Time {
			return time.Now().UTC()
		},
		flights:       make(map[Kind]*refreshFlight),
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

// Browse refreshes stale state on demand, then returns the durable projection.
func (s *CatalogService) Browse(
	ctx context.Context,
	kind Kind,
	query string,
	offset int,
	limit int,
) (BrowseResult, error) {
	if err := s.checkReady(ctx, kind); err != nil {
		return BrowseResult{}, err
	}
	refreshErr := s.ensureFresh(ctx, kind)
	page, listErr := s.store.ListKind(ctx, kind, query, offset, limit)
	if listErr != nil {
		return BrowseResult{}, errors.Join(refreshErr, listErr)
	}
	state, stateErr := s.store.KindState(ctx, kind)
	if stateErr != nil {
		return BrowseResult{}, errors.Join(refreshErr, stateErr)
	}
	if refreshErr != nil && len(page.Entries) == 0 {
		return BrowseResult{}, refreshErr
	}
	return BrowseResult{Entries: page.Entries, Total: page.Total, State: *state}, nil
}

// Detail refreshes stale state on demand and resolves by immutable entry id.
func (s *CatalogService) Detail(ctx context.Context, kind Kind, entryID string) (*Entry, error) {
	if err := s.checkReady(ctx, kind); err != nil {
		return nil, err
	}
	refreshErr := s.ensureFresh(ctx, kind)
	entry, getErr := s.store.GetEntry(ctx, kind, entryID)
	if getErr != nil {
		return nil, errors.Join(refreshErr, getErr)
	}
	return entry, nil
}

// Refresh force-fetches selected kinds even when their TTL is still fresh.
func (s *CatalogService) Refresh(ctx context.Context, kinds ...Kind) (RefreshReport, error) {
	if ctx == nil {
		return RefreshReport{}, errors.New("marketplace catalog: refresh context is required")
	}
	if err := s.lifecycleError(); err != nil {
		return RefreshReport{}, err
	}
	selected, err := s.selectKinds(kinds)
	if err != nil {
		return RefreshReport{}, err
	}
	report := RefreshReport{Outcomes: make([]RefreshOutcome, 0, len(selected))}
	refreshErrors := make([]error, 0)
	for _, kind := range selected {
		outcome, refreshErr := s.withRefreshFlight(ctx, kind)
		report.Outcomes = append(report.Outcomes, outcome)
		if refreshErr != nil {
			refreshErrors = append(refreshErrors, fmt.Errorf("refresh %s: %w", kind, refreshErr))
		}
	}
	return report, errors.Join(refreshErrors...)
}

// Status returns deterministic state for every configured kind.
func (s *CatalogService) Status(ctx context.Context) ([]KindState, error) {
	if ctx == nil {
		return nil, errors.New("marketplace catalog: status context is required")
	}
	if err := s.lifecycleError(); err != nil {
		return nil, err
	}
	kinds, err := s.selectKinds(nil)
	if err != nil {
		return nil, err
	}
	states := make([]KindState, 0, len(kinds))
	for _, kind := range kinds {
		state, stateErr := s.store.KindState(ctx, kind)
		if errors.Is(stateErr, ErrKindStateMissing) {
			states = append(states, KindState{Kind: kind})
			continue
		}
		if stateErr != nil {
			return nil, stateErr
		}
		states = append(states, *state)
	}
	return states, nil
}

func (s *CatalogService) checkReady(ctx context.Context, kind Kind) error {
	if ctx == nil {
		return errors.New("marketplace catalog: service context is required")
	}
	if s == nil || s.store == nil {
		return errors.New("marketplace catalog: service is required")
	}
	if err := s.lifecycleError(); err != nil {
		return err
	}
	if _, ok := s.sources[kind]; !ok {
		if _, err := kindFilename(kind); err != nil {
			return err
		}
		return fmt.Errorf("marketplace catalog: source for %q is not configured", kind)
	}
	return nil
}

func (s *CatalogService) selectKinds(requested []Kind) ([]Kind, error) {
	if s == nil || len(s.sources) == 0 {
		return nil, errors.New("marketplace catalog: service sources are required")
	}
	if err := s.lifecycleError(); err != nil {
		return nil, err
	}
	if len(requested) == 0 {
		kinds := make([]Kind, 0, len(s.sources))
		for kind := range s.sources {
			kinds = append(kinds, kind)
		}
		slices.Sort(kinds)
		return kinds, nil
	}
	seen := make(map[Kind]struct{}, len(requested))
	kinds := make([]Kind, 0, len(requested))
	for _, kind := range requested {
		if _, err := kindFilename(kind); err != nil {
			return nil, err
		}
		if _, ok := s.sources[kind]; !ok {
			return nil, fmt.Errorf("marketplace catalog: source for %q is not configured", kind)
		}
		if _, exists := seen[kind]; exists {
			continue
		}
		seen[kind] = struct{}{}
		kinds = append(kinds, kind)
	}
	slices.Sort(kinds)
	return kinds, nil
}
