package worktree

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
)

type StatusDetails struct {
	Status      *Status
	ForgeStatus *ForgeStatus
}

func (s *Service) StatusDetails(
	ctx context.Context,
	workspaceID string,
	id string,
	refresh bool,
	refreshForge bool,
) (*StatusDetails, error) {
	status, err := s.Status(ctx, workspaceID, id, refresh)
	if err != nil {
		return nil, err
	}
	item, err := s.store.Get(ctx, workspaceID, id)
	if err != nil || item == nil {
		if err == nil {
			err = ErrNotFound
		}
		return nil, err
	}
	forgeStatus, err := s.store.GetForgeStatus(ctx, workspaceID, id)
	if err != nil {
		return nil, fmt.Errorf("worktree: read cached forge status: %w", err)
	}
	if !refreshForge {
		return &StatusDetails{Status: status, ForgeStatus: forgeStatus}, nil
	}
	if s.forge == nil {
		return nil, ErrForgeUnavailable
	}
	remoteURLs := s.readOriginRemoteURLs(ctx, item.Path)
	if len(remoteURLs) == 0 {
		return nil, fmt.Errorf("%w: origin remote is unavailable", ErrForge)
	}
	forgeStatus, err = s.forge.Status(ctx, ForgeStatusRequest{
		WorkspaceID: workspaceID,
		WorktreeID:  id,
		RemoteURLs:  remoteURLs,
		Branch:      item.Branch,
	})
	if err != nil {
		kind := ErrForge
		if errors.Is(err, ErrForgeUnavailable) {
			kind = ErrForgeUnavailable
		}
		safeErr := errors.New(diagnostics.RedactAndBound(err.Error(), 2048))
		return nil, NewForgeFailure(kind, ForgeFailureCause(err), safeErr)
	}
	if forgeStatus == nil {
		return nil, fmt.Errorf("%w: provider returned no status", ErrForge)
	}
	if err := s.store.SaveForgeStatus(ctx, workspaceID, id, *forgeStatus); err != nil {
		return nil, fmt.Errorf("worktree: persist forge status: %w", err)
	}
	return &StatusDetails{Status: status, ForgeStatus: forgeStatus}, nil
}
