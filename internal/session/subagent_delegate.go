package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func (s *subagentService) Delegate(ctx context.Context, req SubagentRequest) (Subagent, error) {
	req, err := normalizeSubagentRequest(req)
	if err != nil {
		return Subagent{}, err
	}
	snap, err := s.caller(ctx, req.Caller)
	if err != nil {
		return Subagent{}, err
	}
	mode, policy, err := subagentPermissions(snap.Info, req)
	if err != nil {
		return Subagent{}, &SubagentError{Code: "permission_escalation_denied", Message: err.Error(), Err: err}
	}
	fingerprint, err := subagentFingerprint(req)
	if err != nil {
		return Subagent{}, err
	}
	id := subagentID(req.Caller.SessionID, req.IdempotencyKey)
	unlock := s.lock(req.Caller.SessionID)
	existing, lookupErr := s.store.GetSubagent(ctx, req.Caller.WorkspaceID, id)
	if lookupErr == nil {
		unlock()
		if existing.RequestFingerprint != fingerprint {
			return Subagent{}, invalidSubagent("idempotency_key reused with a different request.")
		}
		return s.replay(ctx, existing)
	}
	if !errors.Is(lookupErr, store.ErrSubagentNotFound) {
		unlock()
		return Subagent{}, lookupErr
	}
	target, err := s.runtime.Resolve(ctx, snap.Info, req.Target)
	if err != nil {
		unlock()
		return Subagent{}, err
	}
	depth, err := s.depth(ctx, req.Caller.SessionID)
	if err != nil {
		unlock()
		return Subagent{}, err
	}
	row, created, err := s.reserveDelegation(ctx, req, target, depth, fingerprint)
	if err != nil {
		unlock()
		if errors.Is(err, store.ErrSubagentIdempotencyConflict) {
			err = invalidSubagent("idempotency_key reused with a different request.")
		}
		return Subagent{}, err
	}
	if !created {
		unlock()
		return s.replay(ctx, row)
	}
	s.mu.Lock()
	done := make(chan struct{})
	s.flights[id] = done
	s.mu.Unlock()
	s.publish(ctx, row)
	s.runtime.PublishParent(ctx, row.ParentSessionID)
	unlock()
	finishFlight := sync.OnceFunc(func() { s.mu.Lock(); delete(s.flights, id); close(done); s.mu.Unlock() })
	defer finishFlight()
	opts := SpawnOpts{
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
		IdempotencyKey:   id,
		Permissions:      mode,
		PermissionPolicy: policy,
		Subagent:         &hookspkg.SubagentSpawnPayload{Title: row.Title, Role: row.Role, TaskChars: row.TaskChars},
	}
	return s.startDelegation(ctx, req, row, opts, finishFlight)
}

func (s *subagentService) startDelegation(
	ctx context.Context,
	req SubagentRequest,
	row store.SessionSubagent,
	opts SpawnOpts,
	finishFlight func(),
) (Subagent, error) {
	id := row.ID
	child, err := s.runtime.Spawn(ctx, opts)
	if err != nil {
		if errors.Is(err, ErrSubagentCapabilityDenied) {
			if deleteErr := s.store.DeleteReserved(context.WithoutCancel(ctx), id); deleteErr != nil {
				return Subagent{}, errors.Join(err, deleteErr)
			}
			s.runtime.PublishParent(ctx, row.ParentSessionID)
			return Subagent{}, err
		}
		return s.failDelegation(ctx, row, child, err)
	}
	unlock := s.lock(row.ParentSessionID)
	current, readErr := s.store.GetSubagent(ctx, row.WorkspaceID, row.ID)
	if readErr != nil {
		unlock()
		return s.failDelegation(ctx, row, child, readErr)
	}
	if store.IsSubagentStatusTerminal(current.Status) {
		unlock()
		if err := s.runtime.Stop(ctx, child); err != nil {
			return Subagent{}, err
		}
		return presentSubagent(current), nil
	}
	linked, linkErr := s.store.LinkChild(ctx, id, child, s.now().UTC())
	err = linkErr
	if err == nil {
		row = linked
	}
	if err == nil {
		s.publish(ctx, row)
	}
	unlock()
	if err != nil {
		return s.failDelegation(ctx, row, child, err)
	}
	finishFlight()
	// Stop can win while the provider starts; never admit work after that boundary.
	if _, err = s.caller(ctx, req.Caller); err != nil {
		return s.failDelegation(ctx, row, child, err)
	}
	if err = s.runtime.Admit(ctx, row, subagentPrompt(row.Role, req.Task)); err != nil {
		return s.failDelegation(ctx, row, child, err)
	}
	if err := s.store.MarkFirstPromptAdmitted(ctx, id); err != nil {
		return Subagent{}, err
	}
	if err := s.OnChildSettled(ctx, child); err != nil {
		return Subagent{}, err
	}
	if req.Mode == SubagentModeWait {
		return s.wait(ctx, row, req.Timeout)
	}
	return s.Get(ctx, row.WorkspaceID, id)
}

