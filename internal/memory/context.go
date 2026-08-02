package memory

import (
	"context"
	"fmt"
)

func requireMemoryContext(ctx context.Context, action string) error {
	if ctx == nil {
		return fmt.Errorf("memory: %s context is required", action)
	}
	return ctx.Err()
}
