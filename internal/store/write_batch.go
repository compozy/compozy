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

type BackgroundWriteOperation struct {
	Name string
	Run  func(context.Context, *WriteTx) error
}

// BackgroundWriter owns a bounded queue of coalesced writes until Close drains it.
type BackgroundWriter struct {
	db       *sql.DB
	ctx      context.Context
	cancel   context.CancelCauseFunc
	mu       sync.Mutex
	queue    []*backgroundWrite
	closed   bool
	closeErr error
	slots    chan struct{}
	wake     chan struct{}
	closing  chan struct{}
	done     chan struct{}
}

func NewBackgroundWriter(db *sql.DB) *BackgroundWriter {
	w := &BackgroundWriter{
		db: db, slots: make(chan struct{}, backgroundWriteQueueSize),
		wake: make(chan struct{}, 1), closing: make(chan struct{}), done: make(chan struct{}),
	}
	w.ctx, w.cancel = context.WithCancelCause(context.Background())
	go w.run()
	return w
}

// Submit acknowledges only committed writes; cancellation before execution removes the item.
func (w *BackgroundWriter) Submit(
	ctx context.Context, operation string, fn func(context.Context, *WriteTx) error,
) error {
	item, err := w.submit(ctx, operation, fn)
	if err != nil {
		return err
	}
	return item.wait()
}

// SubmitBatch returns one committed outcome per operation, in submission order.
func (w *BackgroundWriter) SubmitBatch(ctx context.Context, operations []BackgroundWriteOperation) []error {
	items := make([]*backgroundWrite, len(operations))
	results := make([]error, len(operations))
	for i, operation := range operations {
		items[i], results[i] = w.submit(ctx, operation.Name, operation.Run)
	}
	for i, item := range items {
		if item != nil {
			results[i] = item.wait()
		}
	}
	return results
}

func (w *BackgroundWriter) submit(
	ctx context.Context, operation string, fn func(context.Context, *WriteTx) error,
) (*backgroundWrite, error) {
	if ctx == nil || w.db == nil || fn == nil {
		return nil, errors.New("store: background write requires context, database and callback")
	}
	for owner := writeContextParent(ctx); owner != nil; owner = owner.parent {
		if owner.db == w.db && owner.active.Load() {
			return nil, errWriteReentry
		}
	}
	item := &backgroundWrite{ctx: ctx, operation: operation, fn: fn, result: make(chan error, 1)}
	if err := w.enqueue(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (item *backgroundWrite) wait() error {
	select {
	case err := <-item.result:
		return err
	case <-item.ctx.Done():
		item.mu.Lock()
		started := item.started
		item.mu.Unlock()
		if !started {
			return item.ctx.Err()
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
	if ctx == nil {
		return errors.New("store: close background writer context is required")
	}
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.closing)
		w.signal()
	}
	w.mu.Unlock()
	select {
	case <-w.done:
	case <-ctx.Done():
		w.mu.Lock()
		if w.closeErr == nil {
			w.closeErr = ctx.Err()
		}
		w.cancel(w.closeErr)
		w.mu.Unlock()
		<-w.done
	}
	w.cancel(nil)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeErr
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
		ctx, cancel := w.itemContext(item.ctx)
		defer cancel()
		item.result <- ExecuteWriteOperation(ctx, w.db, item.operation, func(ctx context.Context, tx *WriteTx) error {
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
	err := executeWrite(w.ctx, w.db, cfg, func(ctx context.Context, tx *WriteTx) error {
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
	itemCtx, cancel := w.itemContext(context.WithValue(item.ctx, writeContextKey{}, owner))
	defer cancel()
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

func (w *BackgroundWriter) itemContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(w.ctx, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
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
