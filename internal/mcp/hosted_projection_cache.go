package mcp

import (
	"context"
	"sync"

	"github.com/compozy/compozy/internal/tools"
)

const hostedProjectionGenerationRetries = 3

// hostedProjectionCache holds immutable projection snapshots. Entries and flight results are
// shared with every reader, so no path may mutate a response after it enters the cache.
type hostedProjectionCache struct {
	mu          sync.RWMutex
	entries     map[string]hostedProjectionCacheEntry
	flights     map[hostedProjectionFlightKey]*hostedProjectionFlight
	unversioned map[string]hostedUnversionedProjection
}

// hostedProjectionBuilder produces a projection when no authoritative generation can key the cache.
type hostedProjectionBuilder func(context.Context, tools.Registry, *hostedBindRecord) (HostedProjectionResponse, error)

type hostedProjectionCacheEntry struct {
	generation string
	response   HostedProjectionResponse
}

type hostedProjectionFlightKey struct {
	bindID     string
	generation string
}

type hostedProjectionFlight struct {
	done     chan struct{}
	response HostedProjectionResponse
	err      error
	retry    bool
}

func newHostedProjectionCache() hostedProjectionCache {
	return hostedProjectionCache{
		entries:     make(map[string]hostedProjectionCacheEntry),
		flights:     make(map[hostedProjectionFlightKey]*hostedProjectionFlight),
		unversioned: make(map[string]hostedUnversionedProjection),
	}
}

// projectionForGeneration coalesces known generations and rebuilds views whenever generation authority is absent.
// The response may be a cached snapshot shared with other readers; callers must not mutate it.
func (s *HostedService) projectionForGeneration(
	ctx context.Context,
	record *hostedBindRecord,
	registry tools.Registry,
	unversioned hostedProjectionBuilder,
) (HostedProjectionResponse, error) {
	if s.projectionGeneration == nil {
		return unversioned(ctx, registry, record)
	}

	scope := record.scope()
	for range hostedProjectionGenerationRetries {
		generation, known := s.projectionGeneration(ctx, scope)
		if !known {
			return unversioned(ctx, registry, record)
		}
		key := hostedProjectionFlightKey{bindID: record.bindID, generation: generation}
		if cached, ok := s.projectionCache.load(key); ok {
			return cached, nil
		}

		flight, leader := s.projectionCache.begin(key)
		if !leader {
			response, retry, err := waitHostedProjectionFlight(ctx, flight)
			if retry {
				continue
			}
			return response, err
		}

		response, err := buildHostedProjection(ctx, registry, record)
		if err != nil {
			s.projectionCache.finish(key, HostedProjectionResponse{}, err, false, false)
			return HostedProjectionResponse{}, err
		}
		after, stillKnown := s.projectionGeneration(ctx, scope)
		stable := stillKnown && after == generation
		stored := s.finishProjectionGeneration(key, record, response, stable)
		if stable {
			if stored {
				return response, nil
			}
			return HostedProjectionResponse{}, ErrHostedBindNotFound
		}
	}
	return buildHostedProjection(ctx, registry, record)
}

func (s *HostedService) finishProjectionGeneration(
	key hostedProjectionFlightKey,
	record *hostedBindRecord,
	response HostedProjectionResponse,
	stable bool,
) bool {
	if !stable {
		s.projectionCache.finish(key, response, nil, true, false)
		return false
	}
	s.mu.Lock()
	owned := record != nil && s.ownsBindRecord(record)
	s.projectionCache.finish(key, response, nil, false, owned)
	s.mu.Unlock()
	return owned
}

// buildHostedProjection lists current policy-filtered views before applying binding-local digest reuse.
func buildHostedProjection(
	ctx context.Context,
	registry tools.Registry,
	record *hostedBindRecord,
) (HostedProjectionResponse, error) {
	views, err := registry.List(ctx, record.scope())
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	return hostedProjectionResponse(views, record.digestMemo), nil
}

// load returns the shared immutable snapshot cached for the exact bind generation.
func (c *hostedProjectionCache) load(key hostedProjectionFlightKey) (HostedProjectionResponse, bool) {
	if c == nil {
		return HostedProjectionResponse{}, false
	}
	c.mu.RLock()
	entry, ok := c.entries[key.bindID]
	c.mu.RUnlock()
	if !ok || entry.generation != key.generation {
		return HostedProjectionResponse{}, false
	}
	return entry.response, true
}

func (c *hostedProjectionCache) begin(key hostedProjectionFlightKey) (*hostedProjectionFlight, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// A builder can publish between the caller's cache lookup and acquiring this lock.
	if entry, ok := c.entries[key.bindID]; ok && entry.generation == key.generation {
		flight := &hostedProjectionFlight{done: make(chan struct{}), response: entry.response}
		close(flight.done)
		return flight, false
	}
	if flight := c.flights[key]; flight != nil {
		return flight, false
	}
	flight := &hostedProjectionFlight{done: make(chan struct{})}
	c.flights[key] = flight
	return flight, true
}

func (c *hostedProjectionCache) finish(
	key hostedProjectionFlightKey,
	response HostedProjectionResponse,
	err error,
	retry bool,
	store bool,
) {
	c.mu.Lock()
	flight := c.flights[key]
	if store {
		c.entries[key.bindID] = hostedProjectionCacheEntry{
			generation: key.generation,
			response:   response,
		}
	}
	if flight != nil {
		flight.response = response
		flight.err = err
		flight.retry = retry
		delete(c.flights, key)
		close(flight.done)
	}
	c.mu.Unlock()
}

func (c *hostedProjectionCache) remove(bindID string) {
	if c == nil || bindID == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, bindID)
	delete(c.unversioned, bindID)
	c.mu.Unlock()
}

func (c *hostedProjectionCache) size() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries) + len(c.unversioned)
}

func waitHostedProjectionFlight(
	ctx context.Context,
	flight *hostedProjectionFlight,
) (HostedProjectionResponse, bool, error) {
	select {
	case <-ctx.Done():
		return HostedProjectionResponse{}, false, ctx.Err()
	case <-flight.done:
		return flight.response, flight.retry, flight.err
	}
}

// cloneHostedProjectionResponse detaches a shared snapshot before it reaches a caller that may mutate it.
func cloneHostedProjectionResponse(response HostedProjectionResponse) HostedProjectionResponse {
	return HostedProjectionResponse{
		Tools:  cloneToolViews(response.Tools),
		Digest: response.Digest,
	}
}

func (s *HostedService) projectionCacheSize() int {
	if s == nil {
		return 0
	}
	return s.projectionCache.size()
}
