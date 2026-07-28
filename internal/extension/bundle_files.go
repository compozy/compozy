package extensionpkg

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func collectBundleFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("extension: stat bundle resource %q: %w", root, err)
	}
	if !info.IsDir() {
		if isBundleFile(root) {
			return []string{root}, nil
		}
		return nil, fmt.Errorf("%w: unsupported bundle resource %q", ErrBundleInvalid, root)
	}

	files := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if isBundleFile(path) {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("extension: collect bundle files from %q: %w", root, err)
	}

	slices.Sort(files)
	return files, nil
}

func isBundleFile(path string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".toml", ".json":
		return true
	default:
		return false
	}
}

func normalizeBundleChannels(values []BundleChannel) []BundleChannel {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]BundleChannel, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, BundleChannel{
			Name:        strings.TrimSpace(value.Name),
			Description: strings.TrimSpace(value.Description),
		})
	}
	return normalized
}

func normalizeBundleBridges(values []BundleBridgePreset) []BundleBridgePreset {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]BundleBridgePreset, 0, len(values))
	for _, value := range values {
		next := value
		next.Name = strings.TrimSpace(next.Name)
		next.ExtensionName = strings.TrimSpace(next.ExtensionName)
		next.Platform = strings.TrimSpace(next.Platform)
		next.DisplayName = strings.TrimSpace(next.DisplayName)
		next.DeliveryDefaults = cloneRawMessage(next.DeliveryDefaults)
		next.SecretSlots = slices.Clone(next.SecretSlots)
		for idx := range next.SecretSlots {
			next.SecretSlots[idx].Name = strings.TrimSpace(next.SecretSlots[idx].Name)
			next.SecretSlots[idx].Kind = strings.TrimSpace(next.SecretSlots[idx].Kind)
			next.SecretSlots[idx].Description = strings.TrimSpace(next.SecretSlots[idx].Description)
		}
		normalized = append(normalized, next)
	}
	return normalized
}
