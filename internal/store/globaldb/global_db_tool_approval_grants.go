package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"uuid"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
	lastUsedAt := store.FormatTimestamp(g.now().UTC())
	row, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.LookupApprovalGrant",
		func(ctx context.Context, queries *sqlcgen.Queries) (sqlcgen.ToolApprovalGrant, error) {
			if key.AccessScope != "" {
				// A cross-scope read grant matches exactly; no wider tool grant stands in for it.
				return queries.LookupAccessScopeGrant(ctx, sqlcgen.LookupAccessScopeGrantParams{
					LastUsedAt:  lastUsedAt,
					ProfileID:   key.ProfileID,
					WorkspaceID: key.WorkspaceID,
					AgentName:   key.AgentName,
					ToolID:      key.ToolID.String(),
					InputDigest: key.InputDigest,
					AccessScope: key.AccessScope,
				})
			}
			return queries.LookupApprovalGrant(ctx, sqlcgen.LookupApprovalGrantParams{
				LastUsedAt:  lastUsedAt,
				ProfileID:   key.ProfileID,
				WorkspaceID: key.WorkspaceID,
				ToolID:      key.ToolID.String(),
				AgentName:   key.AgentName,
				InputDigest: key.InputDigest,
			})
		},
	)
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
	owner, err := g.approvalGrantProfileOwner(ctx, grant.ProfileID)
	if err != nil {
		return toolspkg.ApprovalGrant{}, err
	}
	now := g.now().UTC()
	if grant.ID == "" {
		grant.ID = uuid.NewV4().String()
	}
	if grant.CreatedAt.IsZero() {
		grant.CreatedAt = now
	}
	if grant.LastUsedAt.IsZero() {
		grant.LastUsedAt = now
	}
	row, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.PutApprovalGrant",
		func(ctx context.Context, queries *sqlcgen.Queries) (sqlcgen.ToolApprovalGrant, error) {
			return queries.PutApprovalGrant(ctx, sqlcgen.PutApprovalGrantParams{
				ProfileID:   grant.ProfileID,
				ID:          grant.ID,
				WorkspaceID: grant.WorkspaceID,
				AgentName:   grant.AgentName,
				ToolID:      grant.ToolID.String(),
				InputDigest: grant.InputDigest,
				Decision:    string(grant.Decision),
				CreatedAt:   store.FormatTimestamp(grant.CreatedAt),
				LastUsedAt:  store.FormatTimestamp(grant.LastUsedAt),
				AccessScope: grant.AccessScope,
			})
		},
	)
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
	stored, err := approvalGrantFromRow(row)
	if err != nil {
		return toolspkg.ApprovalGrant{}, err
	}
	stored.ProfileName = owner.ProfileName
	stored.ProfileColor = owner.ProfileColor
	stored.ProfileIcon = owner.ProfileIcon
	stored.ProfileEmoji = owner.ProfileEmoji
	stored.ProfileArchived = owner.ProfileArchived
	return stored, nil
}

func (g *ApprovalGrantRepo) approvalGrantProfileOwner(
	ctx context.Context,
	profileID string,
) (toolspkg.ApprovalGrant, error) {
	var owner toolspkg.ApprovalGrant
	err := g.readDB.QueryRowContext(
		ctx,
		`SELECT name, color, COALESCE(icon, ''), COALESCE(emoji, ''), state = 'archived'
		 FROM profiles WHERE id = ?`,
		strings.TrimSpace(profileID),
	).Scan(
		&owner.ProfileName,
		&owner.ProfileColor,
		&owner.ProfileIcon,
		&owner.ProfileEmoji,
		&owner.ProfileArchived,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ApprovalGrant{}, fmt.Errorf(
			"%w: approval profile %q does not exist",
			toolspkg.ErrApprovalGrantInvalid,
			strings.TrimSpace(profileID),
		)
	}
	if err != nil {
		return toolspkg.ApprovalGrant{}, fmt.Errorf(
			"store: read tool approval grant profile %q: %w",
			strings.TrimSpace(profileID),
			err,
		)
	}
	return owner, nil
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
	statement, args := store.BuildAndQuery(
		`SELECT g.id, g.profile_id, g.workspace_id, g.agent_name,
		g.tool_id, g.input_digest, g.decision, g.created_at, g.last_used_at, g.access_scope,
		p.name, p.color, COALESCE(p.icon, ''), COALESCE(p.emoji, ''), p.state = 'archived'
		FROM tool_approval_grants g JOIN profiles p ON p.id = g.profile_id WHERE g.workspace_id = ?`,
		` ORDER BY g.created_at DESC, g.id`,
		store.ReadScopeClause("g.profile_id", readScope),
	)
	args = append([]any{workspaceID}, args...)
	rows, err := g.readDB.QueryContext(ctx, statement, args...)
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
			&row.InputDigest, &row.Decision, &row.CreatedAt, &row.LastUsedAt, &row.AccessScope,
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
	if profileID == "" || id == "" {
		return fmt.Errorf("%w: profile_id and id are required", toolspkg.ErrApprovalGrantInvalid)
	}
	rows, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.RevokeApprovalGrant",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.RevokeApprovalGrant(ctx, sqlcgen.RevokeApprovalGrantParams{
				ProfileID: profileID, WorkspaceID: workspaceID, ID: id,
			})
		},
	)
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
		ID:          row.ID,
		ProfileID:   row.ProfileID,
		WorkspaceID: row.WorkspaceID,
		AgentName:   row.AgentName,
		ToolID:      toolspkg.ToolID(row.ToolID),
		InputDigest: row.InputDigest,
		AccessScope: row.AccessScope,
		Decision:    toolspkg.ApprovalGrantDecision(row.Decision),
		CreatedAt:   createdAt,
		LastUsedAt:  lastUsedAt,
	}.Normalize()
	if err := grant.Validate(); err != nil {
		return toolspkg.ApprovalGrant{}, fmt.Errorf("store: decode tool approval grant row: %w", err)
	}
	return grant, nil
}
