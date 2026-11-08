package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/extension/agentplugin"
)

func loadAgentPluginManifest(root, dataDir string, pluginName ...string) (*Manifest, error) {
	document, err := agentplugin.ReadManifest(root)
	if missing, ok := errors.AsType[*agentplugin.NotManifestError](err); ok {
		for _, component := range []string{"skills", "mcp.json", ".mcp.json"} {
			if _, statErr := os.Lstat(filepath.Join(root, component)); statErr == nil {
				name := filepath.Base(root)
				if len(pluginName) > 0 {
					name = pluginName[0]
				}
				return loadManifestlessPlugin(root, dataDir, name)
			}
		}
		return nil, missing
	}
	if err != nil {
		return nil, err
	}
	return loadAgentPluginDocument(root, dataDir, document, pluginName...)
}

func loadAgentPluginDocument(
	root, dataDir string,
	document *agentplugin.ManifestDocument,
	pluginName ...string,
) (*Manifest, error) {
	status, declared := document.Classify()
	if status == agentplugin.SchemaUnsupportedVersion {
		return nil, &agentplugin.SchemaUnsupportedError{Root: root, Path: document.Path, Declared: declared}
	}
	if document.Layout == agentplugin.LayoutStandard && status == agentplugin.SchemaUnrelated && document.ValidJSON() &&
		len(pluginName) == 0 {
		if declared != "" {
			return nil, &AgentPluginNotManifestError{Root: root, Checked: []string{document.Path}}
		}
		pluginName = []string{filepath.Base(root)}
	}
	nameOverride := ""
	if (document.Layout != agentplugin.LayoutStandard || status == agentplugin.SchemaUnrelated) && len(pluginName) > 0 {
		nameOverride = pluginName[0]
	}
	if dataDir == "" {
		name, err := document.Name()
		if err != nil && nameOverride != "" {
			name, err = nameOverride, agentplugin.ValidateName(nameOverride)
		}
		if err != nil {
			return nil, agentPluginLoadError(document.Path, err)
		}
		dataDir, err = defaultAgentPluginDataDir(root, name)
		if err != nil {
			return nil, err
		}
	}
	pkg, err := document.Load(agentplugin.LoadOptions{DataDir: dataDir, Name: nameOverride})
	if err != nil {
		return nil, agentPluginLoadError(document.Path, err)
	}
	return SynthesizeAgentPluginManifest(pkg, root)
}

func agentPluginLoadError(manifestPath string, err error) error {
	if manifestErr, ok := errors.AsType[*agentplugin.ManifestError](err); ok {
		return newAgentPluginManifestValidationError(manifestPath, manifestErr)
	}
	return fmt.Errorf("extension: load Agent Plugins manifest %q: %w", manifestPath, err)
}

func loadManifestlessPlugin(root, dataDir, name string) (*Manifest, error) {
	if dataDir == "" {
		var err error
		dataDir, err = defaultAgentPluginDataDir(root, name)
		if err != nil {
			return nil, err
		}
	}
	pkg, err := agentplugin.Load(root, agentplugin.LoadOptions{DataDir: dataDir, Name: name})
	if err != nil {
		return nil, err
	}
	return SynthesizeAgentPluginManifest(pkg, root)
}
