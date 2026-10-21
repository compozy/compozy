package session

import (
	"context"
	"strings"
)

// creatorAwaitsChildReply reports whether the creator holds an open reply watch on the
// child's just-completed turn. That watch already wakes the creator with the turn's
// final answer, so the bare "completed" spawn wake would only duplicate it. A watch on
// a message still queued behind this turn does not cover it.
func (m *Manager) creatorAwaitsChildReply(
	ctx context.Context,
	creatorSessionID string,
	childSessionID string,
	completedTurnID string,
) bool {
	turnID := strings.TrimSpace(completedTurnID)
	service, ok := m.ReplyWatches().(*replyWatchService)
	if turnID == "" || !ok || service == nil || service.store == nil {
		return false
	}
	watches, err := service.store.ListPendingReplyWatchesBySender(ctx, strings.TrimSpace(creatorSessionID))
	if err != nil {
		m.logReplyWatchLookupFailure(ctx, creatorSessionID, childSessionID, err)
		return false
	}
	for _, watch := range watches {
		if strings.TrimSpace(watch.TargetSessionID) != strings.TrimSpace(childSessionID) {
			continue
		}
		if watch.TurnID != "" {
			if watch.TurnID == turnID {
				return true
			}
			continue
		}
		admission, inputs, err := service.store.ReplyWatchEvidence(ctx, watch)
		if err != nil {
			m.logReplyWatchLookupFailure(ctx, creatorSessionID, childSessionID, err)
			continue
		}
		consumed, err := m.replyMessageTurn(ctx, watch, admission, inputs, false)
		if err != nil {
			m.logReplyWatchLookupFailure(ctx, creatorSessionID, childSessionID, err)
			continue
		}
		if consumed == turnID {
			return true
		}
	}
	return false
}

func (m *Manager) logReplyWatchLookupFailure(ctx context.Context, creatorSessionID, childSessionID string, err error) {
	m.logger.WarnContext(
		ctx,
		"session.spawn_wake.reply_watch_lookup_failed",
		"creator_session_id", creatorSessionID,
		"child_session_id", childSessionID,
		"error", SanitizeSpawnWakeText(err.Error()),
	)
}
