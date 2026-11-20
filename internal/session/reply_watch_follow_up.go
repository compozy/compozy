package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (s *replyWatchService) reconcileFollowUp(
	ctx context.Context, w store.ReplyWatch, target *Info, pass replyWatchPass,
) error {
	if target.State == StateStopped {
		return s.reconcileStoppedFollowUp(ctx, w, target)
	}
	if target.State != StateActive {
		return nil
	}
	terminal, terminalErr := s.latestReplyTerminal(ctx, w)
	if terminalErr != nil {
		return s.settleReplyAfterGrace(ctx, w, pass, replySettlement{condition: replyTerminalLookupFailed}, terminalErr)
	}
	_, err := s.manager.ActivePromptRun(ctx, target.ID)
	if !errors.Is(err, ErrPromptNotActive) || s.manager.IsPrompting(target.ID) {
		s.settleGrace.Delete(w.ID)
		if errors.Is(err, ErrPromptNotActive) {
			return nil
		}
		return err
	}
	waiting, err := s.replyContinuationWork(ctx, w)
	if waiting != "" || err != nil {
		s.settleGrace.Delete(w.ID)
		return err
	}
	var result TurnResult
	var resultErr error
	if terminal.TurnID != "" {
		result, resultErr = s.manager.replyTurnResult(ctx, w.TargetSessionID, terminal.TurnID, pass == replyWatchEvent)
	}
	// Work may have completed while it was inspected or the answer was read.
	latest, err := s.latestReplyTerminal(ctx, w)
	if err != nil || latest.ID != terminal.ID || latest.Sequence != terminal.Sequence ||
		s.manager.IsPrompting(target.ID) {
		s.settleGrace.Delete(w.ID)
		return err
	}
	if terminal.TurnID == "" {
		return s.settleReplyAfterGrace(ctx, w, pass, replySettlement{condition: replyNoContinuationTerminal}, nil)
	}
	if resultErr != nil || result.Outcome == "" {
		return s.settleReplyAfterGrace(ctx, w, pass, replySettlement{
			condition: replyTerminalUnreadable, turnID: terminal.TurnID, terminalID: terminal.ID,
		}, resultErr)
	}
	return s.fireTurnReply(ctx, w, terminal.TurnID, result, "")
}

func (s *replyWatchService) reconcileStoppedFollowUp(
	ctx context.Context, w store.ReplyWatch, target *Info,
) error {
	if target.StopReason == store.StopShutdown {
		s.lifecycleMu.Lock()
		closed := s.closed
		s.lifecycleMu.Unlock()
		if closed {
			return nil
		}
		waiting, err := s.replyContinuationWork(ctx, w)
		if waiting != "" || err != nil {
			return err
		}
	}
	return s.replyAfterTargetStop(ctx, w, target)
}

func (s *replyWatchService) replyAfterTargetStop(ctx context.Context, w store.ReplyWatch, target *Info) error {
	result := TurnResult{Outcome: store.ReplyOutcomeCanceled}
	if target.Failure != nil && target.Failure.Kind != store.FailureCanceled {
		result.Outcome, result.Error = store.ReplyOutcomeFailed, target.Failure.Summary
	}
	terminal, err := s.latestReplyTerminal(ctx, w)
	s.logWatchError(ctx, w, err)
	turn := terminal.TurnID
	if turn != "" {
		later, err := s.manager.replyTurnResult(ctx, w.TargetSessionID, turn, false)
		s.logWatchError(ctx, w, err)
		result.Text = later.Text
	}
	return s.fireTurnReply(ctx, w, turn, result, "")
}

func (s *replyWatchService) latestReplyTerminal(
	ctx context.Context, w store.ReplyWatch,
) (store.SessionEvent, error) {
	events, err := s.manager.Events(ctx, w.TargetSessionID, store.EventQuery{
		Types:        []string{acp.EventTypeDone, acp.EventTypeError, EventTypeSessionStopped},
		TopLevelOnly: true, ExcludeTurnID: w.TurnID, Limit: 1,
	})
	if err != nil {
		return store.SessionEvent{}, err
	}
	if len(events) == 0 || events[0].Sequence <= w.FollowUpAfter {
		return store.SessionEvent{}, nil
	}
	return events[0], nil
}

func (m *Manager) replyContinuationHeader(ctx context.Context, sessionID string) string {
	if m.ReplyWatches() == nil {
		return ""
	}
	db, ok := m.inputQueueStore.(store.ReplyWatchStore)
	if !ok {
		return ""
	}
	watches, err := db.ListPendingReplyWatchesByTarget(ctx, sessionID)
	if err != nil {
		m.logger.WarnContext(
			ctx,
			"reply_watch.continuation_header_failed",
			"target_session_id",
			sessionID,
			"error",
			err,
		)
		return ""
	}
	var lines []string
	for _, w := range watches {
		if w.FollowUpAfter == 0 || (w.State == store.ReplyWatchFired && w.InterimWait == "") ||
			m.replyWatchPastTTL(w) {
			continue
		}
		lines = append(
			lines,
			fmt.Sprintf("Session %q is awaiting a follow-up to message %q.", w.SenderSessionID, w.MessageID),
		)
	}
	if len(lines) == 0 {
		return ""
	}
	return "[Pending session replies. " + strings.Join(lines, " ") +
		" CompozyOS sends your answer automatically when this session settles. If you end this turn with " +
		"another registered wait, it stays outstanding.]"
}

func (m *Manager) replyPromptOrigin(ctx context.Context, origin *acp.PromptOriginMeta) *acp.PromptOriginMeta {
	if origin == nil || !origin.NotifyOnComplete {
		return origin
	}
	if db, ok := m.inputQueueStore.(store.ReplyWatchStore); ok {
		w, err := db.GetReplyWatch(ctx, origin.ReplyWatchID)
		if err == nil && w.State == store.ReplyWatchArmed && !m.replyWatchPastTTL(w) {
			return origin
		}
	}
	display := *origin
	display.NotifyOnComplete = false
	return &display
}

func (m *Manager) replyWatchPastTTL(w store.ReplyWatch) bool {
	ttl := m.messagingConfig().ReplyWatchTTL
	return ttl > 0 && m.now().Sub(w.CreatedAt) >= ttl
}
