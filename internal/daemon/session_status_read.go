package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

type sessionStatusReader interface {
	Status(context.Context, string) (*session.Info, error)
}

func storedSessionStatus(ctx context.Context, reader sessionStatusReader, id string) (*session.Info, error) {
	if stored, ok := reader.(interface {
		StoredStatus(context.Context, string) (*session.Info, error)
	}); ok {
		return stored.StoredStatus(ctx, id)
	}
	return reader.Status(ctx, id)
}
