package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

func (m *Manager) resolveCreateParticipation(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	request *participation.Request,
	resolved *participation.Spec,
	authority *participation.AuthorityScope,
) (participation.Spec, *participation.ResolvedObservation, error) {
	if request != nil && resolved != nil {
		return participation.Spec{}, nil, fmt.Errorf(
			"%w: network participation request and resolved binding are mutually exclusive",
			ErrValidation,
		)
	}
	if resolved != nil && authority != nil {
		return participation.Spec{}, nil, fmt.Errorf(
			"%w: resolved network participation cannot carry delegated authority",
			ErrValidation,
		)
	}
	if resolved != nil {
		spec := *resolved
		if err := validateSessionParticipationWorkspace(spec, workspaceID); err != nil {
			return participation.Spec{}, nil, err
		}
		return spec, nil, nil
	}
	if m == nil || m.participationResolver == nil {
		if request != nil {
			return participation.Spec{}, nil, fmt.Errorf(
				"session: participation resolver is required for session %q with network intent",
				sessionID,
			)
		}
		return participation.LocalSpec(), nil, nil
	}
	owner := participation.OwnerRef{
		WorkspaceID: strings.TrimSpace(workspaceID),
		Kind:        participation.OwnerKindSession,
		ID:          strings.TrimSpace(sessionID),
	}
	spec, err := m.participationResolver.Resolve(ctx, participation.ResolveInput{
		WorkspaceID: strings.TrimSpace(workspaceID),
		Owner:       owner,
		Request:     request,
		Authority:   authority,
	})
	if err != nil {
		return participation.Spec{}, nil, err
	}
	if err := validateSessionParticipationWorkspace(spec, workspaceID); err != nil {
		return participation.Spec{}, nil, err
	}
	return spec, &participation.ResolvedObservation{
		WorkspaceID: strings.TrimSpace(workspaceID),
		Owner:       owner,
		Spec:        spec,
	}, nil
}

func (m *Manager) observeCommittedParticipation(
	ctx context.Context,
	observation *participation.ResolvedObservation,
) {
	if m == nil || observation == nil {
		return
	}
	if err := participation.ObserveResolved(
		context.WithoutCancel(ctx),
		m.participationResolver,
		*observation,
	); err != nil {
		m.logger.ErrorContext(
			ctx,
			"session: committed network participation observation failed",
			"error", err,
			"workspace_id", observation.WorkspaceID,
			"owner_id", observation.Owner.ID,
		)
	}
}

func validateSessionParticipationWorkspace(spec participation.Spec, workspaceID string) error {
	if err := participation.ValidateSpec(spec); err != nil {
		return fmt.Errorf("session: validate network participation snapshot: %w", err)
	}
	if spec.Mode == participation.ModeLive &&
		strings.TrimSpace(spec.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return fmt.Errorf(
			"%w: live participation workspace %q does not match session workspace %q",
			ErrValidation,
			spec.WorkspaceID,
			workspaceID,
		)
	}
	return nil
}
