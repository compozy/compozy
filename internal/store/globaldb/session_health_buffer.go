package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type pendingSessionHealth struct {
	health  heartbeat.SessionHealth
	version uint64
}

type sessionHealthBuffer struct {
	base     *repoBase
	mu       sync.Mutex
	pending  map[string]pendingSessionHealth
	version  uint64
	ctx      context.Context
	cancel   context.CancelFunc
	closing  chan context.Context
	done     chan struct{}
	closeErr error
}

func newSessionHealthBuffer(base *repoBase, interval time.Duration) *sessionHealthBuffer {
	buffer := &sessionHealthBuffer{
		base: base, pending: make(map[string]pendingSessionHealth),
		closing: make(chan context.Context, 1), done: make(chan struct{}),
	}
	buffer.ctx, buffer.cancel = context.WithCancel(context.Background())
	go buffer.run(interval)
	return buffer
}

func (b *sessionHealthBuffer) put(ctx context.Context, health heartbeat.SessionHealth) error {
	owner, err := b.base.readQueries.GetSessionHealthOwner(ctx, health.SessionID)
	if err != nil {
		return fmt.Errorf("store: resolve session health owner: %w", err)
	}
	if owner != health.WorkspaceID {
		return fmt.Errorf("%w: session health workspace differs from its owner", heartbeat.ErrInvalidSessionHealth)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.base.closed.Load() != 0 {
		return store.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.putLocked(health)
	return nil
}

func (b *sessionHealthBuffer) putLocked(health heartbeat.SessionHealth) {
	b.version++
	b.pending[health.SessionID] = pendingSessionHealth{health: health, version: b.version}
}

func (b *sessionHealthBuffer) run(interval time.Duration) {
	defer close(b.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := b.flush(b.ctx); err != nil && b.ctx.Err() == nil {
				slog.WarnContext(b.ctx, "store: flush session health", "error", err)
			}
		case ctx := <-b.closing:
			b.closeErr = b.flush(ctx)
			return
		}
	}
}

func (b *sessionHealthBuffer) close(ctx context.Context) error {
	stop := context.AfterFunc(ctx, b.cancel)
	defer stop()
	defer b.cancel()
	b.closing <- ctx
	<-b.done
	return b.closeErr
}

func (b *sessionHealthBuffer) flush(ctx context.Context) error {
	b.mu.Lock()
	pending := maps.Clone(b.pending)
	b.mu.Unlock()
	operations := make([]store.BackgroundWriteOperation, 0, len(pending))
	entries := make([]pendingSessionHealth, 0, len(pending))
	for _, entry := range pending {
		entries = append(entries, entry)
		operations = append(operations, store.BackgroundWriteOperation{
			Name: "upsert session health",
			Run: func(ctx context.Context, tx *store.WriteTx) error {
				queries := sqlcgen.New(tx)
				owner, err := queries.GetSessionHealthOwner(ctx, entry.health.SessionID)
				if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != entry.health.WorkspaceID) {
					return nil
				}
				if err != nil {
					return err
				}
				return queries.UpsertSessionHealth(ctx, sessionHealthParams(entry.health))
			},
		})
	}
	results := b.base.background.SubmitBatch(ctx, operations)
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, entry := range entries {
		if results[i] == nil && b.pending[entry.health.SessionID].version == entry.version {
			delete(b.pending, entry.health.SessionID)
		}
	}
	return errors.Join(results...)
}
