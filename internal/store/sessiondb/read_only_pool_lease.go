package sessiondb

import (
	"context"
	"errors"
	"sync"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type readOnlyPoolLease struct {
	pool  *ReadOnlyPool
	key   readOnlyPoolKey
	entry *readOnlyPoolEntry
	once  sync.Once
	err   error
}

var _ store.EventReadCloser = (*readOnlyPoolLease)(nil)
var _ transcript.Reader = (*readOnlyPoolLease)(nil)
var _ store.ConversationRewindReader = (*readOnlyPoolLease)(nil)

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

func (l *readOnlyPoolLease) ConversationRewindTarget(
	ctx context.Context,
	messageID string,
) (store.ConversationRewindTarget, error) {
	reader, err := l.conversationRewindReader()
	if err != nil {
		return store.ConversationRewindTarget{}, err
	}
	return reader.ConversationRewindTarget(ctx, messageID)
}

func (l *readOnlyPoolLease) ConversationRewindState(
	ctx context.Context,
) (store.ConversationRewindState, bool, error) {
	reader, err := l.conversationRewindReader()
	if err != nil {
		return store.ConversationRewindState{}, false, err
	}
	return reader.ConversationRewindState(ctx)
}

func (l *readOnlyPoolLease) ConversationRewindReceipt(
	ctx context.Context,
	idempotencyKey string,
	requestHash string,
) (store.ConversationRewindResult, bool, error) {
	reader, err := l.conversationRewindReader()
	if err != nil {
		return store.ConversationRewindResult{}, false, err
	}
	return reader.ConversationRewindReceipt(ctx, idempotencyKey, requestHash)
}

func (l *readOnlyPoolLease) conversationRewindReader() (store.ConversationRewindReader, error) {
	if l == nil || l.entry == nil || l.entry.recorder == nil {
		return nil, errors.New("store: read-only pool lease recorder is required")
	}
	reader, ok := l.entry.recorder.(store.ConversationRewindReader)
	if !ok {
		return nil, errors.New("store: pooled recorder has no conversation rewind projection")
	}
	return reader, nil
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
