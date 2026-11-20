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
	lifecycleMu   sync.Mutex
	cancel        context.CancelFunc
	done          chan struct{}
	runCtx        context.Context
	store         store.ReplyWatchStore
	manager       *Manager
	sweepInterval time.Duration
	recoveryGate  <-chan struct{}
	// resuming holds senders with a reply-driven resume in flight; wakes joins those resumes on Close.
	// closed (under lifecycleMu) stops admitting new resumes once shutdown begins.
	resuming    sync.Map
	wakes       sync.WaitGroup
	closed      bool
	settleGrace sync.Map
}

var _ ReplyWatchService = (*replyWatchService)(nil)

type ReplyWatchOption func(*replyWatchService)

func NewReplyWatchService(
	db store.ReplyWatchStore, manager *Manager, opts ...ReplyWatchOption,
) (ReplyWatchService, error) {
	if db == nil || manager == nil {
		return nil, errors.New("session: reply watches require store and manager")
	}
	service := &replyWatchService{store: db, manager: manager, sweepInterval: replyWatchSweepInterval}
	for _, opt := range opts {
		opt(service)
	}
	return service, nil
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
	return s.reconcile(ctx, id, replyWatchEvent)
}

type replyWatchPass uint8

const (
	replyWatchEvent replyWatchPass = iota
	replyWatchRecovery
	replyWatchPeriodic
)

func (s *replyWatchService) reconcile(ctx context.Context, id string, pass replyWatchPass) error {
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
	return s.reconcileArmed(ctx, w, pass)
}

func (s *replyWatchService) reconcileArmed(ctx context.Context, w store.ReplyWatch, pass replyWatchPass) error {
	if !s.reconciliationReady() {
		return nil
	}
	id := w.ID
	sender, err := s.manager.StoredStatus(ctx, w.SenderSessionID)
	if errors.Is(err, ErrSessionNotFound) || (err == nil && sender.ArchivedAt != nil) {
		return s.abandon(ctx, id, "sender_gone")
	}
	if err != nil {
		return err
	}
	// A deleted target has no recoverable admission or transcript. Preserve the
	// sender's watch as an explicit uncertain reply instead of failing daemon boot.
	target, err := s.manager.StoredStatus(ctx, w.TargetSessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return s.replyTargetUnavailable(ctx, w)
	}
	if err != nil {
		return err
	}
	if pass == replyWatchEvent && s.replyWatchExpired(w, s.manager.now()) {
		return s.expire(ctx, w)
	}
	if pass == replyWatchPeriodic && (target.State == StateStarting || target.State == StateStopping) {
		s.settleGrace.Delete(w.ID)
		return nil
	}
	if w.FollowUpAfter > 0 {
		if target.ArchivedAt != nil {
			return s.replyTargetUnavailable(ctx, w)
		}
		return s.reconcileFollowUp(ctx, w, target, pass)
	}
	return s.reconcileInitial(ctx, w, target, pass)
}

func (s *replyWatchService) replyTargetUnavailable(ctx context.Context, w store.ReplyWatch) error {
	switch {
	case w.FollowUpAfter > 0:
		return s.fire(ctx, w, store.ReplyOutcomeUnknown, "The target session is unavailable. The interim reply stands.")
	case w.TurnID != "":
		return s.fire(ctx, w, store.ReplyOutcomeUnknown, replyResultUnreadable)
	default:
		return s.fire(ctx, w, store.ReplyOutcomeUnknown, "")
	}
}

