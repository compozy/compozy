package clientstate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type operationKind uint8

const (
	regularOperation operationKind = iota
	purgeOperation
)

// Engine is the bbolt-backed client-state service.
type Engine struct {
	store    *stateStore
	resolver WorkspaceResolver
	limits   Limits
	logger   *slog.Logger
	now      func() time.Time
	metrics  *runtimeMetrics

	mu         sync.Mutex
	closed     bool
	sequencers map[WorkspaceID]*workspaceSequencer
	closeOnce  sync.Once
	closeDone  chan struct{}
	closeErr   error
}

var _ io.Closer = (*Engine)(nil)

// Open constructs a client-state engine over path.
func Open(
	path string,
	resolver WorkspaceResolver,
	limits Limits,
	options ...Option,
) (*Engine, error) {
	if resolver == nil {
		return nil, errors.New("clientstate: workspace resolver is required")
	}
	if err := limits.validate(); err != nil {
		return nil, err
	}
	resolvedOptions, err := resolveEngineOptions(options)
	if err != nil {
		return nil, err
	}
	store, err := openStateStore(path, resolvedOptions.openTimeout)
	if err != nil {
		return nil, err
	}
	engine := &Engine{
		store:      store,
		resolver:   resolver,
		limits:     limits,
		logger:     resolvedOptions.logger,
		now:        resolvedOptions.now,
		metrics:    &runtimeMetrics{},
		sequencers: make(map[WorkspaceID]*workspaceSequencer),
		closeDone:  make(chan struct{}),
	}
	if err := engine.recoverWorkspacePurges(context.Background()); err != nil {
		closeErr := store.close()
		return nil, errors.Join(err, closeErr)
	}
	engine.logger.Info("clientstate.store.open", "path", path)
	return engine, nil
}

// Get returns one live entry.
func (e *Engine) Get(
	ctx context.Context,
	ws WorkspaceID,
	domain string,
	key string,
) (Entry, error) {
	var entry Entry
	err := e.execute(
		ctx,
		ws,
		regularOperation,
		func(runCtx context.Context, _ *workspaceSequencer) error {
			var getErr error
			entry, getErr = e.store.get(runCtx, ws, domain, key)
			return getErr
		},
	)
	if err != nil {
		return Entry{}, err
	}
	return cloneEntry(entry), nil
}

// List returns all live entries in one domain sorted by key.
func (e *Engine) List(
	ctx context.Context,
	ws WorkspaceID,
	domain string,
) ([]Entry, error) {
	var entries []Entry
	err := e.execute(
		ctx,
		ws,
		regularOperation,
		func(runCtx context.Context, _ *workspaceSequencer) error {
			var listErr error
			entries, listErr = e.store.list(runCtx, ws, domain)
			return listErr
		},
	)
	if err != nil {
		return nil, err
	}
	return cloneEntries(entries), nil
}

