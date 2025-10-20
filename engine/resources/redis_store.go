package resources

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisResourceStore implements ResourceStore backed by Redis with Pub/Sub watch.
type RedisResourceStore struct {
	r           cache.RedisInterface
	prefix      string
	reconcile   time.Duration
	watchBuffer int
	closed      atomic.Bool
}

const (
	defaultPrefix               = "res"
	defaultReconcile            = 30 * time.Second
	minReconcile                = time.Second
	watchBackpressureWarn       = 500 * time.Millisecond
	watchSendInterval           = 25 * time.Millisecond
	scanCount             int64 = 256
)

const redisPutIfMatchScript = `local current = redis.call("GET", KEYS[1])
if not current then return {err="NOT_FOUND"} end
local expected = ARGV[1]
if current ~= expected then return {err="MISMATCH"} end
redis.call("SET", KEYS[1], ARGV[2])
redis.call("SET", KEYS[2], ARGV[3])
return ARGV[3]`

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
		if d < minReconcile {
			s.reconcile = defaultReconcile
			return
		}
		s.reconcile = d
	}
}

// WithWatchBuffer overrides the watch channel capacity (default 256).
func WithWatchBuffer(n int) RedisStoreOption {
	return func(s *RedisResourceStore) {
		if n > 0 {
			s.watchBuffer = n
		}
	}
}

// NewRedisResourceStore creates a new Redis-backed resource store.
func NewRedisResourceStore(client cache.RedisInterface, opts ...RedisStoreOption) *RedisResourceStore {
	if client == nil {
		panic("NewRedisResourceStore: nil Redis client")
	}
	s := &RedisResourceStore{
		r:           client,
		prefix:      defaultPrefix,
		reconcile:   defaultReconcile,
		watchBuffer: defaultWatchBuffer,
	}
	for _, o := range opts {
		o(s)
	}
	if s.reconcile <= 0 {
		s.reconcile = defaultReconcile
	}
	if s.watchBuffer <= 0 {
		s.watchBuffer = defaultWatchBuffer
	}
	return s
}

