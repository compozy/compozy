package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
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
	if err := g.queries.UpsertDeadEntity(ctx, sqlcgen.UpsertDeadEntityParams{
		WorkspaceID: normalized.WorkspaceID,
		Kind:        string(normalized.Kind),
		EntityID:    normalized.EntityID,
		Reason:      normalized.Reason,
		MarkedAt:    store.FormatTimestamp(normalized.MarkedAt),
	}); err != nil {
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
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) error {
	if err := g.checkReady(ctx, "clear dead entity"); err != nil {
		return err
	}
	key := store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID}.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	if _, err := g.queries.DeleteDeadEntity(ctx, sqlcgen.DeleteDeadEntityParams{
		WorkspaceID: key.WorkspaceID,
		Kind:        string(key.Kind),
		EntityID:    key.EntityID,
	}); err != nil {
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
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) (store.DeadEntity, bool, error) {
	if err := g.checkReady(ctx, "find dead entity"); err != nil {
		return store.DeadEntity{}, false, err
	}
	key := store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID}.Normalize()
	if err := key.Validate(); err != nil {
		return store.DeadEntity{}, false, err
	}
	row, err := g.queries.GetDeadEntity(ctx, sqlcgen.GetDeadEntityParams{
		WorkspaceID: key.WorkspaceID,
		Kind:        string(key.Kind),
		EntityID:    key.EntityID,
	})
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
	workspaceID string,
) ([]store.DeadEntity, error) {
	if err := g.checkReady(ctx, "list dead entities"); err != nil {
		return nil, err
	}
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if trimmedWorkspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", store.ErrInvalidDeadEntity)
	}
	rows, err := g.queries.ListDeadEntities(ctx, trimmedWorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: list dead entities for workspace %q: %w", trimmedWorkspaceID, err)
	}
	entities := make([]store.DeadEntity, 0, len(rows))
	for _, row := range rows {
		entity, err := deadEntityFromRow(row)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
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
