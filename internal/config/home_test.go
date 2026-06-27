package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHomePathsFrom(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "home")
	paths, err := ResolveHomePathsFrom(root)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	if got, want := paths.HomeDir, root; got != want {
		t.Fatalf("HomeDir = %q, want %q", got, want)
	}
	if got, want := paths.DaemonDir, filepath.Join(root, "daemon"); got != want {
		t.Fatalf("DaemonDir = %q, want %q", got, want)
	}
	if got, want := paths.WorktreesDir, filepath.Join(root, "state", "worktrees"); got != want {
		t.Fatalf("WorktreesDir = %q, want %q", got, want)
	}
	if got, want := paths.SocketPath, filepath.Join(root, "daemon", "daemon.sock"); got != want {
		t.Fatalf("SocketPath = %q, want %q", got, want)
	}
	if got, want := paths.DBDir, filepath.Join(root, "db"); got != want {
		t.Fatalf("DBDir = %q, want %q", got, want)
	}
	if got, want := paths.GlobalDBPath, filepath.Join(root, "db", "global.db"); got != want {
		t.Fatalf("GlobalDBPath = %q, want %q", got, want)
	}
	if got, want := paths.RunsDir, filepath.Join(root, "runs"); got != want {
		t.Fatalf("RunsDir = %q, want %q", got, want)
	}
	if got, want := paths.LogsDir, filepath.Join(root, "logs"); got != want {
		t.Fatalf("LogsDir = %q, want %q", got, want)
	}
	if got, want := paths.CacheDir, filepath.Join(root, "cache"); got != want {
		t.Fatalf("CacheDir = %q, want %q", got, want)
	}
}

func TestResolveHomePathsFromExpandsTilde(t *testing.T) {
	homeDir := t.TempDir()
	stubConfigUserHomeDir(t, func() (string, error) {
		return homeDir, nil
	})

	paths, err := ResolveHomePathsFrom("~/daemon-home")
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	if got, want := paths.HomeDir, filepath.Join(homeDir, "daemon-home"); got != want {
		t.Fatalf("HomeDir = %q, want %q", got, want)
	}
}

func TestResolveHomePathsUsesUserHome(t *testing.T) {
	t.Setenv(HomeEnvVar, "")
	homeDir := t.TempDir()
	stubConfigUserHomeDir(t, func() (string, error) {
		return homeDir, nil
	})

	paths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}

	if got, want := paths.HomeDir, filepath.Join(homeDir, ".compozy"); got != want {
		t.Fatalf("HomeDir = %q, want %q", got, want)
	}
}

func TestResolveHomePathsUsesHomeIndependentlyOfWorkingDirectory(t *testing.T) {
	t.Setenv(HomeEnvVar, "")
	homeDir := t.TempDir()
	stubConfigUserHomeDir(t, func() (string, error) {
		return homeDir, nil
	})

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	nestedDir := filepath.Join(workspaceRoot, "pkg", "feature")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(nestedDir); err != nil {
		t.Fatalf("Chdir(%s) error = %v", nestedDir, err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	paths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}

	if got, want := paths.HomeDir, filepath.Join(homeDir, ".compozy"); got != want {
		t.Fatalf("HomeDir = %q, want %q", got, want)
	}
	if got := paths.HomeDir; got == filepath.Join(workspaceRoot, ".compozy") {
		t.Fatalf("HomeDir should not be workspace-scoped: %q", got)
	}
}

func TestEnsureHomeLayoutCreatesDirectories(t *testing.T) {
	t.Parallel()

	paths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	if err := EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	dirs := []string{
		paths.HomeDir,
		paths.AgentsDir,
		paths.ExtensionsDir,
		paths.StateDir,
		paths.WorktreesDir,
		paths.DaemonDir,
		paths.DBDir,
		paths.RunsDir,
		paths.LogsDir,
		paths.CacheDir,
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}

	daemonInfo, err := os.Stat(paths.DaemonDir)
	if err != nil {
		t.Fatalf("stat daemon dir: %v", err)
	}
	if daemonInfo.Mode().Perm() != 0o700 {
		t.Fatalf("daemon dir mode = %o, want 700", daemonInfo.Mode().Perm())
	}
}

func TestEnsureHomeLayoutRejectsEmptyPaths(t *testing.T) {
	t.Parallel()

	if err := EnsureHomeLayout(HomePaths{}); err == nil {
		t.Fatal("EnsureHomeLayout() error = nil, want non-nil")
	}
}

func TestResolveHomeDirReturnsUserHomeErrors(t *testing.T) {
	t.Setenv(HomeEnvVar, "")
	homeErr := errors.New("home unavailable")
	stubConfigUserHomeDir(t, func() (string, error) {
		return "", homeErr
	})

	_, err := ResolveHomeDir()
	if !errors.Is(err, homeErr) {
		t.Fatalf("ResolveHomeDir() error = %v, want %v", err, homeErr)
	}
}

func TestResolveHomePathsFromRejectsEmptyBaseDir(t *testing.T) {
	t.Parallel()

	if _, err := ResolveHomePathsFrom(" "); err == nil {
		t.Fatal("ResolveHomePathsFrom() error = nil, want non-nil")
	}
}

func TestResolvePathHandlesEmptyAndRelativePaths(t *testing.T) {
	t.Parallel()

	if got, err := ResolvePath(" "); err != nil || got != "" {
		t.Fatalf("ResolvePath(empty) = %q, %v; want empty string, nil", got, err)
	}

	got, err := ResolvePath("daemon.sock")
	if err != nil {
		t.Fatalf("ResolvePath(relative) error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("ResolvePath(relative) = %q, want absolute path", got)
	}
}

func TestResolvePathPropagatesUserHomeErrors(t *testing.T) {
	homeErr := errors.New("home unavailable")
	stubConfigUserHomeDir(t, func() (string, error) {
		return "", homeErr
	})

	if _, err := ResolvePath("~"); !errors.Is(err, homeErr) {
		t.Fatalf("ResolvePath(\"~\") error = %v, want %v", err, homeErr)
	}
}

func TestEnsureHomeLayoutRejectsFileTarget(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "home")
	paths, err := ResolveHomePathsFrom(base)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.CacheDir), 0o755); err != nil {
		t.Fatalf("mkdir cache parent: %v", err)
	}
	if err := os.WriteFile(paths.CacheDir, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	if err := EnsureHomeLayout(paths); err == nil {
		t.Fatal("EnsureHomeLayout() error = nil, want non-nil")
	}
}

