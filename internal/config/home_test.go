package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHomeDirUsesAGHHomeOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "custom-home")
	t.Setenv("AGH_HOME", override)

	got, err := ResolveHomeDir()
	if err != nil {
		t.Fatalf("ResolveHomeDir() error = %v", err)
	}
	if got != override {
		t.Fatalf("ResolveHomeDir() = %q, want %q", got, override)
	}
}

func TestResolveOperatorHomeDirWithLookupUsesHome(t *testing.T) {
	t.Parallel()

	t.Run("Should use HOME from the injected lookup", func(t *testing.T) {
		t.Parallel()

		operatorHome := filepath.Join(t.TempDir(), "operator-home")

		got, err := ResolveOperatorHomeDirWithLookup(HomePaths{}, func(key string) (string, bool) {
			if key == "HOME" {
				return operatorHome, true
			}
			return "", false
		})
		if err != nil {
			t.Fatalf("ResolveOperatorHomeDirWithLookup() error = %v", err)
		}
		if got != operatorHome {
			t.Fatalf("ResolveOperatorHomeDirWithLookup() = %q, want %q", got, operatorHome)
		}
	})

	t.Run("Should canonicalize an existing HOME alias", func(t *testing.T) {
		t.Parallel()

		operatorHome := t.TempDir()
		linkParent := t.TempDir()
		homeAlias := filepath.Join(linkParent, "operator-home")
		if err := os.Symlink(operatorHome, homeAlias); err != nil {
			t.Fatalf("Symlink(%q, %q) error = %v", operatorHome, homeAlias, err)
		}
		canonicalHome, err := filepath.EvalSymlinks(operatorHome)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q) error = %v", operatorHome, err)
		}

		got, err := ResolveOperatorHomeDirWithLookup(HomePaths{}, func(key string) (string, bool) {
			if key == "HOME" {
				return homeAlias, true
			}
			return "", false
		})
		if err != nil {
			t.Fatalf("ResolveOperatorHomeDirWithLookup() error = %v", err)
		}
		if got != canonicalHome {
			t.Fatalf("ResolveOperatorHomeDirWithLookup() = %q, want %q", got, canonicalHome)
		}
	})
}

func TestResolveOperatorHomeDirWithLookupFallsBackFromAGHHome(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to the parent of AGH home", func(t *testing.T) {
		t.Parallel()

		operatorHome := filepath.Join(t.TempDir(), "operator-home")
		homePaths, err := ResolveHomePathsFrom(filepath.Join(operatorHome, DirName))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		got, err := resolveOperatorHomeDir(homePaths, nil, func() (string, error) {
			return "", os.ErrNotExist
		})
		if err != nil {
			t.Fatalf("resolveOperatorHomeDir() error = %v", err)
		}
		if got != operatorHome {
			t.Fatalf("resolveOperatorHomeDir() = %q, want %q", got, operatorHome)
		}
	})
}

func TestEnsureHomeLayoutCreatesRequiredDirectories(t *testing.T) {
	paths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	if err := EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	for _, dir := range []string{
		paths.HomeDir,
		paths.AgentsDir,
		paths.SkillsDir,
		paths.LoopsDir,
		paths.MemoryDir,
		paths.SessionsDir,
		paths.RestartsDir,
		paths.LogsDir,
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("Stat(%q) IsDir() = false, want true", dir)
		}
	}
}

func TestResolveHomePathsFromExpandsTildePaths(t *testing.T) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	paths, err := ResolveHomePathsFrom("~/agh-test-home")
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if paths.HomeDir != filepath.Join(userHome, "agh-test-home") {
		t.Fatalf(
			"ResolveHomePathsFrom() HomeDir = %q, want %q",
			paths.HomeDir,
			filepath.Join(userHome, "agh-test-home"),
		)
	}
	if got, want := paths.MemoryDir, filepath.Join(userHome, "agh-test-home", MemoryDirName); got != want {
		t.Fatalf("ResolveHomePathsFrom() MemoryDir = %q, want %q", got, want)
	}
	if got, want := paths.SkillsDir, filepath.Join(userHome, "agh-test-home", SkillsDirName); got != want {
		t.Fatalf("ResolveHomePathsFrom() SkillsDir = %q, want %q", got, want)
	}
	if got, want := paths.LoopsDir, filepath.Join(userHome, "agh-test-home", LoopsDirName); got != want {
		t.Fatalf("ResolveHomePathsFrom() LoopsDir = %q, want %q", got, want)
	}
	if got, want := paths.RestartsDir, filepath.Join(userHome, "agh-test-home", RestartsDirName); got != want {
		t.Fatalf("ResolveHomePathsFrom() RestartsDir = %q, want %q", got, want)
	}
	if got, want := paths.NetworkAuditFile, filepath.Join(
		userHome,
		"agh-test-home",
		LogsDirName,
		NetworkAuditFileName,
	); got != want {
		t.Fatalf("ResolveHomePathsFrom() NetworkAuditFile = %q, want %q", got, want)
	}
}

func TestResolvePathVariants(t *testing.T) {
	if got, err := ResolvePath(""); err != nil || got != "" {
		t.Fatalf("ResolvePath(blank) = %q, %v, want empty nil", got, err)
	}

	got, err := ResolvePath("daemon.sock")
	if err != nil {
		t.Fatalf("ResolvePath(relative) error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("ResolvePath(relative) = %q, want absolute path", got)
	}
}

func TestEnsureHomeLayoutRejectsEmptyPaths(t *testing.T) {
	if err := EnsureHomeLayout(HomePaths{}); err == nil {
		t.Fatal("EnsureHomeLayout() error = nil, want non-nil")
	}
}
