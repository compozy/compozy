package daemon

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
)

type scopedExtensionResourceSnapshot struct {
	extension *extensionpkg.Extension
	scope     resources.ResourceScope
}

type scopedExtensionRuntime interface {
	GetForInstance(extensionpkg.InstanceKey) (*extensionpkg.Extension, error)
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
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	for _, info := range infos {
		if !info.Enabled {
			continue
		}
		ext, loadErr := loadExtensionSnapshot(registry, runtime, logger, info.Name)
		if loadErr != nil {
			return nil, fmt.Errorf("daemon: load installed extension %q for resource sync: %w", info.Name, loadErr)
		}
		snapshots = append(snapshots, scopedExtensionResourceSnapshot{extension: ext, scope: globalScope})
	}

	links, err := registry.ListDevLinks()
	if err != nil {
		return nil, fmt.Errorf("daemon: list development extensions for resource sync: %w", err)
	}
	if len(links) == 0 {
		return snapshots, nil
	}
	if runtime == nil {
		return snapshots, nil
	}
	scopedRuntime, ok := runtime.(scopedExtensionRuntime)
	if !ok {
		logUnsupportedDevelopmentLinks(logger, len(links))
		return snapshots, nil
	}
	for _, link := range links {
		key := (extensionpkg.InstanceKey{Name: link.ExtensionName, WorkspaceID: link.WorkspaceID}).Normalize()
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
				"daemon: load development extension %q for workspace %q resource sync: %w",
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

func logUnsupportedDevelopmentLinks(logger *slog.Logger, count int) {
	if logger != nil {
		logger.Warn("extension.resource_sync.dev_links_unsupported", "dev_link_count", count)
	}
}
