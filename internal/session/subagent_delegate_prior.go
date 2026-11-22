package session

import (
	"context"
	"errors"
	"fmt"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func (s *subagentService) resolvePriorDelegation(ctx context.Context, req *SubagentRequest) error {
	if req.PriorSubagentID == "" {
		return nil
	}
	prior, err := s.store.GetSubagent(ctx, req.Caller.WorkspaceID, req.PriorSubagentID)
	if err != nil {
		if errors.Is(err, store.ErrSubagentNotFound) {
			return invalidSubagent(fmt.Sprintf("prior_subagent_id %q not found.", req.PriorSubagentID))
		}
		return fmt.Errorf("lookup prior subagent %q: %w", req.PriorSubagentID, err)
	}
	if prior.ParentSessionID != req.Caller.SessionID {
		return invalidSubagent("prior_subagent_id belongs to a different session.")
	}
	if req.Isolation == SubagentIsolationWorktree && req.BaseRef == "" {
		if prior.Worktree != nil && prior.Worktree.Branch != "" {
			req.BaseRef = prior.Worktree.Branch
		} else if prior.Isolation == SubagentIsolationWorktree {
			return invalidSubagent(fmt.Sprintf("prior subagent %q has no associated worktree branch yet.", req.PriorSubagentID))
		}
	}
	return nil
}

func (s *subagentService) trackFlight(id string) func() {
	s.mu.Lock()
	flight := &subagentFlight{done: make(chan struct{})}
	s.flights[id] = flight
	s.mu.Unlock()
	return sync.OnceFunc(func() {
		s.mu.Lock()
		delete(s.flights, id)
		close(flight.done)
		s.mu.Unlock()
	})
}

func (s *subagentService) prepareIsolationOpts(
	ctx context.Context,
	req SubagentRequest,
	parent *Info,
	row *store.SessionSubagent,
	opts *SpawnOpts,
) error {
	if req.Isolation != SubagentIsolationWorktree {
		return nil
	}
	if err := s.provisionIsolation(ctx, req, parent, row); err != nil {
		return fmt.Errorf("subagent isolation: %w", err)
	}
	opts.IsolatedWorktreeID = row.WorktreeState().ID
	opts.Subagent.WorktreeID = row.WorktreeState().ID
	return nil
}

func subagentSpawnOptions(
	row store.SessionSubagent,
	target SubagentTarget,
	mode compozyconfig.PermissionMode,
	policy store.SessionPermissionPolicy,
) SpawnOpts {
	return SpawnOpts{
		ParentSessionID:  row.ParentSessionID,
		ParentTurnID:     row.ParentTurnID,
		AgentName:        target.Agent,
		Provider:         target.Provider,
		Model:            target.Model,
		ReasoningEffort:  target.ReasoningEffort,
		Speed:            speedpkg.Speed(target.Speed),
		ACPOptions:       target.ACPOptions,
		Name:             row.Title,
		SpawnRole:        store.SubagentSpawnRole,
		AutoStopOnParent: true,
		NotifyCreatorSet: true,
		IdempotencyKey:   row.ID,
		Permissions:      mode,
		PermissionPolicy: policy,
		Subagent: &hookspkg.SubagentSpawnPayload{
			Title:     row.Title,
			Role:      row.Role,
			TaskChars: int(row.TaskChars),
			Isolation: row.Isolation,
		},
	}
}
