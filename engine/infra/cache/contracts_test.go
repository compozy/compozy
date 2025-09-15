package cache

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// test-only in-memory adapter implementing contracts
type memKV struct {
	mu   sync.RWMutex
	data map[string]memItem
}
type memItem struct {
	val string
	exp time.Time
}

func newMemKV() *memKV { return &memKV{data: map[string]memItem{}} }
func (m *memKV) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	it, ok := m.data[key]
	m.mu.RUnlock()
	if !ok {
		return "", ErrNotFound
	}
	if !it.exp.IsZero() && time.Now().After(it.exp) {
		m.mu.Lock()
		delete(m.data, key)
		m.mu.Unlock()
		return "", ErrNotFound
	}
	return it.val, nil
}
func (m *memKV) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		s = fmt.Sprintf("%v", v)
	}
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.data[key] = memItem{val: s, exp: exp}
	m.mu.Unlock()
	return nil
}
func (m *memKV) Del(_ context.Context, keys ...string) (int64, error) {
	var n int64
	m.mu.Lock()
	for _, k := range keys {
		if _, ok := m.data[k]; ok {
			delete(m.data, k)
			n++
		}
	}
	m.mu.Unlock()
	return n, nil
}
func (m *memKV) MGet(ctx context.Context, keys ...string) ([]string, error) {
	out := make([]string, len(keys))
	for i, k := range keys {
		v, err := m.Get(ctx, k)
		if err != nil {
			out[i] = ""
		} else {
			out[i] = v
		}
	}
	return out, nil
}
func (m *memKV) Expire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	it, ok := m.data[key]
	if !ok {
		m.mu.Unlock()
		return false, ErrNotFound
	}
	if ttl <= 0 {
		it.exp = time.Time{}
	} else {
		it.exp = time.Now().Add(ttl)
	}
	m.data[key] = it
	m.mu.Unlock()
	return true, nil
}

type memLists struct {
	mu    sync.RWMutex
	lists map[string][]string
}

func newMemLists() *memLists { return &memLists{lists: map[string][]string{}} }
func (l *memLists) LRange(_ context.Context, key string, start, stop int64) ([]string, error) {
	l.mu.RLock()
	arr := append([]string(nil), l.lists[key]...)
	l.mu.RUnlock()
	n := int64(len(arr))
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if n == 0 || start > stop {
		return []string{}, nil
	}
	return append([]string(nil), arr[start:stop+1]...), nil
}
func (l *memLists) LLen(_ context.Context, key string) (int64, error) {
	l.mu.RLock()
	n := len(l.lists[key])
	l.mu.RUnlock()
	return int64(n), nil
}
func (l *memLists) LTrim(_ context.Context, key string, start, stop int64) error {
	l.mu.Lock()
	arr := l.lists[key]
	n := int64(len(arr))
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if n == 0 || start > stop {
		l.lists[key] = []string{}
		l.mu.Unlock()
		return nil
	}
	l.lists[key] = append([]string(nil), arr[start:stop+1]...)
	l.mu.Unlock()
	return nil
}
func (l *memLists) RPush(_ context.Context, key string, values ...any) (int64, error) {
	l.mu.Lock()
	for _, v := range values {
		switch t := v.(type) {
		case string:
			l.lists[key] = append(l.lists[key], t)
		case []byte:
			l.lists[key] = append(l.lists[key], string(t))
		default:
			l.lists[key] = append(l.lists[key], fmt.Sprintf("%v", t))
		}
	}
	n := len(l.lists[key])
	l.mu.Unlock()
	return int64(n), nil
}

type memHashes struct {
	mu   sync.RWMutex
	maps map[string]map[string]string
}

