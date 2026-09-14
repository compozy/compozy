package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/extension/agentplugin"
	"github.com/compozy/compozy/internal/fileutil"
)

var installerClientPluginDirectories = []string{installerClaudePluginDirectory, ".codex-plugin", ".cursor-plugin"}

func clientManifestNameAtRoot(root *fileutil.Directory) (string, error) {
	for _, layout := range installerClientPluginDirectories {
		directory, err := root.OpenDirectory(layout)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", mapExtractionAccessError(err)
		}
		contents, readErr := readExtractionFile(directory, installerAgentPluginManifestName)
		closeErr := directory.Close()
		if errors.Is(readErr, os.ErrNotExist) && closeErr == nil {
			continue
		}
		if err := errors.Join(readErr, closeErr); err != nil {
			return "", err
		}
		name := filepath.Join(layout, installerAgentPluginManifestName)
		if err := validateInstallerClientSchema(contents, name); err != nil {
			return "", err
		}
		return name, nil
	}
	return "", nil
}

func validateInstallerClientSchema(contents []byte, name string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(contents, &fields); err != nil || fields == nil {
		return fmt.Errorf("registry: manifest %q must be a JSON object", name)
	}
	if _, declared := fields["$schema"]; !declared {
		return nil
	}
	status, declared := agentplugin.ClassifyManifestContent(contents)
	switch status {
	case agentplugin.SchemaSupported:
		return nil
	case agentplugin.SchemaUnsupportedVersion:
		return &agentplugin.SchemaUnsupportedError{Path: name, Declared: declared}
	default:
		return fmt.Errorf("%w: unrelated client manifest %q", errInstallMissingManifest, name)
	}
}

func readInstalledManifest(root *fileutil.Directory, name string) (content []byte, err error) {
	parent := filepath.Dir(name)
	if parent == "." {
		return readExtractionFile(root, name)
	}
	if !slices.Contains(installerClientPluginDirectories, parent) {
		return nil, fmt.Errorf("registry: unsupported manifest path %q", name)
	}
	directory, err := root.OpenDirectory(parent)
	if err != nil {
		return nil, mapExtractionAccessError(err)
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	return readExtractionFile(directory, filepath.Base(name))
}
