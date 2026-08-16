package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/extension/agentplugin"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/frontmatter"
	yaml "gopkg.in/yaml.v3"
)

type installedPackage struct {
	parent      *fileutil.Directory
	name        string
	directory   *fileutil.Directory
	closeParent bool
}

func (p *installedPackage) Close() error {
	if p == nil || p.directory == nil {
		return nil
	}
	if err := p.directory.Close(); err != nil {
		return fmt.Errorf("registry: close installed package directory: %w", err)
	}
	return nil
}

func (p *installedPackage) CloseParent() error {
	if p == nil || !p.closeParent || p.parent == nil {
		return nil
	}
	p.closeParent = false
	if err := p.parent.Close(); err != nil {
		return fmt.Errorf("registry: close installed package parent directory: %w", err)
	}
	return nil
}

func (p *installedPackage) CloseAll() error {
	return errors.Join(p.Close(), p.CloseParent())
}

func loadInstalledPackageMetadata(
	rootParent *fileutil.Directory,
	rootName string,
	root *fileutil.Directory,
) (*installedPackage, installedPackageMetadata, error) {
	_, manifestName, installed, err := locateInstallManifestRoot(rootParent, rootName, root)
	if err != nil {
		return nil, installedPackageMetadata{}, err
	}
	metadata, err := parseInstalledPackageMetadata(installed.directory, manifestName)
	if err != nil {
		closeErr := installed.CloseAll()
		return nil, installedPackageMetadata{}, errors.Join(err, closeErr)
	}
	metadata.manifestPath = manifestName
	if strings.TrimSpace(metadata.name) == "" {
		closeErr := installed.CloseAll()
		return nil, installedPackageMetadata{}, errors.Join(
			fmt.Errorf("registry: manifest %q is missing name", manifestName),
			closeErr,
		)
	}
	return installed, metadata, nil
}

func locateInstallManifestRoot(
	rootParent *fileutil.Directory,
	rootName string,
	root *fileutil.Directory,
) (_ *fileutil.Directory, _ string, packageRoot *installedPackage, err error) {
	if rootParent == nil || root == nil {
		return nil, "", nil, ErrArchiveRootRequired
	}
	currentParent := rootParent
	currentName := rootName
	current := root
	closeCurrentOnError := false
	descended := false
	defer func() {
		if err == nil {
			return
		}
		if closeCurrentOnError {
			if closeErr := current.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("registry: close manifest search directory: %w", closeErr))
			}
		}
		if currentParent != rootParent {
			if closeErr := currentParent.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("registry: close manifest search parent: %w", closeErr))
			}
		}
	}()

	for {
		manifestName, findErr := manifestNameAtRoot(current)
		if findErr != nil {
			return nil, "", nil, findErr
		}
		if manifestName != "" {
			return currentParent, manifestName, &installedPackage{
				parent:      currentParent,
				name:        currentName,
				directory:   current,
				closeParent: currentParent != rootParent,
			}, nil
		}

		if descended {
			return nil, "", nil, fmt.Errorf("%w: %q", errInstallMissingManifest, currentName)
		}
		nextName, descend, descendErr := singleExtractionDirectory(current)
		if descendErr != nil {
			return nil, "", nil, descendErr
		}
		if !descend {
			return nil, "", nil, fmt.Errorf("%w: %q", errInstallMissingManifest, currentName)
		}
		if isClientSpecificInstallDirectory(nextName) {
			return nil, "", nil, fmt.Errorf(
				"%w: archive contains client-specific layout %q; Agent Plugins require plugin.json at the package root",
				errInstallMissingManifest,
				nextName+"/plugin.json",
			)
		}

		next, openErr := current.OpenDirectory(nextName)
		if openErr != nil {
			return nil, "", nil, fmt.Errorf(
				"registry: open extracted directory %q: %w",
				nextName,
				mapExtractionAccessError(openErr),
			)
		}
		if closeCurrentOnError {
			if closeErr := currentParent.Close(); closeErr != nil {
				if nextCloseErr := next.Close(); nextCloseErr != nil {
					return nil, "", nil, errors.Join(
						fmt.Errorf("registry: close superseded extraction parent: %w", closeErr),
						fmt.Errorf("registry: close extracted child after parent failure: %w", nextCloseErr),
					)
				}
				return nil, "", nil, fmt.Errorf("registry: close superseded extraction parent: %w", closeErr)
			}
		}
		currentParent = current
		currentName = nextName
		current = next
		closeCurrentOnError = true
		descended = true
	}
}

