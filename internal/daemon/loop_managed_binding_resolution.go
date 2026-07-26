package daemon

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/network/participation"
)

func (b *loopActionSessionBinder) bindFromActiveSession(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
	active goalpkg.SessionBinding,
) (looppkg.ActionSessionBinding, bool, error) {
	if err := b.validateActiveBindingPolicy(ctx, req, active); err != nil {
		return looppkg.ActionSessionBinding{}, true, err
	}
	if active.Ownership == goalpkg.BindingOwnershipOriginBorrowed &&
		strings.TrimSpace(req.OriginSessionID) != "" {
		matches, err := b.originParticipationMatches(ctx, req)
		if err != nil {
			return looppkg.ActionSessionBinding{}, true, err
		}
		if !matches {
			return looppkg.ActionSessionBinding{}, true, bindingMismatch(
				"origin session network participation differs from the Loop Run snapshot",
			)
		}
		binding, err := b.adoptOriginBinding(ctx, req, key)
		return binding, true, err
	}
	if active.BindingEpoch >= req.TargetBindingEpoch {
		return actionBindingFromGoal(req, active), true, nil
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
		matches, err := b.originParticipationMatches(ctx, req)
		if err != nil {
			return looppkg.ActionSessionBinding{}, err
		}
		if matches {
			return b.adoptOriginBinding(ctx, req, key)
		}
		req.OriginSessionID = ""
	}
	return b.ensureRunOwnedBinding(ctx, creator, req, key, active, activeFound)
}

func (b *loopActionSessionBinder) originParticipationMatches(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
) (bool, error) {
	sessionID := strings.TrimSpace(req.OriginSessionID)
	info, err := b.sessions.Status(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("daemon: load origin session %q: %w", sessionID, err)
	}
	if info == nil {
		return false, fmt.Errorf("daemon: load origin session %q: empty session info", sessionID)
	}
	if strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(string(req.WorkspaceID)) {
		return false, bindingMismatch("origin session workspace differs from the Loop Run workspace")
	}
	want := participationSnapshotValue(req.NetworkParticipation)
	if err := participation.ValidateSpec(want); err != nil {
		return false, fmt.Errorf("daemon: validate Loop Run network participation: %w", err)
	}
	got := participationSnapshotValue(&info.NetworkParticipation)
	if err := participation.ValidateSpec(got); err != nil {
		return false, fmt.Errorf("daemon: validate origin session network participation: %w", err)
	}
	return got == want, nil
}
