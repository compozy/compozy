package core

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

// SessionCatalogEventSubscriber exposes catalog wakes across workspaces.
type SessionCatalogEventSubscriber interface {
	SubscribeSessionCatalogEvents(
		ctx context.Context,
		scope session.CatalogScope,
	) (<-chan session.CatalogEvent, func(), error)
}
