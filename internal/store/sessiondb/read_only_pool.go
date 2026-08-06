package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	defaultReadOnlyPoolTTL         = 30 * time.Second
	defaultReadOnlyPoolOpenTimeout = 30 * time.Second
)

var (
	errReadOnlyPoolClosed               = errors.New("store: read-only pool is closed")
	errReadOnlyPoolCloseDependencyCycle = errors.New("store: read-only pool close dependency cycle")
)

// ErrReadOnlyPoolQuiescing reports that destructive session work has blocked
// new read-only leases while existing leases drain.
var ErrReadOnlyPoolQuiescing = errors.New("store: read-only pool session is quiescing")

// ReadOnlyPoolOpener opens the reader stored behind a pooled read-only lease.
type ReadOnlyPoolOpener func(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (store.EventReadCloser, error)

// ReadOnlyPoolConfig customizes read-only recorder pooling.
type ReadOnlyPoolConfig struct {
	TTL  time.Duration
	Now  func() time.Time
	Open ReadOnlyPoolOpener
}

// ReadOnlyPool reuses short-lived read-only session database handles for hot inactive sessions.
type ReadOnlyPool struct {
	mu          sync.Mutex
	ttl         time.Duration
	openTimeout time.Duration
	now         func() time.Time
	open        ReadOnlyPoolOpener
	entries     map[readOnlyPoolKey]*readOnlyPoolEntry
	openings    map[readOnlyPoolKey]*readOnlyPoolOpening
	closings    map[readOnlyPoolKey]map[*readOnlyPoolClose]struct{}
	quiescing   map[readOnlyPoolKey]*readOnlyPoolQuiescence
	closed      bool
	shutdown    *readOnlyPoolShutdown
}

type readOnlyPoolKey struct {
	owner store.SessionDBOwner
	path  string
}

type readOnlyPoolEntry struct {
	recorder  store.EventReadCloser
	refs      int
	expiresAt time.Time
	idle      chan struct{}
}

type readOnlyPoolOpening struct {
	ready    chan struct{}
	err      error
	closeErr error
	closeOp  *readOnlyPoolClose
}

type readOnlyPoolClose struct {
	key                   readOnlyPoolKey
	recorder              store.EventReadCloser
	op                    string
	runCtx                context.Context
	ready                 chan struct{}
	err                   error
	opening               *readOnlyPoolOpening
	quiescence            *readOnlyPoolQuiescence
	responsibility        *readOnlyPoolQuiescenceResponsibility
	deferredQuiescence    *readOnlyPoolQuiescence
	started               bool
	waitingForClosings    map[*readOnlyPoolClose]int
	waitingForOpenings    map[*readOnlyPoolOpening]int
	waitingForQuiescences map[*readOnlyPoolQuiescence]int
}

type readOnlyPoolCloseContextKey struct{}

// readOnlyPoolCloseExecution is a private lifecycle capability propagated only
// while invoking a recorder close. Re-entrant pool barriers use it to avoid
// waiting on the callback whose return will publish that close operation.
type readOnlyPoolCloseExecution struct {
	pool      *ReadOnlyPool
	operation *readOnlyPoolClose
}

type readOnlyPoolQuiescence struct {
	ready     chan struct{}
	owners    int
	err       error
	completed bool
	closings  map[*readOnlyPoolClose]struct{}
	openings  map[*readOnlyPoolOpening]struct{}
	entries   map[*readOnlyPoolEntry]*readOnlyPoolQuiescenceResponsibility
}

type readOnlyPoolQuiescenceStart struct {
	state    *readOnlyPoolQuiescence
	entry    *readOnlyPoolEntry
	opening  *readOnlyPoolOpening
	idle     <-chan struct{}
	deferred bool
}

type readOnlyPoolShutdown struct {
	ready     chan struct{}
	closings  []*readOnlyPoolClose
	openings  []*readOnlyPoolOpening
	err       error
	completed bool
}

// NewReadOnlyPool constructs a read-only session recorder pool.
func NewReadOnlyPool(config ReadOnlyPoolConfig) *ReadOnlyPool {
	ttl := config.TTL
	if ttl <= 0 {
		ttl = defaultReadOnlyPoolTTL
	}
	now := config.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	open := config.Open
	if open == nil {
		open = func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
			return OpenSessionDBReadOnly(ctx, owner, path)
		}
	}
	return &ReadOnlyPool{
		ttl:         ttl,
		openTimeout: defaultReadOnlyPoolOpenTimeout,
		now:         now,
		open:        open,
		entries:     make(map[readOnlyPoolKey]*readOnlyPoolEntry),
		openings:    make(map[readOnlyPoolKey]*readOnlyPoolOpening),
		closings:    make(map[readOnlyPoolKey]map[*readOnlyPoolClose]struct{}),
		quiescing:   make(map[readOnlyPoolKey]*readOnlyPoolQuiescence),
	}
}

