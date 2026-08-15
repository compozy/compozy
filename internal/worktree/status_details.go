package worktree

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
)

type StatusDetails struct {
	WorktreeID  string
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
	item, err := s.store.Get(ctx, workspaceID, id)
	if err != nil || item == nil {
		if err == nil {
			err = ErrNotFound
		}
		return nil, err
	}
	status, err := s.Status(ctx, workspaceID, item.ID, refresh)
	if err != nil {
		return nil, err
	}
	forgeStatus, err := s.store.GetForgeStatus(ctx, workspaceID, item.ID)
	if err != nil {
		return nil, fmt.Errorf("worktree: read cached forge status: %w", err)
	}
	if !refreshForge {
		return &StatusDetails{WorktreeID: item.ID, Status: status, ForgeStatus: forgeStatus}, nil
	}
	if s.forge == nil {
		return nil, ErrForgeUnavailable
	}
	remoteURLs, err := s.readOriginRemoteURLs(ctx, item.Path)
	if err != nil {
		return nil, err
	}
	if len(remoteURLs) == 0 {
		return nil, fmt.Errorf("%w: origin remote is unavailable", ErrForge)
	}
	forgeStatus, err = s.forge.Status(ctx, ForgeStatusRequest{
		WorkspaceID: workspaceID,
		WorktreeID:  item.ID,
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
	if forgeStatus.PRURL != "" {
		forgeStatus.PRURL, err = sanitizeForgeWebURL(forgeStatus.PRURL)
		if err != nil {
			return nil, fmt.Errorf("%w: provider returned an invalid pull request URL", ErrForge)
		}
	}
	if err := s.store.SaveForgeStatus(ctx, workspaceID, item.ID, *forgeStatus); err != nil {
		return nil, fmt.Errorf("worktree: persist forge status: %w", err)
	}
	return &StatusDetails{WorktreeID: item.ID, Status: status, ForgeStatus: forgeStatus}, nil
}
