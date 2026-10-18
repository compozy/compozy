package sessiondb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
)

type conversationRewindBaselineRefresh struct {
	coveredThrough int64
	messagesJSON   string
}

func (s *SessionDB) RefreshConversationRewindBaseline(
	ctx context.Context,
	coveredThrough int64,
	messagesJSON string,
) error {
	if s == nil || ctx == nil {
		return errors.New("store: conversation rewind baseline database and context are required")
	}
	if coveredThrough < 0 || !json.Valid([]byte(messagesJSON)) {
		return errors.New("store: conversation rewind baseline boundary or JSON is invalid")
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return store.ErrClosed
	}
	request := sessionWriteRequest{
		ctx: ctx, kind: sessionWriteConversationRewindBaselineRefresh,
		baseline: &conversationRewindBaselineRefresh{coveredThrough: coveredThrough, messagesJSON: messagesJSON},
		result:   make(chan sessionWriteResult, 1),
	}
	select {
	case s.writeCh <- request:
	case <-ctx.Done():
		return fmt.Errorf("store: enqueue conversation rewind baseline refresh: %w", ctx.Err())
	}
	select {
	case result := <-request.result:
		return result.err
	case <-ctx.Done():
		return fmt.Errorf("store: wait for conversation rewind baseline refresh: %w", ctx.Err())
	}
}

func (s *SessionDB) writeConversationRewindBaselineRefresh(
	ctx context.Context,
	request *conversationRewindBaselineRefresh,
) error {
	if request == nil {
		return errors.New("store: conversation rewind baseline refresh request is required")
	}
	return store.ExecuteWrite(ctx, s.db, func(ctx context.Context, tx *store.WriteTx) error {
		count, err := sqlcgen.New(tx).
			RefreshConversationRewindBaseline(ctx, sqlcgen.RefreshConversationRewindBaselineParams{
				MessagesJson: request.messagesJSON, CoveredThroughSequence: request.coveredThrough,
				UpdatedAt: s.now().UTC().Format(time.RFC3339Nano),
			})
		if err != nil {
			return fmt.Errorf("store: refresh conversation rewind baseline: %w", err)
		}
		if count != 1 {
			return store.ErrConversationRewindFenceConflict
		}
		return nil
	})
}