// Put stores or replaces a resource value and publishes a PUT event.
func (s *RedisResourceStore) Put(ctx context.Context, key ResourceKey, value any) (ETag, error) {
	if err := ctx.Err(); err != nil {
		return ETag(""), fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if s.closed.Load() {
		return ETag(""), fmt.Errorf("store is closed")
	}
	if value == nil {
		return ETag(""), fmt.Errorf("nil value is not allowed")
	}
	cp, err := core.DeepCopy(value)
	if err != nil {
		return ETag(""), fmt.Errorf("deep copy failed: %w", err)
	}
	jsonBytes := core.StableJSONBytes(cp)
	sum := sha256.Sum256(jsonBytes)
	etag := hex.EncodeToString(sum[:])
	k := s.keyFor(key)
	etagKey := s.etagKey(key)
	pipe := s.r.TxPipeline()
	pipe.Set(ctx, k, jsonBytes, 0)
	pipe.Set(ctx, etagKey, etag, 0)
	if _, err := pipe.Exec(ctx); err != nil {
		return ETag(""), fmt.Errorf("redis pipeline exec: %w", err)
	}
	evt := Event{Type: EventPut, Key: key, ETag: ETag(etag), At: time.Now().UTC()}
	if err := s.publish(ctx, key.Project, key.Type, &evt); err != nil {
		log.Warn("publish put failed", "error", err)
	}
	return ETag(etag), nil
}

// PutIfMatch updates a resource only when the supplied ETag matches the current value.
func (s *RedisResourceStore) PutIfMatch(
	ctx context.Context,
	key ResourceKey,
	value any,
	expectedETag ETag,
) (ETag, error) {
	if err := ctx.Err(); err != nil {
		return ETag(""), fmt.Errorf("context canceled: %w", err)
	}
	if s.closed.Load() {
		return ETag(""), fmt.Errorf("store is closed")
	}
	if value == nil {
		return ETag(""), fmt.Errorf("nil value is not allowed")
	}
	jsonBytes, newETag, err := prepareRedisPayload(value)
	if err != nil {
		return ETag(""), err
	}
	valueKey := s.keyFor(key)
	etagKey := s.etagKey(key)
	current, err := s.r.Get(ctx, valueKey).Bytes()
	if err != nil {
		etag, handleErr := s.handlePutIfMatchMiss(ctx, key, expectedETag, jsonBytes, newETag, valueKey, etagKey, err)
		return etag, handleErr
	}
	if err := ensureExpectedRedisETag(current, expectedETag); err != nil {
		return ETag(""), err
	}
	if err := s.updateValueWithCAS(ctx, valueKey, etagKey, current, jsonBytes, newETag); err != nil {
		return ETag(""), err
	}
	s.emitPutEvent(ctx, key, newETag)
	return ETag(newETag), nil
}

func (s *RedisResourceStore) handlePutIfMatchMiss(
	ctx context.Context,
	key ResourceKey,
	expectedETag ETag,
	jsonBytes []byte,
	newETag string,
	valueKey string,
	etagKey string,
	getErr error,
) (ETag, error) {
	if getErr != redis.Nil {
		return ETag(""), fmt.Errorf("redis GET current value: %w", getErr)
	}
	if expectedETag != "" {
		return ETag(""), ErrNotFound
	}
	if err := s.createIfAbsent(ctx, key, jsonBytes, newETag, valueKey, etagKey); err != nil {
		return ETag(""), err
	}
	return ETag(newETag), nil
}

func (s *RedisResourceStore) createIfAbsent(
	ctx context.Context,
	key ResourceKey,
	jsonBytes []byte,
	newETag string,
	valueKey string,
	etagKey string,
) error {
	const insertScript = `if redis.call("EXISTS", KEYS[1]) == 1 then return {err="ALREADY_EXISTS"} end
redis.call("SET", KEYS[1], ARGV[1])
redis.call("SET", KEYS[2], ARGV[2])
return ARGV[2]`
	cmd := s.r.Eval(ctx, insertScript, []string{valueKey, etagKey}, string(jsonBytes), newETag)
	if evalErr := cmd.Err(); evalErr != nil {
		switch {
		case strings.Contains(evalErr.Error(), "ALREADY_EXISTS"):
			return ErrETagMismatch
		default:
			return fmt.Errorf("redis create eval: %w", evalErr)
		}
	}
	evt := Event{Type: EventPut, Key: key, ETag: ETag(newETag), At: time.Now().UTC()}
	if err := s.publish(ctx, key.Project, key.Type, &evt); err != nil {
		logger.FromContext(ctx).Warn("publish put failed", "error", err)
	}
	return nil
}

// Get retrieves a resource by key. Returns ErrNotFound if not present.
func (s *RedisResourceStore) Get(ctx context.Context, key ResourceKey) (any, ETag, error) {
	if err := ctx.Err(); err != nil {
		return nil, ETag(""), fmt.Errorf("context canceled: %w", err)
	}
	if s.closed.Load() {
		return nil, ETag(""), fmt.Errorf("store is closed")
	}
	k := s.keyFor(key)
	bs, err := s.r.Get(ctx, k).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ETag(""), ErrNotFound
		}
		return nil, ETag(""), fmt.Errorf("redis GET value: %w", err)
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(bs))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, ETag(""), fmt.Errorf("unmarshal failed: %w", err)
	}
	var etag ETag
	et, err := s.r.Get(ctx, s.etagKey(key)).Result()
	switch err {
	case nil:
		etag = ETag(et)
	case redis.Nil:
		sum := sha256.Sum256(bs)
		etag = ETag(hex.EncodeToString(sum[:]))
	default:
		return nil, ETag(""), fmt.Errorf("redis GET etag: %w", err)
	}
	return v, etag, nil
}

