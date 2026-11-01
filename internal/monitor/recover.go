package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrOwnerStopped reports that the owner was stopped on purpose (an operator or
// agent decision), so its monitors must not outlive that decision.
var ErrOwnerStopped = errors.New("monitor: owner was stopped on purpose")

// OwnerIneligibleError reports that a restart-stopped owner may not be resumed
// automatically. Reason is the session restart-eligibility reason.
type OwnerIneligibleError struct{ Reason string }

func (e *OwnerIneligibleError) Error() string {
	return "monitor: owner cannot be resumed after restart: " + e.Reason
}

// recoveredMinTerminalWait keeps a re-armed terminal wait from starting already expired.
const recoveredMinTerminalWait = time.Second

// Recover restores durable monitors after a daemon restart. It never reruns a
// command: a timer resumes on its original deadline, a monitor that had already
// fired is re-delivered under its stable message id, and a terminal that no
// longer exists fires terminal_unavailable. A monitor whose owner was stopped on
// purpose is canceled; one whose owner cannot be resumed is recorded as
// interrupted, visibly, without a wake.
func (s *Service) Recover(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	records, err := s.store.LoadMonitors(ctx)
	if err != nil {
		return fmt.Errorf("monitor: load durable monitors: %w", err)
	}
	touched := make(map[string]struct{})
	restored := make(map[string]struct{})
	verdicts := make(map[string]error)
	var occupying []*entry
	for _, record := range records {
		item := entryFromRecord(record)
		if !item.occupies() {
			if s.restoreHistory(item) {
				restored[item.state.SessionID] = struct{}{}
			}
			continue
		}
		touched[item.state.SessionID] = struct{}{}
		occupying = append(occupying, item)
	}
	// Every record is registered before the first owner check, so an owner stopped at any point of the
	// recovery cancels all of its monitors through the normal stop path instead of racing the pass.
	retryCtxs := s.registerRecovering(occupying)
	for _, item := range occupying {
		if retryCtx, ok := retryCtxs[item]; ok {
			s.recoverOne(ctx, retryCtx, item, verdicts)
		}
	}
	s.capRestoredHistory(restored)
	s.pruneStore()
	for sessionID := range touched {
		s.notify(sessionID)
	}
	return nil
}

func entryFromRecord(record Record) *entry {
	mon := record.Monitor
	key := terminalKey(Spec{
		Kind: mon.Kind, WorkspaceID: record.WorkspaceID, ProfileID: record.ProfileID, TerminalID: mon.TerminalID,
	})
	return &entry{
		state: mon, terminalKey: key, timeout: record.Timeout, workspaceID: record.WorkspaceID,
		profileID: record.ProfileID, binding: record.OwnerBinding, version: record.Version,
		ownerCheckFailedAt: record.OwnerCheckFailedAt,
	}
}

// restoreHistory keeps interrupted and delivered outcomes inspectable until retention ends and
// reports whether the entry was restored.
func (s *Service) restoreHistory(item *entry) bool {
	if item.state.Status == StatusCanceled || s.now().Sub(item.state.FiredAt) > finishedRetention {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bucketLocked(item.state.SessionID)[item.state.ID] = item
	return true
}

// capRestoredHistory applies the per-session history cap to every session restored from the store.
func (s *Service) capRestoredHistory(sessions map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for sessionID := range sessions {
		s.pruneSessionLocked(sessionID, now)
	}
}

func (s *Service) bucketLocked(sessionID string) map[string]*entry {
	bucket := s.sessions[sessionID]
	if bucket == nil {
		bucket = make(map[string]*entry)
		s.sessions[sessionID] = bucket
	}
	return bucket
}

func (s *Service) recoverOne(
	ctx, retryCtx context.Context, item *entry, verdicts map[string]error,
) {
	if s.applyOwnerOutcome(item, s.ownerVerdict(ctx, item, verdicts)) {
		return
	}
	s.noteOwnerCheckFailed(item)
	s.wg.Go(func() { s.retryOwnerResume(retryCtx, item) })
}

// noteOwnerCheckFailed persists the first transient owner-check failure, which anchors the retry horizon
// across restarts so repeated restarts cannot keep an overdue monitor alive forever.
func (s *Service) noteOwnerCheckFailed(item *entry) {
	s.mu.Lock()
	if !item.ownerCheckFailedAt.IsZero() || !item.occupies() {
		s.mu.Unlock()
		return
	}
	item.ownerCheckFailedAt = s.now()
	record := s.decideLocked(item)
	s.mu.Unlock()
	// A lost anchor would restart the window at the next boot, so the write rides the decision queue:
	// immediate retries, then the background retry and the shutdown flush.
	s.writeDecision(item, record, saveRetries)
}

// registerRecovering registers every occupying record in one pass under the service lock: listed,
// counted, and cancelable with a per-monitor context, before any owner is checked. A stopped service
// registers nothing.
func (s *Service) registerRecovering(items []*entry) map[*entry]context.Context {
	ctxs := make(map[*entry]context.Context, len(items))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ctxs
	}
	for _, item := range items {
		retryCtx, cancel := context.WithCancel(s.ctx)
		item.cancel, item.recovering = cancel, true
		ctxs[item] = retryCtx
		s.bucketLocked(item.state.SessionID)[item.state.ID] = item
		s.occupied++
		if item.terminalKey != "" {
			s.perTerminal[item.terminalKey]++
		}
	}
	return ctxs
}

// A deleted inactive owner raises no stop notification, so its recovering monitors are canceled by
// the next owner check (at most ownerResumeBackoffMax later), not at deletion time.

// applyOwnerOutcome acts on a definitive owner outcome and reports false for a transient failure,
// which leaves the durable row untouched so it can be retried (and found again by the next boot).
func (s *Service) applyOwnerOutcome(item *entry, err error) bool {
	switch {
	case err == nil:
		s.rearm(item)
	case errors.Is(err, ErrOwnerStopped), errors.Is(err, ErrSessionGone):
		s.closeRecovered(item, StatusCanceled, ReasonSessionStopped)
	default:
		ineligible, ok := errors.AsType[*OwnerIneligibleError](err)
		if !ok {
			return false
		}
		s.closeRecovered(item, StatusInterrupted, ReasonOwnerPrefix+ineligible.Reason)
	}
	return true
}

// Transient owner-resume failures (a store read, a provider hiccup) are retried with backoff for as
// long as the monitor itself would have lived (at least deliveryRetryWindow), so an outage at boot
// does not drop a wake; only then is the monitor recorded as interrupted.
const (
	ownerResumeBackoff    = 2 * time.Second
	ownerResumeBackoffMax = 30 * time.Second
)

func (s *Service) retryOwnerResume(ctx context.Context, item *entry) {
	// The window is anchored on the first persisted failure, so restarts cannot extend it.
	s.mu.Lock()
	horizon := item.state.DeadlineAt
	if floor := item.ownerCheckFailedAt.Add(deliveryRetryWindow); floor.After(horizon) {
		horizon = floor
	}
	s.mu.Unlock()
	backoff := ownerResumeBackoff
	for s.now().Before(horizon) {
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, ownerResumeBackoffMax)
		resumeCtx, cancel := context.WithTimeout(ctx, storeTimeout*6)
		err := s.owners.Resume(resumeCtx, item.state.SessionID, item.binding)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if s.applyOwnerOutcome(item, err) {
			s.notify(item.state.SessionID)
			return
		}
	}
	s.closeRecovered(item, StatusInterrupted, ReasonOwnerPrefix+"resume_failed")
	s.notify(item.state.SessionID)
}

