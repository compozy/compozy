package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
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
	_, err := s.manager.ActivePromptRun(ctx, target.ID)
	if !errors.Is(err, ErrPromptNotActive) {
		s.settleGrace.Delete(w.ID)
		return err
	}
	waiting, err := s.replyContinuationWork(ctx, w)
	if err != nil {
		return err
	}
	if waiting != "" {
		s.settleGrace.Delete(w.ID)
		return s.recordIntermediateReplyTurn(ctx, w)
	}
	queue, err := s.manager.InputQueueSummary(ctx, target.ID)
	if err != nil || queue.PendingInputs > 0 {
		s.settleGrace.Delete(w.ID)
		return err
	}
	terminal, err := s.latestReplyTerminal(ctx, w)
	if err != nil {
		return s.settleReplyAfterGrace(ctx, w, pass, store.ReplyOutcomeUnknown, replyResultUnreadable, err)
	}
	if terminal.TurnID == "" {
		return s.settleReplyAfterGrace(ctx, w, pass, store.ReplyOutcomeCompleted,
			"The target settled without a further answer. The interim reply stands.", nil)
	}
	turn := terminal.TurnID
	result, err := s.manager.replyTurnResult(ctx, w.TargetSessionID, turn, pass == replyWatchEvent)
	if err != nil || result.Outcome == "" {
		return s.settleReplyAfterGrace(ctx, w, pass, store.ReplyOutcomeUnknown, replyResultUnreadable, err)
	}
	return s.fireTurnReply(ctx, w, turn, result, "")
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

func (s *replyWatchService) recordIntermediateReplyTurn(ctx context.Context, w store.ReplyWatch) error {
	terminal, err := s.latestReplyTerminal(ctx, w)
	if err != nil || terminal.Sequence == 0 {
		return err
	}
	return s.store.AdvanceReplyWatchFollowUp(ctx, w.ID, terminal.Sequence)
}

func (s *replyWatchService) latestReplyTerminal(
	ctx context.Context, w store.ReplyWatch,
) (store.SessionEvent, error) {
	// Use the newest bounded ledger page; forward cursor reads would select the oldest continuation.
	events, err := s.manager.Events(ctx, w.TargetSessionID, store.EventQuery{
		Types: []string{acp.EventTypeDone, acp.EventTypeError, EventTypeSessionStopped},
		Limit: 64,
	})
	if err != nil {
		return store.SessionEvent{}, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := &events[i]
		if event.Sequence <= w.FollowUpAfter || event.TurnID == "" || event.TurnID == w.TurnID {
			continue
		}
		decoded, err := transcript.UnmarshalAgentEvent(event.Content)
		if err == nil && decoded.ParentToolCallID() != "" {
			continue
		}
		return *event, nil
	}
	return store.SessionEvent{}, nil
}

func (s *replyWatchService) replyContinuationWork(ctx context.Context, w store.ReplyWatch) (string, error) {
	var waiting []string
	for _, kind := range []WorkSignalKind{WorkSignalActiveMonitor, WorkSignalActiveChild, WorkSignalScheduledWait} {
		signals, err := s.manager.workSignals.inspectKind(ctx, w.TargetSessionID, kind, s.manager.now())
		if err != nil {
			return "", err
		}
		for _, signal := range signals {
			if kind != WorkSignalActiveChild || s.replyChildWillWake(ctx, signal.Ref) {
				waiting = append(waiting, replyWaitLabel(kind))
				break
			}
		}
	}
	replies, err := s.store.ListPendingReplyWatchesBySender(ctx, w.TargetSessionID)
	if err != nil {
		return "", err
	}
	now := s.manager.now()
	for _, reply := range replies {
		// Waiting on the sender cannot be the reason to hold the sender's answer.
		if reply.TargetSessionID != w.SenderSessionID && !s.replyWatchExpired(reply, now) {
			waiting = append(waiting, "replies from other sessions")
			break
		}
	}
	return strings.Join(waiting, ", "), nil
}

func (s *replyWatchService) replyChildWillWake(ctx context.Context, id string) bool {
	child, live := s.manager.Get(id)
	if !live {
		// Starting children and the subagent store are owned by the signal source.
		return true
	}
	info := child.Info()
	if info.Lineage == nil || info.Lineage.SpawnRole == store.SubagentSpawnRole {
		return true
	}
	if !info.Lineage.NotifyCreator {
		return false
	}
	if info.State != StateActive || info.Prompting {
		return true
	}
	if info.Supervision != nil {
		for _, signal := range info.Supervision.WorkSignals {
			switch signal.Kind {
			case WorkSignalActiveMonitor, WorkSignalScheduledWait, WorkSignalAwaitingReply, WorkSignalActiveChild:
				if !signal.ValidUntil.Before(s.manager.now()) {
					return true
				}
			}
		}
	}
	queue, err := s.manager.InputQueueSummary(ctx, id)
	return err != nil || queue.PendingInputs > 0
}

func replyWaitLabel(kind WorkSignalKind) string {
	switch kind {
	case WorkSignalActiveMonitor:
		return "monitors"
	case WorkSignalActiveChild:
		return "subagents or child sessions"
	default:
		return "scheduled waits"
	}
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
