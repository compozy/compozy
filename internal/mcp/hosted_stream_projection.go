package mcp

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

// HostedProjectionFallbackRefresh bounds how long a projection stream waits without a change
// signal. Some native availability inputs (session and service state) carry no epoch, so streams
// still re-list at this slow cadence; epoch advances rebuild immediately.
const HostedProjectionFallbackRefresh = 3 * time.Second

// hostedUnversionedStreamRefresh bounds how often a stream rebuilds a projection that has no
// authoritative generation while the daemon epoch stays unchanged.
const hostedUnversionedStreamRefresh = HostedProjectionFallbackRefresh

type hostedUnversionedProjection struct {
	epoch    uint64
	builtAt  time.Time
	response HostedProjectionResponse
}

// HostedProjectionChanges carries signals closed when a bound projection may have changed.
// A nil channel never fires.
type HostedProjectionChanges struct {
	Epoch    <-chan struct{}
	Released <-chan struct{}
}

// StreamRefreshInterval returns the projection stream fallback re-read interval.
func (s *HostedService) StreamRefreshInterval() time.Duration {
	if s == nil || s.streamRefresh <= 0 {
		return hostedUnversionedStreamRefresh
	}
	return s.streamRefresh
}

// ProjectionChanges captures the current change signals. Capture them before reading a
// projection so a change racing that read still wakes the stream.
func (s *HostedService) ProjectionChanges() HostedProjectionChanges {
	if s == nil {
		return HostedProjectionChanges{}
	}
	changes := HostedProjectionChanges{Released: s.bindReleases.wait()}
	if s.projectionChanges != nil {
		changes.Epoch = s.projectionChanges()
	}
	return changes
}

// StreamProjection omits unchanged tools and isolates changed views from the shared cache.
func (s *HostedService) StreamProjection(
	ctx context.Context,
	bindID string,
	peer PeerInfo,
	lastDigest string,
) (HostedProjectionResponse, error) {
	record, err := s.recordForBind(ctx, bindID, peer)
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	s.acknowledgeHostedBind(record)
	// A fresh unversioned entry exists only after the generation was unknown; serving it first
	// skips repeating that lookup inside the refresh window.
	if cached, ok := s.projectionCache.loadUnversioned(
		record.bindID, s.currentProjectionEpoch(), s.now(), s.StreamRefreshInterval(),
	); ok {
		return hostedStreamProjectionAfter(cached, lastDigest), nil
	}
	registry := s.currentRegistry()
	if registry == nil {
		return HostedProjectionResponse{}, ErrHostedRegistryRequired
	}
	unversioned := func(
		ctx context.Context,
		registry tools.Registry,
		record *hostedBindRecord,
	) (HostedProjectionResponse, error) {
		return s.boundedUnversionedProjection(ctx, registry, record)
	}
	response, err := s.projectionForGeneration(ctx, record, registry, unversioned)
	return hostedStreamProjectionAfter(response, lastDigest), err
}

func hostedStreamProjectionAfter(response HostedProjectionResponse, lastDigest string) HostedProjectionResponse {
	if lastDigest != "" && response.Digest == lastDigest {
		return HostedProjectionResponse{Digest: response.Digest}
	}
	return cloneHostedProjectionResponse(response)
}

func (s *HostedService) boundedUnversionedProjection(
	ctx context.Context,
	registry tools.Registry,
	record *hostedBindRecord,
) (HostedProjectionResponse, error) {
	if runtime, ok := registry.(*tools.RuntimeRegistry); ok {
		response, shared, err := s.sharedStreamProjection(ctx, runtime, record)
		if shared || err != nil {
			return response, err
		}
	}
	epoch := s.currentProjectionEpoch()
	if cached, ok := s.projectionCache.loadUnversioned(
		record.bindID, epoch, s.now(), s.StreamRefreshInterval(),
	); ok {
		return cached, nil
	}
	var response HostedProjectionResponse
	for attempt := 0; ; attempt++ {
		var err error
		response, err = buildHostedProjection(ctx, registry, record)
		if err != nil {
			return HostedProjectionResponse{}, err
		}
		// An epoch advance during the build may have invalidated the listed views.
		after := s.currentProjectionEpoch()
		if after == epoch {
			break
		}
		if attempt+1 >= hostedProjectionGenerationRetries {
			// Keep the fresher build but leave it uncached so the next read rebuilds.
			return response, nil
		}
		epoch = after
	}
	s.mu.Lock()
	if s.ownsBindRecord(record) {
		s.projectionCache.storeUnversioned(record.bindID, hostedUnversionedProjection{
			epoch: epoch, builtAt: s.now(), response: response,
		})
	}
	s.mu.Unlock()
	return response, nil
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

// releaseProjectionCaches drops released binds' snapshots and wakes their streams so they observe
// the release instead of waiting for the fallback refresh.
func (s *HostedService) releaseProjectionCaches(bindIDs []string) {
	if len(bindIDs) == 0 {
		return
	}
	for _, bindID := range bindIDs {
		s.projectionCache.remove(bindID)
	}
	s.sharedProjections.clear()
	s.bindReleases.notify(nil)
}

// loadUnversioned returns the shared snapshot built for the same bind and epoch inside the
// refresh window. Run identity is not a key: session tool availability does not depend on it.
func (c *hostedProjectionCache) loadUnversioned(
	bindID string,
	epoch uint64,
	now time.Time,
	refresh time.Duration,
) (HostedProjectionResponse, bool) {
	c.mu.RLock()
	entry, ok := c.unversioned[bindID]
	c.mu.RUnlock()
	if !ok || entry.epoch != epoch || now.Sub(entry.builtAt) >= refresh || now.Before(entry.builtAt) {
		return HostedProjectionResponse{}, false
	}
	return entry.response, true
}

func (c *hostedProjectionCache) storeUnversioned(bindID string, entry hostedUnversionedProjection) {
	c.mu.Lock()
	c.unversioned[bindID] = entry
	c.mu.Unlock()
}
