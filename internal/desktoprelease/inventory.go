package desktoprelease

import (
	"context"
	"fmt"
	"os"
	"slices"
)

func DesktopArtifactNames(version string) []string {
	return []string{
		"CompozyOS_" + version + "_universal.app.tar.gz",
		"CompozyOS_" + version + "_universal.app.tar.gz.sig",
		"CompozyOS_" + version + "_universal.dmg",
		"CompozyOS_" + version + "_amd64.AppImage",
		"CompozyOS_" + version + "_amd64.AppImage.sig",
		"CompozyOS_" + version + "_amd64.deb",
		"CompozyOS_" + version + "_x64-setup.exe",
		"CompozyOS_" + version + "_x64-setup.exe.sig",
	}
}

func RuntimeArtifactNames() map[string]string {
	return map[string]string{
		platformDarwinAArch64: "compozy_darwin_arm64.tar.gz",
		platformDarwinX8664:   "compozy_darwin_x86_64.tar.gz",
		platformLinuxX8664:    "compozy_linux_x86_64.tar.gz",
		platformWindowsX8664:  "compozy_windows_x86_64.zip",
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
		if entry.IsDir() {
			return fmt.Errorf("desktop release: unexpected artifact directory %q", entry.Name())
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return fmt.Errorf("desktop release: inspect artifact %q: %w", entry.Name(), infoErr)
		}
		if info.Size() == 0 {
			return fmt.Errorf("desktop release: artifact %q is empty", entry.Name())
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

func AssertRuntimeInventory(ctx context.Context, dir string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("desktop release: runtime inventory canceled: %w", err)
	}
	artifacts := RuntimeArtifactNames()
	expected := make([]string, 0, len(artifacts))
	for _, name := range artifacts {
		expected = append(expected, name)
	}
	slices.Sort(expected)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("desktop release: read runtime artifact directory: %w", err)
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("desktop release: inspect runtime artifact %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("desktop release: runtime artifact %q is not a non-empty file", entry.Name())
		}
		actual = append(actual, entry.Name())
	}
	slices.Sort(actual)
	if !slices.Equal(actual, expected) {
		return fmt.Errorf("desktop release: runtime artifact inventory = %v, want %v", actual, expected)
	}
	return nil
}
