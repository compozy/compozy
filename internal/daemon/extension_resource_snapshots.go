package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
)

type scopedExtensionResourceSnapshot struct {
	extension *extensionpkg.Extension
	scope     resources.ResourceScope
	profileID string
}

type scopedExtensionRuntime interface {
	GetForInstance(extensionpkg.InstanceKey) (*extensionpkg.Extension, error)
}

type profiledExtensionRuntime interface {
	ProjectForProfile(
		context.Context,
		extensionpkg.InstanceKey,
		extensionpkg.ProfileLens,
	) (*extensionpkg.Extension, bool, error)
}

type extensionProfileCatalog interface {
	List(context.Context) ([]profilepkg.WithCounts, error)
}

func extensionResourceSnapshots(
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
	logger *slog.Logger,
) ([]scopedExtensionResourceSnapshot, error) {
	infos, err := registry.List()
	if err != nil {
		return nil, fmt.Errorf("daemon: list installed extensions for resource sync: %w", err)
	}
	slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
		return strings.Compare(left.Name, right.Name)
	})
	snapshots := make([]scopedExtensionResourceSnapshot, 0, len(infos))
	for _, info := range infos {
		if !info.Enabled {
			continue
		}
		snapshot, loadErr := installedDefaultProfileResourceSnapshot(registry, runtime, logger, info.Name)
		if errors.Is(loadErr, extensionpkg.ErrExtensionNotFound) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		snapshots = append(snapshots, snapshot)
	}

	development, err := workspaceExtensionResourceSnapshots(registry, runtime, logger)
	return append(snapshots, development...), err
}

func installedDefaultProfileResourceSnapshot(
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
	logger *slog.Logger,
	name string,
) (scopedExtensionResourceSnapshot, error) {
	installation, err := registry.ResolveInstallation(context.Background(), name, extensionpkg.InstallationScope{
		ProfileID: store.DefaultProfileID,
	})
	if err != nil {
		return scopedExtensionResourceSnapshot{}, err
	}
	ext, err := loadExtensionSnapshot(registry, runtime, logger, name)
	if err != nil {
		return scopedExtensionResourceSnapshot{}, fmt.Errorf(
			"daemon: load installed extension %q for resource sync: %w",
			name,
			err,
		)
	}
	snapshot := scopedExtensionResourceSnapshot{
		extension: ext,
		scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}
	if installation.Scope.ProfileID != "" {
		snapshot.scope = resources.ResourceScope{
			Kind: resources.ResourceScopeKindProfile,
			ID:   installation.Scope.ProfileID,
		}
		snapshot.profileID = installation.Scope.ProfileID
	}
	return snapshot, nil
}

func workspaceExtensionResourceSnapshots(
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
	logger *slog.Logger,
) ([]scopedExtensionResourceSnapshot, error) {
	snapshots := make([]scopedExtensionResourceSnapshot, 0)

	keys, err := workspaceExtensionKeys(context.Background(), registry)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return snapshots, nil
	}
	if runtime == nil {
		return snapshots, nil
	}
	scopedRuntime, ok := runtime.(scopedExtensionRuntime)
	if !ok {
		logUnsupportedDevelopmentLinks(logger, len(keys))
		return snapshots, nil
	}
	for _, key := range keys {
		ext, loadErr := scopedRuntime.GetForInstance(key)
		if loadErr != nil {
			if errors.Is(loadErr, extensionpkg.ErrExtensionNotFound) {
				if logger != nil {
					logger.Debug(
						"extension.resource_sync.dev_link_inactive",
						"extension", key.Name,
						"workspace_id", key.WorkspaceID,
					)
				}
				continue
			}
			return nil, fmt.Errorf(
				"daemon: load workspace extension %q for workspace %q resource sync: %w",
				key.Name,
				key.WorkspaceID,
				loadErr,
			)
		}
		if ext == nil || strings.TrimSpace(ext.Status.WorkspaceID) != key.WorkspaceID {
			if logger != nil {
				logger.Debug(
					"extension.resource_sync.dev_link_not_activated",
					"extension", key.Name,
					"workspace_id", key.WorkspaceID,
				)
			}
			continue
		}
		snapshots = append(snapshots, scopedExtensionResourceSnapshot{
			extension: ext,
			scope: resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspace,
				ID:   key.WorkspaceID,
			},
		})
	}
	return snapshots, nil
}

func extensionResourceSnapshotsForProfiles(
	ctx context.Context,
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
	profiles extensionProfileCatalog,
) ([]scopedExtensionResourceSnapshot, error) {
	if ctx == nil {
		return nil, errors.New("daemon: extension profile resource context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	infos, err := registry.List()
	if err != nil {
		return nil, fmt.Errorf("daemon: list installed extensions for profile resource sync: %w", err)
	}
	slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
		return strings.Compare(left.Name, right.Name)
	})
	return extensionProfileResourceSnapshots(ctx, registry, runtime, infos, profiles)
}

