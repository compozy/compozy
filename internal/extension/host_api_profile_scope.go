package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// hostAPIProfileID resolves the bridge-owned profile for every profile-scoped
// Host API read and write.
func hostAPIProfileID(ctx context.Context) (string, error) {
	runtime := hostAPIBridgeRuntimeFromContext(ctx)
	if runtime == nil {
		return store.DefaultProfileID, nil
	}
	managed, err := runtime.SingleManagedInstance()
	if err != nil {
		return "", fmt.Errorf("resolve Host API profile from bridge runtime: %w", err)
	}
	profileID := strings.TrimSpace(managed.Instance.ProfileID)
	if profileID == "" {
		return "", errors.New("resolve Host API profile from bridge runtime: profile id is required")
	}
	return profileID, nil
}

func hostAPIProfileReadScope(ctx context.Context) (store.ReadScope, error) {
	profileID, err := hostAPIProfileID(ctx)
	if err != nil {
		return store.ReadScope{}, err
	}
	return store.ReadScope{ProfileID: profileID}, nil
}
