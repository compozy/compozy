package sugardb

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	ccache "github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/pkg/logger"
	sdk "github.com/echovault/sugardb/sugardb"
)

// Adapter implements cache contracts (KV, Lists, Hashes, AtomicListWithMetadata)
// on top of an embedded SugarDB instance. Keys iteration is not supported
// by SugarDB at the time of writing; the capability is reported as false.
type Adapter struct {
	db *sdk.SugarDB
	mu sync.Mutex // guards atomic operations
}

// NewAdapter creates a SugarDB-based cache adapter.
func NewAdapter(db *sdk.SugarDB) (*Adapter, error) {
	if db == nil {
		return nil, fmt.Errorf("sugardb instance cannot be nil")
	}
	return &Adapter{db: db}, nil
}

// Capabilities returns supported features for the SugarDB adapter.
func (a *Adapter) Capabilities() ccache.Capabilities {
	return ccache.Capabilities{
		KV:                     true,
		Lists:                  true,
		Hashes:                 true,
		PubSub:                 true,
		Locks:                  true,
		KeysIteration:          false,
		AtomicListWithMetadata: true,
	}
}

// --------------- KV ---------------
func (a *Adapter) Get(_ context.Context, key string) (string, error) {
    vals, err := a.db.MGet(key)
    if err != nil { return "", err }
    if len(vals) == 0 { return "", ccache.ErrNotFound }
    if vals[0] == "" || vals[0] == "nil" || vals[0] == "(nil)" || vals[0] == "<nil>" { return "", ccache.ErrNotFound }
    return vals[0], nil
}

func (a *Adapter) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		s = fmt.Sprintf("%v", v)
	}
	opt := sdk.SETOptions{}
	if ttl > 0 {
		opt.ExpireOpt = sdk.SETPX
		opt.ExpireTime = int(ttl.Milliseconds())
	}
	_, _, err := a.db.Set(key, s, opt)
	return err
}

func (a *Adapter) Del(_ context.Context, keys ...string) (int64, error) {
	n, err := a.db.Del(keys...)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

func (a *Adapter) MGet(_ context.Context, keys ...string) ([]string, error) {
	return a.db.MGet(keys...)
}

func (a *Adapter) Expire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ok, err := a.db.Persist(key)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, ccache.ErrNotFound
		}
		return true, nil
	}
	ok, err := a.db.PExpire(key, int(ttl.Milliseconds()))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ccache.ErrNotFound
	}
	return true, nil
}

// --------------- Lists ---------------
func (a *Adapter) LRange(_ context.Context, key string, start, stop int64) ([]string, error) {
	return a.db.LRange(key, int(start), int(stop))
}
func (a *Adapter) LLen(_ context.Context, key string) (int64, error) {
	n, err := a.db.LLen(key)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}
func (a *Adapter) LTrim(_ context.Context, key string, start, stop int64) error {
	ok, err := a.db.LTrim(key, int(start), int(stop))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("ltrim failed")
	}
	return nil
}
func (a *Adapter) RPush(_ context.Context, key string, values ...any) (int64, error) {
	vs := make([]string, len(values))
	for i, v := range values {
		switch t := v.(type) {
		case string:
			vs[i] = t
		case []byte:
			vs[i] = string(t)
		default:
			vs[i] = fmt.Sprintf("%v", t)
		}
	}
	n, err := a.db.RPush(key, vs...)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

// --------------- Hashes ---------------
func (a *Adapter) HSet(_ context.Context, key string, values ...any) (int64, error) {
	if len(values)%2 != 0 {
		return 0, ccache.ErrNotSupported
	}
	m := make(map[string]string, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		m[fmt.Sprintf("%v", values[i])] = fmt.Sprintf("%v", values[i+1])
	}
	n, err := a.db.HSet(key, m)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

func (a *Adapter) HGet(_ context.Context, key, field string) (string, error) {
	vals, err := a.db.HMGet(key, field)
	if err != nil {
		return "", err
	}
	if len(vals) == 0 || vals[0] == "" {
		exists, err := a.db.Exists(key)
		if err != nil {
			return "", err
		}
		if exists == 0 {
			return "", ccache.ErrNotFound
		}
		fields, err := a.db.HKeys(key)
		if err != nil {
			return "", err
		}
		for _, f := range fields {
			if f == field {
				return "", nil
			}
		}
		return "", ccache.ErrNotFound
	}
	return vals[0], nil
}

func (a *Adapter) HIncrBy(_ context.Context, key, field string, incr int64) (int64, error) {
	f, err := a.db.HIncrBy(key, field, int(incr))
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

func (a *Adapter) HDel(_ context.Context, key string, fields ...string) (int64, error) {
	n, err := a.db.HDel(key, fields...)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

// --------------- Atomic list + metadata ---------------
func (a *Adapter) AppendAndTrimWithMetadata(
	ctx context.Context,
	key string,
	messages []string,
	tokenDelta int,
	maxLen int,
	ttl time.Duration,
) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	vals := make([]string, len(messages))
	copy(vals, messages)
	if _, err := a.db.RPush(key, vals...); err != nil {
		logger.FromContext(ctx).Debug("sugar: rpush failed", "error", err, "key", key)
		return 0, err
	}
	if maxLen >= 0 {
		ok, err := a.db.LTrim(key, -maxLen, -1)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("ltrim failed")
		}
	}
	if tokenDelta != 0 {
		_, err := a.db.IncrBy(key+":tokens", strconv.Itoa(tokenDelta))
		if err != nil {
			return 0, err
		}
	}
	if ttl > 0 {
		if ok, err := a.db.PExpire(key, int(ttl.Milliseconds())); err != nil {
			return 0, err
		} else if !ok {
			return 0, ccache.ErrNotFound
		}
		ok2, err2 := a.db.PExpire(key+":tokens", int(ttl.Milliseconds()))
		if err2 != nil {
			return 0, err2
		}
		if !ok2 {
			if _, perr := a.db.Persist(key + ":tokens"); perr != nil {
				return 0, perr
			}
		}
	}
	n, err := a.db.LLen(key)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}
