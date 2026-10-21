package session

import (
	"context"
	"strings"
)

// creatorAwaitsChildReply reports whether the creator holds an open reply watch on the
// child. That watch already wakes the creator with the child's final answer, so the
// bare "completed" spawn wake would only duplicate it.
func (m *Manager) creatorAwaitsChildReply(ctx context.Context, creatorSessionID, childSessionID string) bool {
	service, ok := m.ReplyWatches().(*replyWatchService)
	if !ok || service == nil || service.store == nil {
		return false
	}
	watches, err := service.store.ListPendingReplyWatchesBySender(ctx, strings.TrimSpace(creatorSessionID))
	if err != nil {
		m.logger.WarnContext(
			ctx,
			"session.spawn_wake.reply_watch_lookup_failed",
			"creator_session_id", creatorSessionID,
			"child_session_id", childSessionID,
			"error", SanitizeSpawnWakeText(err.Error()),
		)
		return false
	}
	for _, watch := range watches {
		if strings.TrimSpace(watch.TargetSessionID) == strings.TrimSpace(childSessionID) {
			return true
		}
	}
	return false
}
