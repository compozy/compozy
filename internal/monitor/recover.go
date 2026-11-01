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
	verdicts := make(map[string]error)
	for _, record := range records {
		item := entryFromRecord(record)
		if !item.occupies() {
			s.restoreHistory(item)
			continue
		}
		touched[item.state.SessionID] = struct{}{}
		s.recoverOne(ctx, item, verdicts)
	}
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

// restoreHistory keeps interrupted and delivered outcomes inspectable until retention ends.
func (s *Service) restoreHistory(item *entry) {
	if item.state.Status == StatusCanceled || s.now().Sub(item.state.FiredAt) > finishedRetention {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bucketLocked(item.state.SessionID)[item.state.ID] = item
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
	err := s.ownerVerdict(ctx, item, verdicts)
	switch {
	case err == nil:
		s.rearm(item)
	case errors.Is(err, ErrOwnerStopped), errors.Is(err, ErrSessionGone):
		s.closeRecovered(item, StatusCanceled, ReasonSessionStopped)
	default:
		reason := "resume_failed"
		if ineligible, ok := errors.AsType[*OwnerIneligibleError](err); ok {
			reason = ineligible.Reason
		}
		s.closeRecovered(item, StatusInterrupted, ReasonOwnerPrefix+reason)
	}
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
	record := item.recordLocked()
	if status != StatusCanceled {
		s.bucketLocked(item.state.SessionID)[item.state.ID] = item
	}
	s.mu.Unlock()
	s.save(record)
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
		s.deliver(ctx, Notification{
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
	s.run(ctx, mon, spec, watch)
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
