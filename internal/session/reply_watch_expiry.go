package session

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// replyWatchTTL is how long an armed watch may wait for the target before it expires. Zero, the
// default, never expires a watch: a sender waiting on a long-running target stays awake.
func (s *replyWatchService) replyWatchTTL() time.Duration {
	return s.manager.messagingConfig().ReplyWatchTTL
}

func (s *replyWatchService) replyWatchExpired(w store.ReplyWatch, now time.Time) bool {
	ttl := s.replyWatchTTL()
	return ttl > 0 && w.State == store.ReplyWatchArmed && !w.CreatedAt.IsZero() && now.Sub(w.CreatedAt) >= ttl
}

// expireStale fires every armed watch past its TTL with the expired outcome, which wakes the sender
// once and stops the watch from keeping the sender awake.
func (s *replyWatchService) expireStale(ctx context.Context) {
	if s.replyWatchTTL() <= 0 {
		return
	}
	rows, err := s.store.ListReplyWatches(ctx, store.ReplyWatchFilter{State: store.ReplyWatchArmed})
	if err != nil {
		s.logError(ctx, err)
		return
	}
	now := s.manager.now()
	for _, w := range rows {
		if !s.replyWatchExpired(w, now) {
			continue
		}
		s.logError(ctx, s.expire(ctx, w))
	}
}

func (s *replyWatchService) expire(ctx context.Context, w store.ReplyWatch) error {
	text := fmt.Sprintf(
		"No reply arrived within %s, so the reply watch expired. The message may still be pending; "+
			"check the target session before resending.",
		s.replyWatchTTL(),
	)
	s.manager.logger.InfoContext(ctx, "reply_watch.expired", "id", w.ID, "created_at", w.CreatedAt)
	return s.fire(ctx, w, store.ReplyOutcomeExpired, text)
}
