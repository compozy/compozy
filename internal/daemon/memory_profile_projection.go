package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	storepkg "github.com/compozy/compozy/internal/store"
)

func (d *Daemon) memoryRecallStoreResolver(state *bootState) memory.RecallStoreResolver {
	return func(ctx context.Context, rawProfileID string) (*memory.Store, error) {
		profileID := strings.TrimSpace(rawProfileID)
		if profileID == "" || profileID == storepkg.DefaultProfileID {
			return state.memoryStore, nil
		}
		if state.profiles == nil {
			return nil, fmt.Errorf("daemon: profile catalog is not configured for memory recall")
		}
		profileName, err := state.profiles.ProfileName(ctx, profileID)
		if err != nil {
			return nil, fmt.Errorf("daemon: resolve profile for memory recall: %w", err)
		}
		return state.memoryStore.ForProfile(
			profileID,
			filepath.Join(d.homePaths.ProfilesDir, profileName, compozyconfig.MemoryDirName),
		), nil
	}
}
