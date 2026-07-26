package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

const defaultReadOnlyPoolTTL = 30 * time.Second

var errReadOnlyPoolClosed = errors.New("store: read-only pool is closed")

// ErrReadOnlyPoolQuiescing reports that destructive session work has blocked
// new read-only leases while existing leases drain.
var ErrReadOnlyPoolQuiescing = errors.New("store: read-only pool session is quiescing")

// ReadOnlyPoolOpener opens the reader stored behind a pooled read-only lease.
type ReadOnlyPoolOpener func(ctx context.Context, sessionID string, path string) (store.EventReadCloser, error)

// ReadOnlyPoolConfig customizes read-only recorder pooling.
type ReadOnlyPoolConfig struct {
	TTL  time.Duration
	Now  func() time.Time
	Open ReadOnlyPoolOpener
}

// ReadOnlyPool reuses short-lived read-only session database handles for hot inactive sessions.
type ReadOnlyPool struct {
	mu        sync.Mutex
	ttl       time.Duration
	now       func() time.Time
	open      ReadOnlyPoolOpener
	entries   map[readOnlyPoolKey]*readOnlyPoolEntry
	quiescing map[readOnlyPoolKey]*readOnlyPoolQuiescence
	closed    bool
}

type readOnlyPoolKey struct {
	sessionID string
	path      string
}

type readOnlyPoolEntry struct {
	recorder  store.EventReadCloser
	refs      int
	expiresAt time.Time
	idle      chan struct{}
}

type readOnlyPoolQuiescence struct {
	ready     chan struct{}
	owners    int
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
		open = func(ctx context.Context, sessionID string, path string) (store.EventReadCloser, error) {
			return OpenSessionDBReadOnly(ctx, sessionID, path)
		}
	}
	return &ReadOnlyPool{
		ttl:       ttl,
		now:       now,
		open:      open,
		entries:   make(map[readOnlyPoolKey]*readOnlyPoolEntry),
		quiescing: make(map[readOnlyPoolKey]*readOnlyPoolQuiescence),
	}
}

// Open returns a lease for a session-keyed read-only recorder.
func (p *ReadOnlyPool) Open(ctx context.Context, sessionID string, path string) (store.EventReadCloser, error) {
	if p == nil {
		return nil, errors.New("store: read-only pool is required")
	}
	if ctx == nil {
		return nil, errors.New("store: open read-only pool context is required")
	}
	key, err := normalizeReadOnlyPoolKey(sessionID, path)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errReadOnlyPoolClosed
	}
	if _, blocked := p.quiescing[key]; blocked {
		return nil, fmt.Errorf("%w: %s", ErrReadOnlyPoolQuiescing, key.sessionID)
	}
	if err := p.closeExpiredLocked(ctx, p.now()); err != nil {
		return nil, err
	}
	if entry := p.entries[key]; entry != nil {
		if entry.refs == 0 {
			entry.idle = make(chan struct{})
		}
		entry.refs++
		entry.expiresAt = time.Time{}
		return newReadOnlyPoolLease(p, key, entry), nil
	}

	recorder, err := p.open(ctx, key.sessionID, key.path)
	if err != nil {
		return nil, err
	}
	entry := &readOnlyPoolEntry{recorder: recorder, refs: 1, idle: make(chan struct{})}
	p.entries[key] = entry
	return newReadOnlyPoolLease(p, key, entry), nil
}

