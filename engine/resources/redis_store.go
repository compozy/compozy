package resources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisResourceStore implements ResourceStore backed by Redis with Pub/Sub watch.
type RedisResourceStore struct {
	r         cache.RedisInterface
	prefix    string
	reconcile time.Duration
	closed    atomic.Bool
}

// RedisStoreOption configures RedisResourceStore.
type RedisStoreOption func(*RedisResourceStore)

// WithPrefix sets a custom key prefix (default "res").
func WithPrefix(p string) RedisStoreOption {
	return func(s *RedisResourceStore) {
		s.prefix = p
	}
}

// WithReconcileInterval sets how often Watch emits synthetic PUTs for reconciliation (default 30s).
func WithReconcileInterval(d time.Duration) RedisStoreOption {
	return func(s *RedisResourceStore) {
		s.reconcile = d
	}
}

// NewRedisResourceStore creates a new Redis-backed resource store.
func NewRedisResourceStore(client cache.RedisInterface, opts ...RedisStoreOption) *RedisResourceStore {
	s := &RedisResourceStore{r: client, prefix: "res", reconcile: 30 * time.Second}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Put stores or replaces a resource value and publishes a PUT event.
func (s *RedisResourceStore) Put(ctx context.Context, key ResourceKey, value any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled: %w", err)
	}
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if s.closed.Load() {
		return "", fmt.Errorf("store is closed")
	}
	if value == nil {
		return "", fmt.Errorf("nil value is not allowed")
	}
	cp, err := core.DeepCopy[any](value)
	if err != nil {
		return "", fmt.Errorf("deep copy failed: %w", err)
	}
	jsonBytes := core.StableJSONBytes(cp)
	sum := sha256.Sum256(jsonBytes)
	etag := hex.EncodeToString(sum[:])
	k := s.keyFor(key)
	if err := s.r.Set(ctx, k, jsonBytes, 0).Err(); err != nil {
		return "", err
	}
	evt := Event{Type: EventPut, Key: key, ETag: etag, At: time.Now().UTC()}
	if err := s.publish(ctx, key.Project, key.Type, &evt); err != nil {
		log.Warn("publish put failed", "error", err)
	}
	return etag, nil
}

// Get retrieves a resource by key. Returns ErrNotFound if not present.
func (s *RedisResourceStore) Get(ctx context.Context, key ResourceKey) (any, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", fmt.Errorf("context canceled: %w", err)
	}
	_ = config.FromContext(ctx)
	if s.closed.Load() {
		return nil, "", fmt.Errorf("store is closed")
	}
	k := s.keyFor(key)
	bs, err := s.r.Get(ctx, k).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	var v any
	if err := json.Unmarshal(bs, &v); err != nil {
		return nil, "", fmt.Errorf("unmarshal failed: %w", err)
	}
	sum := sha256.Sum256(bs)
	etag := hex.EncodeToString(sum[:])
	return v, etag, nil
}

// Delete removes a resource by key and publishes a DELETE event if it existed.
func (s *RedisResourceStore) Delete(ctx context.Context, key ResourceKey) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if s.closed.Load() {
		return fmt.Errorf("store is closed")
	}
	k := s.keyFor(key)
	// Atomic GET + DEL using Lua to preserve ETag of removed value
	// Returns bulk string of previous value or nil
	script := "local v=redis.call('GET', KEYS[1]); if v then redis.call('DEL', KEYS[1]); end; return v"
	cmd := s.r.Eval(ctx, script, []string{k})
	res, err := cmd.Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}
	if res == nil {
		return nil
	}
	bs, ok := res.(string)
	if !ok {
		// handle binary-safe return; try []byte
		if bsb, ok2 := res.([]byte); ok2 {
			bs = string(bsb)
		} else {
			// fallback: publish delete without etag
			evt := Event{Type: EventDelete, Key: key, ETag: "", At: time.Now().UTC()}
			if err := s.publish(ctx, key.Project, key.Type, &evt); err != nil {
				log.Warn("publish delete failed", "error", err)
			}
			return nil
		}
	}
	sum := sha256.Sum256([]byte(bs))
	etag := hex.EncodeToString(sum[:])
	evt := Event{Type: EventDelete, Key: key, ETag: etag, At: time.Now().UTC()}
	if err := s.publish(ctx, key.Project, key.Type, &evt); err != nil {
		log.Warn("publish delete failed", "error", err)
	}
	return nil
}

