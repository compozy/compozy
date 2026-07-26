package registry

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"

	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/compozy/agh/internal/frontmatter"
	yaml "gopkg.in/yaml.v3"
)

func loadInstalledPackageMetadata(extractRoot string) (string, installedPackageMetadata, error) {
	packageRoot, manifestPath, err := locateInstallManifestRoot(extractRoot)
	if err != nil {
		return "", installedPackageMetadata{}, err
	}

	metadata, err := parseInstalledPackageMetadata(manifestPath)
	if err != nil {
		return "", installedPackageMetadata{}, err
	}
	metadata.manifestPath = manifestPath

	if strings.TrimSpace(metadata.name) == "" {
		return "", installedPackageMetadata{}, fmt.Errorf("registry: manifest %q is missing name", manifestPath)
	}

	return packageRoot, metadata, nil
}

func locateInstallManifestRoot(extractRoot string) (string, string, error) {
	current := extractRoot
	for {
		manifestPath, err := manifestPathAtRoot(current)
		if err != nil {
			return "", "", err
		}
		if manifestPath != "" {
			return current, manifestPath, nil
		}

		entries, err := os.ReadDir(current)
		if err != nil {
			return "", "", fmt.Errorf("registry: read extracted root %q: %w", current, err)
		}

		dirs := make([]string, 0, len(entries))
		files := 0
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry.Name())
				continue
			}
			files++
		}

		if len(dirs) == 1 && files == 0 {
			current = filepath.Join(current, dirs[0])
			continue
		}

		return "", "", fmt.Errorf("%w: %q", errInstallMissingManifest, extractRoot)
	}
}

func manifestPathAtRoot(root string) (string, error) {
	extensionManifest := filepath.Join(root, installerExtensionManifestName)
	skillManifest := filepath.Join(root, installerSkillManifestName)

	hasExtensionManifest, err := manifestFileExists(extensionManifest)
	if err != nil {
		return "", fmt.Errorf("registry: inspect manifest %q: %w", extensionManifest, err)
	}
	hasSkillManifest, err := manifestFileExists(skillManifest)
	if err != nil {
		return "", fmt.Errorf("registry: inspect manifest %q: %w", skillManifest, err)
	}

	switch {
	case hasExtensionManifest && hasSkillManifest:
		return "", fmt.Errorf(
			"registry: archive root %q contains both %s and %s",
			root,
			installerExtensionManifestName,
			installerSkillManifestName,
		)
	case hasExtensionManifest:
		return extensionManifest, nil
	case hasSkillManifest:
		return skillManifest, nil
	default:
		return "", nil
	}
}

func parseInstalledPackageMetadata(manifestPath string) (installedPackageMetadata, error) {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return installedPackageMetadata{}, fmt.Errorf("registry: read manifest %q: %w", manifestPath, err)
	}

	switch filepath.Base(manifestPath) {
	case installerSkillManifestName:
		var meta skillManifestHeader
		parts, err := frontmatter.Split(content)
		if err != nil {
			return installedPackageMetadata{}, fmt.Errorf("registry: parse skill manifest %q: %w", manifestPath, err)
		}
		if err := yaml.Unmarshal(parts.Metadata, &meta); err != nil {
			return installedPackageMetadata{}, fmt.Errorf("registry: decode skill manifest %q: %w", manifestPath, err)
		}
		return installedPackageMetadata{
			name:          strings.TrimSpace(meta.Name),
			version:       strings.TrimSpace(meta.Version),
			verifyContent: parts.Body,
		}, nil
	case installerExtensionManifestName:
		var meta extensionManifestHeader
		if _, err := toml.Decode(string(content), &meta); err != nil {
			return installedPackageMetadata{}, fmt.Errorf(
				"registry: decode extension manifest %q: %w",
				manifestPath,
				err,
			)
		}
		return installedPackageMetadata{
			name:          firstNonEmpty(meta.Name, meta.Extension.Name),
			version:       firstNonEmpty(meta.Version, meta.Extension.Version),
			verifyContent: string(content),
		}, nil
	default:
		return installedPackageMetadata{}, fmt.Errorf("registry: unsupported manifest %q", manifestPath)
	}
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

	slices.Sort(messages)
	return fmt.Errorf("%w: %s", errVerificationBlocked, strings.Join(messages, "; "))
}

func manifestFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if info.Mode().IsRegular() {
			return true, nil
		}
		return false, fmt.Errorf("manifest %q must be a regular file", path)
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}
