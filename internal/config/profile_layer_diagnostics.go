package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ProfileLayerOrphanedCode = "config_profile_layer_orphaned"

// ProfileLayerDiagnostic reports a config or MCP sidecar whose directory does not bind to a known profile.
type ProfileLayerDiagnostic struct {
	Code    string
	Layer   string
	Profile string
	Path    string
	Message string
}

// InspectProfileLayerFiles finds dormant profile-layer files without reading or applying their contents.
func InspectProfileLayerFiles(
	homePaths HomePaths,
	workspaceRoot string,
	knownProfiles []string,
) ([]ProfileLayerDiagnostic, error) {
	known := make(map[string]struct{}, len(knownProfiles)+1)
	known["default"] = struct{}{}
	for _, name := range knownProfiles {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			known[trimmed] = struct{}{}
		}
	}
	roots := []struct {
		layer string
		path  string
	}{{layer: RoleFieldSourceProfile, path: homePaths.ProfilesDir}}
	if root := strings.TrimSpace(workspaceRoot); root != "" {
		roots = append(roots, struct {
			layer string
			path  string
		}{layer: RoleFieldSourceWorkspaceProfile, path: filepath.Join(root, DirName, ProfilesDirName)})
	}

	diagnostics := make([]ProfileLayerDiagnostic, 0)
	for _, root := range roots {
		items, err := inspectProfileLayerRoot(root.path, root.layer, known)
		if err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, items...)
	}
	sort.Slice(diagnostics, func(i, j int) bool { return diagnostics[i].Path < diagnostics[j].Path })
	return diagnostics, nil
}

func inspectProfileLayerRoot(
	root string,
	layer string,
	known map[string]struct{},
) ([]ProfileLayerDiagnostic, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect %s profile layers %q: %w", layer, root, err)
	}
	diagnostics := make([]ProfileLayerDiagnostic, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		profileName := entry.Name()
		if _, exists := known[profileName]; exists {
			continue
		}
		for _, fileName := range []string{ConfigName, MCPJSONName} {
			path := filepath.Join(root, profileName, fileName)
			info, statErr := os.Lstat(path)
			if statErr != nil {
				if errors.Is(statErr, os.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("inspect %s profile layer file %q: %w", layer, path, statErr)
			}
			if !info.Mode().IsRegular() {
				continue
			}
			diagnostics = append(diagnostics, ProfileLayerDiagnostic{
				Code:    ProfileLayerOrphanedCode,
				Layer:   layer,
				Profile: profileName,
				Path:    path,
				Message: fmt.Sprintf("%s is ignored because profile %q does not exist", fileName, profileName),
			})
		}
	}
	return diagnostics, nil
}
