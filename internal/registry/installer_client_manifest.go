package registry

import (
	"context"
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

const claudeCodePluginSchemaID = "https://json.schemastore.org/claude-code-plugin-manifest.json"

// ValidateInstallPackageRoot applies the archive installer's checks to an extracted catalog root.
func ValidateInstallPackageRoot(ctx context.Context, path string) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := fileutil.OpenDirectory(path)
	if err != nil {
		return mapExtractionAccessError(err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	name, err := manifestNameAtRoot(root)
	if err != nil {
		return err
	}
	if name == "" {
		if present, err := hasManifestlessPluginComponents(ctx, root); present || err != nil {
			return err
		}
		return errInstallMissingManifest
	}
	metadata, err := readInstalledPackageMetadata(ctx, root, name)
	if err != nil {
		return err
	}
	return verifyInstallerContent(metadata.verifyContent)
}

func hasManifestlessPluginComponents(ctx context.Context, root *fileutil.Directory) (bool, error) {
	for _, name := range []string{"mcp.json", ".mcp.json"} {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if present, err := manifestFileExists(root, name); present || err != nil {
			return present, err
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	skills, err := root.OpenDirectory("skills")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, mapExtractionAccessError(err)
	}
	return true, skills.Close()
}

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
		manifestErr := invalidAgentPluginManifest(name, "$", "must be a JSON object")
		manifestErr.Cause = err
		return manifestErr
	}
	if _, declared := fields["$schema"]; !declared {
		return nil
	}
	status, declared := agentplugin.ClassifyManifestContent(contents)
	switch {
	case status == agentplugin.SchemaSupported,
		filepath.Dir(name) == installerClaudePluginDirectory && declared == claudeCodePluginSchemaID:
		return nil
	case status == agentplugin.SchemaUnsupportedVersion:
		return &agentplugin.SchemaUnsupportedError{Path: name, Declared: declared}
	default:
		err := invalidAgentPluginManifest(name, "$schema", "unrelated client manifest")
		if name == installerAgentPluginManifestName {
			return fmt.Errorf("%w: %w", errInstallMissingManifest, err)
		}
		return err
	}
}

func invalidAgentPluginManifest(name, field, reason string) *agentplugin.ManifestError {
	return &agentplugin.ManifestError{
		ManifestPath: filepath.ToSlash(name),
		Issues:       []agentplugin.Issue{{Path: field, Message: reason}},
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
