package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type ReplyWatchService interface {
	Close(context.Context) error
	Reconcile(context.Context, string) error
	OnTurnSettled(context.Context, string, string)
	OnQueueEntryTerminal(context.Context, *store.SessionInputQueueEntry)
	OnSteerResolved(context.Context, string, string)
	OnSendResult(context.Context, string, error)
	Deliver(context.Context, string) error
	OnSenderTurnSettled(context.Context, string)
	OnSessionResumed(context.Context, string)
	OnSenderGone(context.Context, string) error
	OnTargetGone(context.Context, string)
	Recover(context.Context) error
	ListForSender(context.Context, string, string) ([]store.ReplyWatch, error)
	// AwaitingReplySignals reports the sender's open watches that still count as work until the given time.
	AwaitingReplySignals(context.Context, string, time.Time) ([]WorkSignal, error)
}

// replyNoText stands in for an empty answer in the provider-bound wake; the typed summary stays empty.
const replyNoText = "(no reply text)"

type replyWatchService struct {
	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
	runCtx      context.Context
	store       store.ReplyWatchStore
	manager     *Manager
	// resuming holds senders with a reply-driven resume in flight; wakes joins those resumes on Close.
	// closed (under lifecycleMu) stops admitting new resumes once shutdown begins.
	resuming sync.Map
	wakes    sync.WaitGroup
	closed   bool
}

var _ ReplyWatchService = (*replyWatchService)(nil)

func NewReplyWatchService(db store.ReplyWatchStore, manager *Manager) (ReplyWatchService, error) {
	if db == nil || manager == nil {
		return nil, errors.New("session: reply watches require store and manager")
	}
	return &replyWatchService{store: db, manager: manager}, nil
}

func (m *Manager) SetReplyWatchService(s ReplyWatchService) {
	m.mu.Lock()
	m.replyWatches = s
	m.mu.Unlock()
	if service, ok := s.(*replyWatchService); ok {
		service.start(m.fallbackLifecycleContext())
	}
}

func (m *Manager) ReplyWatches() ReplyWatchService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.replyWatches
}

func (s *replyWatchService) Reconcile(ctx context.Context, id string) error {
	return s.reconcile(ctx, id, false)
}

func (s *replyWatchService) reconcile(ctx context.Context, id string, recovering bool) error {
	w, err := s.store.GetReplyWatch(ctx, id)
	if err != nil {
		return err
	}
	if w.State == store.ReplyWatchFired {
		return s.Deliver(ctx, id)
	}
	if w.State != store.ReplyWatchArmed {
		return nil
	}
	sender, err := s.manager.Status(ctx, w.SenderSessionID)
	if errors.Is(err, ErrSessionNotFound) || (err == nil && sender.ArchivedAt != nil) {
		return s.abandon(ctx, id, "sender_gone")
	}
	if err != nil {
		return err
	}
	// A deleted target has no recoverable admission or transcript. Preserve the
	// sender's watch as an explicit uncertain reply instead of failing daemon boot.
	target, err := s.manager.Status(ctx, w.TargetSessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return s.fire(ctx, w, store.ReplyOutcomeUnknown, "")
	}
	if err != nil {
		return err
	}
	admission, inputs, err := s.store.ReplyWatchEvidence(ctx, w)
	if err != nil {
		return err
	}
	queueID := w.QueueEntryID
	for i := range inputs {
		queueID = inputs[i].ID
	}
	turn, err := s.manager.replyMessageTurn(ctx, w, admission, inputs, recovering)
	if err != nil {
		return err
	}
	if turn != "" {
		return s.reconcileConsumedTurn(ctx, w, turn, queueID)
	}
	if target.ArchivedAt != nil {
		return s.fire(ctx, w, store.ReplyOutcomeUnknown, "")
	}
	if err := s.store.BindReplyWatch(ctx, id, "", queueID); err != nil {
		return err
	}
	return s.reconcileUnconsumed(ctx, w, admission, inputs, recovering)
}

func (s *replyWatchService) reconcileUnconsumed(
	ctx context.Context, w store.ReplyWatch, admission store.SessionPromptAdmission,
	inputs []store.SessionInputQueueEntry, recovering bool,
) error {
	outcome := replyNonDispatchOutcome(admission, inputs)
	if outcome == "" {
		return nil
	}
	if outcome == store.ReplyOutcomeUnknown && !recovering {
		settled, err := s.manager.replyTargetSettledSince(ctx, w.TargetSessionID, replyDispatchTime(admission, inputs))
		if err != nil {
			return err
		}
		if !settled {
			return nil
		}
	}
	return s.fire(ctx, w, outcome, "")
}