// Apply atomically commits and publishes a mutation batch.
func (e *Engine) Apply(
	ctx context.Context,
	ws WorkspaceID,
	domain string,
	ops []Op,
	opts ApplyOptions,
) (entries []Entry, err error) {
	start := time.Now()
	defer func() {
		e.metrics.applyObserved(time.Since(start), err)
		if errors.Is(err, ErrRevConflict) {
			e.metrics.revConflictObserved()
		}
	}()

	err = e.execute(
		ctx,
		ws,
		regularOperation,
		func(runCtx context.Context, sequencer *workspaceSequencer) error {
			committed, applyErr := e.store.apply(runCtx, ws, domain, ops, e.limits, e.now())
			if applyErr != nil {
				return applyErr
			}
			events := make([]Event, len(committed))
			for index, entry := range committed {
				events[index] = Event{Workspace: ws, Entry: cloneEntry(entry), Origin: opts.Origin}
			}
			sequencer.hub.publish(events)
			for _, entry := range committed {
				e.logger.Debug(
					"clientstate.apply",
					"workspace", ws,
					"domain", domain,
					"key", entry.Key,
					"rev", entry.Rev,
					"seq", entry.Seq,
					"conn_id", opts.Origin,
				)
			}
			entries = committed
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return cloneEntries(entries), nil
}

// Watch returns a snapshot and a gap-free stream of later workspace commits.
func (e *Engine) Watch(
	ctx context.Context,
	ws WorkspaceID,
	domains []string,
) (Subscription, error) {
	domainList := append([]string(nil), domains...)
	var subscription *stateSubscription
	err := e.execute(
		ctx,
		ws,
		regularOperation,
		func(runCtx context.Context, sequencer *workspaceSequencer) error {
			selected, normalizeErr := normalizeDomains(domainList)
			if normalizeErr != nil {
				return normalizeErr
			}
			start := time.Now()
			asOfSeq, snapshot, snapshotErr := e.store.snapshot(runCtx, ws, selected)
			if snapshotErr != nil {
				return snapshotErr
			}
			e.metrics.snapshotObserved(snapshot, time.Since(start))
			subscription = sequencer.hub.add(asOfSeq, snapshot, selected)
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	go closeSubscriptionWithContext(ctx, subscription)
	return subscription, nil
}

// PurgeWorkspace closes workspace subscriptions and deletes its entire bucket.
func (e *Engine) PurgeWorkspace(ctx context.Context, ws WorkspaceID) error {
	preparation, err := e.PrepareWorkspacePurge(ctx, ws)
	if err != nil {
		return err
	}
	return preparation.Commit(ctx)
}

// PrepareWorkspacePurge closes subscriptions and removes the live workspace
// bucket while retaining a rollback copy until the owning row deletion commits.
func (e *Engine) PrepareWorkspacePurge(
	ctx context.Context,
	ws WorkspaceID,
) (WorkspacePurgePreparation, error) {
	err := e.execute(
		ctx,
		ws,
		purgeOperation,
		func(runCtx context.Context, sequencer *workspaceSequencer) error {
			sequencer.hub.closeAll(nil)
			if purgeErr := e.store.stagePurge(runCtx, ws); purgeErr != nil {
				e.metrics.purgeFailureObserved()
				e.logger.Warn(
					"clientstate.workspace.purge_failed",
					"workspace", ws,
					"error", purgeErr,
				)
				return purgeErr
			}
			e.logger.Info("clientstate.workspace.purge", "workspace", ws)
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &workspacePurgePreparation{store: e.store, workspace: ws}, nil
}

// Metrics returns a consistent low-cardinality runtime snapshot.
func (e *Engine) Metrics() MetricsSnapshot {
	return e.metrics.snapshot()
}

// ConnectionOpened records one transport connection.
func (e *Engine) ConnectionOpened() {
	e.metrics.connectionOpened()
}

// ConnectionClosed records one transport connection closure.
func (e *Engine) ConnectionClosed() {
	e.metrics.connectionClosed()
}

// RecordReconnect records one transport reconnect.
func (e *Engine) RecordReconnect() {
	e.metrics.reconnectRecorded()
}

// RecordOutboundQueueDepth observes one WebSocket writer-queue sample.
func (e *Engine) RecordOutboundQueueDepth(depth int) {
	e.metrics.observeQueueDepth(depth)
}

// RecordSlowConsumerEviction records a transport-level writer-queue eviction.
func (e *Engine) RecordSlowConsumerEviction() {
	e.metrics.slowConsumerEvicted()
}

// Close stops sequencers and subscriptions before closing bbolt.
func (e *Engine) Close() error {
	e.closeOnce.Do(func() {
		e.mu.Lock()
		e.closed = true
		sequencers := make([]*workspaceSequencer, 0, len(e.sequencers))
		for _, sequencer := range e.sequencers {
			sequencers = append(sequencers, sequencer)
		}
		e.mu.Unlock()

		for _, sequencer := range sequencers {
			sequencer.shutdown()
		}
		metrics := e.Metrics()
		e.closeErr = e.store.close()
		if e.closeErr != nil {
			e.logger.Warn("clientstate.store.close_failed", "error", e.closeErr)
		} else {
			e.logger.Info(
				"clientstate.store.close",
				"applies", metrics.Applies,
				"apply_errors", metrics.ApplyErrors,
				"rev_conflicts", metrics.RevConflicts,
				"snapshots", metrics.Snapshots,
				"slow_consumer_evictions", metrics.SlowConsumerEvictions,
				"purge_failures", metrics.PurgeFailures,
			)
		}
		close(e.closeDone)
	})
	<-e.closeDone
	return e.closeErr
}

type sequenceOperation func(context.Context, *workspaceSequencer) error

func (e *Engine) execute(
	ctx context.Context,
	ws WorkspaceID,
	kind operationKind,
	operation sequenceOperation,
) error {
	if err := e.ensureOpen(); err != nil {
		return err
	}
	generation, err := e.resolveWorkspace(ctx, ws, kind)
	if err != nil {
		return err
	}
	sequencer, err := e.sequencerFor(ws)
	if err != nil {
		return err
	}
	return sequencer.submit(ctx, func(runCtx context.Context) error {
		currentGeneration, resolveErr := e.resolveWorkspace(runCtx, ws, kind)
		if resolveErr != nil {
			return resolveErr
		}
		if currentGeneration != generation {
			return fmt.Errorf(
				"clientstate: workspace %q generation changed from %q to %q: %w",
				ws,
				generation,
				currentGeneration,
				ErrWorkspaceNotFound,
			)
		}
		return operation(runCtx, sequencer)
	})
}

func (e *Engine) ensureOpen() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	return nil
}

func (e *Engine) resolveWorkspace(
	ctx context.Context,
	ws WorkspaceID,
	kind operationKind,
) (WorkspaceGeneration, error) {
	if err := contextError(ctx); err != nil {
		return "", err
	}
	if strings.TrimSpace(string(ws)) == "" {
		return "", ErrWorkspaceNotFound
	}
	var (
		generation WorkspaceGeneration
		err        error
	)
	if kind == purgeOperation {
		generation, err = e.resolver.ResolveWorkspaceForPurge(ctx, ws)
	} else {
		generation, err = e.resolver.ResolveWorkspace(ctx, ws)
	}
	if err != nil {
		return "", fmt.Errorf("clientstate: resolve workspace %q: %w", ws, err)
	}
	if strings.TrimSpace(string(generation)) == "" {
		return "", fmt.Errorf("clientstate: workspace %q has no generation: %w", ws, ErrWorkspaceNotFound)
	}
	return generation, nil
}

func (e *Engine) sequencerFor(ws WorkspaceID) (*workspaceSequencer, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, ErrClosed
	}
	if sequencer := e.sequencers[ws]; sequencer != nil {
		return sequencer, nil
	}
	hub := newSubscriptionHub(ws, e.logger, e.metrics)
	sequencer := newWorkspaceSequencer(hub)
	e.sequencers[ws] = sequencer
	return sequencer, nil
}

func closeSubscriptionWithContext(ctx context.Context, subscription *stateSubscription) {
	select {
	case <-ctx.Done():
		subscription.close()
	case <-subscription.done:
	}
}
