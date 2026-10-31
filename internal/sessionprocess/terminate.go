package sessionprocess

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

// Scope selects what a session-wide stop covers besides its explicit targets.
type Scope int

const (
	// Targets stops only the trees of the given targets.
	Targets Scope = iota
	// AllLeftRunning adds every Background and Terminal tree; tool calls stay with their turn.
	AllLeftRunning
	// AllIncludingTools adds every tree, tool calls included (stop, archive, delete cleanup).
	AllIncludingTools
	// Captured stops exactly the given identities, never their trees' later members: delayed
	// cleanup captured them before the session could resume and start new work.
	Captured
)

// Terminate resolves targets only inside the caller's authorized ownership set and stops each
// target's whole tree. An empty session ID is reserved for the operator aggregate boundary.
func (s *Service) Terminate(
	ctx context.Context,
	sessionID string,
	targets []Identity,
	scope Scope,
	allowed func(Owner) bool,
) ([]Result, error) {
	snap, err := s.Refresh(ctx)
	if err != nil {
		return nil, err
	}
	if (scope == AllLeftRunning || scope == AllIncludingTools) && sessionID == "" {
		return nil, errors.New("all requires a session")
	}
	indexed := make(map[Identity]Process)
	for _, p := range snap.Processes {
		if (sessionID == "" || p.Owner.SessionID == sessionID) && allowed(p.Owner) {
			indexed[identity(p.ScannedProcess)] = p
		}
	}
	targets = expandTrees(snap.Processes, indexed, targets, scope)
	// Serialize signaling and the durable audit against periodic observation publication.
	if err := s.lockScan(ctx); err != nil {
		return nil, err
	}
	results, err := s.terminateTargets(ctx, targets, indexed)
	s.scanGate.Release(1)
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

// expandTrees replaces each target with every authorized member of its tree, root first, and adds
// the trees a session-wide scope covers. A target outside every tree stays, to report it.
func expandTrees(rows []Process, indexed map[Identity]Process, targets []Identity, scope Scope) []Identity {
	if scope == Captured {
		return targets
	}
	trees := Trees(rows)
	byMember := make(map[Identity]int)
	for i, tree := range trees {
		for _, member := range tree.Members {
			byMember[member] = i
		}
	}
	var out []Identity
	add := func(tree Tree) {
		for _, member := range tree.Members {
			if _, ok := indexed[member]; ok {
				out = append(out, member)
			}
		}
	}
	for _, target := range targets {
		if i, ok := byMember[target]; ok {
			if _, allowed := indexed[target]; allowed {
				add(trees[i])
				continue
			}
		}
		out = append(out, target)
	}
	if scope == AllLeftRunning || scope == AllIncludingTools {
		for _, tree := range trees {
			if scope == AllIncludingTools || tree.Kind != kindTool {
				add(tree)
			}
		}
	}
	return out
}

func unavailableTarget(id Identity) Result {
	return Result{Identity: id, Outcome: "unproven", Error: "process ownership could not be verified"}
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
	// Freeze identities before waiting; a resumed session must not expand this stop's targets.
	s.mu.RLock()
	var targets []Identity
	for _, p := range s.snapshot.Processes {
		if p.Owner.SessionID == sessionID {
			targets = append(targets, identity(p.ScannedProcess))
		}
	}
	s.mu.RUnlock()
	s.work.Go(func() {
		err := s.awaitCleanupStop(await)
		if err != nil {
			s.logger.Error("Await session stop before process cleanup", "session_id", sessionID, "error", err)
			return
		}
		ctx, cancel := context.WithTimeout(s.runCtx, 2*time.Minute)
		defer cancel()
		results, err := s.Terminate(
			ctx,
			sessionID,
			targets,
			Captured,
			func(o Owner) bool { return o.SessionID == sessionID },
		)
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

// A slow accepted stop remains owned until it settles or this service shuts down.
func (s *Service) awaitCleanupStop(await func(context.Context) error) error {
	for {
		ctx, cancel := context.WithTimeout(s.runCtx, max(2*time.Minute, s.stopBudget))
		err := await(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) || s.runCtx.Err() != nil {
			return err
		}
		retry := time.NewTimer(time.Second)
		select {
		case <-s.runCtx.Done():
			retry.Stop()
			return s.runCtx.Err()
		case <-retry.C:
		}
	}
}