func isClientSpecificInstallDirectory(name string) bool {
	switch strings.TrimSpace(name) {
	case ".claude-plugin", ".codex-plugin", ".cursor-plugin":
		return true
	default:
		return false
	}
}

func manifestNameAtRoot(root *fileutil.Directory) (string, error) {
	hasExtensionManifest, err := manifestFileExists(root, installerExtensionManifestName)
	if err != nil {
		return "", fmt.Errorf("registry: inspect manifest %q: %w", installerExtensionManifestName, err)
	}
	hasSkillManifest, err := manifestFileExists(root, installerSkillManifestName)
	if err != nil {
		return "", fmt.Errorf("registry: inspect manifest %q: %w", installerSkillManifestName, err)
	}
	hasAgentPluginManifest, err := manifestFileExists(root, installerAgentPluginManifestName)
	if err != nil {
		return "", fmt.Errorf("registry: inspect manifest %q: %w", installerAgentPluginManifestName, err)
	}
	switch {
	case hasExtensionManifest && hasSkillManifest:
		return "", fmt.Errorf(
			"registry: archive root contains both %s and %s",
			installerExtensionManifestName,
			installerSkillManifestName,
		)
	case hasExtensionManifest:
		return installerExtensionManifestName, nil
	case hasSkillManifest:
		return installerSkillManifestName, nil
	case hasAgentPluginManifest:
		content, readErr := readExtractionFile(root, installerAgentPluginManifestName)
		if readErr != nil {
			return "", fmt.Errorf("registry: read manifest %q: %w", installerAgentPluginManifestName, readErr)
		}
		status, declared := agentplugin.ClassifyManifestContent(content)
		switch status {
		case agentplugin.SchemaSupported:
			return installerAgentPluginManifestName, nil
		case agentplugin.SchemaUnsupportedVersion:
			return "", fmt.Errorf(
				"registry: unsupported Agent Plugins schema %q; supported schema is %q",
				declared,
				agentplugin.PluginSchemaID,
			)
		default:
			return "", fmt.Errorf("%w: unrelated plugin.json", errInstallMissingManifest)
		}
	default:
		return "", nil
	}
}

func singleExtractionDirectory(root *fileutil.Directory) (string, bool, error) {
	names, err := root.ReadDir()
	if err != nil {
		return "", false, fmt.Errorf("registry: read extracted root: %w", err)
	}
	directoryName := ""
	fileCount := 0
	for _, name := range names {
		directory, openErr := root.OpenDirectory(name)
		if openErr == nil {
			if closeErr := directory.Close(); closeErr != nil {
				return "", false, fmt.Errorf("registry: close extracted directory %q: %w", name, closeErr)
			}
			directoryName = name
			continue
		}
		if !errors.Is(openErr, fileutil.ErrNotDirectory) {
			return "", false, fmt.Errorf(
				"registry: inspect extracted entry %q: %w",
				name,
				mapExtractionAccessError(openErr),
			)
		}
		file, fileErr := root.OpenRegularFile(name)
		if fileErr != nil {
			return "", false, fmt.Errorf(
				"registry: inspect extracted file %q: %w",
				name,
				mapExtractionAccessError(fileErr),
			)
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", false, fmt.Errorf("registry: close extracted file %q: %w", name, closeErr)
		}
		fileCount++
	}
	return directoryName, directoryName != "" && fileCount == 0 && len(names) == 1, nil
}