// Quiesce prevents new leases for one session, waits for active leases to
// return, and closes the pooled database handle. The returned function must be
// called after the caller's destructive operation finishes so failed
// operations can resume read access.
func (p *ReadOnlyPool) Quiesce(
	ctx context.Context,
	sessionID string,
	path string,
) (func(), error) {
	if p == nil {
		return nil, errors.New("store: read-only pool is required")
	}
	if ctx == nil {
		return nil, errors.New("store: quiesce read-only pool context is required")
	}
	key, err := normalizeReadOnlyPoolKey(sessionID, path)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errReadOnlyPoolClosed
	}
	if state := p.quiescing[key]; state != nil {
		state.owners++
		p.mu.Unlock()
		return p.waitForQuiescence(ctx, key, state)
	}

	state := &readOnlyPoolQuiescence{ready: make(chan struct{}), owners: 1}
	p.quiescing[key] = state
	entry := p.entries[key]
	var idle <-chan struct{}
	if entry != nil && entry.refs > 0 {
		if entry.idle == nil {
			entry.idle = make(chan struct{})
		}
		idle = entry.idle
	}
	p.mu.Unlock()

	if idle != nil {
		select {
		case <-idle:
		case <-ctx.Done():
			quiesceErr := fmt.Errorf(
				"store: quiesce read-only pool session %q: %w",
				key.sessionID,
				ctx.Err(),
			)
			p.completeQuiescence(state, quiesceErr)
			p.releaseQuiescence(key, state)
			return nil, quiesceErr
		}
	}

	var recorder store.EventReadCloser
	p.mu.Lock()
	if current := p.entries[key]; current != nil && current == entry && current.refs == 0 {
		delete(p.entries, key)
		recorder = current.recorder
	}
	p.mu.Unlock()

	var closeErr error
	if recorder != nil {
		if err := recorder.Close(ctx); err != nil {
			closeErr = fmt.Errorf(
				"store: close quiesced read-only recorder for session %q: %w",
				key.sessionID,
				err,
			)
		}
	}
	p.completeQuiescence(state, closeErr)
	return p.waitForQuiescence(ctx, key, state)
}

func (p *ReadOnlyPool) waitForQuiescence(
	ctx context.Context,
	key readOnlyPoolKey,
	state *readOnlyPoolQuiescence,
) (func(), error) {
	select {
	case <-state.ready:
	case <-ctx.Done():
		p.releaseQuiescence(key, state)
		return nil, fmt.Errorf("store: wait for read-only pool quiescence for %q: %w", key.sessionID, ctx.Err())
	}

	p.mu.Lock()
	err := state.err
	p.mu.Unlock()
	if err != nil {
		p.releaseQuiescence(key, state)
		return nil, err
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			p.releaseQuiescence(key, state)
		})
	}, nil
}

func (p *ReadOnlyPool) completeQuiescence(state *readOnlyPoolQuiescence, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if state == nil || state.completed {
		return
	}
	state.err = err
	state.completed = true
	close(state.ready)
}

func (p *ReadOnlyPool) releaseQuiescence(key readOnlyPoolKey, state *readOnlyPoolQuiescence) {
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.quiescing[key]
	if current == nil || current != state {
		return
	}
	if current.owners > 0 {
		current.owners--
	}
	if current.owners == 0 {
		delete(p.quiescing, key)
	}
}

// CloseExpired closes idle handles whose TTL has elapsed.
func (p *ReadOnlyPool) CloseExpired(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close expired read-only pool context is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeExpiredLocked(ctx, p.now())
}

// Close closes every pooled handle and rejects future opens.
func (p *ReadOnlyPool) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close read-only pool context is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for _, state := range p.quiescing {
		if state == nil || state.completed {
			continue
		}
		state.err = errReadOnlyPoolClosed
		state.completed = true
		close(state.ready)
	}
	var closeErr error
	for key, entry := range p.entries {
		delete(p.entries, key)
		if entry == nil {
			continue
		}
		if entry.idle != nil {
			close(entry.idle)
			entry.idle = nil
		}
		if entry.recorder == nil {
			continue
		}
		if err := entry.recorder.Close(ctx); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("store: close pooled read-only recorder: %w", err))
		}
	}
	return closeErr
}

