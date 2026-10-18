package extensionpkg

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

func hostAPIProfileID(ctx context.Context) string {
	if key, ok := hostAPIInstanceKeyFromContext(ctx); ok && key.ProfileID != "" {
		return key.ProfileID
	}
	return store.DefaultProfileID
}
