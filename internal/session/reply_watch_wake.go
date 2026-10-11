package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// AwaitingReplySignals keeps a sender out of the inactivity stop while its answer can still arrive: an
// armed watch whose target is live (starting, active or stopping), or a fired watch whose wake is not
// delivered yet. A target that is stopped or gone gives no evidence, so a sender waiting on it can go
// quiet; the reply then resumes the sender the daemon stopped (replySenderWakeable).
func (s *replyWatchService) AwaitingReplySignals(
	ctx context.Context,
	sender string,
	until time.Time,
) ([]WorkSignal, error) {
	rows, err := s.store.ListPendingReplyWatchesBySender(ctx, sender)
	if err != nil {
		return nil, err
	}
	signals := make([]WorkSignal, 0, len(rows))
	for _, w := range rows {
		if w.State == store.ReplyWatchArmed && !s.replyTargetLive(w.TargetSessionID) {
			continue
		}
		signals = append(signals, WorkSignal{
			Kind: WorkSignalAwaitingReply, Since: w.CreatedAt, Ref: w.ID, ValidUntil: until,
		})
	}
	return signals, nil
}

func (s *replyWatchService) replyTargetLive(target string) bool {
	session, live := s.manager.Get(target)
	if !live {
		return false
	}
	switch session.Info().State {
	case StateStarting, StateActive, StateStopping:
		return true
	default:
		return false
	}
}

// replySenderWakeable reports whether a reply may resume a stopped sender. The daemon stopped it for
// inactivity or a timeout, at shutdown, or after an agent crash; an operator stop or a completed
// session stays stopped until the operator resumes it, and the wake is delivered then.
func replySenderWakeable(info *Info) bool {
	if info == nil || info.State != StateStopped || info.ArchivedAt != nil {
		return false
	}
	switch info.StopReason {
	case store.StopTimeout, store.StopShutdown, store.StopAgentCrashed:
		return true
	default:
		return false
	}
}

// resumeSender resumes a stopped sender off the caller's path: delivery runs inside settle and queue
// edges of the target, which must not wait for another session's provider to start. A resume that
// succeeds redelivers through OnSessionResumed; one that fails leaves the watch fired for the retry
// ticker. At most one resume per sender is in flight.
func (s *replyWatchService) resumeSender(ctx context.Context, watchID, sender string) {
	if _, loaded := s.resuming.LoadOrStore(sender, struct{}{}); loaded {
		return
	}
	runCtx := s.backgroundContext()
	s.wakes.Go(func() {
		defer s.resuming.Delete(sender)
		if _, err := s.manager.Resume(runCtx, sender); err != nil {
			s.manager.logger.WarnContext(
				runCtx, "reply_watch.sender_resume_failed", "id", watchID, "sender_session_id", sender, "error", err,
			)
			return
		}
		s.manager.logger.InfoContext(runCtx, "reply_watch.sender_resumed", "id", watchID, "sender_session_id", sender)
	})
	s.manager.logger.InfoContext(ctx, "reply_watch.delivery_deferred", "id", watchID, "reason", "sender_resuming")
}

func (s *replyWatchService) backgroundContext() context.Context {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.runCtx != nil {
		return s.runCtx
	}
	return s.manager.fallbackLifecycleContext()
}
