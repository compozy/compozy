package core

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type MigrationNoticeStore interface {
	ListMigrationNotices(context.Context) ([]store.MigrationNotice, error)
	DismissMigrationNotice(context.Context, string, time.Time) error
}