func newMemHashes() *memHashes { return &memHashes{maps: map[string]map[string]string{}} }
func (h *memHashes) HSet(_ context.Context, key string, values ...any) (int64, error) {
	if len(values)%2 != 0 {
		return 0, ErrNotSupported
	}
	h.mu.Lock()
	m, ok := h.maps[key]
	if !ok {
		m = map[string]string{}
		h.maps[key] = m
	}
	var c int64
	for i := 0; i < len(values); i += 2 {
		f := fmt.Sprintf("%v", values[i])
		v := fmt.Sprintf("%v", values[i+1])
		if _, exists := m[f]; !exists {
			c++
		}
		m[f] = v
	}
	h.mu.Unlock()
	return c, nil
}
func (h *memHashes) HGet(_ context.Context, key, field string) (string, error) {
	h.mu.RLock()
	m := h.maps[key]
	val, ok := m[field]
	h.mu.RUnlock()
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}
func (h *memHashes) HIncrBy(_ context.Context, key, field string, incr int64) (int64, error) {
	h.mu.Lock()
	m, ok := h.maps[key]
	if !ok {
		m = map[string]string{}
		h.maps[key] = m
	}
	var cur int64
	if s, ok := m[field]; ok {
		fmt.Sscan(s, &cur)
	}
	cur += incr
	m[field] = fmt.Sprintf("%d", cur)
	h.mu.Unlock()
	return cur, nil
}
func (h *memHashes) HDel(_ context.Context, key string, fields ...string) (int64, error) {
	h.mu.Lock()
	m := h.maps[key]
	var n int64
	for _, f := range fields {
		if _, ok := m[f]; ok {
			delete(m, f)
			n++
		}
	}
	h.mu.Unlock()
	return n, nil
}

type memKeys struct{ kv *memKV }
type sliceIter struct {
	mu   sync.Mutex
	once bool
	keys []string
}

func (k *memKeys) Keys(_ context.Context, pattern string) (KeyIterator, error) {
	kv := k.kv
	kv.mu.RLock()
	keys := make([]string, 0, len(kv.data))
	for key := range kv.data {
		if matches(pattern, key) {
			keys = append(keys, key)
		}
	}
	kv.mu.RUnlock()
	sort.Strings(keys)
	return &sliceIter{once: false, keys: keys}, nil
}
func (it *sliceIter) Next(_ context.Context) ([]string, bool, error) {
	it.mu.Lock()
	if it.once {
		it.mu.Unlock()
		return nil, true, nil
	}
	it.once = true
	res := it.keys
	done := len(it.keys) == 0
	it.mu.Unlock()
	return res, done, nil
}
func matches(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		p := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, p)
	}
	return pattern == key
}

type memAtomic struct {
	mu    sync.Mutex
	lists *memLists
	kv    *memKV
}

func (a *memAtomic) AppendAndTrimWithMetadata(
	ctx context.Context,
	key string,
	messages []string,
	tokenDelta int,
	maxLen int,
	ttl time.Duration,
) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	vals := make([]any, len(messages))
	for i, s := range messages {
		vals[i] = s
	}
	_, err := a.lists.RPush(ctx, key, vals...)
	if err != nil {
		return 0, err
	}
	if maxLen >= 0 {
		start := int64(-maxLen)
		stop := int64(-1)
		_ = a.lists.LTrim(ctx, key, start, stop)
	}
	if tokenDelta != 0 {
		cur, _ := a.kv.Get(ctx, key+":tokens")
		var c int64
		if cur != "" {
			fmt.Sscan(cur, &c)
		}
		c += int64(tokenDelta)
		_ = a.kv.Set(ctx, key+":tokens", fmt.Sprintf("%d", c), ttl)
	}
	if ttl > 0 {
		_, _ = a.kv.Expire(ctx, key, ttl)
	}
	n, _ := a.lists.LLen(ctx, key)
	return n, nil
}

func TestCacheContracts_KV(t *testing.T) {
	t.Run("Should set, get, expire and delete keys with neutral errors", func(t *testing.T) {
		ctx := context.Background()
		kv := newMemKV()
		err := kv.Set(ctx, "a", "1", 0)
		require.NoError(t, err)
		v, err := kv.Get(ctx, "a")
		require.NoError(t, err)
		assert.Equal(t, "1", v)
		ok, err := kv.Expire(ctx, "a", 10*time.Millisecond)
		require.NoError(t, err)
		assert.True(t, ok)
		time.Sleep(20 * time.Millisecond)
		_, err = kv.Get(ctx, "a")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
		_ = kv.Set(ctx, "x", "x", 0)
		_ = kv.Set(ctx, "y", "y", 0)
		n, err := kv.Del(ctx, "x", "y")
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
	})
}

