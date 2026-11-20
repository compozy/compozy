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

// WithReplyWatchSweepInterval sets both the periodic cadence and the settlement grace.
func WithReplyWatchSweepInterval(interval time.Duration) ReplyWatchOption {
	return func(s *replyWatchService) {
		if interval > 0 {
			s.sweepInterval = interval
		}
	}
}

// WithReplyWatchRecoveryGate defers settlement until boot has restored wake-producing work.
func WithReplyWatchRecoveryGate(ready <-chan struct{}) ReplyWatchOption {
	return func(s *replyWatchService) { s.recoveryGate = ready }
}

func (s *replyWatchService) reconciliationReady() bool {
	if s.recoveryGate == nil {
		return true
	}
	select {
	case <-s.recoveryGate:
		return true
	default:
		return false
	}
}

func (s *replyWatchService) sweep(ctx context.Context) {
	if !s.reconciliationReady() {
		return
	}
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

type replySettlementCondition uint8

const (
	replyAdmissionUnreadable replySettlementCondition = iota
	replyNoDispatchReceipt
	replyUncertainDispatch
	replyTerminalLookupFailed
	replyTerminalUnreadable
	replyNoContinuationTerminal
)

type replySettlement struct {
	condition  replySettlementCondition
	turnID     string
	terminalID string
}

func (e replySettlement) outcome() (string, string) {
	switch e.condition {
	case replyNoContinuationTerminal:
		return store.ReplyOutcomeCompleted, "The target settled without a further answer. The interim reply stands."
	case replyTerminalLookupFailed, replyTerminalUnreadable:
		return store.ReplyOutcomeUnknown, replyResultUnreadable
	default:
		return store.ReplyOutcomeUnknown, ""
	}
}

type replyGraceObservation struct {
	evidence replySettlement
	since    time.Time
}

func (s *replyWatchService) settleReplyAfterGrace(
	ctx context.Context, w store.ReplyWatch, pass replyWatchPass, evidence replySettlement, err error,
) error {
	if pass != replyWatchPeriodic {
		return err
	}
	if s.manager.IsPrompting(w.TargetSessionID) {
		s.settleGrace.Delete(w.ID)
		return err
	}
	now := s.manager.now()
	raw, _ := s.settleGrace.LoadOrStore(w.ID, replyGraceObservation{since: now, evidence: evidence})
	observation, ok := raw.(replyGraceObservation)
	if !ok || observation.evidence != evidence {
		s.settleGrace.Store(w.ID, replyGraceObservation{since: now, evidence: evidence})
		return err
	}
	if now.Sub(observation.since) < s.sweepInterval {
		return err
	}
	outcome, text := evidence.outcome()
	return errors.Join(err, s.fire(ctx, w, outcome, text))
}

func (s *replyWatchService) logWatchError(ctx context.Context, w store.ReplyWatch, err error) {
	if err != nil {
		s.manager.logger.ErrorContext(ctx, "reply_watch.reconcile_failed",
			"id", w.ID, "sender_session_id", w.SenderSessionID, "target_session_id", w.TargetSessionID, "error", err)
	}
}
