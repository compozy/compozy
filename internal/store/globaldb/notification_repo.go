package globaldb

import (
	"github.com/compozy/compozy/internal/notifications"
)

// NotificationRepo owns notification cursor persistence.
type NotificationRepo struct {
	*repoBase
}

var (
	_ notifications.CursorStore = (*NotificationRepo)(nil)
	_ notifications.CursorStore = (*GlobalDB)(nil)
)
