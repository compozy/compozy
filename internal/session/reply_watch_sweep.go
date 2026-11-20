package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	replyWatchSweepInterval = 30 * time.Second
	replyResultUnreadable   = "The reply could not be read. Check the target session."
)

func (s *replyWatchService) sweep(ctx context.Context) {
	rows, err := s.store.ListReplyWatches(ctx, store.ReplyWatchFilter{})
	if err != nil {
		s.logError(ctx, err)
		return
	}
	active := make(map[string]bool)
	pending := make(map[string]bool, len(rows))
	for _, w := range rows {
		if ctx.Err() != nil {
			return
		}
		pending[w.ID] = true
		if w.State == store.ReplyWatchFired {
			s.logWatchError(ctx, w, s.Deliver(ctx, w.ID))
			continue
		}
		if s.replyWatchExpired(w, s.manager.now()) {
			s.logWatchError(ctx, w, s.expire(ctx, w))
			continue
		}
		busy, known := active[w.TargetSessionID]
		if !known {
			_, err := s.manager.ActivePromptRun(ctx, w.TargetSessionID)
			busy = !errors.Is(err, ErrPromptNotActive) || s.manager.IsPrompting(w.TargetSessionID)
			active[w.TargetSessionID] = busy
		}
		if busy {
			s.settleGrace.Delete(w.ID)
			continue
		}
		s.logWatchError(ctx, w, s.reconcileArmed(ctx, w, replyWatchPeriodic))
	}
	s.settleGrace.Range(func(id, _ any) bool {
		if watchID, ok := id.(string); ok && !pending[watchID] {
			s.settleGrace.Delete(id)
		}
		return true
	})
}

func (s *replyWatchService) settleReplyAfterGrace(
	ctx context.Context, w store.ReplyWatch, pass replyWatchPass, outcome, text string, err error,
) error {
	if pass != replyWatchPeriodic {
		return err
	}
	if s.manager.IsPrompting(w.TargetSessionID) {
		s.settleGrace.Delete(w.ID)
		return err
	}
	now := s.manager.now()
	since, _ := s.settleGrace.LoadOrStore(w.ID, now)
	if observed, ok := since.(time.Time); !ok || now.Sub(observed) < replyWatchSweepInterval {
		return err
	}
	return errors.Join(err, s.fire(ctx, w, outcome, text))
}

func (s *replyWatchService) logWatchError(ctx context.Context, w store.ReplyWatch, err error) {
	if err != nil {
		s.manager.logger.ErrorContext(ctx, "reply_watch.reconcile_failed",
			"id", w.ID, "sender_session_id", w.SenderSessionID, "target_session_id", w.TargetSessionID, "error", err)
	}
}
