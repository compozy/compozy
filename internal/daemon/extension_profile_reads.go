package daemon

import (
	"context"
	"errors"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) extensionReadProfile(
	ctx context.Context,
	actor taskpkg.ActorContext,
) (extensionpkg.ProfileLens, error) {
	profileID := strings.TrimSpace(actor.ReadScope.ProfileID)
	if profileID == "" || actor.ReadScope.AllProfiles {
		return extensionpkg.ProfileLens{}, errors.New("daemon: one extension read profile is required")
	}
	if profileID == store.DefaultProfileID && s.profiles == nil {
		return extensionpkg.ProfileLens{ID: profileID, Name: "default"}, nil
	}
	if s.profiles == nil {
		return extensionpkg.ProfileLens{}, errors.New("daemon: profile manager is required for extension reads")
	}
	profileName, err := s.profiles.ProfileName(ctx, profileID)
	if err != nil {
		return extensionpkg.ProfileLens{}, err
	}
	return extensionpkg.ProfileLens{ID: profileID, Name: profileName}, nil
}

func (s *daemonExtensionService) projectExtensionReadProfile(
	ctx context.Context,
	runtime extensionDevRuntime,
	key extensionpkg.InstanceKey,
	profile extensionpkg.ProfileLens,
) (*extensionpkg.Extension, error) {
	if projector, ok := runtime.(profiledExtensionRuntime); ok {
		ext, _, err := projector.ProjectForProfile(ctx, key, profile)
		return ext, err
	}
	ext, err := runtime.GetForInstance(key)
	if err != nil {
		return nil, err
	}
	enabled, err := s.registry.IsEnabledForProfile(key.Name, profile.ID)
	if err != nil {
		return nil, err
	}
	ext.Info.Enabled = enabled
	ext.Status.Enabled = enabled
	return ext, nil
}