func (s *replyWatchService) reconcileConsumedTurn(ctx context.Context, w store.ReplyWatch, turn, queueID string) error {
	active, activeErr := s.manager.ActivePromptRun(ctx, w.TargetSessionID)
	if activeErr != nil && !errors.Is(activeErr, ErrPromptNotActive) {
		return activeErr
	}
	if activeErr == nil && active.TurnID == turn {
		return nil
	}
	if err := s.store.BindReplyWatch(ctx, w.ID, turn, queueID); err != nil {
		return err
	}
	result, err := s.manager.TurnResult(ctx, w.TargetSessionID, turn)
	if err != nil {
		return err
	}
	if result.Outcome == "" {
		return nil
	}
	text := result.Text
	if result.Outcome == store.ReplyOutcomeFailed {
		text = result.Error
	}
	return s.fire(ctx, w, result.Outcome, text)
}

func replyNonDispatchOutcome(admission store.SessionPromptAdmission, inputs []store.SessionInputQueueEntry) string {
	terminal := false
	for i := range inputs {
		entry := &inputs[i]
		if entry.Status == store.SessionInputQueueStatusSent &&
			entry.SteerDelivery == store.SteerDeliveryPendingInjection {
			return ""
		}
		switch entry.Status {
		case store.SessionInputQueueStatusQueued, store.SessionInputQueueStatusDispatching:
			return ""
		case store.SessionInputQueueStatusCanceled, store.SessionInputQueueStatusFailed:
			terminal = true
		}
	}
	if terminal {
		return store.ReplyOutcomeDropped
	}
	if admission.State == store.SessionPromptAdmissionDispatchCommitted ||
		admission.State == store.SessionPromptAdmissionIndeterminate {
		return store.ReplyOutcomeUnknown
	}
	if admission.Result != nil && admission.Result.Status == store.SessionPromptResultStatusCanceled {
		return store.ReplyOutcomeDropped
	}
	return ""
}

func (s *replyWatchService) fire(ctx context.Context, w store.ReplyWatch, outcome, text string) error {
	switch outcome {
	case store.ReplyOutcomeDropped:
		text = "The message was removed from the queue before it ran."
	case store.ReplyOutcomeUnknown:
		text = "The message may not have been delivered; check the target session."
	}
	if strings.TrimSpace(text) == "" {
		text = replyNoText
	}
	runes := []rune(text)
	truncated := len(runes) > 12000
	if truncated {
		text = string(runes[:12000])
	}
	fired, err := s.store.FireReplyWatch(
		ctx,
		w.ID,
		store.ReplyWatchFire{Outcome: outcome, Text: text, Truncated: truncated},
	)
	if err != nil {
		return err
	}
	if fired {
		s.manager.logger.InfoContext(ctx, "reply_watch.fired", "id", w.ID, "outcome", outcome, "truncated", truncated)
	}
	return s.Deliver(ctx, w.ID)
}

func (s *replyWatchService) abandon(ctx context.Context, id, reason string) error {
	if err := s.store.AbandonReplyWatch(ctx, id, reason); err != nil {
		return err
	}
	s.manager.logger.InfoContext(ctx, "reply_watch.abandoned", "id", id, "reason", reason)
	return nil
}