func (s *subagentService) failDelegation(
	ctx context.Context,
	row store.SessionSubagent,
	child string,
	cause error,
) (Subagent, error) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	unlock := s.lock(row.ParentSessionID)
	summary := cause.Error()
	settled, changed, err := s.store.FinalizeSubagent(
		cleanup,
		store.SubagentFinalize{
			ID:        row.ID,
			Status:    store.SubagentStatusFailed,
			WorkState: store.SubagentWorkStateResultAvailable,
			Error:     &summary,
			SettledAt: s.now().UTC(),
		},
	)
	if err == nil && changed {
		s.publishTerminal(cleanup, settled)
	}
	unlock()
	if child != "" {
		err = errors.Join(err, s.runtime.Stop(cleanup, child))
	}
	if err != nil {
		return Subagent{}, errors.Join(cause, err)
	}
	return presentSubagent(settled), nil
}

func (s *subagentService) replay(ctx context.Context, row store.SessionSubagent) (Subagent, error) {
	if row.Status != store.SubagentStatusQueued {
		return presentSubagent(row), nil
	}
	s.mu.Lock()
	done := s.flights[row.ID]
	s.mu.Unlock()
	if done == nil {
		return presentSubagent(row), nil
	}
	timer := time.NewTimer(defaultLifecycleTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Subagent{}, ctx.Err()
	case <-timer.C:
	case <-done:
	}
	return s.Get(ctx, row.WorkspaceID, row.ID)
}

func (s *subagentService) wait(
	ctx context.Context,
	row store.SessionSubagent,
	timeout time.Duration,
) (Subagent, error) {
	updates, cancel, err := s.SubscribeSubagentUpdates(ctx, row.ParentSessionID)
	if err != nil {
		return Subagent{}, err
	}
	defer cancel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		current, err := s.Get(ctx, row.WorkspaceID, row.ID)
		if err != nil {
			return Subagent{}, err
		}
		if store.IsSubagentStatusTerminal(current.Status) || current.Delivery == store.SubagentDeliveryDisposed {
			return current, nil
		}
		select {
		case <-ctx.Done():
			return Subagent{}, ctx.Err()
		case <-s.ctx.Done():
			return Subagent{}, s.ctx.Err()
		case _, open := <-updates:
			if !open {
				return Subagent{}, fmt.Errorf("session: subagent update stream closed")
			}
		case <-timer.C:
			if err := s.UpgradeWakePolicy(ctx, row.ID); err != nil {
				return Subagent{}, err
			}
			current, err = s.Get(ctx, row.WorkspaceID, row.ID)
			current.WaitTimedOut = true
			return current, err
		}
	}
}

func (s *subagentService) reserveDelegation(
	ctx context.Context,
	req SubagentRequest,
	target SubagentTarget,
	depth int,
	fingerprint string,
) (store.SessionSubagent, bool, error) {
	id := subagentID(req.Caller.SessionID, req.IdempotencyKey)
	now := s.now().UTC()
	policyName := store.SubagentWakePolicyAlways
	if req.Mode == SubagentModeWait {
		policyName = store.SubagentWakePolicySettledOnly
	}
	return s.store.ReserveSubagent(ctx, store.SessionSubagent{
		ID: id, WorkspaceID: req.Caller.WorkspaceID, ParentSessionID: req.Caller.SessionID,
		ParentTurnID: req.Caller.TurnID, ParentToolCallID: req.Caller.ToolCallID,
		Origin: store.SubagentOriginDelegated, IdempotencyKey: req.IdempotencyKey, RequestFingerprint: fingerprint,
		Title: req.Title, Role: req.Role, TaskChars: utf8.RuneCountInString(req.Task), PendingTask: &req.Task,
		RuntimeAgent: target.Agent, RuntimeProvider: target.Provider, RuntimeModel: target.Model,
		RuntimeEffort: target.ReasoningEffort, RuntimeSpeed: target.Speed, Depth: depth + 1,
		Status: store.SubagentStatusQueued, WorkState: store.SubagentWorkStateWorking,
		WakePolicy: policyName, Delivery: store.SubagentDeliveryNone, CreatedAt: now, UpdatedAt: now,
	})
}
