package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
	row, err := sqlcgen.New(g.db).LookupApprovalGrant(ctx, sqlcgen.LookupApprovalGrantParams{
		LastUsedAt:  store.FormatTimestamp(g.now().UTC()),
		ProfileID:   key.ProfileID,
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
	row, err := sqlcgen.New(g.db).PutApprovalGrant(ctx, sqlcgen.PutApprovalGrantParams{
		ProfileID:   grant.ProfileID,
		ID:          grant.ID,
		WorkspaceID: grant.WorkspaceID,
		AgentName:   grant.AgentName,
		ToolID:      grant.ToolID.String(),
		InputDigest: grant.InputDigest,
		Decision:    string(grant.Decision),
		CreatedAt:   store.FormatTimestamp(grant.CreatedAt),
		LastUsedAt:  store.FormatTimestamp(grant.LastUsedAt),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ApprovalGrant{}, fmt.Errorf(
			"%w: approval key belongs to another profile", toolspkg.ErrApprovalGrantInvalid,
		)
	}
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

// ListApprovalGrants returns one workspace's grants under the requested profile
// scope in stable creation order. Aggregate reads include profile owner labels.
func (g *ApprovalGrantRepo) ListApprovalGrants(
	ctx context.Context,
	readScope store.ReadScope,
	workspaceID string,
) (grants []toolspkg.ApprovalGrant, err error) {
	if err := g.checkReady(ctx, "list tool approval grants"); err != nil {
		return nil, err
	}
	if err := readScope.Validate(); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", toolspkg.ErrApprovalGrantInvalid)
	}
	statement, args := store.BuildWhereQuery(
		`SELECT g.id, g.profile_id, g.workspace_id, g.agent_name,
		g.tool_id, g.input_digest, g.decision, g.created_at, g.last_used_at,
		p.name, p.color, COALESCE(p.icon, ''), COALESCE(p.emoji, ''), p.state = 'archived'
		FROM tool_approval_grants g JOIN profiles p ON p.id = g.profile_id`,
		` ORDER BY g.created_at DESC, g.id`,
		store.ReadScopeClause("g.profile_id", readScope),
		store.StringClause("g.workspace_id", workspaceID),
	)
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list tool approval grants for %q: %w", workspaceID, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close tool approval grant rows: %w", closeErr))
		}
	}()
	grants = make([]toolspkg.ApprovalGrant, 0)
	for rows.Next() {
		var row sqlcgen.ToolApprovalGrant
		var owner toolspkg.ApprovalGrant
		if err := rows.Scan(
			&row.ID, &row.ProfileID, &row.WorkspaceID, &row.AgentName, &row.ToolID,
			&row.InputDigest, &row.Decision, &row.CreatedAt, &row.LastUsedAt,
			&owner.ProfileName, &owner.ProfileColor, &owner.ProfileIcon, &owner.ProfileEmoji,
			&owner.ProfileArchived,
		); err != nil {
			return nil, fmt.Errorf("store: scan tool approval grant: %w", err)
		}
		grant, err := approvalGrantFromRow(row)
		if err != nil {
			return nil, err
		}
		grant.ProfileName = owner.ProfileName
		grant.ProfileColor = owner.ProfileColor
		grant.ProfileIcon = owner.ProfileIcon
		grant.ProfileEmoji = owner.ProfileEmoji
		grant.ProfileArchived = owner.ProfileArchived
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate tool approval grants: %w", err)
	}
	return grants, nil
}

// RevokeApprovalGrant removes one profile/workspace-scoped remembered decision.
func (g *ApprovalGrantRepo) RevokeApprovalGrant(ctx context.Context, profileID, workspaceID, id string) error {
	if err := g.checkReady(ctx, "revoke tool approval grant"); err != nil {
		return err
	}
	profileID = strings.TrimSpace(profileID)
	workspaceID = strings.TrimSpace(workspaceID)
	id = strings.TrimSpace(id)
	if profileID == "" || workspaceID == "" || id == "" {
		return fmt.Errorf("%w: profile_id, workspace_id and id are required", toolspkg.ErrApprovalGrantInvalid)
	}
	rows, err := sqlcgen.New(g.db).RevokeApprovalGrant(ctx, sqlcgen.RevokeApprovalGrantParams{
		ProfileID: profileID, WorkspaceID: workspaceID, ID: id,
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
			ProfileID:   row.ProfileID,
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