func (s *replyWatchService) OnTurnSettled(ctx context.Context, sessionID, _ string) {
	s.reconcileTarget(ctx, sessionID, "")
	s.OnSenderTurnSettled(ctx, sessionID)
}
func (s *replyWatchService) OnQueueEntryTerminal(ctx context.Context, entry *store.SessionInputQueueEntry) {
	s.reconcileTarget(ctx, entry.SessionID, entry.MessageID)
}
func (s *replyWatchService) OnSteerResolved(ctx context.Context, sessionID, messageID string) {
	s.reconcileTarget(ctx, sessionID, messageID)
}
func (s *replyWatchService) OnSendResult(ctx context.Context, id string, sendErr error) {
	if sendErr != nil && !errors.Is(sendErr, store.ErrSessionPromptDispatchIndeterminate) {
		s.logError(ctx, s.abandon(ctx, id, "send_failed"))
		return
	}
	if sendErr == nil {
		if w, err := s.store.GetReplyWatch(ctx, id); err == nil {
			s.manager.logger.InfoContext(
				ctx,
				"reply_watch.registered",
				"id",
				id,
				"turn_id",
				w.TurnID,
				"queue_entry_id",
				w.QueueEntryID,
			)
		}
	}
	s.logError(ctx, s.Reconcile(ctx, id))
}
func (s *replyWatchService) reconcileTarget(ctx context.Context, target, message string) {
	rows, err := s.store.ListReplyWatches(
		ctx,
		store.ReplyWatchFilter{TargetSessionID: target, State: store.ReplyWatchArmed},
	)
	s.logError(ctx, err)
	for _, w := range rows {
		if message == "" || w.MessageID == message {
			s.logError(ctx, s.Reconcile(ctx, w.ID))
		}
	}
}
func (s *replyWatchService) OnSenderTurnSettled(ctx context.Context, sender string) {
	s.retry(ctx, sender)
}
func (s *replyWatchService) OnSessionResumed(ctx context.Context, sender string) {
	s.retry(ctx, sender)
}
func (s *replyWatchService) retry(ctx context.Context, sender string) {
	rows, err := s.store.ListReplyWatches(
		ctx,
		store.ReplyWatchFilter{SenderSessionID: sender, State: store.ReplyWatchFired},
	)
	s.logError(ctx, err)
	for _, w := range rows {
		s.logError(ctx, s.Deliver(ctx, w.ID))
	}
}
func (s *replyWatchService) OnTargetGone(ctx context.Context, target string) {
	s.reconcileTarget(ctx, target, "")
}

func (s *replyWatchService) OnSenderGone(ctx context.Context, sender string) error {
	rows, err := s.store.ListReplyWatches(ctx, store.ReplyWatchFilter{SenderSessionID: sender})
	if err != nil {
		return err
	}
	for _, w := range rows {
		if err := s.abandon(ctx, w.ID, "sender_gone"); err != nil {
			return err
		}
	}
	return nil
}

// ListForSender lists the sender's pending watches in exactly one scope; an empty workspace is the
// User scope, never a wildcard.
func (s *replyWatchService) ListForSender(ctx context.Context, workspace, sender string) ([]store.ReplyWatch, error) {
	if sender == "" {
		return nil, errors.New("session: reply watch sender scope is required")
	}
	rows, err := s.store.ListReplyWatches(ctx, store.ReplyWatchFilter{WorkspaceID: workspace, SenderSessionID: sender})
	if err != nil || workspace != "" {
		return rows, err
	}
	return slices.DeleteFunc(rows, func(w store.ReplyWatch) bool { return w.WorkspaceID != "" }), nil
}
func (s *replyWatchService) Recover(ctx context.Context) error {
	rows, err := s.store.ListReplyWatches(ctx, store.ReplyWatchFilter{})
	if err != nil {
		return err
	}
	for _, w := range rows {
		if err := s.reconcile(ctx, w.ID, true); err != nil {
			s.manager.logger.ErrorContext(ctx, "reply_watch.recover_failed", "id", w.ID, "error", err)
		}
	}
	s.expireStale(ctx)
	s.manager.logger.InfoContext(ctx, "reply_watch.recovered", "count", len(rows))
	return nil
}
func (s *replyWatchService) run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.expireStale(ctx)
			s.retry(ctx, "")
		}
	}
}
func (s *replyWatchService) logError(ctx context.Context, err error) {
	if err != nil {
		s.manager.logger.ErrorContext(ctx, "reply_watch.reconcile_failed", "error", err)
	}
}

func (s *replyWatchService) start(ctx context.Context) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.done != nil {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.cancel, s.done, s.runCtx = cancel, done, runCtx
	go func() { defer close(done); s.run(runCtx) }()
}

func (s *replyWatchService) Close(ctx context.Context) error {
	s.lifecycleMu.Lock()
	s.closed = true
	cancel, done := s.cancel, s.done
	s.lifecycleMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	joined := make(chan struct{})
	go func() {
		<-done
		s.wakes.Wait()
		close(joined)
	}()
	select {
	case <-joined:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
