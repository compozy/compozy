package desktoprelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

func DesktopArtifactNames(version string) []string {
	return []string{
		"CompozyOS-" + version + "-mac-arm64.dmg",
		"CompozyOS-" + version + "-mac-arm64.zip",
		"CompozyOS-" + version + "-mac-x64.dmg",
		"CompozyOS-" + version + "-mac-x64.zip",
		"CompozyOS-" + version + "-linux-x64.AppImage",
		"CompozyOS-" + version + "-linux-x64.deb",
	}
}

func AssertExactDesktopInventory(ctx context.Context, dir, version string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("desktop release: inventory canceled: %w", err)
	}
	if err := ValidateVersion(version); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("desktop release: read artifact directory: %w", err)
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			return fmt.Errorf("desktop release: inspect artifact %q: %w", entry.Name(), infoErr)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("desktop release: artifact %q is not a non-empty regular file", entry.Name())
		}
		actual = append(actual, entry.Name())
	}
	expected := DesktopArtifactNames(version)
	slices.Sort(actual)
	slices.Sort(expected)
	if !slices.Equal(actual, expected) {
		return fmt.Errorf("desktop release: artifact inventory = %v, want %v", actual, expected)
	}
	return nil
}

func InspectDesktopInventory(ctx context.Context, dir, version string) ([]Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("desktop release: inventory canceled: %w", err)
	}
	if err := ValidateVersion(version); err != nil {
		return nil, err
	}
	names := DesktopArtifactNames(version)
	slices.Sort(names)
	artifacts := make([]Artifact, 0, len(names))
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("desktop release: inventory canceled: %w", err)
		}
		artifact, err := inspectArtifact(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func inspectArtifact(path string) (Artifact, error) {
	file, err := os.Open(path)
	if err != nil {
		return Artifact{}, fmt.Errorf("desktop release: open artifact %q: %w", filepath.Base(path), err)
	}
	digest := sha256.New()
	size, copyErr := io.Copy(digest, file)
	closeErr := file.Close()
	if copyErr != nil {
		return Artifact{}, fmt.Errorf("desktop release: hash artifact %q: %w", filepath.Base(path), copyErr)
	}
	if closeErr != nil {
		return Artifact{}, fmt.Errorf("desktop release: close artifact %q: %w", filepath.Base(path), closeErr)
	}
	return Artifact{Name: filepath.Base(path), SHA256: hex.EncodeToString(digest.Sum(nil)), Size: size}, nil
}