// ownerVerdict resumes each distinct owner binding once per recovery.
func (s *Service) ownerVerdict(ctx context.Context, item *entry, verdicts map[string]error) error {
	if s.owners == nil {
		return nil
	}
	key := item.state.SessionID + "\x00" + item.binding
	if err, ok := verdicts[key]; ok {
		return err
	}
	err := s.owners.Resume(ctx, item.state.SessionID, item.binding)
	verdicts[key] = err
	return err
}

func (s *Service) closeRecovered(item *entry, status Status, reason string) {
	s.mu.Lock()
	if item.recovering {
		// Cancel or an owner stop may already have ended it; otherwise return its slots and stop the retry.
		if !item.occupies() {
			s.mu.Unlock()
			return
		}
		item.recovering = false
		if !item.released {
			s.releaseLocked(item)
		}
		item.cancel()
		if status == StatusCanceled {
			delete(s.sessions[item.state.SessionID], item.state.ID)
		}
	}
	item.state.Status = status
	item.state.Reason = reason
	item.state.FiredAt = s.now()
	s.logLifecycle("monitor."+string(status), item.state, reason)
	record := s.decideLocked(item)
	if status != StatusCanceled {
		s.bucketLocked(item.state.SessionID)[item.state.ID] = item
	}
	s.mu.Unlock()
	s.writeDecision(item, record, saveRetries)
}

