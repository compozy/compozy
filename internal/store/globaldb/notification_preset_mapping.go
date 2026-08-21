package globaldb

import (
	"encoding/json"
	"fmt"

	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type notificationPresetScanner interface {
	Scan(dest ...any) error
}

func scanNotificationPresetWithEnablement(scanner notificationPresetScanner) (presetspkg.Preset, error) {
	var row sqlcgen.NotificationPreset
	var enabled bool
	if err := scanNotificationPresetRow(scanner, &row, &enabled); err != nil {
		return presetspkg.Preset{}, fmt.Errorf("store: scan notification preset enablement: %w", err)
	}
	preset, err := notificationPresetFromGenerated(row)
	if err != nil {
		return presetspkg.Preset{}, err
	}
	preset.Enabled = enabled
	return preset, nil
}

func scanNotificationPresetRow(
	scanner notificationPresetScanner,
	row *sqlcgen.NotificationPreset,
	extraDestinations ...any,
) error {
	destinations := []any{
		&row.Name,
		&row.Events,
		&row.Targets,
		&row.Filter,
		&row.BuiltIn,
		&row.DefaultVersion,
		&row.DefaultHash,
		&row.UserModified,
		&row.DefaultUpdateAvailable,
		&row.CreatedAt,
		&row.UpdatedAt,
	}
	destinations = append(destinations, extraDestinations...)
	return scanner.Scan(destinations...)
}

func notificationPresetFromGenerated(row sqlcgen.NotificationPreset) (presetspkg.Preset, error) {
	preset := presetspkg.Preset{
		Name:                   row.Name,
		Filter:                 row.Filter,
		Enabled:                true,
		BuiltIn:                row.BuiltIn,
		DefaultVersion:         row.DefaultVersion,
		DefaultHash:            row.DefaultHash,
		UserModified:           row.UserModified,
		DefaultUpdateAvailable: row.DefaultUpdateAvailable,
	}
	if err := json.Unmarshal([]byte(row.Events), &preset.Events); err != nil {
		return presetspkg.Preset{}, fmt.Errorf("store: decode notification preset events: %w", err)
	}
	if err := json.Unmarshal([]byte(row.Targets), &preset.Targets); err != nil {
		return presetspkg.Preset{}, fmt.Errorf("store: decode notification preset targets: %w", err)
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return presetspkg.Preset{}, fmt.Errorf("store: parse notification preset creation time: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return presetspkg.Preset{}, fmt.Errorf("store: parse notification preset update time: %w", err)
	}
	preset.CreatedAt = createdAt
	preset.UpdatedAt = updatedAt
	preset = presetspkg.ApplyDefaultDrift(preset)
	if err := preset.Validate(); err != nil {
		return presetspkg.Preset{}, err
	}
	return preset, nil
}