func stubConfigUserHomeDir(t *testing.T, fn func() (string, error)) {
	t.Helper()

	original := osUserHomeDir
	osUserHomeDir = fn
	t.Cleanup(func() {
		osUserHomeDir = original
	})
}

func stubConfigLookupEnv(t *testing.T, fn func(string) (string, bool)) {
	t.Helper()

	original := osLookupEnv
	osLookupEnv = fn
	t.Cleanup(func() {
		osLookupEnv = original
	})
}

func TestResolveHomeDirAndPathsRespectCompozyHome(t *testing.T) {
	tests := []struct {
		name            string
		envTemplate     string
		stubHome        bool
		stubLookupUnset bool
		usePaths        bool
		wantSubdir      string
	}{
		{
			name:        "Should use COMPOZY_HOME env var when set",
			envTemplate: "$TMP/custom-compozy",
			wantSubdir:  "custom-compozy",
		},
		{
			name:        "Should trim whitespace from COMPOZY_HOME",
			envTemplate: "  $TMP/custom-compozy \t",
			wantSubdir:  "custom-compozy",
		},
		{
			name:        "Should fall back to user home when COMPOZY_HOME is empty",
			envTemplate: "",
			stubHome:    true,
			wantSubdir:  DirName,
		},
		{
			name:        "Should fall back to user home when COMPOZY_HOME is whitespace only",
			envTemplate: "   \t  ",
			stubHome:    true,
			wantSubdir:  DirName,
		},
		{
			name:        "Should expand tilde in COMPOZY_HOME",
			envTemplate: "~/alt-compozy",
			stubHome:    true,
			wantSubdir:  "alt-compozy",
		},
		{
			name:        "Should derive full home paths from COMPOZY_HOME",
			envTemplate: "$TMP/custom-compozy",
			usePaths:    true,
			wantSubdir:  "custom-compozy",
		},
		{
			name:            "Should fall back to user home when COMPOZY_HOME is unset",
			stubHome:        true,
			stubLookupUnset: true,
			wantSubdir:      DirName,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			if tt.stubHome {
				stubConfigUserHomeDir(t, func() (string, error) {
					return tempDir, nil
				})
			}
			if tt.stubLookupUnset {
				stubConfigLookupEnv(t, func(string) (string, bool) {
					return "", false
				})
			} else {
				t.Setenv(HomeEnvVar, strings.ReplaceAll(tt.envTemplate, "$TMP", tempDir))
			}

			wantHomeDir := filepath.Join(tempDir, tt.wantSubdir)

			if tt.usePaths {
				paths, err := ResolveHomePaths()
				if err != nil {
					t.Fatalf("ResolveHomePaths() error = %v", err)
				}
				if got := paths.HomeDir; got != wantHomeDir {
					t.Fatalf("paths.HomeDir = %q, want %q", got, wantHomeDir)
				}
				if got, want := paths.DaemonDir, filepath.Join(wantHomeDir, "daemon"); got != want {
					t.Fatalf("paths.DaemonDir = %q, want %q", got, want)
				}
				return
			}

			homeDir, err := ResolveHomeDir()
			if err != nil {
				t.Fatalf("ResolveHomeDir() error = %v", err)
			}
			if got := homeDir; got != wantHomeDir {
				t.Fatalf("ResolveHomeDir() = %q, want %q", got, wantHomeDir)
			}
		})
	}
}
