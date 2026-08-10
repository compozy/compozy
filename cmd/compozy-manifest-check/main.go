package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
)

type rootPackageManifest struct {
	Version string `json:"version"`
}

func main() {
	version := flag.String("version", "", "stamped CompozyOS version")
	flag.Parse()
	if err := run(strings.TrimSpace(*version)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(version string) error {
	if version == "" {
		resolved, err := readWorkspaceVersion("package.json")
		if err != nil {
			return err
		}
		version = resolved
	}
	paths, err := findInRepositoryManifests([]string{"extensions", filepath.Join("sdk", "examples")})
	if err != nil {
		return err
	}
	for _, path := range paths {
		manifest, err := extensionpkg.LoadManifest(filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("manifest-version gate: load %q: %w", path, err)
		}
		if err := extensionpkg.ValidateManifestForCompozyVersion(manifest, version); err != nil {
			return fmt.Errorf("manifest-version gate: %q is incompatible with CompozyOS %s: %w", path, version, err)
		}
	}
	fmt.Printf("manifest-version gate: %d manifests support CompozyOS %s\n", len(paths), version)
	return nil
}

func readWorkspaceVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("manifest-version gate: read %q: %w", path, err)
	}
	var manifest rootPackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("manifest-version gate: decode %q: %w", path, err)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return "", fmt.Errorf("manifest-version gate: %q version is required", path)
	}
	return strings.TrimSpace(manifest.Version), nil
}

func findInRepositoryManifests(roots []string) ([]string, error) {
	paths := make([]string, 0)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Name() == "extension.toml" || entry.Name() == "extension.json" {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("manifest-version gate: scan %q: %w", root, err)
		}
	}
	slices.Sort(paths)
	return paths, nil
}
