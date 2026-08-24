package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	memorypkg "github.com/compozy/compozy/internal/memory"
	storepkg "github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var errNativeMemoryCallerProfileRequired = errors.New("daemon: memory caller profile is required")

func nativeMemoryToolRequiresCallerProfile(id toolspkg.ToolID) bool {
	switch id {
	case toolspkg.ToolIDMemoryList,
		toolspkg.ToolIDMemoryShow,
		toolspkg.ToolIDMemorySearch,
		toolspkg.ToolIDMemoryPropose,
		toolspkg.ToolIDMemoryNote:
		return true
	default:
		return false
	}
}

func (n *daemonNativeTools) profileMemoryStore(
	ctx context.Context,
	callerScope toolspkg.Scope,
) (*memorypkg.Store, error) {
	if n == nil || n.deps == nil || n.deps.MemoryStore == nil {
		return nil, fmt.Errorf("daemon: memory store is not configured")
	}
	if strings.TrimSpace(callerScope.ProfileID) == "" && strings.TrimSpace(callerScope.SessionID) == "" {
		return nil, errNativeMemoryCallerProfileRequired
	}
	profileID, _, _, err := n.nativeCurrentProfileIdentity(ctx, callerScope)
	if err != nil {
		return nil, err
	}
	if profileID == storepkg.DefaultProfileID {
		return n.deps.MemoryStore, nil
	}
	if n.deps.Profiles == nil {
		return nil, fmt.Errorf("daemon: profile catalog is not configured for memory projection")
	}
	profileName, err := n.deps.Profiles.ProfileName(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve memory caller profile %q: %w", profileID, err)
	}
	profileName = strings.TrimSpace(profileName)
	profilesDir := strings.TrimSpace(n.deps.HomePaths.ProfilesDir)
	if profilesDir == "" {
		return nil, fmt.Errorf("daemon: profiles directory is not configured for memory projection")
	}
	return n.deps.MemoryStore.ForProfile(
		profileID,
		filepath.Join(profilesDir, profileName, compozyconfig.MemoryDirName),
	), nil
}
