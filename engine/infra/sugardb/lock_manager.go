package sugardb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	ccache "github.com/compozy/compozy/engine/infra/cache"
	"github.com/echovault/sugardb/sugardb"
)

// SugarLockManager implements the cache.LockManager using SugarDB primitives.
// Semantics are single-process best-effort suitable for standalone mode.
type SugarLockManager struct{ db *sugardb.SugarDB }

func NewSugarLockManager(db *sugardb.SugarDB) (*SugarLockManager, error) {
	if db == nil {
		return nil, fmt.Errorf("sugardb instance cannot be nil")
	}
	return &SugarLockManager{db: db}, nil
}

func (m *SugarLockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (ccache.Lock, error) {
	if resource == "" {
		return nil, fmt.Errorf("resource cannot be empty")
	}
	key := "lock:" + resource
	token := genToken()
	opt := sugardb.SETOptions{WriteOpt: sugardb.SETNX}
	if ttl > 0 {
		opt.ExpireOpt = sugardb.SETPX
		opt.ExpireTime = int(ttl.Milliseconds())
	}
	_, ok, err := m.db.Set(key, token, opt)
	if err != nil {
		return nil, fmt.Errorf("setnx failed: %w", err)
	}
	if !ok {
		return nil, ccache.ErrLockNotAcquired
	}
	l := &sugarLock{m: m, key: key, token: token, ttl: ttl, held: true, ctx: ctx}
	l.startAutoRenew()
	return l, nil
}

type sugarLock struct {
	m     *SugarLockManager
	key   string
	token string
	ttl   time.Duration
	held  bool
	mu    sync.RWMutex
	stop  chan struct{}
	ctx   context.Context
}

func (l *sugarLock) Release(_ context.Context) error {
	l.mu.Lock()
	if !l.held {
		l.mu.Unlock()
		return ccache.ErrLockNotHeld
	}
	l.held = false
	if l.stop != nil {
		close(l.stop)
	}
	l.mu.Unlock()
	v, err := l.m.db.Get(l.key)
	if err != nil {
		return err
	}
	if v != l.token {
		return ccache.ErrLockNotOwned
	}
	_, err = l.m.db.Del(l.key)
	return err
}

func (l *sugarLock) Refresh(_ context.Context) error {
	l.mu.RLock()
	if !l.held {
		l.mu.RUnlock()
		return ccache.ErrLockNotHeld
	}
	ttl := l.ttl
	l.mu.RUnlock()
	v, err := l.m.db.Get(l.key)
	if err != nil {
		return err
	}
	if v != l.token {
		l.mu.Lock()
		l.held = false
		l.mu.Unlock()
		return ccache.ErrLockNotOwned
	}
	ok, err := l.m.db.PExpire(l.key, int(ttl.Milliseconds()))
	if err != nil {
		return err
	}
	if !ok {
		return ccache.ErrLockNotOwned
	}
	return nil
}

func (l *sugarLock) Resource() string {
	if len(l.key) > 5 {
		return l.key[5:]
	}
	return l.key
}
func (l *sugarLock) IsHeld() bool { l.mu.RLock(); defer l.mu.RUnlock(); return l.held }

func (l *sugarLock) startAutoRenew() {
	if l.ttl <= 0 {
		return
	}
	l.stop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(l.ttl / 3)
		defer ticker.Stop()
		for {
			select {
			case <-l.stop:
				return
			case <-l.ctx.Done():
				return
			case <-ticker.C:
				if err := l.Refresh(l.ctx); err != nil {
					return
				}
			}
		}
	}()
}

func genToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tok_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
