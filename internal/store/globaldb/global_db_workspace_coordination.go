package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var _ workspacepkg.CoordinationSettings = (*WorkspaceRepo)(nil)

// Get returns the workspace coordination setting. An absent record is the
// explicit disabled default and has revision zero.
func (g *WorkspaceRepo) Get(
	ctx context.Context,
	workspaceID string,
) (workspacepkg.CoordinationSetting, error) {
	if err := g.checkReady(ctx, "get workspace network coordination"); err != nil {
		return workspacepkg.CoordinationSetting{}, err
	}
	id := strings.TrimSpace(workspaceID)
	if id == "" {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf("store: workspace id is required")
	}
	row, err := g.queries.GetWorkspaceNetworkCoordination(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		if _, workspaceErr := g.queries.GetWorkspace(ctx, id); workspaceErr != nil {
			return workspacepkg.CoordinationSetting{}, mapCoordinationWorkspaceReadError(id, workspaceErr)
		}
		return workspacepkg.CoordinationSetting{
			WorkspaceID: id,
			ScopeKind:   workspacepkg.InvitationScopeWorkspace,
			ScopeID:     id,
		}, nil
	}
	if err != nil {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf(
			"store: get workspace network coordination %q: %w",
			id,
			err,
		)
	}
	return coordinationSettingFromGenerated(row)
}

func coordinationSettingFromGenerated(
	row sqlcgen.WorkspaceNetworkCoordination,
) (workspacepkg.CoordinationSetting, error) {
	if row.Enabled != 0 && row.Enabled != 1 {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf(
			"store: workspace network coordination enabled must be 0 or 1: %d",
			row.Enabled,
		)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf(
			"store: parse workspace network coordination updated_at: %w",
			err,
		)
	}
	updatedBy := strings.TrimSpace(row.UpdatedBy)
	if updatedBy == "" {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf(
			"store: workspace network coordination updated_by is required",
		)
	}
	return workspacepkg.CoordinationSetting{
		WorkspaceID: strings.TrimSpace(row.WorkspaceID),
		ScopeKind:   workspacepkg.InvitationScopeWorkspace,
		ScopeID:     strings.TrimSpace(row.WorkspaceID),
		Enabled:     row.Enabled == 1,
		Revision:    row.Revision,
		UpdatedAt:   updatedAt,
		UpdatedBy:   updatedBy,
	}, nil
}

func mapCoordinationWorkspaceReadError(workspaceID string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: workspace %q: %w", workspaceID, workspacepkg.ErrWorkspaceNotFound)
	}
	return fmt.Errorf("store: get workspace %q for network coordination: %w", workspaceID, err)
}
