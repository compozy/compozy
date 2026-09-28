package extensionpkg

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"slices"
	"strings"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/fileutil"

	looppkg "github.com/compozy/compozy/internal/loop"

	skillspkg "github.com/compozy/compozy/internal/skills"
)

func daemonRequestMethods() []string {
	return []string{managerHealthCheckKey, managerShutdownKey}
}

func capabilityMethods(provides []string) []string {
	return extensionprotocol.CapabilityServiceMethods(provides)
}

func hostAPIMethodsFromStrings(values []string) []extensionprotocol.HostAPIMethod {
	normalized := normalizeUniqueStrings(values)
	methods := make([]extensionprotocol.HostAPIMethod, 0, len(normalized))
	for _, value := range normalized {
		methods = append(methods, extensionprotocol.HostAPIMethod(value))
	}
	return methods
}

func skillSourceForExtension(source ExtensionSource) skillspkg.SkillSource {
	switch source {
	case SourceBundled:
		return skillspkg.SourceBundled
	case SourceWorkspace:
		return skillspkg.SourceWorkspace
	case SourceMarketplace:
		return skillspkg.SourceMarketplace
	default:
		return skillspkg.SourceUser
	}
}

func loopSourceForExtension(source ExtensionSource) looppkg.Source {
	switch source {
	case SourceBundled, SourceMarketplace:
		return looppkg.SourceMarketplace
	case SourceWorkspace:
		return looppkg.SourceWorkspace
	default:
		return looppkg.SourceUser
	}
}

func resolveResourcePath(rootDir string, value string) (string, error) {
	return resolvePathWithinRoot(rootDir, value)
}

func resolvePathWithinRoot(rootDir string, value string) (string, error) {
	trimmedRoot := strings.TrimSpace(rootDir)
	if trimmedRoot == "" {
		return "", errors.New("extension: root directory is required")
	}
	resolvedRoot, err := fileutil.CanonicalPathWithExistingPrefix(trimmedRoot)
	if err != nil {
		return "", fmt.Errorf("extension: resolve root directory %q: %w", trimmedRoot, err)
	}

	resolved := strings.TrimSpace(value)
	if resolved == "" {
		return "", nil
	}

	var candidate string
	if filepath.IsAbs(resolved) {
		candidate = resolved
	} else {
		candidate = filepath.Join(resolvedRoot, resolved)
	}
	resolvedCandidate, err := fileutil.CanonicalPathWithExistingPrefix(candidate)
	if err != nil {
		return "", fmt.Errorf("extension: resolve path %q: %w", resolved, err)
	}

	contained, err := fileutil.PathWithinRoot(resolvedRoot, resolvedCandidate)
	if err != nil {
		return "", fmt.Errorf("extension: compare path %q to root: %w", resolved, err)
	}
	if !contained {
		return "", fmt.Errorf(
			"%w: path %q escapes extension root %q",
			ErrPathEscapesExtensionRoot,
			resolved,
			resolvedRoot,
		)
	}

	return resolvedCandidate, nil
}

func collectSkillDefinitionFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if filepath.Base(root) == "SKILL.md" {
			return []string{root}, nil
		}
		return nil, fmt.Errorf("resource path %q is not a SKILL.md file", root)
	}

	files := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "SKILL.md" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}

func collectLoopDefinitionFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if looppkg.IsDefinitionFileName(filepath.Base(root)) {
			return []string{root}, nil
		}
		return nil, fmt.Errorf("resource path %q is not a loop YAML file", root)
	}

	files := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if looppkg.IsDefinitionFileName(entry.Name()) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}
