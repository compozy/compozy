package mcp

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

const (
	hostedSharedProjectionRetention = time.Minute
	hostedSharedProjectionCapacity  = 128
)

type hostedVisibilityKey struct {
	registry *tools.RuntimeRegistry
	class    string
}

type hostedSharedFlightKey struct {
	hostedVisibilityKey
	fence   hostedProjectionFence
	builtAt time.Time
}

type hostedSharedProjectionCache struct {
	mu      sync.Mutex
	entries map[hostedVisibilityKey]hostedUnversionedProjection
	flights map[hostedSharedFlightKey]*hostedProjectionFlight
}

func (s *HostedService) sharedStreamProjection(
	ctx context.Context,
	registry *tools.RuntimeRegistry,
	record *hostedBindRecord,
) (HostedProjectionResponse, bool, error) {
	for range hostedProjectionGenerationRetries {
		if err := ctx.Err(); err != nil {
			return HostedProjectionResponse{}, true, err
		}
		fence := s.projectionFence()
		listingCtx, builtAt := s.withProjectionListings(
			registry.ReadContext(tools.WithImmutableProjection(ctx)), fence.epoch,
		)
		class, known := registry.ProjectionVisibility(listingCtx, record.scope())
		if !known {
			return HostedProjectionResponse{}, false, nil
		}
		key := hostedSharedFlightKey{hostedVisibilityKey{registry, class}, fence, builtAt}
		matchCtx := ctx
		var epochBacked bool
		if s.projectionEpoch != nil {
			// Class and build share one availability snapshot; the epoch fences all versioned inputs.
			matchCtx = listingCtx
			epochBacked = hostedCatalogHasEpoch(ctx, registry) && ctx.Value(hostedProjectionObservationKey{}) == nil
		}
		cached, flight, leader := s.sharedProjections.begin(key, s.now(), s.StreamRefreshInterval(), epochBacked)
		if cached != nil {
			if !s.sharedProjectionMatches(matchCtx, record, key) {
				continue
			}
			return s.checkedSharedProjection(ctx, record, cached.response, fence, cached.builtAt)
		}
		if !leader {
			response, retry, err := waitHostedProjectionFlight(ctx, flight)
			if ctx.Err() == nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				continue
			}
			if err != nil {
				return HostedProjectionResponse{}, true, err
			}
			if retry || !s.sharedProjectionMatches(matchCtx, record, key) {
				continue
			}
			return s.checkedSharedProjection(ctx, record, response, fence, key.builtAt)
		}
		buildRecord := record.clone()
		buildRecord.digestMemo = &hostedProjectionDigestMemo{
			views: flight.previous.Tools, digest: flight.previous.Digest,
		}
		response, err := buildHostedProjection(listingCtx, registry, buildRecord)
		if err == nil {
			err = ctx.Err()
		}
		stable := false
		if err == nil {
			stable = s.sharedProjectionMatches(matchCtx, record, key)
		}
		s.mu.Lock()
		owned := s.ownsBindRecord(record)
		s.sharedProjections.finish(key, response, err, stable, owned)
		s.mu.Unlock()
		if err != nil {
			return HostedProjectionResponse{}, true, err
		}
		if !owned {
			return HostedProjectionResponse{}, true, ErrHostedBindNotFound
		}
		if stable {
			return s.checkedSharedProjection(ctx, record, response, fence, builtAt)
		}
	}
	// Churning inputs keep the existing uncached path rather than publishing an uncertain class.
	return HostedProjectionResponse{}, false, nil
}

func (s *HostedService) sharedProjectionMatches(
	ctx context.Context,
	record *hostedBindRecord,
	key hostedSharedFlightKey,
) bool {
	class, known := key.registry.ProjectionVisibility(ctx, record.scope())
	return known && class == key.class && s.projectionFence() == key.fence
}

func (s *HostedService) checkedSharedProjection(
	ctx context.Context,
	record *hostedBindRecord,
	response HostedProjectionResponse,
	fence hostedProjectionFence, builtAt time.Time,
) (HostedProjectionResponse, bool, error) {
	if err := ctx.Err(); err != nil {
		return HostedProjectionResponse{}, true, err
	}
	s.mu.Lock()
	owned := s.ownsBindRecord(record)
	if owned && s.projectionEpoch != nil && s.projectionFence() == fence &&
		ctx.Value(hostedProjectionObservationKey{}) == nil {
		s.projectionCache.storeUnversioned(record.bindID, hostedUnversionedProjection{
			fence: fence, builtAt: builtAt, response: response,
		})
	}
	s.mu.Unlock()
	if !owned {
		return HostedProjectionResponse{}, true, ErrHostedBindNotFound
	}
	return response, true, nil
}

