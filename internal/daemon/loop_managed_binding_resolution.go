package daemon

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
)

func (b *loopActionSessionBinder) bindFromActiveSession(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
	active goalpkg.SessionBinding,
) (looppkg.ActionSessionBinding, bool, error) {
	if active.Ownership == goalpkg.BindingOwnershipOriginBorrowed &&
		strings.TrimSpace(req.OriginSessionID) != "" {
		err := b.validateOriginWorkspace(ctx, req)
		if err != nil {
			return looppkg.ActionSessionBinding{}, true, err
		}
		binding, err := b.adoptOriginBinding(ctx, req, key)
		return binding, true, err
	}
	appliedRuntime, err := b.validateActiveBindingPolicy(ctx, req, active)
	if err != nil {
		return looppkg.ActionSessionBinding{}, true, err
	}
	if active.BindingEpoch >= req.TargetBindingEpoch {
		return actionBindingFromGoal(req, active, appliedRuntime), true, nil
	}
	if req.TargetBindingEpoch != active.BindingEpoch+1 {
		return looppkg.ActionSessionBinding{}, true, bindingMismatch(
			"requested binding epoch skips the active epoch",
		)
	}
	return looppkg.ActionSessionBinding{}, false, nil
}

func (b *loopActionSessionBinder) bindMissingOrAdvancedSession(
	ctx context.Context,
	creator loopManagedSessionManager,
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
	active goalpkg.SessionBinding,
	activeFound bool,
) (looppkg.ActionSessionBinding, error) {
	if !activeFound && strings.TrimSpace(req.OriginSessionID) != "" {
		err := b.validateOriginWorkspace(ctx, req)
		if err != nil {
			return looppkg.ActionSessionBinding{}, err
		}
		return b.adoptOriginBinding(ctx, req, key)
	}
	return b.ensureRunOwnedBinding(ctx, creator, req, key, active, activeFound)
}

func (b *loopActionSessionBinder) validateOriginWorkspace(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
) error {
	sessionID := strings.TrimSpace(req.OriginSessionID)
	info, err := b.sessions.Status(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("daemon: load origin session %q: %w", sessionID, err)
	}
	if info == nil {
		return fmt.Errorf("daemon: load origin session %q: empty session info", sessionID)
	}
	if strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(string(req.WorkspaceID)) {
		return bindingMismatch("origin session workspace differs from the Loop Run workspace")
	}
	return nil
}
