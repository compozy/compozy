package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"os"
	"path/filepath"
	"slices"
	"strings"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"

	looppkg "github.com/compozy/agh/internal/loop"

	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/subprocess"
)

func (m *Manager) resolveBridgeRuntime(
	ctx context.Context,
	ext *managedExtension,
) (*subprocess.InitializeBridgeRuntime, error) {
	if ext == nil || ext.manifest == nil {
		return nil, nil
	}
	if !slices.Contains(ext.manifest.Capabilities.Provides, extensionprotocol.CapabilityProvideBridgeAdapter) {
		return nil, nil
	}
	if m.bridgeRuntimeResolver == nil {
		return nil, fmt.Errorf("%w for %q", ErrBridgeRuntimeResolverRequired, ext.info.Name)
	}

	bridgeRuntime, err := m.bridgeRuntimeResolver.ResolveBridgeRuntime(ctx, ext.info.Name)
	if err != nil {
		return nil, fmt.Errorf("extension: resolve bridge runtime for %q: %w", ext.info.Name, err)
	}
	if bridgeRuntime == nil {
		return nil, fmt.Errorf("extension: bridge runtime is required for %q", ext.info.Name)
	}
	if err := bridgeRuntime.Validate(); err != nil {
		return nil, fmt.Errorf("extension: resolve bridge runtime for %q: %w", ext.info.Name, err)
	}

	resolved := subprocess.CloneInitializeBridgeRuntime(bridgeRuntime)
	if resolved == nil {
		return nil, fmt.Errorf("extension: bridge runtime is required for %q", ext.info.Name)
	}
	if strings.TrimSpace(resolved.Provider) != ext.info.Name {
		return nil, fmt.Errorf(
			"extension: bridge runtime provider %q, want %q",
			resolved.Provider,
			ext.info.Name,
		)
	}
	if len(resolved.ManagedInstances) == 0 {
		return nil, fmt.Errorf("extension: bridge runtime managed instances are required for %q", ext.info.Name)
	}
	for _, managed := range resolved.ManagedInstances {
		if strings.TrimSpace(managed.Instance.ExtensionName) != ext.info.Name {
			return nil, fmt.Errorf(
				"extension: bridge runtime instance %q belongs to extension %q, want %q",
				managed.Instance.ID,
				managed.Instance.ExtensionName,
				ext.info.Name,
			)
		}
	}

	return resolved, nil
}

func daemonRequestMethods() []string {
	return []string{"execute_hook", managerHealthCheckKey, managerShutdownKey}
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
	trimmedRoot := filepath.Clean(strings.TrimSpace(rootDir))
	if trimmedRoot == "" {
		return "", errors.New("extension: root directory is required")
	}

	resolved := strings.TrimSpace(value)
	if resolved == "" {
		return "", nil
	}

	var candidate string
	if filepath.IsAbs(resolved) {
		candidate = filepath.Clean(resolved)
	} else {
		candidate = filepath.Clean(filepath.Join(trimmedRoot, resolved))
	}

	rel, err := filepath.Rel(trimmedRoot, candidate)
	if err != nil {
		return "", fmt.Errorf("extension: resolve path %q: %w", resolved, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf(
			"%w: path %q escapes extension root %q",
			ErrPathEscapesExtensionRoot,
			resolved,
			trimmedRoot,
		)
	}

	return candidate, nil
}

func collectMarkdownFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.EqualFold(filepath.Ext(root), ".md") {
			return []string{root}, nil
		}
		return nil, fmt.Errorf("resource path %q is not a markdown file", root)
	}

	files := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
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
