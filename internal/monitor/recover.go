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
	for _, record := range records {
		item := entryFromRecord(record)
		if !item.occupies() {
			if s.restoreHistory(item) {
				restored[item.state.SessionID] = struct{}{}
			}
			continue
		}
		touched[item.state.SessionID] = struct{}{}
		s.recoverOne(ctx, item, verdicts)
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

func (s *Service) recoverOne(ctx context.Context, item *entry, verdicts map[string]error) {
	if !s.applyOwnerOutcome(item, s.ownerVerdict(ctx, item, verdicts)) {
		s.wg.Go(func() { s.retryOwnerResume(item) })
	}
}

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

// Transient owner-resume failures (a store read, a provider hiccup) are retried with backoff before
// the monitor is recorded as interrupted, so one bad moment at boot does not drop a wake.
const (
	ownerResumeRetries    = 5
	ownerResumeBackoff    = 2 * time.Second
	ownerResumeBackoffMax = 30 * time.Second
)

func (s *Service) retryOwnerResume(item *entry) {
	backoff := ownerResumeBackoff
	for range ownerResumeRetries {
		timer := time.NewTimer(backoff)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, ownerResumeBackoffMax)
		ctx, cancel := context.WithTimeout(s.ctx, storeTimeout*6)
		err := s.owners.Resume(ctx, item.state.SessionID, item.binding)
		cancel()
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
	item.state.Status = status
	item.state.Reason = reason
	item.state.FiredAt = s.now()
	s.logLifecycle("monitor."+string(status), item.state, reason)
	record := s.decideLocked(item)
	if status != StatusCanceled {
		s.bucketLocked(item.state.SessionID)[item.state.ID] = item
	}
	s.mu.Unlock()
	_ = s.persistDecision(item, record, saveRetries)
}

func (s *Service) rearm(item *entry) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(s.ctx)
	item.cancel = cancel
	s.bucketLocked(item.state.SessionID)[item.state.ID] = item
	s.occupied++
	if item.terminalKey != "" {
		s.perTerminal[item.terminalKey]++
	}
	state := item.state
	s.logLifecycle("monitor.recovered", state, "registered")
	s.mu.Unlock()
	s.wg.Go(func() {
		defer cancel()
		s.runRecovered(runCtx, state, item)
	})
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