func (p *ReadOnlyPool) release(key readOnlyPoolKey, entry *readOnlyPoolEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.entries[key]
	if current == nil || current != entry {
		return nil
	}
	if current.refs > 0 {
		current.refs--
	}
	if current.refs == 0 {
		current.expiresAt = p.now().Add(p.ttl)
		if current.idle != nil {
			close(current.idle)
			current.idle = nil
		}
	}
	return nil
}

func (p *ReadOnlyPool) closeExpiredLocked(ctx context.Context, now time.Time) error {
	var closeErr error
	for key, entry := range p.entries {
		if entry == nil || entry.refs > 0 || entry.expiresAt.IsZero() || entry.expiresAt.After(now) {
			continue
		}
		delete(p.entries, key)
		if entry.recorder == nil {
			continue
		}
		if err := entry.recorder.Close(ctx); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("store: close expired read-only recorder: %w", err))
		}
	}
	return closeErr
}

func normalizeReadOnlyPoolKey(sessionID string, path string) (readOnlyPoolKey, error) {
	cleanSessionID := strings.TrimSpace(sessionID)
	if cleanSessionID == "" {
		return readOnlyPoolKey{}, errors.New("store: read-only pool session id is required")
	}
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return readOnlyPoolKey{}, errors.New("store: read-only pool path is required")
	}
	return readOnlyPoolKey{
		sessionID: cleanSessionID,
		path:      filepath.Clean(cleanPath),
	}, nil
}

type readOnlyPoolLease struct {
	pool  *ReadOnlyPool
	key   readOnlyPoolKey
	entry *readOnlyPoolEntry
	once  sync.Once
	err   error
}

var _ store.EventReadCloser = (*readOnlyPoolLease)(nil)
var _ transcript.Reader = (*readOnlyPoolLease)(nil)

func newReadOnlyPoolLease(
	pool *ReadOnlyPool,
	key readOnlyPoolKey,
	entry *readOnlyPoolEntry,
) *readOnlyPoolLease {
	return &readOnlyPoolLease{
		pool:  pool,
		key:   key,
		entry: entry,
	}
}

func (l *readOnlyPoolLease) Query(
	ctx context.Context,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	if l == nil || l.entry == nil || l.entry.recorder == nil {
		return nil, errors.New("store: read-only pool lease recorder is required")
	}
	return l.entry.recorder.Query(ctx, query)
}

func (l *readOnlyPoolLease) History(
	ctx context.Context,
	query store.EventQuery,
) ([]store.TurnHistory, error) {
	if l == nil || l.entry == nil || l.entry.recorder == nil {
		return nil, errors.New("store: read-only pool lease recorder is required")
	}
	return l.entry.recorder.History(ctx, query)
}

func (l *readOnlyPoolLease) TranscriptPage(
	ctx context.Context,
	query transcript.PageQuery,
) (transcript.Page, error) {
	if l == nil || l.entry == nil || l.entry.recorder == nil {
		return transcript.Page{}, errors.New("store: read-only pool lease recorder is required")
	}
	reader, ok := l.entry.recorder.(transcript.Reader)
	if !ok {
		return transcript.Page{}, errors.New("store: pooled recorder has no transcript projection")
	}
	return reader.TranscriptPage(ctx, query)
}

func (l *readOnlyPoolLease) TranscriptChanges(
	ctx context.Context,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	if l == nil || l.entry == nil || l.entry.recorder == nil {
		return transcript.ChangePage{}, errors.New("store: read-only pool lease recorder is required")
	}
	reader, ok := l.entry.recorder.(transcript.Reader)
	if !ok {
		return transcript.ChangePage{}, errors.New("store: pooled recorder has no transcript projection")
	}
	return reader.TranscriptChanges(ctx, query)
}

func (l *readOnlyPoolLease) Close(context.Context) error {
	if l == nil || l.pool == nil || l.entry == nil {
		return nil
	}
	l.once.Do(func() {
		l.err = l.pool.release(l.key, l.entry)
	})
	return l.err
}