func (s *replyWatchService) reconcileInitial(
	ctx context.Context, w store.ReplyWatch, target *Info, pass replyWatchPass,
) error {
	admission, inputs, err := s.store.ReplyWatchEvidence(ctx, w)
	if err != nil {
		if w.TurnID != "" {
			return s.reconcileConsumedTurn(ctx, w, target, w.TurnID, w.QueueEntryID, pass)
		}
		return s.settleReplyAfterGrace(ctx, w, pass, replySettlement{condition: replyAdmissionUnreadable}, err)
	}
	queueID := w.QueueEntryID
	for i := range inputs {
		queueID = inputs[i].ID
	}
	turn := w.TurnID
	if turn == "" {
		recoverInput := pass == replyWatchRecovery || (pass == replyWatchPeriodic && !replyQueuedForConsumption(inputs))
		turn, err = s.manager.replyMessageTurn(ctx, w, admission, inputs, recoverInput)
	}
	if err != nil {
		return err
	}
	if turn != "" {
		return s.reconcileConsumedTurn(ctx, w, target, turn, queueID, pass)
	}
	if target.ArchivedAt != nil {
		return s.replyTargetUnavailable(ctx, w)
	}
	if queueID != w.QueueEntryID {
		if err := s.store.BindReplyWatch(ctx, w.ID, "", queueID); err != nil {
			return err
		}
	}
	return s.reconcileUnconsumed(ctx, w, admission, inputs, pass)
}

func (s *replyWatchService) reconcileUnconsumed(
	ctx context.Context, w store.ReplyWatch, admission store.SessionPromptAdmission,
	inputs []store.SessionInputQueueEntry, pass replyWatchPass,
) error {
	if replyQueuedForConsumption(inputs) {
		s.settleGrace.Delete(w.ID)
		return nil
	}
	outcome := replyNonDispatchOutcome(admission, inputs)
	if outcome == store.ReplyOutcomeUnknown && s.manager.IsPrompting(w.TargetSessionID) {
		s.settleGrace.Delete(w.ID)
		return nil
	}
	if outcome == "" {
		return s.settleReplyAfterGrace(ctx, w, pass, replySettlement{condition: replyNoDispatchReceipt}, nil)
	}
	if outcome == store.ReplyOutcomeUnknown && pass != replyWatchRecovery {
		settled, err := s.manager.replyTargetSettledSince(ctx, w.TargetSessionID, replyDispatchTime(admission, inputs))
		if err != nil || !settled {
			return s.settleReplyAfterGrace(ctx, w, pass, replySettlement{condition: replyUncertainDispatch}, err)
		}
	}
	return s.fire(ctx, w, outcome, "")
}

func (s *replyWatchService) reconcileConsumedTurn(
	ctx context.Context, w store.ReplyWatch, target *Info, turn, queueID string, pass replyWatchPass,
) error {
	active, activeErr := s.manager.ActivePromptRun(ctx, w.TargetSessionID)
	if activeErr != nil && !errors.Is(activeErr, ErrPromptNotActive) {
		return activeErr
	}
	if activeErr == nil && active.TurnID == turn {
		return nil
	}
	if turn != w.TurnID || queueID != w.QueueEntryID {
		if err := s.store.BindReplyWatch(ctx, w.ID, turn, queueID); err != nil {
			return err
		}
	}
	result, err := s.manager.replyTurnResult(ctx, w.TargetSessionID, turn, pass == replyWatchEvent)
	if err != nil || result.Outcome == "" {
		return s.settleReplyAfterGrace(ctx, w, pass, replySettlement{
			condition: replyTerminalUnreadable, turnID: turn, terminalID: result.terminalID,
		}, err)
	}
	if target.ArchivedAt != nil || (target.State == StateStopped && target.StopReason != store.StopShutdown) {
		return s.fireTurnReply(ctx, w, turn, result, "")
	}
	waiting, err := s.replyContinuationWork(ctx, w)
	if err != nil {
		return err
	}
	return s.fireTurnReply(ctx, w, turn, result, waiting)
}

