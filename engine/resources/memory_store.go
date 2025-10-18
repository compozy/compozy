package resources

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	resmetrics "github.com/compozy/compozy/engine/resources/metrics"
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
	mu     sync.Mutex
	ch     chan Event
	closed bool
}

const defaultWatchBuffer = 256

func (w *watcher) trySend(evt *Event) (bool, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return false, false
	}
	select {
	case w.ch <- *evt:
		return true, false
	default:
		return false, true
	}
}

func (w *watcher) closeChannel() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	close(w.ch)
}

func cloneWatchers(list []*watcher) []*watcher {
	if len(list) == 0 {
		return nil
	}
	cloned := make([]*watcher, len(list))
	copy(cloned, list)
	return cloned
}

func notifyWatchers(log logger.Logger, watchers []*watcher, evt *Event) {
	for _, w := range watchers {
		if w == nil {
			continue
		}
		if _, dropped := w.trySend(evt); dropped {
			log.Warn(
				"watch channel full; dropping event",
				"project", evt.Key.Project,
				"type", string(evt.Key.Type),
				"id", evt.Key.ID,
			)
		}
	}
}

// NewMemoryResourceStore constructs a new MemoryResourceStore.
func NewMemoryResourceStore() *MemoryResourceStore {
	return &MemoryResourceStore{items: make(map[ResourceKey]storedEntry), watchers: make(map[string][]*watcher)}
}

// Put inserts or replaces a resource value at the given key and broadcasts an event.
func (s *MemoryResourceStore) Put(ctx context.Context, key ResourceKey, value any) (etag ETag, err error) {
	start := time.Now()
	defer func() { recordStoreOperation(ctx, start, "put", key.Type, err) }()
	if err = ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if value == nil {
		return "", fmt.Errorf("nil value is not allowed")
	}
	cp, copyErr := core.DeepCopy[any](value)
	if copyErr != nil {
		err = fmt.Errorf("deep copy failed: %w", copyErr)
		return "", err
	}
	etag = ETag(core.ETagFromAny(cp))
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", fmt.Errorf("store is closed")
	}
	_, existed := s.items[key]
	s.items[key] = storedEntry{value: cp, etag: etag}
	evt := Event{Type: EventPut, Key: key, ETag: etag, At: time.Now().UTC()}
	watchers := cloneWatchers(s.watchers[watcherKeyspace(key.Project, key.Type)])
	s.mu.Unlock()
	if !existed {
		resmetrics.AdjustStoreSize(string(key.Type), 1)
	}
	notifyWatchers(log, watchers, &evt)
	return etag, nil
}

// PutIfMatch updates a resource only when the provided ETag matches the current value.
func (s *MemoryResourceStore) PutIfMatch(
	ctx context.Context,
	key ResourceKey,
	value any,
	expectedETag ETag,
) (etag ETag, err error) {
	start := time.Now()
	defer func() { recordStoreOperation(ctx, start, "put", key.Type, err) }()
	if err = ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if value == nil {
		return "", fmt.Errorf("nil value is not allowed")
	}
	cp, copyErr := core.DeepCopy[any](value)
	if copyErr != nil {
		err = fmt.Errorf("deep copy failed: %w", copyErr)
		return "", err
	}
	etag = ETag(core.ETagFromAny(cp))
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", fmt.Errorf("store is closed")
	}
	entry, ok := s.items[key]
	created := false
	if !ok {
		if expectedETag != "" {
			s.mu.Unlock()
			err = ErrNotFound
			return "", err
		}
		s.items[key] = storedEntry{value: cp, etag: etag}
		created = true
	} else {
		if entry.etag != expectedETag {
			s.mu.Unlock()
			resmetrics.RecordETagMismatch(ctx, string(key.Type))
			err = ErrETagMismatch
			return "", err
		}
		s.items[key] = storedEntry{value: cp, etag: etag}
	}
	evt := Event{Type: EventPut, Key: key, ETag: etag, At: time.Now().UTC()}
	watchers := cloneWatchers(s.watchers[watcherKeyspace(key.Project, key.Type)])
	s.mu.Unlock()
	if created {
		resmetrics.AdjustStoreSize(string(key.Type), 1)
	}
	notifyWatchers(log, watchers, &evt)
	return etag, nil
}

// Get retrieves a resource value by key, returning a deep copy.
func (s *MemoryResourceStore) Get(ctx context.Context, key ResourceKey) (value any, etag ETag, err error) {
	start := time.Now()
	defer func() { recordStoreOperation(ctx, start, "get", key.Type, err) }()
	if err = ctx.Err(); err != nil {
		return nil, "", fmt.Errorf("context canceled: %w", err)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, "", fmt.Errorf("store is closed")
	}
	entry, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		err = ErrNotFound
		return nil, "", err
	}
	value, err = core.DeepCopy[any](entry.value)
	if err != nil {
		return nil, "", fmt.Errorf("deep copy failed: %w", err)
	}
	return value, entry.etag, nil
}

