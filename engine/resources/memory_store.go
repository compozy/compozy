package resources

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// MemoryResourceStore is an in-memory implementation of ResourceStore.
// It is safe for concurrent use and intended for dev/tests.
type MemoryResourceStore struct {
	mu       sync.RWMutex
	items    map[ResourceKey]storedEntry
	watchers map[string][]*watcher
	closed   bool
}

type storedEntry struct {
	value any
	etag  ETag
}

type watcher struct {
	ch     chan Event
	closed bool
}

const defaultWatchBuffer = 256

// NewMemoryResourceStore constructs a new MemoryResourceStore.
func NewMemoryResourceStore() *MemoryResourceStore {
	return &MemoryResourceStore{items: make(map[ResourceKey]storedEntry), watchers: make(map[string][]*watcher)}
}

// Put inserts or replaces a resource value at the given key and broadcasts an event.
func (s *MemoryResourceStore) Put(ctx context.Context, key ResourceKey, value any) (ETag, error) {
	if err := ctx.Err(); err != nil {
		return ETag(""), fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if value == nil {
		return ETag(""), fmt.Errorf("nil value is not allowed")
	}
	cp, err := core.DeepCopy[any](value)
	if err != nil {
		return ETag(""), fmt.Errorf("deep copy failed: %w", err)
	}
	etag := ETag(core.ETagFromAny(cp))
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ETag(""), fmt.Errorf("store is closed")
	}
	s.items[key] = storedEntry{value: cp, etag: etag}
	// Broadcast while holding the lock to prevent concurrent channel close.
	evt := Event{Type: EventPut, Key: key, ETag: etag, At: time.Now().UTC()}
	keyspace := watcherKeyspace(key.Project, key.Type)
	for _, w := range s.watchers[keyspace] {
		if w.closed {
			continue
		}
		select {
		case w.ch <- evt:
		default:
			log.Warn(
				"watch channel full; dropping event",
				"project", key.Project,
				"type", string(key.Type),
				"id", key.ID,
			)
		}
	}
	s.mu.Unlock()
	return etag, nil
}

// PutIfMatch updates a resource only when the provided ETag matches the current value.
func (s *MemoryResourceStore) PutIfMatch(
	ctx context.Context,
	key ResourceKey,
	value any,
	expectedETag ETag,
) (ETag, error) {
	if err := ctx.Err(); err != nil {
		return ETag(""), fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if value == nil {
		return ETag(""), fmt.Errorf("nil value is not allowed")
	}
	cp, err := core.DeepCopy[any](value)
	if err != nil {
		return ETag(""), fmt.Errorf("deep copy failed: %w", err)
	}
	etag := ETag(core.ETagFromAny(cp))
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ETag(""), fmt.Errorf("store is closed")
	}
	entry, ok := s.items[key]
	if !ok {
		if expectedETag != "" {
			s.mu.Unlock()
			return ETag(""), ErrNotFound
		}
		s.items[key] = storedEntry{value: cp, etag: etag}
	} else {
		if entry.etag != expectedETag {
			s.mu.Unlock()
			return ETag(""), ErrETagMismatch
		}
		s.items[key] = storedEntry{value: cp, etag: etag}
	}
	evt := Event{Type: EventPut, Key: key, ETag: etag, At: time.Now().UTC()}
	keyspace := watcherKeyspace(key.Project, key.Type)
	for _, w := range s.watchers[keyspace] {
		if w.closed {
			continue
		}
		select {
		case w.ch <- evt:
		default:
			log.Warn(
				"watch channel full; dropping event",
				"project", key.Project,
				"type", string(key.Type),
				"id", key.ID,
			)
		}
	}
	s.mu.Unlock()
	return etag, nil
}

// Get retrieves a resource value by key, returning a deep copy.
func (s *MemoryResourceStore) Get(ctx context.Context, key ResourceKey) (any, ETag, error) {
	if err := ctx.Err(); err != nil {
		return nil, ETag(""), fmt.Errorf("context canceled: %w", err)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ETag(""), fmt.Errorf("store is closed")
	}
	entry, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ETag(""), ErrNotFound
	}
	cp, err := core.DeepCopy[any](entry.value)
	if err != nil {
		return nil, ETag(""), fmt.Errorf("deep copy failed: %w", err)
	}
	return cp, entry.etag, nil
}

// Delete removes a resource by key and broadcasts an event if existed.
func (s *MemoryResourceStore) Delete(ctx context.Context, key ResourceKey) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	existed := false
	var etag ETag
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("store is closed")
	}
	if entry, ok := s.items[key]; ok {
		existed = true
		etag = entry.etag
		delete(s.items, key)
	}
	// Broadcast while holding the lock to prevent concurrent channel close.
	if existed {
		evt := Event{Type: EventDelete, Key: key, ETag: etag, At: time.Now().UTC()}
		keyspace := watcherKeyspace(key.Project, key.Type)
		for _, w := range s.watchers[keyspace] {
			if w.closed {
				continue
			}
			select {
			case w.ch <- evt:
			default:
				log.Warn(
					"watch channel full; dropping event",
					"project", key.Project,
					"type", string(key.Type),
					"id", key.ID,
				)
			}
		}
	}
	s.mu.Unlock()
	return nil
}

