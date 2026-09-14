package extensionpkg

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/extension/agentplugin"
)

func loadAgentPluginManifest(root, dataDir string) (*Manifest, error) {
	document, err := agentplugin.ReadManifest(root)
	if err != nil {
		return nil, err
	}
	return loadAgentPluginDocument(root, dataDir, document)
}

func loadAgentPluginDocument(root, dataDir string, document *agentplugin.ManifestDocument) (*Manifest, error) {
	status, declared := document.Classify()
	if status == agentplugin.SchemaUnsupportedVersion {
		return nil, &agentplugin.SchemaUnsupportedError{Root: root, Path: document.Path, Declared: declared}
	}
	if document.Layout == agentplugin.LayoutStandard && status == agentplugin.SchemaUnrelated && document.ValidJSON() {
		return nil, &AgentPluginNotManifestError{Root: root, Checked: []string{document.Path}}
	}
	if dataDir == "" {
		name, err := document.Name()
		if err != nil {
			return nil, agentPluginLoadError(document.Path, err)
		}
		dataDir, err = defaultAgentPluginDataDir(root, name)
		if err != nil {
			return nil, err
		}
	}
	pkg, err := document.Load(agentplugin.LoadOptions{DataDir: dataDir})
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