// Delete removes a resource by key and publishes a DELETE event if it existed.
func (s *RedisResourceStore) Delete(ctx context.Context, key ResourceKey) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if s.closed.Load() {
		return fmt.Errorf("store is closed")
	}
	k := s.keyFor(key)
	etagKey := s.etagKey(key)
	// Atomic GET + DEL using Lua to preserve ETag of removed value
	// Returns bulk string of previous value or nil
	script := "local v=redis.call('GET', KEYS[1]); if v then " +
		"redis.call('DEL', KEYS[1]); redis.call('DEL', KEYS[2]); end; return v"
	cmd := s.r.Eval(ctx, script, []string{k, etagKey})
	res, err := cmd.Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("redis delete eval: %w", err)
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
			evt := Event{Type: EventDelete, Key: key, ETag: ETag(""), At: time.Now().UTC()}
			if err := s.publish(ctx, key.Project, key.Type, &evt); err != nil {
				log.Warn("publish delete failed", "error", err)
			}
			return nil
		}
	}
	sum := sha256.Sum256([]byte(bs))
	etag := ETag(hex.EncodeToString(sum[:]))
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
	if s.closed.Load() {
		return nil, fmt.Errorf("store is closed")
	}
	pattern := s.keyPrefix(project, typ) + ":*"
	var cursor uint64
	res := make([]ResourceKey, 0, 64)
	for {
		keys, next, err := s.r.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return nil, fmt.Errorf("redis SCAN: %w", err)
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

// ListWithValues returns keys with their JSON values (decoded) and ETags using batched MGET.
func (s *RedisResourceStore) ListWithValues(
	ctx context.Context,
	project string,
	typ ResourceType,
) ([]StoredItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if s.closed.Load() {
		return nil, fmt.Errorf("store is closed")
	}
	// Collect keys using SCAN
	resKeys, redisKeys, etagKeys, err := s.scanResourceKeys(ctx, project, typ)
	if err != nil {
		return nil, err
	}
	if len(redisKeys) == 0 {
		return []StoredItem{}, nil
	}
	var (
		valsCmd *redis.SliceCmd
		etagCmd *redis.SliceCmd
	)
	_, execErr := s.r.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		valsCmd = pipe.MGet(ctx, redisKeys...)
		etagCmd = pipe.MGet(ctx, etagKeys...)
		return nil
	})
	vals, valsErr := valsCmd.Result()
	if valsErr != nil {
		return nil, fmt.Errorf("redis MGET values: %w", valsErr)
	}
	etVals, etErr := etagCmd.Result()
	if etErr != nil {
		log.Warn("mget etags failed; falling back to hashing", "error", etErr)
		etVals = make([]any, len(redisKeys))
	}
	if execErr != nil && (etErr == nil || !errors.Is(execErr, etErr)) {
		return nil, fmt.Errorf("redis pipeline exec: %w", execErr)
	}
	return s.buildStoredItems(ctx, vals, etVals, resKeys), nil
}

