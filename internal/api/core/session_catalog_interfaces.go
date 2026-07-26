package core

import (
	"context"

	"github.com/compozy/agh/internal/session"
)

// SessionCatalogEventSubscriber exposes catalog wakes across workspaces.
type SessionCatalogEventSubscriber interface {
	SubscribeSessionCatalogEvents(
		ctx context.Context,
	) (<-chan session.CatalogEvent, func(), error)
}