func TestCacheContracts_Lists(t *testing.T) {
	t.Run("Should push, range, trim and report length", func(t *testing.T) {
		ctx := context.Background()
		ls := newMemLists()
		n, err := ls.RPush(ctx, "L", "a", "b", "c", "d")
		require.NoError(t, err)
		assert.Equal(t, int64(4), n)
		out, err := ls.LRange(ctx, "L", 1, 2)
		require.NoError(t, err)
		assert.Equal(t, []string{"b", "c"}, out)
		err = ls.LTrim(ctx, "L", -2, -1)
		require.NoError(t, err)
		ln, err := ls.LLen(ctx, "L")
		require.NoError(t, err)
		assert.Equal(t, int64(2), ln)
		out, err = ls.LRange(ctx, "L", 0, -1)
		require.NoError(t, err)
		assert.Equal(t, []string{"c", "d"}, out)
	})
}

func TestCacheContracts_Hashes(t *testing.T) {
	t.Run("Should set, get, incr and delete hash fields", func(t *testing.T) {
		ctx := context.Background()
		h := newMemHashes()
		n, err := h.HSet(ctx, "H", "f1", "v1", "f2", "v2")
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
		v, err := h.HGet(ctx, "H", "f1")
		require.NoError(t, err)
		assert.Equal(t, "v1", v)
		cur, err := h.HIncrBy(ctx, "H", "cnt", 3)
		require.NoError(t, err)
		assert.Equal(t, int64(3), cur)
		cur, err = h.HIncrBy(ctx, "H", "cnt", -1)
		require.NoError(t, err)
		assert.Equal(t, int64(2), cur)
		del, err := h.HDel(ctx, "H", "f2")
		require.NoError(t, err)
		assert.Equal(t, int64(1), del)
		_, err = h.HGet(ctx, "H", "f2")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestCacheContracts_KeysIteration(t *testing.T) {
	t.Run("Should iterate keys by pattern using iterator", func(t *testing.T) {
		ctx := context.Background()
		kv := newMemKV()
		_ = kv.Set(ctx, "workflow:1", "a", 0)
		_ = kv.Set(ctx, "workflow:2", "b", 0)
		_ = kv.Set(ctx, "task:1", "c", 0)
		kp := &memKeys{kv: kv}
		it, err := kp.Keys(ctx, "workflow:*")
		require.NoError(t, err)
		keys, done, err := it.Next(ctx)
		require.NoError(t, err)
		assert.False(t, done)
		assert.Equal(t, []string{"workflow:1", "workflow:2"}, keys)
		_, done, err = it.Next(ctx)
		require.NoError(t, err)
		assert.True(t, done)
	})
}

func TestCacheContracts_AtomicListWithMetadata(t *testing.T) {
	t.Run("Should atomically append, trim and update token metadata", func(t *testing.T) {
		ctx := context.Background()
		kv := newMemKV()
		ls := newMemLists()
		at := &memAtomic{lists: ls, kv: kv}
		n, err := at.AppendAndTrimWithMetadata(ctx, "msgs", []string{"m1", "m2", "m3"}, 15, 3, 50*time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
		n, err = at.AppendAndTrimWithMetadata(ctx, "msgs", []string{"m4", "m5"}, 5, 3, 50*time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
		out, err := ls.LRange(ctx, "msgs", 0, -1)
		require.NoError(t, err)
		assert.Equal(t, []string{"m3", "m4", "m5"}, out)
		tok, err := kv.Get(ctx, "msgs:tokens")
		require.NoError(t, err)
		assert.Equal(t, "20", tok)
	})
}