// Open returns a lease for a session-keyed read-only recorder.
func (p *ReadOnlyPool) Open(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (store.EventReadCloser, error) {
	if p == nil {
		return nil, errors.New("store: read-only pool is required")
	}
	if ctx == nil {
		return nil, errors.New("store: open read-only pool context is required")
	}
	key, err := normalizeReadOnlyPoolKey(owner, path)
	if err != nil {
		return nil, err
	}
	if err := p.checkOpenAdmission(key); err != nil {
		return nil, err
	}
	if err := p.closeExpired(ctx); err != nil {
		return nil, err
	}
	return p.openLease(ctx, key)
}

func (p *ReadOnlyPool) checkOpenAdmission(key readOnlyPoolKey) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.openAdmissionErrorLocked(key)
}

func (p *ReadOnlyPool) openAdmissionErrorLocked(key readOnlyPoolKey) error {
	if p.closed {
		return errReadOnlyPoolClosed
	}
	if p.isQuiescingLocked(key) {
		return fmt.Errorf("%w: %s", ErrReadOnlyPoolQuiescing, key.owner.SessionID)
	}
	return nil
}

func (p *ReadOnlyPool) openLease(ctx context.Context, key readOnlyPoolKey) (store.EventReadCloser, error) {
	p.mu.Lock()
	if err := p.openAdmissionErrorLocked(key); err != nil {
		p.mu.Unlock()
		return nil, err
	}
	if entry := p.entries[key]; entry != nil {
		lease := p.claimEntryLocked(key, entry)
		p.mu.Unlock()
		return lease, nil
	}
	opening := p.openings[key]
	if opening == nil {
		opening = &readOnlyPoolOpening{ready: make(chan struct{})}
		p.openings[key] = opening
		p.mu.Unlock()

		p.runOpening(ctx, key, opening)
		return p.claimOpenedLease(ctx, key, opening)
	}
	p.mu.Unlock()
	return p.claimOpenedLease(ctx, key, opening)
}

func (p *ReadOnlyPool) runOpening(
	ctx context.Context,
	key readOnlyPoolKey,
	opening *readOnlyPoolOpening,
) {
	openingCtx, cancel := p.newOpeningContext(ctx)
	defer cancel()
	recorder, err := p.open(openingCtx, key.owner, key.path)
	p.finishOpening(openingCtx, key, opening, recorder, err)
}

func (p *ReadOnlyPool) newOpeningContext(ctx context.Context) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(ctx)
	bounded, cancel := context.WithTimeout(detached, p.openTimeout)
	callerDeadline, hasCallerDeadline := ctx.Deadline()
	boundedDeadline, hasBoundedDeadline := bounded.Deadline()
	if !hasCallerDeadline || !hasBoundedDeadline || !callerDeadline.Before(boundedDeadline) {
		return bounded, cancel
	}
	cancel()
	return context.WithDeadline(detached, callerDeadline)
}

