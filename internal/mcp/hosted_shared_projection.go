package mcp

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

type hostedVisibilityKey struct {
	registry *tools.RuntimeRegistry
	class    string
}

type hostedSharedFlightKey struct {
	hostedVisibilityKey
	epoch uint64
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
		epoch := s.currentProjectionEpoch()
		class, known := registry.ProjectionVisibility(ctx, record.scope())
		if !known {
			return HostedProjectionResponse{}, false, nil
		}
		key := hostedSharedFlightKey{hostedVisibilityKey{registry, class}, epoch}
		cached, flight, leader := s.sharedProjections.begin(key, s.now(), s.StreamRefreshInterval())
		if cached != nil {
			if !s.sharedProjectionMatches(ctx, record, key) {
				continue
			}
			return s.checkedSharedProjection(ctx, record, *cached)
		}
		if !leader {
			response, retry, err := waitHostedProjectionFlight(ctx, flight)
			if ctx.Err() == nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				continue
			}
			if err != nil {
				return HostedProjectionResponse{}, true, err
			}
			if retry || !s.sharedProjectionMatches(ctx, record, key) {
				continue
			}
			return s.checkedSharedProjection(ctx, record, response)
		}
		response, err := buildHostedProjection(ctx, registry, record)
		if err == nil {
			err = ctx.Err()
		}
		stable := false
		if err == nil {
			stable = s.sharedProjectionMatches(ctx, record, key)
		}
		s.mu.Lock()
		owned := s.ownsBindRecord(record)
		s.sharedProjections.finish(key, response, err, stable, owned, s.now())
		s.mu.Unlock()
		if err != nil {
			return HostedProjectionResponse{}, true, err
		}
		if !owned {
			return HostedProjectionResponse{}, true, ErrHostedBindNotFound
		}
		if stable {
			return s.checkedSharedProjection(ctx, record, response)
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
	return known && class == key.class && s.currentProjectionEpoch() == key.epoch
}

func (s *HostedService) checkedSharedProjection(
	ctx context.Context,
	record *hostedBindRecord,
	response HostedProjectionResponse,
) (HostedProjectionResponse, bool, error) {
	if err := ctx.Err(); err != nil {
		return HostedProjectionResponse{}, true, err
	}
	s.mu.Lock()
	owned := s.ownsBindRecord(record)
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
) (*HostedProjectionResponse, *hostedProjectionFlight, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[hostedVisibilityKey]hostedUnversionedProjection)
		c.flights = make(map[hostedSharedFlightKey]*hostedProjectionFlight)
	}
	for class, entry := range c.entries {
		if entry.epoch != key.epoch || now.Before(entry.builtAt) || now.Sub(entry.builtAt) >= refresh {
			delete(c.entries, class)
		}
	}
	if entry, ok := c.entries[key.hostedVisibilityKey]; ok {
		return &entry.response, nil, false
	}
	if flight := c.flights[key]; flight != nil {
		return nil, flight, false
	}
	flight := &hostedProjectionFlight{done: make(chan struct{})}
	c.flights[key] = flight
	return nil, flight, true
}

func (c *hostedSharedProjectionCache) finish(
	key hostedSharedFlightKey,
	response HostedProjectionResponse,
	err error,
	stable, owned bool,
	now time.Time,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if stable && owned {
		c.entries[key.hostedVisibilityKey] = hostedUnversionedProjection{
			epoch: key.epoch, builtAt: now, response: response,
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
