package mcp

import (
	"context"
	"crypto/sha256"
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
	fence       hostedProjectionFence
	builtAt     time.Time
	response    HostedProjectionResponse
	observation *tools.ProjectionObservation
}

type hostedProjectionFence struct {
	registry  *tools.RuntimeRegistry
	epoch     uint64
	redaction [sha256.Size]byte
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

// StreamSnapshot returns the complete stream-cached catalog without changing bind readiness.
func (s *HostedService) StreamSnapshot(
	ctx context.Context,
	bindID string,
	peer PeerInfo,
) (HostedProjectionResponse, error) {
	record, err := s.recordForBind(ctx, bindID, peer)
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	response, err := s.streamProjection(ctx, record)
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	return cloneHostedProjectionResponse(response), nil
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
	// An eager bind already delivered a complete projection. Its first stream is
	// a receipt, not another cold discovery pass racing the launch deadline.
	if record.eagerTools && s.acknowledgeHostedBind(record) {
		return HostedProjectionResponse{Digest: lastDigest}, nil
	}
	response, err := s.streamProjection(ctx, record)
	if err != nil {
		s.failHostedProjection(record)
	} else {
		s.acknowledgeHostedBind(record)
	}
	return hostedStreamProjectionAfter(response, lastDigest), err
}

func (s *HostedService) streamProjection(
	ctx context.Context,
	record *hostedBindRecord,
) (HostedProjectionResponse, error) {
	fence := s.projectionFence()
	now := s.now()
	if cached, ok := s.projectionCache.loadUnversioned(record.bindID, fence, now, s.StreamRefreshInterval()); ok {
		return cached, nil
	}
	registry := s.currentRegistry()
	if registry == nil {
		return HostedProjectionResponse{}, ErrHostedRegistryRequired
	}
	runtime, ok := registry.(*tools.RuntimeRegistry)
	if !ok || s.projectionEpoch == nil {
		return s.projectionForGeneration(ctx, record, registry, s.boundedUnversionedProjection)
	}
	projectionCtx, _ := runtime.HostedProjectionContext(ctx)
	listingCtx, builtAt := s.withProjectionListings(projectionCtx, fence.epoch)
	observation, known := runtime.ObserveProjection(listingCtx, record.scope())
	if !known {
		return s.streamProjectionWithoutObservation(ctx, projectionCtx, runtime, record)
	}
	cached := s.projectionCache.unversionedEntry(record.bindID)
	response := cached.response
	reused := cached.fence == fence && cached.observation != nil && *cached.observation == observation &&
		!now.Before(cached.builtAt) && s.projectionFence() == fence
	if !reused {
		built, stable, err := s.buildObservedStreamProjection(projectionCtx, listingCtx, runtime, record, observation)
		if err != nil || !stable {
			return built, err
		}
		response = built
	}
	if err := ctx.Err(); err != nil {
		return HostedProjectionResponse{}, err
	}
	current, owned := s.storeObservedProjection(record, hostedUnversionedProjection{
		fence: fence, builtAt: builtAt, response: response, observation: &observation,
	})
	if !owned {
		return HostedProjectionResponse{}, ErrHostedBindNotFound
	}
	if reused && !current {
		// Redaction or the epoch moved while the cached snapshot was being reused.
		registry := s.currentRegistry()
		if registry == nil {
			return HostedProjectionResponse{}, ErrHostedRegistryRequired
		}
		return s.projectionForGeneration(ctx, record, registry, s.boundedUnversionedProjection)
	}
	return response, nil
}

func (s *HostedService) buildObservedStreamProjection(
	ctx, listingCtx context.Context, runtime *tools.RuntimeRegistry, record *hostedBindRecord,
	observation tools.ProjectionObservation,
) (HostedProjectionResponse, bool, error) {
	scope := record.scope()
	buildCtx, stable := runtime.BeginProjectionBuild(ctx, scope, observation)
	buildCtx = runtime.FreshReadContext(buildCtx)
	buildCtx, complete, err := runtime.PinProjectionInputs(buildCtx, listingCtx, scope)
	if err != nil {
		response, err := s.streamProjectionWithoutObservation(ctx, ctx, runtime, record)
		return response, false, err
	}
	// A bounded response or flight may have been built without observing these inputs.
	response, err := catalogProjectionWithRedactionCheck(buildCtx, runtime, record, buildHostedProjection)
	if err != nil || !complete {
		return response, false, err
	}
	external, err := runtime.ExternalProjectionFingerprint(buildCtx, scope)
	if err != nil || !observation.MatchesExternal(external) {
		return response, false, nil
	}
	after, known := runtime.ObserveProjection(listingCtx, scope)
	return response, known && after == observation && stable(), nil
}

func (s *HostedService) streamProjectionWithoutObservation(
	ctx, projectionCtx context.Context, registry *tools.RuntimeRegistry, record *hostedBindRecord,
) (HostedProjectionResponse, error) {
	if !registry.HasProjectionPolicyObserver() {
		return s.projectionForGeneration(ctx, record, registry, s.boundedUnversionedProjection)
	}
	buildCtx := registry.FreshReadContext(projectionCtx)
	if tools.IsHostedCatalogRead(buildCtx) {
		return catalogProjectionWithRedactionCheck(buildCtx, registry, record, buildHostedProjection)
	}
	return projectionWithRedactionCheck(buildCtx, registry, record, buildHostedProjection)
}

func (s *HostedService) storeObservedProjection(
	record *hostedBindRecord, snapshot hostedUnversionedProjection,
) (current, owned bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ownsBindRecord(record) {
		return false, false
	}
	if s.projectionFence() != snapshot.fence {
		return false, true
	}
	if previous := s.projectionCache.unversionedEntry(record.bindID); !previous.builtAt.After(snapshot.builtAt) {
		s.projectionCache.storeUnversioned(record.bindID, snapshot)
	}
	return true, true
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
	fence := s.projectionFence()
	if cached, ok := s.projectionCache.loadUnversioned(
		record.bindID, fence, s.now(), s.StreamRefreshInterval(),
	); ok {
		return cached, nil
	}
	var response HostedProjectionResponse
	var builtAt time.Time
	for attempt := 0; ; attempt++ {
		var listingCtx context.Context
		var err error
		listingCtx, builtAt = s.withProjectionListings(ctx, fence.epoch)
		response, err = buildHostedProjection(listingCtx, registry, record)
		if err != nil {
			return HostedProjectionResponse{}, err
		}
		after := s.projectionFence()
		if after == fence {
			break
		}
		if attempt+1 >= hostedProjectionGenerationRetries {
			// Keep the fresher build but leave it uncached so the next read rebuilds.
			return response, nil
		}
		fence = after
	}
	s.mu.Lock()
	if s.ownsBindRecord(record) {
		s.projectionCache.storeUnversioned(record.bindID, hostedUnversionedProjection{
			fence: fence, builtAt: builtAt, response: response,
		})
	}
	s.mu.Unlock()
	return response, nil
}

func (s *HostedService) projectionFence() hostedProjectionFence {
	registry := s.currentRegistry()
	fence := hostedProjectionFence{redaction: projectionRedactionRevision(registry)}
	if runtime, ok := registry.(*tools.RuntimeRegistry); ok {
		fence.registry = runtime
	}
	if s.projectionEpoch != nil {
		fence.epoch = s.projectionEpoch()
	}
	return fence
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
	s.projectionListings.clear()
	s.bindReleases.notify(nil)
}

// loadUnversioned returns the shared snapshot built for the same bind and fence inside the
// refresh window. Run identity is not a key: session tool availability does not depend on it.
func (c *hostedProjectionCache) loadUnversioned(
	bindID string,
	fence hostedProjectionFence,
	now time.Time,
	refresh time.Duration,
) (HostedProjectionResponse, bool) {
	entry := c.unversionedEntry(bindID)
	if entry.builtAt.IsZero() || entry.fence != fence || now.Sub(entry.builtAt) >= refresh ||
		now.Before(entry.builtAt) {
		return HostedProjectionResponse{}, false
	}
	return entry.response, true
}

func (c *hostedProjectionCache) unversionedEntry(bindID string) hostedUnversionedProjection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.unversioned[bindID]
}

func (c *hostedProjectionCache) storeUnversioned(bindID string, entry hostedUnversionedProjection) {
	c.mu.Lock()
	c.unversioned[bindID] = entry
	c.mu.Unlock()
}
