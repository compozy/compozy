package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	memorypkg "github.com/compozy/compozy/internal/memory"
	storepkg "github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) profileMemoryStore(
	ctx context.Context,
	callerScope toolspkg.Scope,
) (*memorypkg.Store, error) {
	if n == nil || n.deps == nil || n.deps.MemoryStore == nil {
		return nil, fmt.Errorf("daemon: memory store is not configured")
	}
	profileID := strings.TrimSpace(callerScope.ProfileID)
	if profileID == "" || profileID == storepkg.DefaultProfileID {
		return n.deps.MemoryStore, nil
	}
	if n.deps.Profiles == nil {
		return nil, fmt.Errorf("daemon: profile catalog is not configured for memory projection")
	}
	profiles, err := n.deps.Profiles.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list profiles for memory projection: %w", err)
	}
	profileName := ""
	for _, profile := range profiles {
		if strings.TrimSpace(profile.ID) == profileID {
			profileName = strings.TrimSpace(profile.Name)
			break
		}
	}
	if profileName == "" {
		return nil, fmt.Errorf("daemon: memory caller profile %q was not found", profileID)
	}
	profilesDir := strings.TrimSpace(n.deps.HomePaths.ProfilesDir)
	if profilesDir == "" {
		return nil, fmt.Errorf("daemon: profiles directory is not configured for memory projection")
	}
	return n.deps.MemoryStore.ForProfile(
		profileID,
		filepath.Join(profilesDir, profileName, compozyconfig.MemoryDirName),
	), nil
}