// List enumerates keys for a project and type using SCAN.
func (s *RedisResourceStore) List(ctx context.Context, project string, typ ResourceType) ([]ResourceKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	_ = config.FromContext(ctx)
	if s.closed.Load() {
		return nil, fmt.Errorf("store is closed")
	}
	pattern := s.keyPrefix(project, typ) + ":*"
	var cursor uint64
	res := make([]ResourceKey, 0, 64)
	for {
		keys, next, err := s.r.Scan(ctx, cursor, pattern, 256).Result()
		if err != nil {
			return nil, err
		}
		for _, full := range keys {
			if rk, ok := s.parseKey(full); ok {
				res = append(res, rk)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return res, nil
}

// Watch subscribes to events for a project and type. It primes and periodically reconciles.
func (s *RedisResourceStore) Watch(ctx context.Context, project string, typ ResourceType) (<-chan Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if s.closed.Load() {
		return nil, fmt.Errorf("store is closed")
	}
	ch := make(chan Event, defaultWatchBuffer)
	topic := s.eventsChannel(project, typ)
	ps := s.r.Subscribe(ctx, topic)
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return nil, fmt.Errorf("subscribe failed: %w", err)
	}
	prime := func() {
		keys, err := s.List(ctx, project, typ)
		if err != nil {
			log.Warn("prime list failed", "error", err)
			return
		}
		for _, k := range keys {
			v, et, err := s.Get(ctx, k)
			if err != nil {
				continue
			}
			_ = v
			evt := Event{Type: EventPut, Key: k, ETag: et, At: time.Now().UTC()}
			select {
			case ch <- evt:
			default:
				log.Warn("watch channel full during prime", "project", project, "type", string(typ))
			}
		}
	}
	prime()
	go func() {
		defer close(ch)
		defer func() { _ = ps.Close() }()
		ticker := time.NewTicker(s.reconcile)
		defer ticker.Stop()
		msgs := ps.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				prime()
			case m, ok := <-msgs:
				if !ok {
					return
				}
				var evt Event
				if err := json.Unmarshal([]byte(m.Payload), &evt); err != nil {
					log.Warn("event decode failed", "error", err)
					continue
				}
				select {
				case ch <- evt:
				default:
					log.Warn("watch channel full; dropping event", "project", project, "type", string(typ))
				}
			}
		}
	}()
	return ch, nil
}

// Close closes the store; watchers will naturally close when contexts are canceled by callers.
func (s *RedisResourceStore) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	return s.r.Close()
}

func (s *RedisResourceStore) publish(ctx context.Context, project string, typ ResourceType, evt *Event) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return s.r.Publish(ctx, s.eventsChannel(project, typ), payload).Err()
}

func (s *RedisResourceStore) keyFor(k ResourceKey) string {
	b := strings.Builder{}
	b.WriteString(s.keyPrefix(k.Project, k.Type))
	b.WriteString(":")
	b.WriteString(k.ID)
	if k.Version != "" {
		b.WriteString(":ver:")
		b.WriteString(k.Version)
	}
	return b.String()
}

func (s *RedisResourceStore) keyPrefix(project string, typ ResourceType) string {
	b := strings.Builder{}
	b.WriteString(s.prefix)
	b.WriteString(":")
	b.WriteString(project)
	b.WriteString(":")
	b.WriteString(string(typ))
	return b.String()
}

func (s *RedisResourceStore) eventsChannel(project string, typ ResourceType) string {
	b := strings.Builder{}
	b.WriteString(s.prefix)
	b.WriteString(":events:")
	b.WriteString(project)
	b.WriteString(":")
	b.WriteString(string(typ))
	return b.String()
}

func (s *RedisResourceStore) parseKey(full string) (ResourceKey, bool) {
	if !strings.HasPrefix(full, s.prefix+":") {
		return ResourceKey{}, false
	}
	rest := strings.TrimPrefix(full, s.prefix+":")
	parts := strings.Split(rest, ":")
	if len(parts) < 3 {
		return ResourceKey{}, false
	}
	project := parts[0]
	typ := ResourceType(parts[1])
	id := parts[2]
	ver := ""
	if len(parts) > 3 {
		for i := 3; i < len(parts); i += 2 {
			if i+1 < len(parts) && parts[i] == "ver" {
				ver = parts[i+1]
			}
		}
	}
	return ResourceKey{Project: project, Type: typ, ID: id, Version: ver}, true
}