func (s *replyWatchService) fireTurnReply(
	ctx context.Context, w store.ReplyWatch, turn string, result TurnResult, waiting string,
) error {
	answer := result.Text
	if result.Outcome == store.ReplyOutcomeFailed && (answer == "" || w.FollowUpAfter == 0) {
		answer = result.Error
	}
	f := store.ReplyWatchFire{Outcome: result.Outcome, Text: answer, TurnID: turn}
	if waiting != "" && result.Sequence > 0 {
		f.FollowUpAfter, f.InterimWait = result.Sequence, waiting
	}
	return s.fireResult(ctx, w, f)
}

func replyQueuedForConsumption(inputs []store.SessionInputQueueEntry) bool {
	for i := range inputs {
		input := &inputs[i]
		if input.Status == store.SessionInputQueueStatusQueued ||
			input.Status == store.SessionInputQueueStatusDispatching ||
			(input.Status == store.SessionInputQueueStatusSent && input.SteerDelivery == store.SteerDeliveryPendingInjection) {
			return true
		}
	}
	return false
}

func replyNonDispatchOutcome(admission store.SessionPromptAdmission, inputs []store.SessionInputQueueEntry) string {
	terminal := false
	for i := range inputs {
		if inputs[i].Status == store.SessionInputQueueStatusCanceled ||
			inputs[i].Status == store.SessionInputQueueStatusFailed {
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
	return s.fireResult(ctx, w, store.ReplyWatchFire{Outcome: outcome, Text: text})
}

func (s *replyWatchService) fireResult(ctx context.Context, w store.ReplyWatch, result store.ReplyWatchFire) error {
	switch result.Outcome {
	case store.ReplyOutcomeDropped:
		result.Text = "The message was removed from the queue before it ran."
	case store.ReplyOutcomeUnknown:
		if result.Text == "" {
			result.Text = "The message may not have been delivered; check the target session."
		}
	}
	if strings.TrimSpace(result.Text) == "" {
		result.Text = replyNoText
	}
	runes := []rune(result.Text)
	if len(runes) > 12000 {
		result.Text, result.Truncated = string(runes[:12000]), true
	}
	result.ExpectedAfter = w.FollowUpAfter
	fired, err := s.store.FireReplyWatch(ctx, w.ID, result)
	if err != nil {
		return err
	}
	if !fired {
		return nil
	}
	s.settleGrace.Delete(w.ID)
	s.manager.logger.InfoContext(ctx, "reply_watch.fired",
		"id", w.ID, "outcome", result.Outcome, "truncated", result.Truncated)
	return s.Deliver(ctx, w.ID)
}

func (s *replyWatchService) abandon(ctx context.Context, id, reason string) error {
	abandoned, err := s.store.AbandonReplyWatch(ctx, id, reason)
	if abandoned {
		s.settleGrace.Delete(id)
		s.manager.logger.InfoContext(ctx, "reply_watch.abandoned", "id", id, "reason", reason)
	}
	return err
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

func (s *replyWatchService) reconcileTarget(ctx context.Context, target, message string) {
	rows, err := s.store.ListPendingReplyWatchesByTarget(ctx, target)
	s.logError(ctx, err)
	for _, w := range rows {
		if w.State == store.ReplyWatchArmed && (message == "" || w.MessageID == message) {
			s.logWatchError(ctx, w, s.reconcileArmed(ctx, w, replyWatchEvent))
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
		s.logWatchError(ctx, w, s.Deliver(ctx, w.ID))
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
	if err := s.store.SupersedeReplyWatchFollowUps(ctx, ""); err != nil {
		return err
	}
	rows, err := s.store.ListReplyWatches(ctx, store.ReplyWatchFilter{})
	if err != nil {
		return err
	}
	s.expireStale(ctx)
	for _, w := range rows {
		s.logWatchError(ctx, w, s.reconcile(ctx, w.ID, replyWatchRecovery))
	}
	s.manager.logger.InfoContext(ctx, "reply_watch.recovered", "count", len(rows))
	return nil
}
func (s *replyWatchService) run(ctx context.Context) {
	ticker := time.NewTicker(s.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
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