func (c *hostedSharedProjectionCache) begin(
	key hostedSharedFlightKey,
	now time.Time,
	refresh time.Duration,
	epochBacked bool,
) (*hostedUnversionedProjection, *hostedProjectionFlight, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[hostedVisibilityKey]hostedUnversionedProjection)
		c.flights = make(map[hostedSharedFlightKey]*hostedProjectionFlight)
	}
	for class, entry := range c.entries {
		if entry.fence != key.fence || now.Before(entry.builtAt) ||
			(!epochBacked && now.Sub(entry.builtAt) >= max(refresh, hostedSharedProjectionRetention)) {
			delete(c.entries, class)
		}
	}
	entry, found := c.entries[key.hostedVisibilityKey]
	if found && (epochBacked || now.Sub(entry.builtAt) < refresh) {
		return &entry, nil, false
	}
	if flight := c.flights[key]; flight != nil {
		return nil, flight, false
	}
	flight := &hostedProjectionFlight{done: make(chan struct{}), previous: entry.response}
	c.flights[key] = flight
	return nil, flight, true
}

func (c *hostedSharedProjectionCache) finish(
	key hostedSharedFlightKey,
	response HostedProjectionResponse,
	err error,
	stable, owned bool,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous := c.entries[key.hostedVisibilityKey]
	if stable && owned && !previous.builtAt.After(key.builtAt) {
		if previous.builtAt.IsZero() && len(c.entries) >= hostedSharedProjectionCapacity {
			var oldest hostedVisibilityKey
			var builtAt time.Time
			for class, entry := range c.entries {
				if builtAt.IsZero() || entry.builtAt.Before(builtAt) {
					oldest, builtAt = class, entry.builtAt
				}
			}
			delete(c.entries, oldest)
		}
		c.entries[key.hostedVisibilityKey] = hostedUnversionedProjection{
			fence: key.fence, builtAt: key.builtAt, response: response,
		}
	}
	flight := c.flights[key]
	flight.response, flight.err, flight.retry = response, err, err == nil && (!stable || !owned)
	delete(c.flights, key)
	close(flight.done)
}

func (c *hostedSharedProjectionCache) clear() {
	c.mu.Lock()
	clear(c.entries)
	c.mu.Unlock()
}

// hostedProjectionListings rotates provider listings per epoch and refresh window.
type hostedProjectionListings struct {
	mu        sync.Mutex
	epoch     uint64
	startedAt time.Time
	listings  *tools.ProjectionListings
}

// withProjectionListings shares provider listings among unversioned builds of the same epoch. It
// returns the share's start as the build's freshness origin: a projection cached from a shared
// listing expires with that listing's window, so staleness stays within one refresh window.
func (s *HostedService) withProjectionListings(ctx context.Context, epoch uint64) (context.Context, time.Time) {
	listings, startedAt := s.projectionListings.current(epoch, s.now(), s.StreamRefreshInterval())
	return tools.WithProjectionListings(ctx, listings), startedAt
}

func (l *hostedProjectionListings) current(
	epoch uint64,
	now time.Time,
	refresh time.Duration,
) (*tools.ProjectionListings, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.listings != nil && epoch < l.epoch {
		// A build that read an older epoch is discarded by its own epoch check; keep it private.
		return tools.NewProjectionListings(), now
	}
	if l.listings == nil || epoch != l.epoch || now.Before(l.startedAt) || now.Sub(l.startedAt) >= refresh {
		l.listings, l.epoch, l.startedAt = tools.NewProjectionListings(), epoch, now
	}
	return l.listings, l.startedAt
}

func (l *hostedProjectionListings) clear() {
	l.mu.Lock()
	l.listings = nil
	l.mu.Unlock()
}

func hostedCatalogHasEpoch(ctx context.Context, registry *tools.RuntimeRegistry) bool {
	return tools.IsHostedCatalogRead(ctx) && registry.HostedCatalogEpochCoversProviders()
}