// scanResourceKeys scans Redis for resource keys and returns parsed keys, raw Redis keys, and ETag keys.
func (s *RedisResourceStore) scanResourceKeys(
	ctx context.Context,
	project string,
	typ ResourceType,
) ([]ResourceKey, []string, []string, error) {
	pattern := s.keyPrefix(project, typ) + ":*"
	var cursor uint64
	redisKeys := make([]string, 0, 128)
	resKeys := make([]ResourceKey, 0, 128)
	etagKeys := make([]string, 0, 128)
	for {
		keys, next, err := s.r.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("redis SCAN: %w", err)
		}
		for _, full := range keys {
			if rk, ok := s.parseKey(full); ok {
				resKeys = append(resKeys, rk)
				redisKeys = append(redisKeys, full)
				etagKeys = append(etagKeys, s.etagKey(rk))
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return resKeys, redisKeys, etagKeys, nil
}

// buildStoredItems decodes MGET results and pairs them with corresponding ETags.
func (s *RedisResourceStore) buildStoredItems(
	ctx context.Context,
	vals []any,
	etVals []any,
	resKeys []ResourceKey,
) []StoredItem {
	log := logger.FromContext(ctx)
	out := make([]StoredItem, 0, len(vals))
	for i, raw := range vals {
		if raw == nil {
			continue
		}
		var bs []byte
		switch t := raw.(type) {
		case string:
			bs = []byte(t)
		case []byte:
			bs = t
		default:
			continue
		}
		var v any
		if err := json.Unmarshal(bs, &v); err != nil {
			var key ResourceKey
			if i < len(resKeys) {
				key = resKeys[i]
			}
			log.Debug("failed to unmarshal stored item", "error", err, "key", key)
			continue
		}
		var etag ETag
		if i < len(etVals) && etVals[i] != nil {
			switch et := etVals[i].(type) {
			case string:
				etag = ETag(et)
			case []byte:
				etag = ETag(string(et))
			default:
				sum := sha256.Sum256(bs)
				etag = ETag(hex.EncodeToString(sum[:]))
			}
		} else {
			sum := sha256.Sum256(bs)
			etag = ETag(hex.EncodeToString(sum[:]))
		}
		out = append(out, StoredItem{Key: resKeys[i], Value: v, ETag: etag})
	}
	return out
}

// ListWithValuesPage returns a page of items and the total count.
func paginateSlice[T any](items []T, offset, limit int) ([]T, int) {
	total := len(items)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = total
	}
	if offset > total {
		return []T{}, total
	}
	end := min(offset+limit, total)
	return items[offset:end], total
}

func (s *RedisResourceStore) ListWithValuesPage(
	ctx context.Context,
	project string,
	typ ResourceType,
	offset, limit int,
) ([]StoredItem, int, error) {
	items, err := s.ListWithValues(ctx, project, typ)
	if err != nil {
		return nil, 0, err
	}
	page, total := paginateSlice(items, offset, limit)
	return page, total, nil
}

// Watch subscribes to events for a project and type. It primes and periodically reconciles.
func (s *RedisResourceStore) Watch(ctx context.Context, project string, typ ResourceType) (<-chan Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if s.closed.Load() {
		return nil, fmt.Errorf("store is closed")
	}
	buffer := s.watchBuffer
	if buffer <= 0 {
		buffer = defaultWatchBuffer
	}
	ch := make(chan Event, buffer)
	topic := s.eventsChannel(project, typ)
	ps := s.r.Subscribe(ctx, topic)
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return nil, fmt.Errorf("subscribe failed: %w", err)
	}
	prime := func() {
		items, err := s.ListWithValues(ctx, project, typ)
		if err != nil {
			log.Warn("prime list failed", "error", err)
			return
		}
		for _, it := range items {
			evt := Event{Type: EventPut, Key: it.Key, ETag: it.ETag, At: time.Now().UTC()}
			if !deliverEvent(ctx, ch, &evt) {
				return
			}
		}
	}
	msgs := ps.Channel()
	interval := s.reconcile
	if interval < minReconcile {
		interval = defaultReconcile
	}
	go s.runWatchLoop(ctx, ch, ps, msgs, interval, prime)
	return ch, nil
}

func (s *RedisResourceStore) runWatchLoop(
	ctx context.Context,
	ch chan Event,
	ps *redis.PubSub,
	msgs <-chan *redis.Message,
	interval time.Duration,
	prime func(),
) {
	log := logger.FromContext(ctx)
	defer close(ch)
	defer func() {
		if err := ps.Close(); err != nil {
			log.Warn("pubsub close failed", "error", err)
		}
	}()
	prime()
	var (
		ticker *time.Ticker
		tickCh <-chan time.Time
	)
	if interval >= minReconcile {
		ticker = time.NewTicker(interval)
		tickCh = ticker.C
		defer ticker.Stop()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tickCh:
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
			if !deliverEvent(ctx, ch, &evt) {
				return
			}
		}
	}
}

