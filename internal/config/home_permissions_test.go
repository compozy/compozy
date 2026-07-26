package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureHomeLayoutPermissionsContract(t *testing.T) {
	t.Parallel()

	t.Run("Should create AGH-owned runtime directories with private mode", func(t *testing.T) {
		t.Parallel()

		paths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		if err := EnsureHomeLayout(paths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}

		assertConfigPathMode(t, paths.HomeDir, 0o700)
		assertConfigPathMode(t, paths.AgentsDir, 0o700)
		assertConfigPathMode(t, paths.SkillsDir, 0o700)
		assertConfigPathMode(t, paths.LoopsDir, 0o700)
		assertConfigPathMode(t, paths.MemoryDir, 0o700)
		assertConfigPathMode(t, paths.SessionsDir, 0o700)
		assertConfigPathMode(t, paths.RestartsDir, 0o700)
		assertConfigPathMode(t, paths.LogsDir, 0o700)
	})

	t.Run("Should tighten existing AGH-owned runtime directories to private mode", func(t *testing.T) {
		t.Parallel()

		paths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		for _, dir := range configRuntimeDirectories(paths) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("MkdirAll(%q) error = %v", dir, err)
			}
			if err := os.Chmod(dir, 0o755); err != nil {
				t.Fatalf("Chmod(%q) error = %v", dir, err)
			}
		}

		if err := EnsureHomeLayout(paths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}

		for _, dir := range configRuntimeDirectories(paths) {
			assertConfigPathMode(t, dir, 0o700)
		}
	})
}

func TestEnsureBootstrapAgentPermissionsContract(t *testing.T) {
	t.Parallel()

	t.Run("Should create the managed agent directory with private mode", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		path, created, err := EnsureBootstrapAgent(homePaths)
		if err != nil {
			t.Fatalf("EnsureBootstrapAgent() error = %v", err)
		}
		if !created {
			t.Fatal("EnsureBootstrapAgent() created = false, want true")
		}

		assertConfigPathMode(t, filepath.Dir(path), 0o700)
		assertConfigPathMode(t, path, 0o600)
	})
}

func configRuntimeDirectories(paths HomePaths) []string {
	return []string{
		paths.HomeDir,
		paths.AgentsDir,
		paths.SkillsDir,
		paths.LoopsDir,
		paths.MemoryDir,
		paths.SessionsDir,
		paths.RestartsDir,
		paths.LogsDir,
	}
}

func assertConfigPathMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("Stat(%q).Mode().Perm() = %#o, want %#o", path, got, want)
	}
}
