package mcp

import (
	"context"
	"sync"

	"github.com/compozy/compozy/internal/tools"
)

const hostedProjectionGenerationRetries = 3

type hostedProjectionCache struct {
	mu      sync.Mutex
	entries map[string]hostedProjectionCacheEntry
	flights map[hostedProjectionFlightKey]*hostedProjectionFlight
}

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
		entries: make(map[string]hostedProjectionCacheEntry),
		flights: make(map[hostedProjectionFlightKey]*hostedProjectionFlight),
	}
}

func (s *HostedService) projectionForGeneration(
	ctx context.Context,
	record *hostedBindRecord,
	registry tools.Registry,
) (HostedProjectionResponse, error) {
	if s.projectionGeneration == nil {
		return buildHostedProjection(ctx, registry, record.scope())
	}

	scope := record.scope()
	for range hostedProjectionGenerationRetries {
		generation, known := s.projectionGeneration(ctx, scope)
		if !known {
			return buildHostedProjection(ctx, registry, scope)
		}
		if cached, ok := s.projectionCache.load(record.bindID, generation); ok {
			return cached, nil
		}

		key := hostedProjectionFlightKey{bindID: record.bindID, generation: generation}
		flight, leader := s.projectionCache.begin(key)
		if !leader {
			response, retry, err := waitHostedProjectionFlight(ctx, flight)
			if retry {
				continue
			}
			return response, err
		}

		response, err := buildHostedProjection(ctx, registry, scope)
		if err != nil {
			s.projectionCache.finish(key, HostedProjectionResponse{}, err, false, false)
			return HostedProjectionResponse{}, err
		}
		after, stillKnown := s.projectionGeneration(ctx, scope)
		stable := stillKnown && after == generation
		stored := s.finishProjectionGeneration(key, record, response, stable)
		if stable {
			if stored {
				return cloneHostedProjectionResponse(response), nil
			}
			return HostedProjectionResponse{}, ErrHostedBindNotFound
		}
	}
	return buildHostedProjection(ctx, registry, scope)
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
	active := s.binds[key.bindID]
	owned := active != nil && record != nil && active.sessionID == record.sessionID &&
		active.peer.PID == record.peer.PID && active.peer.UID == record.peer.UID
	s.projectionCache.finish(key, response, nil, false, owned)
	s.mu.Unlock()
	return owned
}

func buildHostedProjection(
	ctx context.Context,
	registry tools.Registry,
	scope tools.Scope,
) (HostedProjectionResponse, error) {
	views, err := registry.List(ctx, scope)
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	return hostedProjectionResponse(views), nil
}

func (c *hostedProjectionCache) load(bindID string, generation string) (HostedProjectionResponse, bool) {
	if c == nil {
		return HostedProjectionResponse{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[bindID]
	if !ok || entry.generation != generation {
		return HostedProjectionResponse{}, false
	}
	return cloneHostedProjectionResponse(entry.response), true
}

func (c *hostedProjectionCache) begin(key hostedProjectionFlightKey) (*hostedProjectionFlight, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
			response:   cloneHostedProjectionResponse(response),
		}
	}
	if flight != nil {
		flight.response = cloneHostedProjectionResponse(response)
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
	c.mu.Unlock()
}

func (c *hostedProjectionCache) size() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func waitHostedProjectionFlight(
	ctx context.Context,
	flight *hostedProjectionFlight,
) (HostedProjectionResponse, bool, error) {
	select {
	case <-ctx.Done():
		return HostedProjectionResponse{}, false, ctx.Err()
	case <-flight.done:
		return cloneHostedProjectionResponse(flight.response), flight.retry, flight.err
	}
}

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
