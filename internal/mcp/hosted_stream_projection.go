package mcp

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

// hostedUnversionedStreamRefresh bounds how often a stream rebuilds a projection that has no
// authoritative generation. Native tool availability carries no generation, so without this bound
// every bound session re-listed the whole registry on each stream poll.
const hostedUnversionedStreamRefresh = time.Second

type hostedUnversionedProjection struct {
	epoch    uint64
	builtAt  time.Time
	response HostedProjectionResponse
}

// StreamProjection serves projection-stream polls. Authoritative generations reuse the per-bind
// cache; an unversioned projection, and its generation lookup, run at most once per refresh
// interval unless the daemon projection epoch advanced. Projection stays fresh for one-shot reads.
func (s *HostedService) StreamProjection(
	ctx context.Context,
	bindID string,
	peer PeerInfo,
) (HostedProjectionResponse, error) {
	record, err := s.recordForBind(ctx, bindID, peer)
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	// A fresh unversioned entry exists only after the generation was unknown; serving it first
	// skips repeating that lookup inside the refresh window.
	if cached, ok := s.projectionCache.loadUnversioned(
		record.bindID, s.currentProjectionEpoch(), s.now(),
	); ok {
		return cached, nil
	}
	registry := s.currentRegistry()
	if registry == nil {
		return HostedProjectionResponse{}, ErrHostedRegistryRequired
	}
	return s.projectionForGeneration(ctx, record, registry, s.boundedUnversionedProjection)
}

func (s *HostedService) boundedUnversionedProjection(
	ctx context.Context,
	registry tools.Registry,
	record *hostedBindRecord,
) (HostedProjectionResponse, error) {
	epoch := s.currentProjectionEpoch()
	now := s.now()
	if cached, ok := s.projectionCache.loadUnversioned(record.bindID, epoch, now); ok {
		return cached, nil
	}
	response, err := buildHostedProjection(ctx, registry, record)
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	s.mu.Lock()
	if s.ownsBindRecord(record) {
		s.projectionCache.storeUnversioned(record.bindID, hostedUnversionedProjection{
			epoch: epoch, builtAt: now, response: response,
		})
	}
	s.mu.Unlock()
	return cloneHostedProjectionResponse(response), nil
}

func (s *HostedService) currentProjectionEpoch() uint64 {
	if s.projectionEpoch == nil {
		return 0
	}
	return s.projectionEpoch()
}

// ownsBindRecord reports whether record is still the active bind; callers hold s.mu.
func (s *HostedService) ownsBindRecord(record *hostedBindRecord) bool {
	active := s.binds[record.bindID]
	return active != nil && active.sessionID == record.sessionID &&
		active.peer.PID == record.peer.PID && active.peer.UID == record.peer.UID
}

func (c *hostedProjectionCache) loadUnversioned(
	bindID string,
	epoch uint64,
	now time.Time,
) (HostedProjectionResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.unversioned[bindID]
	if !ok || entry.epoch != epoch || now.Sub(entry.builtAt) >= hostedUnversionedStreamRefresh ||
		now.Before(entry.builtAt) {
		return HostedProjectionResponse{}, false
	}
	return cloneHostedProjectionResponse(entry.response), true
}

func (c *hostedProjectionCache) storeUnversioned(bindID string, entry hostedUnversionedProjection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry.response = cloneHostedProjectionResponse(entry.response)
	c.unversioned[bindID] = entry
}
