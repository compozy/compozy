package globaldb

import (
	"github.com/compozy/compozy/internal/notifications"
	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
)

// NotificationRepo owns notification cursor and preset persistence.
type NotificationRepo struct {
	*repoBase
}

var (
	_ notifications.CursorStore = (*NotificationRepo)(nil)
	_ notifications.CursorStore = (*GlobalDB)(nil)
	_ presetspkg.Store          = (*NotificationRepo)(nil)
	_ presetspkg.Store          = (*GlobalDB)(nil)
)