// List returns keys for a project and type.
func (s *MemoryResourceStore) List(ctx context.Context, project string, typ ResourceType) ([]ResourceKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	keys := make([]ResourceKey, 0, len(s.items))
	for k := range s.items {
		if k.Project == project && k.Type == typ {
			keys = append(keys, k)
		}
	}
	s.mu.RUnlock()
	return keys, nil
}

// Watch subscribes to events for project and type. It primes the subscriber with current PUTs.
func (s *MemoryResourceStore) Watch(ctx context.Context, project string, typ ResourceType) (<-chan Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	keyspace := watcherKeyspace(project, typ)
	ch := make(chan Event, defaultWatchBuffer)
	w := &watcher{ch: ch}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("store is closed")
	}
	s.watchers[keyspace] = append(s.watchers[keyspace], w)
	for k, entry := range s.items {
		if k.Project == project && k.Type == typ {
			evt := Event{Type: EventPut, Key: k, ETag: entry.etag, At: time.Now().UTC()}
			select {
			case ch <- evt:
			default:
				log.Warn(
					"watch channel full during prime; dropping event",
					"project",
					project,
					"type",
					string(typ),
					"id",
					k.ID,
				)
			}
		}
	}
	s.mu.Unlock()
	go func() { <-ctx.Done(); s.removeWatcher(project, typ, w) }()
	return ch, nil
}

// Close releases resources and closes all watcher channels.
func (s *MemoryResourceStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for _, list := range s.watchers {
		for _, w := range list {
			if !w.closed {
				close(w.ch)
				w.closed = true
			}
		}
	}
	s.watchers = make(map[string][]*watcher)
	s.items = make(map[ResourceKey]storedEntry)
	s.mu.Unlock()
	return nil
}

func (s *MemoryResourceStore) removeWatcher(project string, typ ResourceType, target *watcher) {
	s.mu.Lock()
	keyspace := watcherKeyspace(project, typ)
	list := s.watchers[keyspace]
	idx := -1
	for i, w := range list {
		if w == target {
			idx = i
			break
		}
	}
	if idx >= 0 {
		tail := append([]*watcher(nil), list[:idx]...)
		tail = append(tail, list[idx+1:]...)
		if len(tail) == 0 {
			delete(s.watchers, keyspace)
		} else {
			s.watchers[keyspace] = tail
		}
	}
	if !target.closed {
		close(target.ch)
		target.closed = true
	}
	s.mu.Unlock()
}

// copyWatchersLocked was removed; broadcasting now occurs while the store lock is held

func watcherKeyspace(project string, typ ResourceType) string { return project + "|" + string(typ) }

// ListWithValues returns copies of keys, values and etags for project/type.
func (s *MemoryResourceStore) ListWithValues(
	ctx context.Context,
	project string,
	typ ResourceType,
) ([]StoredItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	out := make([]StoredItem, 0)
	for k, e := range s.items {
		if k.Project == project && k.Type == typ {
			cp, err := core.DeepCopy[any](e.value)
			if err != nil {
				s.mu.RUnlock()
				return nil, fmt.Errorf("deep copy failed: %w", err)
			}
			out = append(out, StoredItem{Key: k, Value: cp, ETag: e.etag})
		}
	}
	s.mu.RUnlock()
	return out, nil
}

// ListWithValuesPage returns a page of items and the total count.
func (s *MemoryResourceStore) ListWithValuesPage(
	ctx context.Context,
	project string,
	typ ResourceType,
	offset, limit int,
) ([]StoredItem, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("context canceled: %w", err)
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		// max int portable expression
		limit = int(^uint(0) >> 1)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, 0, fmt.Errorf("store is closed")
	}
	total := 0
	end := offset + limit
	out := make([]StoredItem, 0)
	for k, e := range s.items {
		if k.Project != project || k.Type != typ {
			continue
		}
		if total >= offset && total < end {
			cp, err := core.DeepCopy[any](e.value)
			if err != nil {
				s.mu.RUnlock()
				return nil, 0, fmt.Errorf("deep copy failed: %w", err)
			}
			out = append(out, StoredItem{Key: k, Value: cp, ETag: e.etag})
		}
		total++
	}
	s.mu.RUnlock()
	if offset > total {
		return []StoredItem{}, total, nil
	}
	return out, total, nil
}

// removed: local deepCopy/ETag/writeStableJSON in favor of core.DeepCopy and core.ETagFromAny