func deliverEvent(ctx context.Context, ch chan Event, evt *Event) bool {
	if ch == nil {
		return false
	}
	log := logger.FromContext(ctx)
	state := &deliveryState{}
	defer state.stop()

	for {
		select {
		case ch <- *evt:
			logBackpressure(log, evt, state.start)
			state.reset()
			return true
		case <-ctx.Done():
			return false
		default:
		}

		state.startWaiting()
		if !state.wait(ctx) {
			return false
		}
	}
}

type deliveryState struct {
	timer *time.Timer
	start time.Time
}

func (d *deliveryState) startWaiting() {
	if d.timer == nil {
		d.timer = time.NewTimer(watchSendInterval)
		d.start = time.Now()
		return
	}
	if !d.timer.Stop() {
		select {
		case <-d.timer.C:
		default:
		}
	}
	d.timer.Reset(watchSendInterval)
	if d.start.IsZero() {
		d.start = time.Now()
	}
}

func (d *deliveryState) wait(ctx context.Context) bool {
	if d.timer == nil {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-d.timer.C:
		return true
	}
}

func (d *deliveryState) stop() {
	if d.timer == nil {
		return
	}
	if !d.timer.Stop() {
		select {
		case <-d.timer.C:
		default:
		}
	}
}

func (d *deliveryState) reset() {
	d.start = time.Time{}
}

func logBackpressure(log logger.Logger, evt *Event, waitStart time.Time) {
	if waitStart.IsZero() {
		return
	}
	if delay := time.Since(waitStart); delay >= watchBackpressureWarn {
		log.Warn(
			"watch delivery delayed",
			"delay", delay,
			"project", evt.Key.Project,
			"type", string(evt.Key.Type),
			"id", evt.Key.ID,
		)
	}
}

func prepareRedisPayload(value any) ([]byte, string, error) {
	cp, err := core.DeepCopy(value)
	if err != nil {
		return nil, "", fmt.Errorf("deep copy failed: %w", err)
	}
	jsonBytes := core.StableJSONBytes(cp)
	sum := sha256.Sum256(jsonBytes)
	return jsonBytes, hex.EncodeToString(sum[:]), nil
}

func ensureExpectedRedisETag(current []byte, expected ETag) error {
	currentSum := sha256.Sum256(current)
	if hex.EncodeToString(currentSum[:]) != string(expected) {
		return ErrETagMismatch
	}
	return nil
}

func (s *RedisResourceStore) updateValueWithCAS(
	ctx context.Context,
	valueKey string,
	etagKey string,
	current []byte,
	jsonBytes []byte,
	newETag string,
) error {
	cmd := s.r.Eval(
		ctx,
		redisPutIfMatchScript,
		[]string{valueKey, etagKey},
		string(current),
		string(jsonBytes),
		newETag,
	)
	if err := cmd.Err(); err != nil {
		switch {
		case strings.Contains(err.Error(), "NOT_FOUND"):
			return ErrNotFound
		case strings.Contains(err.Error(), "MISMATCH"):
			return ErrETagMismatch
		default:
			return fmt.Errorf("redis CAS eval: %w", err)
		}
	}
	return nil
}

func (s *RedisResourceStore) emitPutEvent(ctx context.Context, key ResourceKey, etag string) {
	evt := Event{Type: EventPut, Key: key, ETag: ETag(etag), At: time.Now().UTC()}
	if err := s.publish(ctx, key.Project, key.Type, &evt); err != nil {
		logger.FromContext(ctx).Warn("publish put failed", "error", err)
	}
}

// Close closes the store; watchers will naturally close when contexts are canceled by callers.
func (s *RedisResourceStore) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	// The store does not own the Redis client; caller manages its lifecycle.
	return nil
}

func (s *RedisResourceStore) publish(ctx context.Context, project string, typ ResourceType, evt *Event) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if err := s.r.Publish(ctx, s.eventsChannel(project, typ), payload).Err(); err != nil {
		return fmt.Errorf("redis publish: %w", err)
	}
	return nil
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

func (s *RedisResourceStore) etagKey(k ResourceKey) string {
	return s.keyFor(k) + ":etag"
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
	if parts[len(parts)-1] == "etag" {
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
