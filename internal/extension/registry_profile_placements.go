package extensionpkg

import (
	"context"
	"fmt"
	"sort"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
)

// PlacementsForProfile returns installed manifest resources bound to one profile name.
func (r *Registry) PlacementsForProfile(
	ctx context.Context,
	profileName string,
) ([]profilepkg.PlacementRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(profileName)
	installed, err := r.List()
	if err != nil {
		return nil, err
	}
	devLinks, err := r.ListDevLinks()
	if err != nil {
		return nil, err
	}
	placements := make([]profilepkg.PlacementRef, 0)
	seen := make(map[string]struct{})
	appendManifest := func(owner string, manifest *Manifest) {
		if manifest == nil {
			return
		}
		for _, placement := range manifest.PlacementMatrix() {
			if placement.Profile != name {
				continue
			}
			ref := profilepkg.PlacementRef{
				Extension: owner, Resource: placement.Resource, ProfileName: name,
			}
			key := ref.Extension + "\x00" + ref.Resource + "\x00" + ref.ProfileName
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			placements = append(placements, ref)
		}
	}
	for _, info := range installed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		manifest, err := loadManifestAtPath(info.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("extension: load placements for %q: %w", info.Name, err)
		}
		appendManifest(info.Name, manifest)
	}
	for _, link := range devLinks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		manifest, err := LoadManifest(link.OriginPath)
		if err != nil {
			return nil, fmt.Errorf(
				"extension: load development placements for %q in workspace %q: %w",
				link.ExtensionName,
				link.WorkspaceID,
				err,
			)
		}
		appendManifest(link.ExtensionName, manifest)
	}
	sort.Slice(placements, func(i, j int) bool {
		left, right := placements[i], placements[j]
		if left.Extension != right.Extension {
			return left.Extension < right.Extension
		}
		return left.Resource < right.Resource
	})
	return placements, nil
}

var _ profilepkg.PlacementCatalog = (*Registry)(nil)
