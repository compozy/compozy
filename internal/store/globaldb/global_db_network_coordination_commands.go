package globaldb

import (
	"context"

	eventspkg "github.com/compozy/agh/internal/events"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var _ workspacepkg.CoordinationCommandStore = (*WorkspaceRepo)(nil)

const networkCoordinationDismissedState = "dismissed"

// GetCoordination returns one explicit coordination view from persisted daemon truth.
func (g *WorkspaceRepo) GetCoordination(
	ctx context.Context,
	ref workspacepkg.CoordinationRef,
) (workspacepkg.CoordinationView, error) {
	if err := g.checkReady(ctx, "get network coordination"); err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	normalized, err := workspacepkg.NormalizeCoordinationRef(ref)
	if err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	return g.coordinationViewWithExecutor(ctx, g.db, normalized)
}

// SetCoordination commits the scoped CAS, future Task intent, and canonical events atomically.
func (g *WorkspaceRepo) SetCoordination(
	ctx context.Context,
	cmd workspacepkg.SetCoordination,
	actor taskpkg.ActorContext,
) (view workspacepkg.CoordinationView, err error) {
	if err := g.checkReady(ctx, "set network coordination"); err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	if err := actor.Validate(); err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	err = g.withTaskImmediateTransaction(ctx, "set network coordination", func(exec taskSQLExecutor) error {
		if err := g.validateCoordinationRefWithExecutor(ctx, exec, cmd.Ref); err != nil {
			return err
		}
		if cmd.Enabled {
			if err := requireCoordinationAvailability(ctx, exec); err != nil {
				return err
			}
		}
		setting, err := g.compareAndSetCoordinationWithExecutor(
			ctx,
			exec,
			cmd.Ref,
			cmd.Enabled,
			cmd.ExpectedRevision,
			actor.Actor.Ref,
		)
		if err != nil {
			return err
		}
		if cmd.Ref.ScopeKind == workspacepkg.InvitationScopeTask {
			if err := g.setTaskCoordinationIntentWithExecutor(ctx, exec, cmd, setting, actor); err != nil {
				return err
			}
		}
		if err := appendCoordinationEventSummary(
			ctx,
			exec,
			eventspkg.NetworkCoordinationSettingChanged,
			cmd.Ref,
			setting,
			actor,
			"",
		); err != nil {
			return err
		}
		view, err = g.coordinationViewWithExecutor(ctx, exec, cmd.Ref)
		return err
	})
	if err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	return view, nil
}

// SetCoordinationInvitation commits the invitation mutation, revision, provenance, and event atomically.
func (g *WorkspaceRepo) SetCoordinationInvitation(
	ctx context.Context,
	cmd workspacepkg.SetInvitation,
	actor taskpkg.ActorContext,
) (view workspacepkg.CoordinationView, err error) {
	if err := g.checkReady(ctx, "set network coordination invitation"); err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	if err := actor.Validate(); err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	err = g.withTaskImmediateTransaction(ctx, "set network coordination invitation", func(exec taskSQLExecutor) error {
		if err := g.validateCoordinationRefWithExecutor(ctx, exec, cmd.Ref); err != nil {
			return err
		}
		current, err := loadCoordinationSetting(ctx, exec, cmd.Ref)
		if err != nil {
			return err
		}
		setting, err := g.compareAndSetCoordinationWithExecutor(
			ctx,
			exec,
			cmd.Ref,
			current.Enabled,
			cmd.ExpectedRevision,
			actor.Actor.Ref,
		)
		if err != nil {
			return err
		}
		if err := mutateCoordinationInvitation(ctx, exec, cmd, actor, g.now().UTC()); err != nil {
			return err
		}
		eventType := eventspkg.NetworkCoordinationInvitationReset
		state := "reset"
		if cmd.Dismissed {
			eventType = eventspkg.NetworkCoordinationInvitationDismissed
			state = networkCoordinationDismissedState
		}
		if err := appendCoordinationEventSummary(ctx, exec, eventType, cmd.Ref, setting, actor, state); err != nil {
			return err
		}
		view, err = g.coordinationViewWithExecutor(ctx, exec, cmd.Ref)
		return err
	})
	if err != nil {
		return workspacepkg.CoordinationView{}, err
	}
	return view, nil
}
