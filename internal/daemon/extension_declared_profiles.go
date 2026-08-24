package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	profilepkg "github.com/compozy/compozy/internal/profile"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func reconcileDeclaredExtensionProfiles(
	ctx context.Context,
	profiles *profilepkg.Manager,
	registry *extensionpkg.Registry,
) error {
	if profiles == nil || registry == nil {
		return nil
	}
	installed, err := registry.List()
	if err != nil {
		return fmt.Errorf("daemon: list extensions for declared profiles: %w", err)
	}
	for _, info := range installed {
		manifestPath := strings.TrimSpace(info.ManifestPath)
		if manifestPath == "" {
			continue
		}
		manifest, err := extensionpkg.LoadManifest(filepath.Dir(manifestPath))
		if err != nil {
			return fmt.Errorf("daemon: load extension %q declared profiles: %w", info.Name, err)
		}
		if len(manifest.Profiles) == 0 {
			continue
		}
		if _, err := extensionpkg.ApplyDeclaredProfiles(ctx, profiles, manifest); err != nil {
			return fmt.Errorf("daemon: reconcile extension %q declared profiles: %w", info.Name, err)
		}
	}
	return nil
}

func (s *daemonExtensionService) recordDeclaredProfileCreatedEvents(
	ctx context.Context,
	actor taskpkg.ActorContext,
	extensionName string,
	results []extensionpkg.DeclaredProfileApplyResult,
) error {
	events := make([]extensionpkg.LifecycleEvent, 0, len(results))
	for _, result := range results {
		if !result.Created {
			continue
		}
		events = append(events, extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionProfileCreated, ExtensionName: extensionName,
			ProfileID: result.Profile.ID, ProfileName: result.Profile.Name,
		})
	}
	return s.recordCanonicalExtensionLifecycleEvents(ctx, actor, events...)
}
