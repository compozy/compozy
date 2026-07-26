package globaldb

import (
	"encoding/json"
	"fmt"

	presetspkg "github.com/compozy/agh/internal/notifications/presets"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type notificationPresetScanner interface {
	Scan(dest ...any) error
}

func scanNotificationPreset(scanner notificationPresetScanner) (presetspkg.Preset, error) {
	var row sqlcgen.NotificationPreset
	if err := scanner.Scan(
		&row.Name,
		&row.Events,
		&row.Targets,
		&row.Filter,
		&row.Enabled,
		&row.BuiltIn,
		&row.DefaultVersion,
		&row.DefaultHash,
		&row.UserModified,
		&row.DefaultUpdateAvailable,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return presetspkg.Preset{}, fmt.Errorf("store: scan notification preset: %w", err)
	}
	return notificationPresetFromGenerated(row)
}

func notificationPresetFromGenerated(row sqlcgen.NotificationPreset) (presetspkg.Preset, error) {
	preset := presetspkg.Preset{
		Name:                   row.Name,
		Filter:                 row.Filter,
		Enabled:                row.Enabled,
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
