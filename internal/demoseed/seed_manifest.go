package demoseed

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	seedMarkerRelative = ".compozy/demo-seed.json"
	seedScenarioID     = "northstar-pay-v1"
)

type seedMarker struct {
	Scenario     string   `json:"scenario"`
	WorkspaceKey string   `json:"workspace_key"`
	Manifest     []string `json:"manifest"`
}

func readSeedMarker(root string) (seedMarker, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(seedMarkerRelative)))
	if err != nil {
		return seedMarker{}, err
	}
	var marker seedMarker
	if err := json.Unmarshal(raw, &marker); err != nil {
		return seedMarker{}, fmt.Errorf("decode marker: %w", err)
	}
	if marker.Scenario != seedScenarioID || strings.TrimSpace(marker.WorkspaceKey) == "" {
		return seedMarker{}, errors.New("marker does not identify the Northstar Pay seed")
	}
	return marker, nil
}

func writeSeedMarker(root string, workspaceKey string) error {
	manifest, err := seedManifestPaths(root)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(seedMarker{
		Scenario: seedScenarioID, WorkspaceKey: workspaceKey, Manifest: manifest,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("demo seed: encode workspace marker: %w", err)
	}
	raw = append(raw, '\n')
	path := filepath.Join(root, filepath.FromSlash(seedMarkerRelative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("demo seed: create marker directory: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("demo seed: write workspace marker: %w", err)
	}
	return nil
}

func seedManifestPaths(root string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		if entry.IsDir() || relative == "." || relative == seedMarkerRelative ||
			relative == ".compozy/workspace.toml" {
			return nil
		}
		paths = append(paths, relative)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("demo seed: inventory workspace %q: %w", root, err)
	}
	slices.Sort(paths)
	return paths, nil
}

func cleanSeedManifest(root string, marker seedMarker) error {
	for _, relative := range marker.Manifest {
		clean := filepath.Clean(filepath.FromSlash(relative))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("demo seed: marker contains unsafe path %q", relative)
		}
		path := filepath.Join(root, clean)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("demo seed: inspect prior fixture %q: %w", path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("demo seed: fixture manifest path %q unexpectedly became a directory", path)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("demo seed: remove prior fixture %q: %w", path, err)
		}
	}
	return nil
}