func parseInstalledPackageMetadata(root *fileutil.Directory, manifestName string) (installedPackageMetadata, error) {
	content, err := readExtractionFile(root, manifestName)
	if err != nil {
		return installedPackageMetadata{}, fmt.Errorf("registry: read manifest %q: %w", manifestName, err)
	}
	switch manifestName {
	case installerSkillManifestName:
		var meta skillManifestHeader
		parts, err := frontmatter.Split(content)
		if err != nil {
			return installedPackageMetadata{}, fmt.Errorf("registry: parse skill manifest %q: %w", manifestName, err)
		}
		if err := yaml.Unmarshal(parts.Metadata, &meta); err != nil {
			return installedPackageMetadata{}, fmt.Errorf("registry: decode skill manifest %q: %w", manifestName, err)
		}
		return installedPackageMetadata{
			name: strings.TrimSpace(meta.Name), version: strings.TrimSpace(meta.Version), verifyContent: parts.Body,
		}, nil
	case installerExtensionManifestName:
		var meta extensionManifestHeader
		if _, err := toml.Decode(string(content), &meta); err != nil {
			return installedPackageMetadata{}, fmt.Errorf(
				"registry: decode extension manifest %q: %w",
				manifestName,
				err,
			)
		}
		return installedPackageMetadata{
			name:    firstNonEmpty(meta.Name, meta.Extension.Name),
			version: firstNonEmpty(meta.Version, meta.Extension.Version), verifyContent: string(content),
		}, nil
	case installerAgentPluginManifestName:
		var meta agentPluginManifestHeader
		if err := json.Unmarshal(content, &meta); err != nil {
			return installedPackageMetadata{}, fmt.Errorf(
				"registry: decode Agent Plugins manifest %q: %w",
				manifestName,
				err,
			)
		}
		return installedPackageMetadata{
			name: strings.TrimSpace(meta.Name), version: strings.TrimSpace(meta.Version), verifyContent: string(content),
		}, nil
	default:
		return installedPackageMetadata{}, fmt.Errorf("registry: unsupported manifest %q", manifestName)
	}
}

func readExtractionFile(root *fileutil.Directory, name string) (content []byte, err error) {
	file, err := root.OpenRegularFile(name)
	if err != nil {
		return nil, mapExtractionAccessError(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("registry: close extraction file %q: %w", name, closeErr))
		}
	}()
	content, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("registry: read extraction file %q: %w", name, err)
	}
	return content, nil
}

func verifyInstallerContent(content string) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	messages := make([]string, 0, len(installerVerificationRules))
	seen := make(map[string]struct{}, len(installerVerificationRules))
	for _, rule := range installerVerificationRules {
		if !rule.regex.MatchString(trimmed) {
			continue
		}
		if _, ok := seen[rule.key]; ok {
			continue
		}
		seen[rule.key] = struct{}{}
		messages = append(messages, rule.message)
	}
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", errVerificationBlocked, strings.Join(messages, "; "))
}

func manifestFileExists(root *fileutil.Directory, name string) (bool, error) {
	file, err := root.OpenRegularFile(name)
	switch {
	case err == nil:
		if closeErr := file.Close(); closeErr != nil {
			return false, fmt.Errorf("close manifest %q: %w", name, closeErr)
		}
		return true, nil
	case errors.Is(err, io.EOF):
		return false, nil
	case errors.Is(err, fileutil.ErrNotDirectory):
		return false, fmt.Errorf("manifest %q must be a regular file", name)
	case errors.Is(err, fileutil.ErrDirectory), errors.Is(err, fileutil.ErrNotRegular):
		return false, fmt.Errorf("manifest %q must be a regular file", name)
	case errors.Is(err, fileutil.ErrSymlink):
		return false, mapExtractionAccessError(err)
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, mapExtractionAccessError(err)
	}
}