func (s *Service) rearm(item *entry) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	// Registered by Recover before any owner check; a monitor canceled in the meantime stays canceled.
	if !item.occupies() {
		s.mu.Unlock()
		return
	}
	item.recovering = false
	item.cancel()
	var cleared *Record
	if !item.ownerCheckFailedAt.IsZero() {
		item.ownerCheckFailedAt = time.Time{}
		record := s.decideLocked(item)
		cleared = &record
	}
	runCtx, cancel := context.WithCancel(s.ctx)
	item.cancel = cancel
	state := item.state
	s.logLifecycle("monitor.recovered", state, "registered")
	s.mu.Unlock()
	s.wg.Go(func() {
		defer cancel()
		// A cleared anchor must be durable before the watch starts: if the clear were lost in a crash, the
		// next boot would find the old anchor and close an overdue monitor on one transient owner error.
		if cleared != nil {
			s.writeDecision(item, *cleared, saveRetries)
			if !s.awaitConfirmed(runCtx, item) {
				return
			}
		}
		s.runRecovered(runCtx, state, item)
	})
}

// awaitConfirmed waits for the entry's latest decision to become durable (the decision queue keeps
// retrying it) and reports false if ctx ends first.
func (s *Service) awaitConfirmed(ctx context.Context, item *entry) bool {
	for !s.confirmed(item) {
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
	return true
}

func (s *Service) runRecovered(ctx context.Context, mon Monitor, item *entry) {
	if mon.Status == StatusDelivering {
		defer func() {
			if ctx.Err() != nil {
				s.logInterrupted(item)
			}
		}()
		s.deliver(ctx, item, Notification{
			Monitor: mon,
			Outcome: Outcome{Reason: mon.Reason, ExitCode: mon.ExitCode, OutputLost: mon.Kind == KindTerminal},
		})
		return
	}
	remaining := max(mon.DeadlineAt.Sub(s.now()), 0)
	spec := Spec{
		WorkspaceID: item.workspaceID, ProfileID: item.profileID, SessionID: mon.SessionID, Kind: mon.Kind,
		Description: mon.Description, TerminalID: mon.TerminalID, Until: mon.Until, Pattern: mon.Pattern,
	}
	var watch TerminalWatch
	if mon.Kind == KindTimer {
		spec.Delay = remaining
	} else {
		spec.Timeout = max(remaining, recoveredMinTerminalWait)
		watch = s.recoveredWatch(ctx, spec)
	}
	s.run(ctx, item, mon, spec, watch)
}

// recoveredWatch re-attaches to a terminal that still exists; a missing terminal
// reports unavailable instead of rerunning anything.
func (s *Service) recoveredWatch(ctx context.Context, spec Spec) TerminalWatch {
	if s.terminals != nil {
		if watch, err := s.terminals.WatchTerminal(ctx, spec); err == nil {
			return watch
		}
	}
	return func(context.Context) (Outcome, error) { return Outcome{Reason: ReasonTerminalFailed}, nil }
}
