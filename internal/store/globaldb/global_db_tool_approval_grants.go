package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/google/uuid"
)

var _ toolspkg.ApprovalGrantStore = (*ApprovalGrantRepo)(nil)

// LookupApprovalGrant returns and touches the most-specific remembered decision.
func (g *ApprovalGrantRepo) LookupApprovalGrant(
	ctx context.Context,
	key toolspkg.ApprovalGrantKey,
) (toolspkg.ApprovalGrant, bool, error) {
	if err := g.checkReady(ctx, "lookup tool approval grant"); err != nil {
		return toolspkg.ApprovalGrant{}, false, err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return toolspkg.ApprovalGrant{}, false, err
	}
	row, err := g.queries.LookupApprovalGrant(ctx, sqlcgen.LookupApprovalGrantParams{
		LastUsedAt:  store.FormatTimestamp(g.now().UTC()),
		WorkspaceID: key.WorkspaceID,
		ToolID:      key.ToolID.String(),
		AgentName:   key.AgentName,
		InputDigest: key.InputDigest,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ApprovalGrant{}, false, nil
	}
	if err != nil {
		return toolspkg.ApprovalGrant{}, false, fmt.Errorf(
			"store: lookup tool approval grant for %q/%q: %w",
			key.WorkspaceID,
			key.ToolID,
			err,
		)
	}
	grant, err := approvalGrantFromRow(row)
	if err != nil {
		return toolspkg.ApprovalGrant{}, false, err
	}
	return grant, true, nil
}

// PutApprovalGrant creates or replaces one exact durable decision.
func (g *ApprovalGrantRepo) PutApprovalGrant(
	ctx context.Context,
	grant toolspkg.ApprovalGrant,
) (toolspkg.ApprovalGrant, error) {
	if err := g.checkReady(ctx, "put tool approval grant"); err != nil {
		return toolspkg.ApprovalGrant{}, err
	}
	grant = grant.Normalize()
	if err := grant.ValidateForPut(); err != nil {
		return toolspkg.ApprovalGrant{}, err
	}
	now := g.now().UTC()
	if grant.ID == "" {
		grant.ID = uuid.NewString()
	}
	if grant.CreatedAt.IsZero() {
		grant.CreatedAt = now
	}
	if grant.LastUsedAt.IsZero() {
		grant.LastUsedAt = now
	}
	row, err := g.queries.PutApprovalGrant(ctx, sqlcgen.PutApprovalGrantParams{
		ID:          grant.ID,
		WorkspaceID: grant.WorkspaceID,
		AgentName:   grant.AgentName,
		ToolID:      grant.ToolID.String(),
		InputDigest: grant.InputDigest,
		Decision:    string(grant.Decision),
		CreatedAt:   store.FormatTimestamp(grant.CreatedAt),
		LastUsedAt:  store.FormatTimestamp(grant.LastUsedAt),
	})
	if err != nil {
		return toolspkg.ApprovalGrant{}, fmt.Errorf(
			"store: put tool approval grant for %q/%q: %w",
			grant.WorkspaceID,
			grant.ToolID,
			err,
		)
	}
	return approvalGrantFromRow(row)
}

// ListApprovalGrants returns one workspace's grants in stable creation order.
func (g *ApprovalGrantRepo) ListApprovalGrants(
	ctx context.Context,
	workspaceID string,
) ([]toolspkg.ApprovalGrant, error) {
	if err := g.checkReady(ctx, "list tool approval grants"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", toolspkg.ErrApprovalGrantInvalid)
	}
	rows, err := g.queries.ListApprovalGrants(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: list tool approval grants for %q: %w", workspaceID, err)
	}
	grants := make([]toolspkg.ApprovalGrant, 0, len(rows))
	for _, row := range rows {
		grant, err := approvalGrantFromRow(row)
		if err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

// RevokeApprovalGrant removes one workspace-scoped remembered decision.
func (g *ApprovalGrantRepo) RevokeApprovalGrant(ctx context.Context, workspaceID, id string) error {
	if err := g.checkReady(ctx, "revoke tool approval grant"); err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	id = strings.TrimSpace(id)
	if workspaceID == "" || id == "" {
		return fmt.Errorf("%w: workspace_id and id are required", toolspkg.ErrApprovalGrantInvalid)
	}
	rows, err := g.queries.RevokeApprovalGrant(ctx, sqlcgen.RevokeApprovalGrantParams{
		WorkspaceID: workspaceID,
		ID:          id,
	})
	if err != nil {
		return fmt.Errorf("store: revoke tool approval grant %q in %q: %w", id, workspaceID, err)
	}
	if rows == 0 {
		return toolspkg.ErrApprovalGrantNotFound
	}
	return nil
}

func approvalGrantFromRow(row sqlcgen.ToolApprovalGrant) (toolspkg.ApprovalGrant, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return toolspkg.ApprovalGrant{}, fmt.Errorf("store: parse tool approval grant %q created_at: %w", row.ID, err)
	}
	lastUsedAt, err := store.ParseTimestamp(row.LastUsedAt)
	if err != nil {
		return toolspkg.ApprovalGrant{}, fmt.Errorf(
			"store: parse tool approval grant %q last_used_at: %w",
			row.ID,
			err,
		)
	}
	grant := toolspkg.ApprovalGrant{
		ID: row.ID,
		ApprovalGrantKey: toolspkg.ApprovalGrantKey{
			WorkspaceID: row.WorkspaceID,
			AgentName:   row.AgentName,
			ToolID:      toolspkg.ToolID(row.ToolID),
			InputDigest: row.InputDigest,
		},
		Decision:   toolspkg.ApprovalGrantDecision(row.Decision),
		CreatedAt:  createdAt,
		LastUsedAt: lastUsedAt,
	}.Normalize()
	if err := grant.Validate(); err != nil {
		return toolspkg.ApprovalGrant{}, fmt.Errorf("store: decode tool approval grant row: %w", err)
	}
	return grant, nil
}
