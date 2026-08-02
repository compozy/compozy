package deadentity

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func validateRequest(ctx context.Context, key store.DeadEntityKey) (store.DeadEntityKey, error) {
	if ctx == nil {
		return store.DeadEntityKey{}, fmt.Errorf("deadentity: context is required")
	}
	if err := contextError(ctx); err != nil {
		return store.DeadEntityKey{}, err
	}
	normalized := key.Normalize()
	if err := normalized.Validate(); err != nil {
		return store.DeadEntityKey{}, err
	}
	return normalized, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func markOperationName(refreshing bool) string {
	if refreshing {
		return "refresh"
	}
	return "mark"
}