func (p *ReadOnlyPool) claimOpenedLease(
	ctx context.Context,
	key readOnlyPoolKey,
	opening *readOnlyPoolOpening,
) (store.EventReadCloser, error) {
	select {
	case <-opening.ready:
	case <-ctx.Done():
		select {
		case <-opening.ready:
		default:
			return nil, fmt.Errorf(
				"store: wait for read-only pool opener for %q: %w",
				key.owner.SessionID,
				ctx.Err(),
			)
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if opening.err != nil {
		return nil, opening.err
	}
	if p.closed {
		return nil, errReadOnlyPoolClosed
	}
	if p.isQuiescingLocked(key) {
		return nil, fmt.Errorf("%w: %s", ErrReadOnlyPoolQuiescing, key.owner.SessionID)
	}
	if entry := p.entries[key]; entry != nil {
		return p.claimEntryLocked(key, entry), nil
	}
	return nil, errors.New("store: read-only pool opener completed without a recorder")
}

func (p *ReadOnlyPool) finishOpening(
	ctx context.Context,
	key readOnlyPoolKey,
	opening *readOnlyPoolOpening,
	recorder store.EventReadCloser,
	openErr error,
) {
	p.mu.Lock()
	if recorder == nil {
		if openErr == nil {
			openErr = errors.New("store: read-only pool opener returned nil recorder")
		}
		p.completeOpeningLocked(key, opening, openErr, nil)
		p.mu.Unlock()
		return
	}

	result := openErr
	shouldClose := openErr != nil || p.closed || p.isQuiescingLocked(key)
	if !shouldClose {
		p.entries[key] = &readOnlyPoolEntry{recorder: recorder}
		p.completeOpeningLocked(key, opening, nil, nil)
		p.mu.Unlock()
		return
	}
	if result == nil {
		if p.closed {
			result = errReadOnlyPoolClosed
		} else {
			result = fmt.Errorf("%w: %s", ErrReadOnlyPoolQuiescing, key.owner.SessionID)
		}
	}
	closeOp := p.startCloseLocked(
		ctx,
		key,
		recorder,
		fmt.Sprintf("close unopened read-only recorder for session %q", key.owner.SessionID),
	)
	closeOp.opening = opening
	opening.closeOp = closeOp
	p.mu.Unlock()

	p.runClose(closeOp)
	closeErr := p.waitForCloseOperations(ctx, []*readOnlyPoolClose{closeOp})
	if closeErr != nil {
		result = errors.Join(result, closeErr)
	}

	p.mu.Lock()
	p.completeOpeningLocked(key, opening, result, closeErr)
	p.mu.Unlock()
}

func (p *ReadOnlyPool) completeOpeningLocked(
	key readOnlyPoolKey,
	opening *readOnlyPoolOpening,
	err error,
	closeErr error,
) {
	opening.err = err
	opening.closeErr = closeErr
	if state := p.quiescing[key]; state != nil {
		delete(state.openings, opening)
	}
	delete(p.openings, key)
	close(opening.ready)
}

func (p *ReadOnlyPool) claimEntryLocked(key readOnlyPoolKey, entry *readOnlyPoolEntry) *readOnlyPoolLease {
	if entry.refs == 0 {
		entry.idle = make(chan struct{})
	}
	entry.refs++
	entry.expiresAt = time.Time{}
	return newReadOnlyPoolLease(p, key, entry)
}

func (p *ReadOnlyPool) isQuiescingLocked(key readOnlyPoolKey) bool {
	state := p.quiescing[key]
	return state != nil && (!state.completed || state.err == nil)
}

// CloseExpired closes idle handles whose TTL has elapsed.
func (p *ReadOnlyPool) CloseExpired(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close expired read-only pool context is required")
	}
	return p.closeExpired(ctx)
}

func (p *ReadOnlyPool) closeExpired(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	closeOps := p.detachExpiredLocked(ctx, p.now())
	p.mu.Unlock()
	p.runCloseAll(closeOps)
	return p.waitForCloseOperations(ctx, closeOps)
}

func (p *ReadOnlyPool) detachExpiredLocked(ctx context.Context, now time.Time) []*readOnlyPoolClose {
	var closeOps []*readOnlyPoolClose
	for key, entry := range p.entries {
		if entry == nil || entry.refs > 0 || entry.expiresAt.IsZero() || entry.expiresAt.After(now) {
			continue
		}
		delete(p.entries, key)
		if entry.recorder == nil {
			p.completeEntryResponsibilityLocked(key, entry, nil)
			continue
		}
		closeOps = append(
			closeOps,
			p.startEntryCloseLocked(ctx, key, entry, "close expired read-only recorder"),
		)
	}
	return closeOps
}

func normalizeReadOnlyPoolKey(owner store.SessionDBOwner, path string) (readOnlyPoolKey, error) {
	normalizedOwner, err := owner.Normalize()
	if err != nil {
		return readOnlyPoolKey{}, err
	}
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return readOnlyPoolKey{}, errors.New("store: read-only pool path is required")
	}
	return readOnlyPoolKey{
		owner: normalizedOwner,
		path:  filepath.Clean(cleanPath),
	}, nil
}
