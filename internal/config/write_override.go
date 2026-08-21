package config

import (
	"errors"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

type configLayerPath struct {
	name string
	path string
}

// WriteOverride reports the highest-precedence file that still wins over a saved value.
func WriteOverride(
	homePaths HomePaths,
	workspaceRoot string,
	target WriteTarget,
	path []string,
) (string, bool, error) {
	layers := higherPrecedenceConfigLayers(homePaths, workspaceRoot, target)
	winner := ""
	for _, layer := range layers {
		present, err := configFileHasPath(layer.path, path)
		if err != nil {
			return "", false, err
		}
		if present {
			winner = layer.name
		}
	}
	return winner, winner != "", nil
}

func higherPrecedenceConfigLayers(
	homePaths HomePaths,
	workspaceRoot string,
	target WriteTarget,
) []configLayerPath {
	profileName := strings.TrimSpace(target.profileName)
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	layers := make([]configLayerPath, 0, 3)
	if target.scope == WriteScopeUser && profileName != "" && profileName != bootstrapDefaultKey {
		layers = append(layers, configLayerPath{
			name: RoleFieldSourceProfile,
			path: profileConfigFile(homePaths, profileName),
		})
	}
	if (target.scope == WriteScopeUser || target.scope == WriteScopeProfile) && workspaceRoot != "" {
		layers = append(layers, configLayerPath{
			name: RoleFieldSourceWorkspace,
			path: workspaceConfigFile(workspaceRoot),
		})
	}
	if workspaceRoot != "" && profileName != "" && profileName != bootstrapDefaultKey {
		layers = append(layers, configLayerPath{
			name: RoleFieldSourceWorkspaceProfile,
			path: workspaceProfileConfigFile(workspaceRoot, profileName),
		})
	}
	return layers
}

func configFileHasPath(path string, wanted []string) (bool, error) {
	contents, _, err := fileutil.ReadRegularFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	document, err := parseOverlayDocument(contents)
	if err != nil {
		return false, err
	}
	return document.findKeyValue(wanted) != nil || document.findTable(wanted) != nil, nil
}
