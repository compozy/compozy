package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	presetspkg "github.com/compozy/agh/internal/notifications/presets"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func (n *NotificationRepo) ListPresets(
	ctx context.Context,
	query presetspkg.Query,
) (items []presetspkg.Preset, err error) {
	if err := n.checkReady(ctx, "list notification presets"); err != nil {
		return nil, err
	}
	normalized := query.Normalize()
	args := make([]any, 0, 3)
	clauses := make([]string, 0, 3)
	if normalized.Enabled != nil {
		clauses = append(clauses, "enabled = ?")
		args = append(args, *normalized.Enabled)
	}
	if normalized.BuiltIn != nil {
		clauses = append(clauses, "built_in = ?")
		args = append(args, *normalized.BuiltIn)
	}
	if normalized.Name != "" {
		clauses = append(clauses, "name = ?")
		args = append(args, normalized.Name)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	limit := ""
	if normalized.Limit > 0 {
		limit = " LIMIT ?"
		args = append(args, normalized.Limit)
	}
	// dynamic-sql: optional preset filters and the limit change the query structure.
	rows, err := n.db.QueryContext(
		ctx,
		`SELECT name, events, targets, filter, enabled, built_in, default_version,
		       default_hash, user_modified, default_update_available, created_at, updated_at
		  FROM notification_presets`+where+`
		 ORDER BY built_in DESC, name ASC`+limit,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query notification presets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close notification preset rows: %w", closeErr))
		}
	}()
	items = make([]presetspkg.Preset, 0)
	for rows.Next() {
		preset, scanErr := scanNotificationPreset(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, preset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate notification presets: %w", err)
	}
	return items, nil
}

func (n *NotificationRepo) GetPreset(ctx context.Context, name string) (presetspkg.Preset, error) {
	if err := n.checkReady(ctx, "get notification preset"); err != nil {
		return presetspkg.Preset{}, err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return presetspkg.Preset{}, fmt.Errorf("%w: name is required", presetspkg.ErrInvalidPreset)
	}
	return getNotificationPreset(ctx, n.queries, trimmed)
}

func (n *NotificationRepo) CreatePreset(
	ctx context.Context,
	preset presetspkg.Preset,
) (presetspkg.Preset, error) {
	if err := n.checkReady(ctx, "create notification preset"); err != nil {
		return presetspkg.Preset{}, err
	}
	normalized := preset.Normalize()
	if err := normalized.Validate(); err != nil {
		return presetspkg.Preset{}, err
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = n.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	eventsJSON, targetsJSON, err := encodeNotificationPresetLists(normalized)
	if err != nil {
		return presetspkg.Preset{}, err
	}
	if err := n.queries.CreateNotificationPreset(ctx, sqlcgen.CreateNotificationPresetParams{
		Name:                   normalized.Name,
		Events:                 eventsJSON,
		Targets:                targetsJSON,
		Filter:                 normalized.Filter,
		Enabled:                normalized.Enabled,
		BuiltIn:                normalized.BuiltIn,
		DefaultVersion:         normalized.DefaultVersion,
		DefaultHash:            normalized.DefaultHash,
		UserModified:           normalized.UserModified,
		DefaultUpdateAvailable: normalized.DefaultUpdateAvailable,
		CreatedAt:              store.FormatTimestamp(normalized.CreatedAt),
		UpdatedAt:              store.FormatTimestamp(normalized.UpdatedAt),
	}); err != nil {
		if isNotificationPresetDuplicateNameError(err) {
			return presetspkg.Preset{}, presetspkg.ErrPresetDuplicateName
		}
		return presetspkg.Preset{}, fmt.Errorf("store: insert notification preset %q: %w", normalized.Name, err)
	}
	return normalized, nil
}

func (n *NotificationRepo) UpdatePreset(
	ctx context.Context,
	name string,
	req presetspkg.UpdateRequest,
) (presetspkg.Preset, error) {
	if err := n.checkReady(ctx, "update notification preset"); err != nil {
		return presetspkg.Preset{}, err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return presetspkg.Preset{}, fmt.Errorf("%w: name is required", presetspkg.ErrInvalidPreset)
	}
	if !req.HasMutableField() {
		return presetspkg.Preset{}, fmt.Errorf(
			"%w: update requires at least one mutable field",
			presetspkg.ErrInvalidPreset,
		)
	}
	current, err := getNotificationPreset(ctx, n.queries, trimmed)
	if err != nil {
		return presetspkg.Preset{}, err
	}
	updated := current.Normalize()
	if req.Events != nil {
		updated.Events = append([]string(nil), (*req.Events)...)
	}
	if req.Targets != nil {
		updated.Targets = append([]presetspkg.Target(nil), (*req.Targets)...)
	}
	if req.Filter != nil {
		updated.Filter = strings.TrimSpace(*req.Filter)
	}
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	updated.UpdatedAt = req.Now
	if updated.UpdatedAt.IsZero() {
		updated.UpdatedAt = n.now()
	}
	updated = presetspkg.ApplyDefaultDrift(updated)
	if err := updated.Validate(); err != nil {
		return presetspkg.Preset{}, err
	}
	eventsJSON, targetsJSON, err := encodeNotificationPresetLists(updated)
	if err != nil {
		return presetspkg.Preset{}, err
	}
	affected, err := n.queries.UpdateNotificationPreset(ctx, sqlcgen.UpdateNotificationPresetParams{
		Events:                 eventsJSON,
		Targets:                targetsJSON,
		Filter:                 updated.Filter,
		Enabled:                updated.Enabled,
		UserModified:           updated.UserModified,
		DefaultUpdateAvailable: updated.DefaultUpdateAvailable,
		UpdatedAt:              store.FormatTimestamp(updated.UpdatedAt),
		Name:                   updated.Name,
	})
	if err != nil {
		return presetspkg.Preset{}, fmt.Errorf("store: update notification preset %q: %w", updated.Name, err)
	}
	if affected == 0 {
		return presetspkg.Preset{}, presetspkg.ErrPresetNotFound
	}
	return updated, nil
}

func (n *NotificationRepo) DeletePreset(ctx context.Context, name string) error {
	if err := n.checkReady(ctx, "delete notification preset"); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: name is required", presetspkg.ErrInvalidPreset)
	}
	current, err := getNotificationPreset(ctx, n.queries, trimmed)
	if err != nil {
		return err
	}
	if current.BuiltIn {
		return presetspkg.ErrPresetBuiltIn
	}
	affected, err := n.queries.DeleteNotificationPreset(ctx, trimmed)
	if err != nil {
		return fmt.Errorf("store: delete notification preset %q: %w", trimmed, err)
	}
	if affected == 0 {
		return presetspkg.ErrPresetNotFound
	}
	return nil
}

// EnsureBuiltInPresets inserts disabled built-ins and records default drift without overwriting user edits.
func (n *NotificationRepo) EnsureBuiltInPresets(
	ctx context.Context,
	defaults []presetspkg.Preset,
) error {
	if err := n.checkReady(ctx, "ensure notification preset defaults"); err != nil {
		return err
	}
	err := store.ExecuteWrite(ctx, n.db, func(writeCtx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		for _, defaultPreset := range defaults {
			if seedErr := seedNotificationPresetDefault(
				writeCtx,
				queries,
				defaultPreset.Normalize(),
				n.now(),
			); seedErr != nil {
				return seedErr
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: seed notification preset defaults: %w", err)
	}
	return nil
}

func getNotificationPreset(
	ctx context.Context,
	queries *sqlcgen.Queries,
	name string,
) (presetspkg.Preset, error) {
	row, err := queries.GetNotificationPreset(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return presetspkg.Preset{}, presetspkg.ErrPresetNotFound
		}
		return presetspkg.Preset{}, fmt.Errorf("store: get notification preset %q: %w", name, err)
	}
	return notificationPresetFromGenerated(row)
}

func encodeNotificationPresetLists(preset presetspkg.Preset) (string, string, error) {
	eventsJSON, err := json.Marshal(preset.Events)
	if err != nil {
		return "", "", fmt.Errorf("store: encode notification preset events: %w", err)
	}
	targetsJSON, err := json.Marshal(preset.Targets)
	if err != nil {
		return "", "", fmt.Errorf("store: encode notification preset targets: %w", err)
	}
	return string(eventsJSON), string(targetsJSON), nil
}

func seedNotificationPresetDefault(
	ctx context.Context,
	queries *sqlcgen.Queries,
	defaultPreset presetspkg.Preset,
	now time.Time,
) error {
	normalized := defaultPreset.Normalize()
	if !normalized.BuiltIn || normalized.DefaultHash == "" {
		return fmt.Errorf("%w: built-in preset default is incomplete", presetspkg.ErrInvalidPreset)
	}
	eventsJSON, targetsJSON, err := encodeNotificationPresetLists(normalized)
	if err != nil {
		return err
	}
	timestamp := store.FormatTimestamp(now.UTC())
	if err := queries.SeedNotificationPresetDefault(ctx, sqlcgen.SeedNotificationPresetDefaultParams{
		Name:           normalized.Name,
		Events:         eventsJSON,
		Targets:        targetsJSON,
		Filter:         normalized.Filter,
		Enabled:        normalized.Enabled,
		DefaultVersion: normalized.DefaultVersion,
		DefaultHash:    normalized.DefaultHash,
		CreatedAt:      timestamp,
		UpdatedAt:      timestamp,
	}); err != nil {
		return fmt.Errorf("store: seed notification preset default %q: %w", normalized.Name, err)
	}
	return nil
}

func isNotificationPresetDuplicateNameError(err error) bool {
	return isSQLitePrimaryKeyConstraint(err) || isSQLiteUniqueConstraint(err)
}
