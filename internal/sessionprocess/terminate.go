package sessionprocess

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

// Terminate resolves targets only inside the caller's authorized ownership set.
// An empty session ID is reserved for the operator aggregate boundary.
func (s *Service) Terminate(
	ctx context.Context,
	sessionID string,
	targets []Identity,
	all bool,
	allowed func(Owner) bool,
) ([]Result, error) {
	snap, err := s.Refresh(ctx)
	if err != nil {
		return nil, err
	}
	if all && sessionID == "" {
		return nil, errors.New("all requires a session")
	}
	indexed := make(map[Identity]Process)
	for _, p := range snap.Processes {
		if (sessionID == "" || p.Owner.SessionID == sessionID) && allowed(p.Owner) {
			indexed[identity(p.ScannedProcess)] = p
			if all {
				targets = append(targets, identity(p.ScannedProcess))
			}
		}
	}
	// Serialize signaling and the durable audit against periodic observation publication.
	s.scanMu.Lock()
	results, err := s.terminateTargets(ctx, targets, indexed)
	s.scanMu.Unlock()
	_, refreshErr := s.Refresh(ctx)
	return results, errors.Join(err, refreshErr)
}

func (s *Service) terminateTargets(
	ctx context.Context,
	targets []Identity,
	indexed map[Identity]Process,
) ([]Result, error) {
	results := make([]Result, 0, len(targets))
	seen := make(map[Identity]bool)
	var auditErrors []error
	for _, id := range targets {
		if seen[id] {
			continue
		}
		seen[id] = true
		p, ok := indexed[id]
		if !ok {
			results = append(results, unavailableTarget(id))
			continue
		}
		result := s.terminateProcess(ctx, p)
		results = append(results, result)
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.source.RecordTermination(auditCtx, p, result)
		cancel()
		if err != nil {
			auditErrors = append(auditErrors, fmt.Errorf("record process %d termination: %w", id.PID, err))
		}
	}
	return results, errors.Join(auditErrors...)
}

func unavailableTarget(id Identity) Result {
	result := Result{Identity: id}
	observed, err := procutil.InspectProcess(id.PID)
	switch {
	case errors.Is(err, procutil.ErrProcessIdentityGone):
		result.Outcome = "already_exited"
	case err == nil && !observed.StartedAt.Equal(id.StartedAt):
		result.Outcome = "identity_mismatch"
	default:
		result.Outcome, result.Error = "unproven", "process ownership could not be verified"
	}
	return result
}

func (s *Service) terminateProcess(ctx context.Context, p Process) Result {
	result := Result{Identity: identity(p.ScannedProcess)}
	if p.TerminalID != "" {
		if err := s.source.CloseTerminal(ctx, p); err != nil {
			result.Outcome, result.Error = "failed", err.Error()
			return result
		}
		current, err := procutil.InspectProcess(p.PID)
		gone := errors.Is(err, procutil.ErrProcessIdentityGone)
		if gone || (err == nil && (current.Zombie || !current.StartedAt.Equal(p.StartedAt))) {
			result.Outcome = "terminated"
		} else {
			result.Outcome = "unproven"
		}
		return result
	}
	var outcome procutil.ProcessTermination
	if p.Attribution == attributionLineage {
		outcome = procutil.TerminateObservedProcess(ctx, p.ScannedProcess, 3*time.Second)
	} else {
		outcome = procutil.TerminateScannedProcess(ctx, p.ScannedProcess, 3*time.Second)
	}
	result.Outcome = outcome.Outcome
	if outcome.Err != nil {
		result.Error = outcome.Err.Error()
	}
	return result
}

// AfterStop owns and joins cleanup after an accepted stop outlives its caller.
func (s *Service) AfterStop(sessionID string, await func(context.Context) error) {
	s.workMu.Lock()
	defer s.workMu.Unlock()
	if s.closing {
		return
	}
	s.work.Go(func() {
		ctx, cancel := context.WithTimeout(s.runCtx, max(2*time.Minute, s.stopBudget))
		err := await(ctx)
		cancel()
		if err != nil {
			s.logger.Error("Await session stop before process cleanup", "session_id", sessionID, "error", err)
			return
		}
		ctx, cancel = context.WithTimeout(s.runCtx, 2*time.Minute)
		defer cancel()
		results, err := s.Terminate(ctx, sessionID, nil, true, func(o Owner) bool { return o.SessionID == sessionID })
		if err != nil {
			s.logger.Error("Terminate stopped session processes", "session_id", sessionID, "error", err)
			return
		}
		for _, result := range results {
			if result.Outcome != "terminated" && result.Outcome != "already_exited" {
				s.logger.Warn(
					"Session process termination unverified",
					"session_id",
					sessionID,
					"pid",
					result.PID,
					"outcome",
					result.Outcome,
				)
			}
		}
	})
}
