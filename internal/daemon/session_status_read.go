package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

type sessionStatusReader interface {
	Status(context.Context, string) (*session.Info, error)
}

func storedSessionStatus(ctx context.Context, reader sessionStatusReader, id string) (*session.Info, error) {
	// A wrapper can override Status to route across managers and still promote StoredStatus.
	if manager, ok := reader.(*session.Manager); ok {
		return manager.StoredStatus(ctx, id)
	}
	return reader.Status(ctx, id)
}
