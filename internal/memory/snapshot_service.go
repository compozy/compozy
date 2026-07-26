package memory

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const defaultSnapshotCacheEntries = 512

// SnapshotService captures prompt-safe memory once per session generation.
type SnapshotService struct {
	store           *Store
	provider        SnapshotProvider
	now             func() time.Time
	maxCharacters   int
	generation      atomic.Uint64
	cacheMu         sync.Mutex
	cache           map[string]*snapshotCacheEntry
	cacheOrder      []string
	maxCacheEntries int
}

type snapshotCacheEntry struct {
	generation uint64
	requestKey string
	ready      chan struct{}
	snapshot   FrozenSnapshot
	err        error
}

// SnapshotServiceOption customizes frozen snapshot capture.
type SnapshotServiceOption func(*SnapshotService)

// WithProviderSnapshotSource installs the active provider snapshot source.
func WithProviderSnapshotSource(provider SnapshotProvider) SnapshotServiceOption {
	return func(service *SnapshotService) {
		if service != nil {
			service.provider = provider
		}
	}
}

// WithSnapshotClock injects a deterministic capture clock.
func WithSnapshotClock(now func() time.Time) SnapshotServiceOption {
	return func(service *SnapshotService) {
		if service != nil && now != nil {
			service.now = now
		}
	}
}

// WithSnapshotMaxCharacters caps the rendered memory prompt section.
func WithSnapshotMaxCharacters(maxCharacters int) SnapshotServiceOption {
	return func(service *SnapshotService) {
		if service != nil && maxCharacters > 0 {
			service.maxCharacters = maxCharacters
		}
	}
}

