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

// CreateNetworkChannel inserts durable channel authority without replacing an existing row.
func (g *NetworkRepo) CreateNetworkChannel(ctx context.Context, entry store.NetworkChannelEntry) error {
	entry, err := g.prepareNetworkChannelEntry(ctx, "create network channel", entry)
	if err != nil {
		return err
	}

	err = g.queries.CreateNetworkChannel(ctx, sqlcgen.CreateNetworkChannelParams{
		Channel:           entry.Channel,
		WorkspaceID:       entry.WorkspaceID,
		Purpose:           entry.Purpose,
		FanoutPolicy:      entry.FanoutPolicy,
		CoordinatorPeerID: entry.CoordinatorPeerID,
		CreatedBy:         entry.CreatedBy,
		CreatedAt:         store.FormatTimestamp(entry.CreatedAt),
		UpdatedAt:         store.FormatTimestamp(entry.UpdatedAt),
	})
	if isSQLiteUniqueConstraint(err) || isSQLitePrimaryKeyConstraint(err) {
		return fmt.Errorf("%w: %s", store.ErrNetworkChannelExists, entry.Channel)
	}
	if err != nil {
		return fmt.Errorf("store: create network channel entry: %w", err)
	}
	return nil
}

// WriteNetworkChannel upserts durable network channel metadata.
func (g *NetworkRepo) WriteNetworkChannel(ctx context.Context, entry store.NetworkChannelEntry) error {
	entry, err := g.prepareNetworkChannelEntry(ctx, "write network channel", entry)
	if err != nil {
		return err
	}

	if err := g.queries.UpsertNetworkChannel(ctx, sqlcgen.UpsertNetworkChannelParams{
		Channel:           entry.Channel,
		WorkspaceID:       entry.WorkspaceID,
		Purpose:           entry.Purpose,
		FanoutPolicy:      entry.FanoutPolicy,
		CoordinatorPeerID: entry.CoordinatorPeerID,
		CreatedBy:         entry.CreatedBy,
		CreatedAt:         store.FormatTimestamp(entry.CreatedAt),
		UpdatedAt:         store.FormatTimestamp(entry.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("store: insert network channel entry: %w", err)
	}

	return nil
}

func (g *NetworkRepo) prepareNetworkChannelEntry(
	ctx context.Context,
	action string,
	entry store.NetworkChannelEntry,
) (store.NetworkChannelEntry, error) {
	entry.Channel = strings.TrimSpace(entry.Channel)
	entry.FanoutPolicy = store.NormalizeNetworkFanoutPolicy(entry.FanoutPolicy)
	entry.CoordinatorPeerID = strings.TrimSpace(entry.CoordinatorPeerID)
	entry.CreatedBy = strings.TrimSpace(entry.CreatedBy)
	if err := g.checkReady(ctx, action); err != nil {
		return store.NetworkChannelEntry{}, err
	}
	if err := entry.Validate(); err != nil {
		return store.NetworkChannelEntry{}, fmt.Errorf("store: validate network channel entry: %w", err)
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = g.now()
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = entry.CreatedAt
	}

	return entry, nil
}

// GetNetworkChannel returns one persisted network channel metadata row.
func (g *NetworkRepo) GetNetworkChannel(
	ctx context.Context,
	ref store.NetworkChannelRef,
) (store.NetworkChannelEntry, error) {
	if err := g.checkReady(ctx, "get network channel"); err != nil {
		return store.NetworkChannelEntry{}, err
	}
	normalized := store.NetworkChannelRef{
		WorkspaceID: strings.TrimSpace(ref.WorkspaceID),
		Channel:     strings.TrimSpace(ref.Channel),
	}
	if err := normalized.Validate(); err != nil {
		return store.NetworkChannelEntry{}, err
	}

	return getNetworkChannel(ctx, g.db, normalized)
}

// PatchNetworkChannel applies one partial metadata update without overwriting
// unspecified channel fields.
func (g *NetworkRepo) PatchNetworkChannel(
	ctx context.Context,
	ref store.NetworkChannelRef,
	patch store.NetworkChannelPatch,
) error {
	if err := g.checkReady(ctx, "patch network channel"); err != nil {
		return err
	}
	normalized := store.NetworkChannelRef{
		WorkspaceID: strings.TrimSpace(ref.WorkspaceID),
		Channel:     strings.TrimSpace(ref.Channel),
	}
	if err := normalized.Validate(); err != nil {
		return err
	}
	if !patch.HasChanges() {
		return errors.New("store: network channel patch must include at least one field")
	}
	return g.withNetworkImmediateTransaction(ctx, "patch network channel", func(exec networkSQLExecutor) error {
		current, err := getNetworkChannel(ctx, exec, normalized)
		if err != nil {
			return err
		}
		next := patch.Apply(current)
		if next.UpdatedAt.IsZero() {
			next.UpdatedAt = g.now()
		}
		if err := next.Validate(); err != nil {
			return fmt.Errorf("store: validate network channel patch: %w", err)
		}

		affected, err := sqlcgen.New(exec).PatchNetworkChannel(ctx, sqlcgen.PatchNetworkChannelParams{
			Purpose:           next.Purpose,
			FanoutPolicy:      next.FanoutPolicy,
			CoordinatorPeerID: next.CoordinatorPeerID,
			UpdatedAt:         store.FormatTimestamp(next.UpdatedAt),
			WorkspaceID:       normalized.WorkspaceID,
			Channel:           normalized.Channel,
		})
		if err != nil {
			return fmt.Errorf("store: patch network channel: %w", err)
		}
		if affected == 0 {
			return sql.ErrNoRows
		}
		return nil
	})
}

// ListNetworkChannels returns persisted network channel metadata rows.
func (g *NetworkRepo) ListNetworkChannels(
	ctx context.Context,
	query store.NetworkChannelQuery,
) (entries []store.NetworkChannelEntry, err error) {
	if err := g.checkReady(ctx, "list network channels"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate network channel query: %w", err)
	}

	// dynamic-sql: optional identity filters and the caller-provided limit change the statement shape.
	sqlQuery := `SELECT channel, workspace_id, purpose, fanout_policy, coordinator_peer_id, created_by, created_at, updated_at FROM network_channels`
	where, args := store.BuildClauses(
		store.StringClause("channel", query.Channel),
		store.StringClause("workspace_id", query.WorkspaceID),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY updated_at DESC, channel ASC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network channels: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network channels rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	entries = make([]store.NetworkChannelEntry, 0)
	for rows.Next() {
		entry, scanErr := scanNetworkChannel(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network channels: %w", err)
	}

	return entries, nil
}

// DeleteNetworkChannel removes one persisted channel metadata row.
func (g *NetworkRepo) DeleteNetworkChannel(ctx context.Context, ref store.NetworkChannelRef) error {
	if err := g.checkReady(ctx, "delete network channel"); err != nil {
		return err
	}
	normalized := store.NetworkChannelRef{
		WorkspaceID: strings.TrimSpace(ref.WorkspaceID),
		Channel:     strings.TrimSpace(ref.Channel),
	}
	if err := normalized.Validate(); err != nil {
		return err
	}

	if err := g.queries.DeleteNetworkChannel(ctx, sqlcgen.DeleteNetworkChannelParams{
		WorkspaceID: normalized.WorkspaceID,
		Channel:     normalized.Channel,
	}); err != nil {
		return fmt.Errorf("store: delete network channel: %w", err)
	}
	return nil
}

func getNetworkChannel(
	ctx context.Context,
	exec networkSQLExecutor,
	normalized store.NetworkChannelRef,
) (store.NetworkChannelEntry, error) {
	row, err := sqlcgen.New(exec).GetNetworkChannel(ctx, sqlcgen.GetNetworkChannelParams{
		WorkspaceID: normalized.WorkspaceID,
		Channel:     normalized.Channel,
	})
	if err != nil {
		return store.NetworkChannelEntry{}, err
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return store.NetworkChannelEntry{}, fmt.Errorf("store: parse network channel created_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return store.NetworkChannelEntry{}, fmt.Errorf("store: parse network channel updated_at: %w", err)
	}
	return store.NetworkChannelEntry{
		Channel: row.Channel, WorkspaceID: row.WorkspaceID, Purpose: row.Purpose,
		FanoutPolicy:      store.NormalizeNetworkFanoutPolicy(row.FanoutPolicy),
		CoordinatorPeerID: row.CoordinatorPeerID, CreatedBy: row.CreatedBy,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func scanNetworkChannel(scanner rowScanner) (store.NetworkChannelEntry, error) {
	var (
		entry        store.NetworkChannelEntry
		coordinator  sql.NullString
		createdBy    sql.NullString
		createdAtRaw string
		updatedAtRaw string
	)
	if err := scanner.Scan(
		&entry.Channel,
		&entry.WorkspaceID,
		&entry.Purpose,
		&entry.FanoutPolicy,
		&coordinator,
		&createdBy,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.NetworkChannelEntry{}, err
		}
		return store.NetworkChannelEntry{}, fmt.Errorf("store: scan network channel: %w", err)
	}

	if value := store.NullString(createdBy); value != nil {
		entry.CreatedBy = *value
	}
	if value := store.NullString(coordinator); value != nil {
		entry.CoordinatorPeerID = *value
	}
	entry.FanoutPolicy = store.NormalizeNetworkFanoutPolicy(entry.FanoutPolicy)

	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return store.NetworkChannelEntry{}, fmt.Errorf("store: parse network channel created_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return store.NetworkChannelEntry{}, fmt.Errorf("store: parse network channel updated_at: %w", err)
	}
	entry.CreatedAt = createdAt
	entry.UpdatedAt = updatedAt
	return entry, nil
}
