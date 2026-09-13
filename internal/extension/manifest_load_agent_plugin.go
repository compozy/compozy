package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/extension/agentplugin"
	"github.com/compozy/compozy/internal/fileutil"
)

func loadAgentPluginManifest(root, dataDir string) (*Manifest, error) {
	path, layout, err := agentplugin.LocateManifest(root)
	if err != nil {
		return nil, err
	}
	content, _, err := fileutil.ReadRegularFile(path)
	if err != nil {
		return nil, err
	}
	status, declared := agentplugin.ClassifyManifestContent(content)
	if status == agentplugin.SchemaUnsupportedVersion {
		return nil, &AgentPluginSchemaUnsupportedError{Root: root, Path: path, Declared: declared}
	}
	if layout == agentplugin.LayoutStandard && status == agentplugin.SchemaUnrelated && json.Valid(content) {
		return nil, &AgentPluginNotManifestError{Root: root, Checked: []string{path}}
	}
	manifest, err := loadSupportedAgentPluginManifest(root, dataDir, path)
	return manifest, err
}

func loadSupportedAgentPluginManifest(root, dataDir, manifestPath string) (*Manifest, error) {
	usesDefaultDataDir := dataDir == ""
	if usesDefaultDataDir {
		var err error
		dataDir, err = defaultAgentPluginDataDir(root, filepath.Base(root))
		if err != nil {
			return nil, err
		}
	}
	pkg, err := loadAgentPluginPackage(root, dataDir, manifestPath)
	if err != nil {
		return nil, err
	}
	if usesDefaultDataDir {
		resolvedDataDir, resolveErr := defaultAgentPluginDataDir(root, pkg.Name)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if resolvedDataDir != dataDir {
			pkg, err = loadAgentPluginPackage(root, resolvedDataDir, manifestPath)
			if err != nil {
				return nil, err
			}
		}
	}
	return SynthesizeAgentPluginManifest(pkg, root)
}

func loadAgentPluginPackage(root, dataDir, manifestPath string) (*agentplugin.Package, error) {
	pkg, err := agentplugin.Load(root, agentplugin.LoadOptions{DataDir: dataDir})
	if err == nil {
		return pkg, nil
	}
	if manifestErr, ok := errors.AsType[*agentplugin.ManifestError](err); ok {
		return nil, newAgentPluginManifestValidationError(manifestPath, manifestErr)
	}
	return nil, fmt.Errorf("extension: load Agent Plugins manifest %q: %w", manifestPath, err)
}