// NewSnapshotService constructs a frozen memory snapshot service.
func NewSnapshotService(store *Store, opts ...SnapshotServiceOption) *SnapshotService {
	service := &SnapshotService{
		store:           store,
		now:             func() time.Time { return time.Now().UTC() },
		maxCharacters:   defaultSnapshotMaxCharacters,
		cache:           make(map[string]*snapshotCacheEntry),
		maxCacheEntries: defaultSnapshotCacheEntries,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

// InvalidateNextBoot records that future session boots must recapture memory.
func (s *SnapshotService) InvalidateNextBoot() uint64 {
	if s == nil {
		return 0
	}
	generation := s.generation.Add(1)
	s.cacheMu.Lock()
	clear(s.cache)
	s.cacheOrder = nil
	s.cacheMu.Unlock()
	return generation + s.storeMutationRevision()
}

// Capture freezes prompt-safe memory for the supplied session boot request.
func (s *SnapshotService) Capture(ctx context.Context, req PromptSnapshotRequest) (FrozenSnapshot, error) {
	if s == nil {
		return FrozenSnapshot{}, nil
	}
	if err := contextErr(ctx); err != nil {
		return FrozenSnapshot{}, err
	}
	req = normalizeSnapshotRequest(req)
	if req.ParentSnapshot != nil {
		return s.InheritForSubAgent(*req.ParentSnapshot, req), nil
	}
	generation := s.currentGeneration()
	if req.SessionID == "" {
		return s.captureFresh(ctx, req, generation)
	}

	entry, capture := s.snapshotCacheEntry(req, generation)
	if !capture {
		select {
		case <-ctx.Done():
			return FrozenSnapshot{}, ctx.Err()
		case <-entry.ready:
			return entry.snapshot.Clone(), entry.err
		}
	}

	snapshot, err := s.captureFresh(ctx, req, generation)
	s.completeSnapshotCacheEntry(req.SessionID, entry, snapshot, err)
	return snapshot.Clone(), err
}

// InheritForSubAgent clones a parent snapshot without re-resolving memory state.
func (s *SnapshotService) InheritForSubAgent(parent FrozenSnapshot, req PromptSnapshotRequest) FrozenSnapshot {
	clone := parent.Clone()
	clone.SessionID = strings.TrimSpace(req.SessionID)
	clone.AgentName = firstSnapshotValue(req.AgentName, parent.AgentName)
	clone.WorkspaceID = firstSnapshotValue(req.WorkspaceID, parent.WorkspaceID)
	clone.WorkspaceRoot = firstSnapshotValue(req.WorkspaceRoot, parent.WorkspaceRoot)
	clone.ControllerMode = SnapshotControllerReadOnly
	clone.InheritedFrom = parent.ID
	if s != nil {
		clone.Generation = s.currentGeneration()
	}
	clone.ID = snapshotID(clone)
	return clone
}

// Clone returns a deep copy of the frozen snapshot.
func (s FrozenSnapshot) Clone() FrozenSnapshot {
	clone := s
	clone.Blocks = append([]SnapshotBlock(nil), s.Blocks...)
	return clone
}

func (s *SnapshotService) captureFresh(
	ctx context.Context,
	req PromptSnapshotRequest,
	generation uint64,
) (FrozenSnapshot, error) {
	capturedAt := s.now().UTC()
	blocks, err := s.captureBlocks(ctx, req, capturedAt)
	if err != nil {
		return FrozenSnapshot{}, err
	}
	header := snapshotHeader(blocks)
	snapshot := FrozenSnapshot{
		SessionID:      req.SessionID,
		WorkspaceID:    req.WorkspaceID,
		WorkspaceRoot:  req.WorkspaceRoot,
		AgentName:      req.AgentName,
		CapturedAt:     capturedAt,
		Generation:     generation,
		ControllerMode: controllerModeForSession(req.SessionType),
		Blocks:         blocks,
		Header:         header,
	}
	snapshot.ID = snapshotID(snapshot)
	snapshot.Section = renderMemorySnapshot(snapshot, s.maxCharacters)
	return snapshot, nil
}

func (s *SnapshotService) snapshotCacheEntry(
	req PromptSnapshotRequest,
	generation uint64,
) (*snapshotCacheEntry, bool) {
	requestKey := snapshotRequestCacheKey(req)
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if existing := s.cache[req.SessionID]; existing != nil &&
		existing.generation == generation && existing.requestKey == requestKey {
		return existing, false
	}
	s.removeSnapshotCacheKey(req.SessionID)
	entry := &snapshotCacheEntry{generation: generation, requestKey: requestKey, ready: make(chan struct{})}
	s.cache[req.SessionID] = entry
	s.cacheOrder = append(s.cacheOrder, req.SessionID)
	s.evictSnapshotCacheEntries()
	return entry, true
}

func (s *SnapshotService) completeSnapshotCacheEntry(
	sessionID string,
	entry *snapshotCacheEntry,
	snapshot FrozenSnapshot,
	err error,
) {
	s.cacheMu.Lock()
	entry.snapshot = snapshot.Clone()
	entry.err = err
	close(entry.ready)
	if err != nil && s.cache[sessionID] == entry {
		delete(s.cache, sessionID)
		s.removeSnapshotCacheKey(sessionID)
	}
	s.cacheMu.Unlock()
}

func (s *SnapshotService) evictSnapshotCacheEntries() {
	for len(s.cacheOrder) > s.maxCacheEntries {
		oldest := s.cacheOrder[0]
		s.cacheOrder = s.cacheOrder[1:]
		delete(s.cache, oldest)
	}
}

func (s *SnapshotService) removeSnapshotCacheKey(sessionID string) {
	for index, key := range s.cacheOrder {
		if key == sessionID {
			s.cacheOrder = append(s.cacheOrder[:index], s.cacheOrder[index+1:]...)
			return
		}
	}
}

func (s *SnapshotService) currentGeneration() uint64 {
	if s == nil {
		return 0
	}
	return s.generation.Load() + s.storeMutationRevision()
}

func (s *SnapshotService) storeMutationRevision() uint64 {
	if s == nil || s.store == nil {
		return 0
	}
	return s.store.committedMutationRevision()
}

func snapshotRequestCacheKey(req PromptSnapshotRequest) string {
	return strings.Join([]string{
		req.WorkspaceID,
		req.WorkspaceRoot,
		req.AgentName,
		string(req.SessionType),
	}, "\x00")
}
