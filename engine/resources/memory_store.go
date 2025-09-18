package resources

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/config"
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
	etag  string
}

type watcher struct {
	ch     chan Event
	closed bool
}

const defaultWatchBuffer = 64

// NewMemoryResourceStore constructs a new MemoryResourceStore.
func NewMemoryResourceStore() *MemoryResourceStore {
	return &MemoryResourceStore{items: make(map[ResourceKey]storedEntry), watchers: make(map[string][]*watcher)}
}

// Put inserts or replaces a resource value at the given key and broadcasts an event.
func (s *MemoryResourceStore) Put(ctx context.Context, key ResourceKey, value any) (string, error) {
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if value == nil {
		return "", fmt.Errorf("nil value is not allowed")
	}
	cp, err := deepCopy(value)
	if err != nil {
		return "", fmt.Errorf("deep copy failed: %w", err)
	}
	etag := computeETag(cp)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", fmt.Errorf("store is closed")
	}
	s.items[key] = storedEntry{value: cp, etag: etag}
	wlist := s.copyWatchersLocked(key.Project, key.Type)
	s.mu.Unlock()
	evt := Event{Type: EventPut, Key: key, ETag: etag, At: time.Now().UTC()}
	for _, w := range wlist {
		select {
		case w.ch <- evt:
		default:
			log.Warn(
				"watch channel full; dropping event",
				"project",
				key.Project,
				"type",
				string(key.Type),
				"id",
				key.ID,
			)
		}
	}
	return etag, nil
}

// Get retrieves a resource value by key, returning a deep copy.
func (s *MemoryResourceStore) Get(ctx context.Context, key ResourceKey) (any, string, error) {
	_ = config.FromContext(ctx)
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, "", fmt.Errorf("store is closed")
	}
	entry, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, "", ErrNotFound
	}
	cp, err := deepCopy(entry.value)
	if err != nil {
		return nil, "", fmt.Errorf("deep copy failed: %w", err)
	}
	return cp, entry.etag, nil
}

// Delete removes a resource by key and broadcasts an event if existed.
func (s *MemoryResourceStore) Delete(ctx context.Context, key ResourceKey) error {
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	existed := false
	etag := ""
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
	wlist := s.copyWatchersLocked(key.Project, key.Type)
	s.mu.Unlock()
	if existed {
		evt := Event{Type: EventDelete, Key: key, ETag: etag, At: time.Now().UTC()}
		for _, w := range wlist {
			select {
			case w.ch <- evt:
			default:
				log.Warn(
					"watch channel full; dropping event",
					"project",
					key.Project,
					"type",
					string(key.Type),
					"id",
					key.ID,
				)
			}
		}
	}
	return nil
}

// List returns keys for a project and type.
func (s *MemoryResourceStore) List(ctx context.Context, project string, typ ResourceType) ([]ResourceKey, error) {
	_ = config.FromContext(ctx)
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
	_ = config.FromContext(ctx)
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
	go func() {
		<-ctx.Done()
		s.removeWatcher(project, typ, w)
	}()
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

func (s *MemoryResourceStore) copyWatchersLocked(project string, typ ResourceType) []*watcher {
	keyspace := watcherKeyspace(project, typ)
	src := s.watchers[keyspace]
	if len(src) == 0 {
		return nil
	}
	dst := make([]*watcher, len(src))
	copy(dst, src)
	return dst
}

func watcherKeyspace(project string, typ ResourceType) string {
	var b strings.Builder
	b.WriteString(project)
	b.WriteString("|")
	b.WriteString(string(typ))
	return b.String()
}

func deepCopy(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func computeETag(v any) string {
	var buf bytes.Buffer
	writeStableJSON(&buf, v)
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}

// writeStableJSON serializes v into a canonical JSON-like form with sorted map keys.
func writeStableJSON(b *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			if bs, err := json.Marshal(k); err == nil {
				b.Write(bs)
			} else {
				b.WriteString("\"")
				b.WriteString(k)
				b.WriteString("\"")
			}
			b.WriteByte(':')
			writeStableJSON(b, t[k])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeStableJSON(b, e)
		}
		b.WriteByte(']')
	case string:
		if bs, err := json.Marshal(t); err == nil {
			b.Write(bs)
		} else {
			b.WriteString("\"")
			b.WriteString(t)
			b.WriteString("\"")
		}
	case float64, bool, nil:
		if bs, err := json.Marshal(t); err == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	default:
		if bs, err := json.Marshal(t); err == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	}
}