func extensionProfileResourceSnapshots(
	ctx context.Context,
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
	infos []extensionpkg.ExtensionInfo,
	profiles extensionProfileCatalog,
) ([]scopedExtensionResourceSnapshot, error) {
	if registry == nil {
		return nil, errors.New("daemon: extension registry is required for profile resource projection")
	}
	projector, ok := runtime.(profiledExtensionRuntime)
	if !ok || projector == nil {
		return nil, errors.New("daemon: extension runtime does not support profile resource projection")
	}
	activeProfiles := []extensionpkg.ProfileLens{{
		ID: store.DefaultProfileID, Name: daemonDefaultProfileName,
	}}
	if profiles != nil {
		profileRows, err := profiles.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: list profiles for extension resource sync: %w", err)
		}
		slices.SortFunc(profileRows, func(left, right profilepkg.WithCounts) int {
			return strings.Compare(left.Name, right.Name)
		})
		activeProfiles = activeExtensionProfileLenses(profileRows)
	}

	snapshots, err := installedExtensionProfileSnapshots(ctx, projector, infos, activeProfiles)
	if err != nil {
		return nil, err
	}
	development, err := workspaceExtensionProfileSnapshots(ctx, registry, projector, activeProfiles)
	if err != nil {
		return nil, err
	}
	return append(snapshots, development...), nil
}

func installedExtensionProfileSnapshots(
	ctx context.Context,
	projector profiledExtensionRuntime,
	infos []extensionpkg.ExtensionInfo,
	profiles []extensionpkg.ProfileLens,
) ([]scopedExtensionResourceSnapshot, error) {
	snapshots := make([]scopedExtensionResourceSnapshot, 0, len(infos)*len(profiles))
	for _, info := range infos {
		for _, profile := range profiles {
			extension, enabled, projectErr := projector.ProjectForProfile(
				ctx,
				extensionpkg.GlobalInstanceKey(info.Name),
				profile,
			)
			if errors.Is(projectErr, extensionpkg.ErrExtensionNotFound) {
				continue
			}
			if projectErr != nil {
				return nil, fmt.Errorf(
					"daemon: project installed extension %q for profile %q: %w",
					info.Name,
					profile.Name,
					projectErr,
				)
			}
			if !enabled {
				continue
			}
			snapshots = append(snapshots, scopedExtensionResourceSnapshot{
				extension: extension,
				scope: resources.ResourceScope{
					Kind: resources.ResourceScopeKindProfile,
					ID:   profile.ID,
				},
				profileID: profile.ID,
			})
		}
	}
	return snapshots, nil
}

func workspaceExtensionProfileSnapshots(
	ctx context.Context,
	registry *extensionpkg.Registry,
	projector profiledExtensionRuntime,
	profiles []extensionpkg.ProfileLens,
) ([]scopedExtensionResourceSnapshot, error) {
	keys, err := workspaceExtensionKeys(ctx, registry)
	if err != nil {
		return nil, err
	}
	snapshots := make([]scopedExtensionResourceSnapshot, 0, len(keys)*len(profiles))
	for _, key := range keys {
		for _, profile := range profiles {
			extension, enabled, projectErr := projector.ProjectForProfile(
				ctx,
				key,
				profile,
			)
			if errors.Is(projectErr, extensionpkg.ErrExtensionNotFound) {
				continue
			}
			if projectErr != nil {
				return nil, fmt.Errorf(
					"daemon: project workspace extension %q for workspace %q and profile %q: %w",
					key.Name,
					key.WorkspaceID,
					profile.Name,
					projectErr,
				)
			}
			if !enabled || extension == nil || strings.TrimSpace(extension.Status.WorkspaceID) != key.WorkspaceID {
				continue
			}
			snapshots = append(snapshots, scopedExtensionResourceSnapshot{
				extension: extension,
				scope: resources.ResourceScope{
					Kind: resources.ResourceScopeKindWorkspaceProfile,
					ID:   key.WorkspaceID + "@pf:" + profile.Name,
				},
				profileID: profile.ID,
			})
		}
	}
	return snapshots, nil
}

func activeExtensionProfileLenses(
	profiles []profilepkg.WithCounts,
) []extensionpkg.ProfileLens {
	active := make([]extensionpkg.ProfileLens, 0, len(profiles))
	for _, profile := range profiles {
		if profile.State != profilepkg.StateActive {
			continue
		}
		active = append(active, extensionpkg.ProfileLens{ID: profile.ID, Name: profile.Name})
	}
	return active
}

func logUnsupportedDevelopmentLinks(logger *slog.Logger, count int) {
	if logger != nil {
		logger.Warn("extension.resource_sync.dev_links_unsupported", "dev_link_count", count)
	}
}

// Published attachments and development overlays share one effective runtime per workspace and profile.
func workspaceExtensionKeys(ctx context.Context, registry *extensionpkg.Registry) ([]extensionpkg.InstanceKey, error) {
	infos, err := registry.List()
	if err != nil {
		return nil, err
	}
	seen := make(map[extensionpkg.InstanceKey]bool)
	for _, info := range infos {
		installations, err := registry.Installations(ctx, info.Name)
		if err != nil {
			return nil, err
		}
		for _, installation := range installations {
			if installation.Scope.WorkspaceID != "" {
				seen[(extensionpkg.InstanceKey{Name: info.Name, WorkspaceID: installation.Scope.WorkspaceID}).Normalize()] = true
			}
		}
	}
	links, err := registry.ListDevLinks()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		seen[(extensionpkg.InstanceKey{Name: link.ExtensionName, WorkspaceID: link.WorkspaceID}).Normalize()] = true
	}
	keys := make([]extensionpkg.InstanceKey, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b extensionpkg.InstanceKey) int {
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}
		return strings.Compare(a.WorkspaceID, b.WorkspaceID)
	})
	return keys, nil
}