// Delete removes a resource by key and broadcasts an event if existed.
func (s *MemoryResourceStore) Delete(ctx context.Context, key ResourceKey) (err error) {
	start := time.Now()
	defer func() { recordStoreOperation(ctx, start, "delete", key.Type, err) }()
	if err = ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	existed := false
	var etag ETag
	var watchers []*watcher
	var evt Event
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("store is closed")
	}
	if entry, ok := s.items[key]; ok {
		existed = true
		etag = entry.etag
		delete(s.items, key)
		watchers = cloneWatchers(s.watchers[watcherKeyspace(key.Project, key.Type)])
		evt = Event{Type: EventDelete, Key: key, ETag: etag, At: time.Now().UTC()}
	}
	s.mu.Unlock()
	if existed {
		resmetrics.AdjustStoreSize(string(key.Type), -1)
		notifyWatchers(log, watchers, &evt)
	}
	return nil
}

// List returns keys for a project and type.
func (s *MemoryResourceStore) List(
	ctx context.Context,
	project string,
	typ ResourceType,
) (keys []ResourceKey, err error) {
	start := time.Now()
	defer func() { recordStoreOperation(ctx, start, "list", typ, err) }()
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	keys = make([]ResourceKey, 0, len(s.items))
	for k := range s.items {
		if k.Project == project && k.Type == typ {
			keys = append(keys, k)
		}
	}
	s.mu.RUnlock()
	resmetrics.SetStoreSize(string(typ), int64(len(keys)))
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
			w.closeChannel()
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
		lastIdx := len(list) - 1
		list[idx] = list[lastIdx]
		list[lastIdx] = nil
		list = list[:lastIdx]
		if len(list) == 0 {
			delete(s.watchers, keyspace)
		} else {
			s.watchers[keyspace] = list
		}
	}
	target.closeChannel()
	s.mu.Unlock()
}

// copyWatchersLocked remains unnecessary; broadcasting now happens after releasing the store lock

func recordStoreOperation(ctx context.Context, start time.Time, operation string, typ ResourceType, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	resmetrics.RecordOperation(ctx, operation, string(typ), outcome, time.Since(start))
}

func watcherKeyspace(project string, typ ResourceType) string { return project + "|" + string(typ) }

// ListWithValues returns copies of keys, values and etags for project/type.
func (s *MemoryResourceStore) ListWithValues(
	ctx context.Context,
	project string,
	typ ResourceType,
) (items []StoredItem, err error) {
	start := time.Now()
	defer func() { recordStoreOperation(ctx, start, "list", typ, err) }()
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	items = make([]StoredItem, 0)
	for k, e := range s.items {
		if k.Project == project && k.Type == typ {
			cp, copyErr := core.DeepCopy[any](e.value)
			if copyErr != nil {
				s.mu.RUnlock()
				err = fmt.Errorf("deep copy failed: %w", copyErr)
				return nil, err
			}
			items = append(items, StoredItem{Key: k, Value: cp, ETag: e.etag})
		}
	}
	s.mu.RUnlock()
	resmetrics.SetStoreSize(string(typ), int64(len(items)))
	return items, nil
}

// ListWithValuesPage returns a page of items and the total count.
func (s *MemoryResourceStore) ListWithValuesPage(
	ctx context.Context,
	project string,
	typ ResourceType,
	offset, limit int,
) (items []StoredItem, total int, err error) {
	start := time.Now()
	defer func() { recordStoreOperation(ctx, start, "list", typ, err) }()
	if err = ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("context canceled: %w", err)
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = int(^uint(0) >> 1)
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, 0, fmt.Errorf("store is closed")
	}
	end := offset + limit
	items = make([]StoredItem, 0)
	for k, e := range s.items {
		if k.Project != project || k.Type != typ {
			continue
		}
		if total >= offset && total < end {
			cp, copyErr := core.DeepCopy[any](e.value)
			if copyErr != nil {
				s.mu.RUnlock()
				err = fmt.Errorf("deep copy failed: %w", copyErr)
				return nil, 0, err
			}
			items = append(items, StoredItem{Key: k, Value: cp, ETag: e.etag})
		}
		total++
	}
	s.mu.RUnlock()
	resmetrics.SetStoreSize(string(typ), int64(total))
	if offset > total {
		return []StoredItem{}, total, nil
	}
	return items, total, nil
}

// removed: local deepCopy/ETag/writeStableJSON in favor of core.DeepCopy and core.ETagFromAny
