package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/extension/agentplugin"
)

func loadAgentPluginManifest(root, dataDir, manifestPath string) (*Manifest, bool, error) {
	status, declared, err := agentplugin.ClassifyManifest(root)
	if err != nil {
		return nil, false, fmt.Errorf("extension: classify Agent Plugins manifest: %w", err)
	}
	switch status {
	case agentplugin.SchemaSupported:
		manifest, loadErr := loadSupportedAgentPluginManifest(root, dataDir, manifestPath)
		return manifest, true, loadErr
	case agentplugin.SchemaUnsupportedVersion:
		return nil, true, &AgentPluginSchemaUnsupportedError{Root: root, Declared: declared}
	case agentplugin.SchemaUnrelated:
		recognized, loadErr := validateUnrelatedAgentPluginManifest(root, dataDir, manifestPath)
		return nil, recognized, loadErr
	default:
		return nil, false, nil
	}
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

func validateUnrelatedAgentPluginManifest(root, dataDir, manifestPath string) (bool, error) {
	info, err := os.Lstat(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		if layout := detectAgentPluginClientLayout(root); layout != "" {
			return true, &AgentPluginClientLayoutError{Root: root, Layout: layout}
		}
		return false, nil
	}
	if err != nil {
		return true, fmt.Errorf("extension: stat %q: %w", manifestPath, err)
	}
	if !info.Mode().IsRegular() {
		_, loadErr := loadAgentPluginPackage(root, dataDir, manifestPath)
		return true, loadErr
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return true, fmt.Errorf("extension: read Agent Plugins manifest %q: %w", manifestPath, err)
	}
	if !json.Valid(content) {
		_, loadErr := loadAgentPluginPackage(root, dataDir, manifestPath)
		return true, loadErr
	}
	return true, &AgentPluginNotManifestError{Root: root}
}

func detectAgentPluginClientLayout(root string) string {
	for _, layout := range []string{".claude-plugin", ".codex-plugin", ".cursor-plugin"} {
		path := filepath.Join(root, layout, agentPluginManifestFileName)
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
			return layout
		}
	}
	return ""
}
