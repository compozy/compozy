package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
)

func clientManifestNameAtRoot(root *fileutil.Directory) (string, error) {
	for _, layout := range []string{installerClaudePluginDirectory, ".codex-plugin", ".cursor-plugin"} {
		directory, err := root.OpenDirectory(layout)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", mapExtractionAccessError(err)
		}
		exists, statErr := manifestFileExists(directory, installerAgentPluginManifestName)
		closeErr := directory.Close()
		if err := errors.Join(statErr, closeErr); err != nil {
			return "", err
		}
		if exists {
			return filepath.Join(layout, installerAgentPluginManifestName), nil
		}
	}
	return "", nil
}

func readInstalledManifest(root *fileutil.Directory, name string) (content []byte, err error) {
	parent := filepath.Dir(name)
	if parent == "." {
		return readExtractionFile(root, name)
	}
	switch parent {
	case installerClaudePluginDirectory, ".codex-plugin", ".cursor-plugin":
	default:
		return nil, fmt.Errorf("registry: unsupported manifest path %q", name)
	}
	directory, err := root.OpenDirectory(parent)
	if err != nil {
		return nil, mapExtractionAccessError(err)
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	return readExtractionFile(directory, filepath.Base(name))
}
