package sessiondb

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
)

// TranscriptCountThrough counts visible entries as of a cut.
func (s *SessionDB) TranscriptCountThrough(ctx context.Context, sequence int64) (int, error) {
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return 0, store.ErrClosed
	}
	count, err := sqlcgen.New(s.readDB).CountTranscriptEntriesThrough(ctx, sequence)
	return int(count), err
}

// TranscriptCountThrough counts visible entries as of a cut.
func (s *ReadOnlySessionDB) TranscriptCountThrough(ctx context.Context, sequence int64) (int, error) {
	count, err := sqlcgen.New(s.db).CountTranscriptEntriesThrough(ctx, sequence)
	return int(count), err
}

func (l *readOnlyPoolLease) TranscriptCountThrough(ctx context.Context, sequence int64) (int, error) {
	counter, ok := l.entry.recorder.(interface {
		TranscriptCountThrough(context.Context, int64) (int, error)
	})
	if !ok {
		return 0, errors.New("store: pooled recorder cannot count transcript entries")
	}
	return counter.TranscriptCountThrough(ctx, sequence)
}
