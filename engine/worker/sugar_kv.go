package worker

import (
	"context"
	"fmt"
	"time"

	ccache "github.com/compozy/compozy/engine/infra/cache"
	sdk "github.com/echovault/sugardb/sugardb"
)

// sugarKV adapts SugarDB to cache.KV for standalone mode.
type sugarKV struct {
	db *sdk.SugarDB
}

func newSugarKV(db *sdk.SugarDB) *sugarKV {
	return &sugarKV{db: db}
}

func (a *sugarKV) Get(_ context.Context, key string) (string, error) {
	v, err := a.db.Get(key)
	if err != nil {
		return "", err
	}
	if v == "" || v == "nil" || v == "(nil)" || v == "<nil>" {
		return "", ccache.ErrNotFound
	}
	return v, nil
}

func (a *sugarKV) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	s := ""
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

func (a *sugarKV) Del(_ context.Context, keys ...string) (int64, error) {
	n, err := a.db.Del(keys...)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

func (a *sugarKV) MGet(_ context.Context, keys ...string) ([]string, error) {
	return a.db.MGet(keys...)
}

func (a *sugarKV) Expire(_ context.Context, key string, ttl time.Duration) (bool, error) {
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
