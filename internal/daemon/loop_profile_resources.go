package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
)

func resolveLoopResourceLens(
	ctx context.Context,
	profiles loopProfileNameResolver,
	workspaceID string,
	profileID string,
) (looppkg.ResourceLens, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return looppkg.ResourceLens{}, fmt.Errorf("%w: profile id is required", looppkg.ErrValidation)
	}
	profileName := ""
	if profiles == nil {
		if profileID != store.DefaultProfileID {
			return looppkg.ResourceLens{}, errors.New("daemon: profile catalog is required for loop resources")
		}
		profileName = daemonDefaultProfileName
	} else {
		resolved, err := profiles.ProfileName(ctx, profileID)
		if err != nil {
			return looppkg.ResourceLens{}, fmt.Errorf("daemon: resolve loop resource profile: %w", err)
		}
		profileName = strings.TrimSpace(resolved)
	}
	if profileName == "" {
		return looppkg.ResourceLens{}, errors.New("daemon: resolved loop resource profile name is empty")
	}
	return looppkg.ResourceLens{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ProfileID:   profileID,
		ProfileName: profileName,
	}, nil
}
