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
	placements := make([]profilepkg.PlacementRef, 0)
	for _, info := range installed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		manifest, err := loadManifestAtPath(info.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("extension: load placements for %q: %w", info.Name, err)
		}
		for _, placement := range manifest.PlacementMatrix() {
			if placement.Profile != name {
				continue
			}
			placements = append(placements, profilepkg.PlacementRef{
				Extension: info.Name, Resource: placement.Resource, ProfileName: name,
			})
		}
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
