package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	backgroundWriteQueueSize = 256
	backgroundWriteBatchSize = 64
	backgroundWriteDelay     = 5 * time.Millisecond
	backgroundWriteBudget    = 10 * time.Millisecond
)

type backgroundWrite struct {
	ctx       context.Context
	operation string
	fn        func(context.Context, *WriteTx) error
	result    chan error
	mu        sync.Mutex
	started   bool
}

// BackgroundWriter owns a bounded queue of coalesced writes until Close drains it.
type BackgroundWriter struct {
	db      *sql.DB
	mu      sync.Mutex
	queue   []*backgroundWrite
	closed  bool
	slots   chan struct{}
	wake    chan struct{}
	closing chan struct{}
	done    chan struct{}
}

func NewBackgroundWriter(db *sql.DB) *BackgroundWriter {
	w := &BackgroundWriter{
		db: db, slots: make(chan struct{}, backgroundWriteQueueSize),
		wake: make(chan struct{}, 1), closing: make(chan struct{}), done: make(chan struct{}),
	}
	go w.run()
	return w
}

// Submit acknowledges only committed writes; cancellation before execution removes the item.
func (w *BackgroundWriter) Submit(
	ctx context.Context, operation string, fn func(context.Context, *WriteTx) error,
) error {
	if ctx == nil || w.db == nil || fn == nil {
		return errors.New("store: background write requires context, database and callback")
	}
	for owner := writeContextParent(ctx); owner != nil; owner = owner.parent {
		if owner.db == w.db && owner.active.Load() {
			return errWriteReentry
		}
	}
	item := &backgroundWrite{ctx: ctx, operation: operation, fn: fn, result: make(chan error, 1)}
	if err := w.enqueue(item); err != nil {
		return err
	}
	select {
	case err := <-item.result:
		return err
	case <-ctx.Done():
		item.mu.Lock()
		started := item.started
		item.mu.Unlock()
		if !started {
			return ctx.Err()
		}
		// A running callback may already have committed; return its actual outcome.
		return <-item.result
	}
}

func (w *BackgroundWriter) enqueue(item *backgroundWrite) error {
	select {
	case w.slots <- struct{}{}:
	case <-item.ctx.Done():
		return item.ctx.Err()
	case <-w.closing:
		return ErrClosed
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		<-w.slots
		return ErrClosed
	}
	w.queue = append(w.queue, item)
	w.signal()
	return nil
}

// Close stops admission and waits for every accepted item to receive its commit outcome.
func (w *BackgroundWriter) Close(ctx context.Context) error {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.closing)
		w.signal()
	}
	w.mu.Unlock()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *BackgroundWriter) signal() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *BackgroundWriter) run() {
	defer close(w.done)
	admission, unregister := registerWriteAdmission(w.db)
	defer unregister()
	for {
		<-w.wake
		timer := time.NewTimer(backgroundWriteDelay)
		select {
		case <-timer.C:
		case <-w.closing:
		}
		timer.Stop()
		w.mu.Lock()
		n := min(len(w.queue), backgroundWriteBatchSize)
		batch := append([]*backgroundWrite(nil), w.queue[:n]...)
		w.queue = w.queue[n:]
		closed := w.closed
		w.mu.Unlock()
		if len(batch) == 0 && closed {
			return
		}
		if len(batch) == 0 {
			continue
		}
		completed := w.commit(batch, admission)
		w.mu.Lock()
		w.queue = append(batch[completed:], w.queue...)
		if len(w.queue) > 0 || w.closed {
			w.signal()
		}
		w.mu.Unlock()
	}
}

func (w *BackgroundWriter) commit(batch []*backgroundWrite, admission *writeAdmission) int {
	// Authenticated mutations retain their fence immediately before their own commit.
	if hasMutationCommitFence(batch[0].ctx) {
		item := batch[0]
		item.result <- ExecuteWriteOperation(item.ctx, w.db, item.operation, func(ctx context.Context, tx *WriteTx) error {
			item.mu.Lock()
			item.started = true
			item.mu.Unlock()
			return item.fn(ctx, tx)
		})
		<-w.slots
		return 1
	}
	results := make([]error, len(batch))
	completed := 0
	cfg := defaultExecuteWriteConfig()
	cfg.operation = "coalesced background writes"
	cfg.background = true
	err := executeWrite(context.Background(), w.db, cfg, func(ctx context.Context, tx *WriteTx) error {
		completed = 0
		started := time.Now()
		for i, item := range batch {
			if hasMutationCommitFence(item.ctx) {
				break
			}
			itemErr, fatal := w.executeItem(ctx, tx, item)
			results[i] = itemErr
			if fatal != nil {
				return fatal
			}
			completed++
			if admission.interactiveWaiting() || time.Since(started) >= backgroundWriteBudget {
				break
			}
		}
		return nil
	})
	if err != nil {
		completed = len(batch)
	}
	for i, item := range batch[:completed] {
		item.result <- errors.Join(results[i], err)
		<-w.slots
	}
	return completed
}

func (w *BackgroundWriter) executeItem(
	ctx context.Context, tx *WriteTx, item *backgroundWrite,
) (error, error) {
	item.mu.Lock()
	err := item.ctx.Err()
	if err == nil {
		item.started = true
	}
	item.mu.Unlock()
	if err != nil {
		return err, nil
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT background_item"); err != nil {
		return nil, err
	}
	owner := &writeContextOwner{db: w.db, parent: writeContextParent(item.ctx)}
	owner.active.Store(true)
	defer owner.active.Store(false)
	itemCtx := context.WithValue(item.ctx, writeContextKey{}, owner)
	err = item.fn(itemCtx, tx)
	if IsSQLiteBusy(err) {
		return nil, err
	}
	if err == nil {
		err = itemCtx.Err()
	}
	if err != nil {
		if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO background_item"); rollbackErr != nil {
			return err, rollbackErr
		}
		err = fmt.Errorf("store: %s: %w", item.operation, err)
	}
	if _, releaseErr := tx.ExecContext(ctx, "RELEASE background_item"); releaseErr != nil {
		return err, releaseErr
	}
	return err, nil
}

func writeContextParent(ctx context.Context) *writeContextOwner {
	owner, ok := ctx.Value(writeContextKey{}).(*writeContextOwner)
	if !ok {
		return nil
	}
	return owner
}

func hasMutationCommitFence(ctx context.Context) bool {
	fence, ok := ctx.Value(mutationCommitFenceContextKey{}).(MutationCommitFence)
	return ok && fence != nil
}
