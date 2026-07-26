package session

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
)

const (
	defaultReadOnlyQueryStoreTTL           = 30 * time.Second
	defaultReadOnlyQueryStoreSweepInterval = defaultReadOnlyQueryStoreTTL / 2
)

type queryStoreRuntime struct {
	pool          *sessiondb.ReadOnlyPool
	logger        *slog.Logger
	sweepInterval time.Duration
	startOnce     sync.Once
	stopOnce      sync.Once
	cancel        context.CancelFunc
	done          chan struct{}
}

func newDefaultQueryStoreRuntime(logger *slog.Logger) *queryStoreRuntime {
	pool := sessiondb.NewReadOnlyPool(sessiondb.ReadOnlyPoolConfig{
		TTL: defaultReadOnlyQueryStoreTTL,
		Open: func(ctx context.Context, sessionID string, path string) (store.EventReadCloser, error) {
			return sessiondb.OpenSessionDBReadOnly(ctx, sessionID, path)
		},
	})
	if logger == nil {
		logger = slog.Default()
	}
	return &queryStoreRuntime{
		pool:          pool,
		logger:        logger,
		sweepInterval: defaultReadOnlyQueryStoreSweepInterval,
	}
}

func (r *queryStoreRuntime) Open(
	ctx context.Context,
	sessionID string,
	path string,
) (EventReadCloser, error) {
	if r == nil || r.pool == nil {
		return nil, errors.New("session: query store runtime is required")
	}
	return r.pool.Open(ctx, sessionID, path)
}

func (r *queryStoreRuntime) Quiesce(
	ctx context.Context,
	sessionID string,
	path string,
) (func(), error) {
	if r == nil || r.pool == nil {
		return func() {}, nil
	}
	return r.pool.Quiesce(ctx, sessionID, path)
}

func (r *queryStoreRuntime) Start(ctx context.Context) {
	if r == nil || r.pool == nil {
		return
	}
	r.startOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}
		r.done = make(chan struct{})
		runtimeCtx, cancel := context.WithCancel(ctx)
		r.cancel = cancel
		go r.sweep(runtimeCtx)
	})
}

func (r *queryStoreRuntime) Shutdown(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: shutdown query store context is required")
	}
	r.stopOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
	})
	if r.done != nil {
		select {
		case <-r.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.pool.Close(ctx)
}

func (r *queryStoreRuntime) sweep(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(r.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.pool.CloseExpired(context.WithoutCancel(ctx)); err != nil {
				r.logger.WarnContext(ctx, "session: sweep read-only query store pool", "error", err)
			}
		}
	}
}

func (m *Manager) ensureQueryStoreRuntime() {
	if m.openQueryStore != nil {
		return
	}
	runtime := newDefaultQueryStoreRuntime(m.logger)
	m.queryStoreRuntime = runtime
	m.openQueryStore = runtime.Open
	if m.lifecycleCtx != nil {
		runtime.Start(m.lifecycleCtx)
	}
}

func (m *Manager) shutdownQueryStoreRuntime(ctx context.Context) error {
	if m == nil || m.queryStoreRuntime == nil {
		return nil
	}
	return m.queryStoreRuntime.Shutdown(ctx)
}
