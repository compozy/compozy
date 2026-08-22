package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// MarkDeadEntity creates or refreshes one confirmed-dead workspace runtime.
func (g *DeadEntityRepo) MarkDeadEntity(ctx context.Context, entity store.DeadEntity) error {
	if err := g.checkReady(ctx, "mark dead entity"); err != nil {
		return err
	}
	normalized := entity.Normalize()
	if normalized.MarkedAt.IsZero() {
		normalized.MarkedAt = g.now().UTC()
	}
	if err := normalized.Validate(); err != nil {
		return err
	}
	var persistedProfileID string
	err := g.db.QueryRowContext(ctx, `INSERT INTO dead_entities (
			profile_id, workspace_id, kind, entity_id, reason, marked_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, workspace_id, kind, entity_id) DO UPDATE SET
			reason = excluded.reason, marked_at = excluded.marked_at
		WHERE dead_entities.profile_id = excluded.profile_id
		RETURNING profile_id`,
		normalized.ProfileID, normalized.WorkspaceID, string(normalized.Kind),
		normalized.EntityID, normalized.Reason, store.FormatTimestamp(normalized.MarkedAt),
	).Scan(&persistedProfileID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf(
			"%w: dead entity %q/%q/%q belongs to another profile",
			store.ErrInvalidDeadEntity,
			normalized.WorkspaceID,
			normalized.Kind,
			normalized.EntityID,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"store: mark dead entity %q/%q/%q: %w",
			normalized.WorkspaceID,
			normalized.Kind,
			normalized.EntityID,
			err,
		)
	}
	return nil
}

// ClearDeadEntity removes one dead mark. Missing rows are a successful no-op.
func (g *DeadEntityRepo) ClearDeadEntity(
	ctx context.Context,
	key store.DeadEntityKey,
) error {
	if err := g.checkReady(ctx, "clear dead entity"); err != nil {
		return err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	if _, err := g.db.ExecContext(ctx, `DELETE FROM dead_entities
		WHERE profile_id = ? AND workspace_id = ? AND kind = ? AND entity_id = ?`,
		key.ProfileID, key.WorkspaceID, string(key.Kind), key.EntityID,
	); err != nil {
		return fmt.Errorf(
			"store: clear dead entity %q/%q/%q: %w",
			key.WorkspaceID,
			key.Kind,
			key.EntityID,
			err,
		)
	}
	return nil
}

// FindDeadEntity returns one exact workspace-scoped dead mark.
func (g *DeadEntityRepo) FindDeadEntity(
	ctx context.Context,
	key store.DeadEntityKey,
) (store.DeadEntity, bool, error) {
	if err := g.checkReady(ctx, "find dead entity"); err != nil {
		return store.DeadEntity{}, false, err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return store.DeadEntity{}, false, err
	}
	row := sqlcgen.DeadEntity{}
	err := g.db.QueryRowContext(ctx, `SELECT profile_id, workspace_id, kind, entity_id, reason, marked_at
		FROM dead_entities WHERE profile_id = ? AND workspace_id = ? AND kind = ? AND entity_id = ?`,
		key.ProfileID, key.WorkspaceID, string(key.Kind), key.EntityID,
	).Scan(&row.ProfileID, &row.WorkspaceID, &row.Kind, &row.EntityID, &row.Reason, &row.MarkedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return store.DeadEntity{}, false, nil
	}
	if err != nil {
		return store.DeadEntity{}, false, fmt.Errorf(
			"store: find dead entity %q/%q/%q: %w",
			key.WorkspaceID,
			key.Kind,
			key.EntityID,
			err,
		)
	}
	entity, err := deadEntityFromRow(row)
	if err != nil {
		return store.DeadEntity{}, false, err
	}
	return entity, true, nil
}

// ListDeadEntities returns one workspace's dead marks in stable forensic order.
func (g *DeadEntityRepo) ListDeadEntities(
	ctx context.Context,
	readScope store.ReadScope,
	workspaceID string,
) (entities []store.DeadEntity, err error) {
	if err := g.checkReady(ctx, "list dead entities"); err != nil {
		return nil, err
	}
	if err := readScope.Validate(); err != nil {
		return nil, err
	}
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if trimmedWorkspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", store.ErrInvalidDeadEntity)
	}
	where, args := store.BuildClauses(
		store.ReadScopeClause("d.profile_id", readScope),
		store.StringClause("d.workspace_id", trimmedWorkspaceID),
	)
	rows, err := g.db.QueryContext(ctx, `SELECT d.profile_id, p.name, p.color,
		COALESCE(p.icon, ''), COALESCE(p.emoji, ''), p.state = 'archived',
		d.workspace_id, d.kind, d.entity_id, d.reason, d.marked_at
		FROM dead_entities d JOIN profiles p ON p.id = d.profile_id
		WHERE `+strings.Join(where, " AND ")+` ORDER BY d.marked_at DESC, d.kind, d.entity_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list dead entities for workspace %q: %w", trimmedWorkspaceID, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close dead entity rows: %w", closeErr))
		}
	}()
	entities = make([]store.DeadEntity, 0)
	for rows.Next() {
		var row sqlcgen.DeadEntity
		var owner store.DeadEntity
		if scanErr := rows.Scan(
			&row.ProfileID, &owner.ProfileName, &owner.ProfileColor, &owner.ProfileIcon,
			&owner.ProfileEmoji, &owner.ProfileArchived, &row.WorkspaceID, &row.Kind,
			&row.EntityID, &row.Reason, &row.MarkedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan dead entity: %w", scanErr)
		}
		entity, decodeErr := deadEntityFromRow(row)
		if decodeErr != nil {
			return nil, decodeErr
		}
		entity.ProfileName = owner.ProfileName
		entity.ProfileColor = owner.ProfileColor
		entity.ProfileIcon = owner.ProfileIcon
		entity.ProfileEmoji = owner.ProfileEmoji
		entity.ProfileArchived = owner.ProfileArchived
		entities = append(entities, entity)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("store: iterate dead entities: %w", rowsErr)
	}
	return entities, nil
}

func deadEntityFromRow(row sqlcgen.DeadEntity) (store.DeadEntity, error) {
	markedAt, err := store.ParseTimestamp(row.MarkedAt)
	if err != nil {
		return store.DeadEntity{}, fmt.Errorf(
			"store: parse dead entity %q/%q/%q marked_at: %w",
			row.WorkspaceID,
			row.Kind,
			row.EntityID,
			err,
		)
	}
	entity := store.DeadEntity{
		DeadEntityKey: store.DeadEntityKey{
			ProfileID:   row.ProfileID,
			WorkspaceID: row.WorkspaceID,
			Kind:        store.DeadEntityKind(row.Kind),
			EntityID:    row.EntityID,
		},
		Reason:   row.Reason,
		MarkedAt: markedAt,
	}.Normalize()
	if err := entity.Validate(); err != nil {
		return store.DeadEntity{}, fmt.Errorf("store: decode dead entity row: %w", err)
	}
	return entity, nil
}
