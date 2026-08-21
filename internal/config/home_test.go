package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHomeDirUsesCompozyHomeOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "custom-home")
	t.Setenv("COMPOZY_HOME", override)

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

func TestResolveOperatorHomeDirWithLookupFallsBackFromCompozyHome(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to the parent of Compozy home", func(t *testing.T) {
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

// Invariant: every extension instance maps to one contained segment under a
// root disjoint from managed installs, including the legal package name "data".
// Owner: Compozy home layout.
// Canonical suite: home path tests.
func TestHomePathsExtensionDataPathEncodesInstanceScope(t *testing.T) {
	t.Parallel()
	paths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	tests := []struct {
		name        string
		extension   string
		workspaceID string
		profileID   string
		wantSegment string
	}{
		{name: "Should encode a global dotted name", extension: "acme.tools", wantSegment: "acme.tools"},
		{
			name:        "Should encode a workspace instance",
			extension:   "acme.tools",
			workspaceID: "abc-123",
			wantSegment: "acme.tools@ws-abc-123",
		},
		{
			name:        "Should encode a rename-stable default profile instance",
			extension:   "acme.tools",
			workspaceID: "abc-123",
			profileID:   "00000000000000000000000000",
			wantSegment: "acme.tools@ws-abc-123@pf-00000000000000000000000000",
		},
		{name: "Should keep the data package under the dedicated root", extension: "data", wantSegment: "data"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, pathErr := paths.ExtensionDataPath(test.extension, test.workspaceID, test.profileID)
			if pathErr != nil {
				t.Fatalf("ExtensionDataPath() error = %v", pathErr)
			}
			if want := filepath.Join(paths.ExtensionDataRoot, test.wantSegment); got != want {
				t.Fatalf("ExtensionDataPath() = %q, want %q", got, want)
			}
			if filepath.Dir(got) != paths.ExtensionDataRoot {
				t.Fatalf("ExtensionDataPath() parent = %q, want dedicated root", filepath.Dir(got))
			}
		})
	}

	dataPath, err := paths.ExtensionDataPath("data", "", "")
	if err != nil {
		t.Fatalf("ExtensionDataPath(data) error = %v", err)
	}
	managedDataPath := filepath.Join(paths.HomeDir, "extensions", "data")
	if dataPath == managedDataPath || filepath.Dir(dataPath) == managedDataPath {
		t.Fatalf("data path %q overlaps managed install tree %q", dataPath, managedDataPath)
	}

	for _, invalid := range []string{"../escape", "name@workspace", "a/b"} {
		if _, pathErr := paths.ExtensionDataPath(invalid, "", ""); pathErr == nil {
			t.Fatalf("ExtensionDataPath(%q) error = nil, want containment rejection", invalid)
		}
	}
	if _, pathErr := paths.ExtensionDataPath("acme.tools", "", "marketing"); pathErr == nil {
		t.Fatal("ExtensionDataPath(profile name) error = nil, want stable profile id rejection")
	} else if !strings.Contains(pathErr.Error(), "profile id") {
		t.Fatalf("ExtensionDataPath(profile name) error = %v, want profile id validation", pathErr)
	}
}

func TestEnsureHomeLayoutCreatesRequiredDirectories(t *testing.T) {
	if got, want := DirName, ".compozy"; got != want {
		t.Fatalf("DirName = %q, want %q", got, want)
	}
	if got, want := DatabaseName, "compozy.db"; got != want {
		t.Fatalf("DatabaseName = %q, want %q", got, want)
	}
	if got, want := LogFileName, "compozy.log"; got != want {
		t.Fatalf("LogFileName = %q, want %q", got, want)
	}

	paths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	if err := EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if got, want := paths.DatabaseFile, filepath.Join(paths.HomeDir, "compozy.db"); got != want {
		t.Fatalf("ResolveHomePathsFrom() DatabaseFile = %q, want %q", got, want)
	}
	if got, want := paths.LogFile, filepath.Join(paths.LogsDir, "compozy.log"); got != want {
		t.Fatalf("ResolveHomePathsFrom() LogFile = %q, want %q", got, want)
	}
	pathContracts := map[string]string{
		"BinDir":                filepath.Join(paths.HomeDir, BinDirName),
		"AppStateFile":          filepath.Join(paths.HomeDir, AppStateFileName),
		"UpdateOperationFile":   filepath.Join(paths.HomeDir, UpdateOperationFileName),
		"UpdateOperationLock":   filepath.Join(paths.HomeDir, UpdateOperationLockName),
		"UpdateHistoryFile":     filepath.Join(paths.LogsDir, UpdateHistoryFileName),
		"DesktopProvenanceFile": filepath.Join(paths.HomeDir, BinDirName, DesktopProvenanceFileName),
		"ProfilesDir":           filepath.Join(paths.HomeDir, ProfilesDirName),
		"DefaultProfileDir":     filepath.Join(paths.HomeDir, ProfilesDirName, DefaultProfileDirName),
	}
	for label, want := range pathContracts {
		var got string
		switch label {
		case "BinDir":
			got = paths.BinDir
		case "AppStateFile":
			got = paths.AppStateFile
		case "UpdateOperationFile":
			got = paths.UpdateOperationFile
		case "UpdateOperationLock":
			got = paths.UpdateOperationLock
		case "UpdateHistoryFile":
			got = paths.UpdateHistoryFile
		case "DesktopProvenanceFile":
			got = paths.DesktopProvenanceFile
		case "ProfilesDir":
			got = paths.ProfilesDir
		case "DefaultProfileDir":
			got = paths.DefaultProfileDir
		}
		if got != want {
			t.Fatalf("ResolveHomePathsFrom() %s = %q, want %q", label, got, want)
		}
	}

	for _, dir := range []string{
		paths.HomeDir,
		paths.AgentsDir,
		paths.SkillsDir,
		paths.LoopsDir,
		paths.ProfilesDir,
		paths.DefaultProfileDir,
		paths.MemoryDir,
		paths.SessionsDir,
		paths.ToolArtifactsDir,
		paths.SessionAttachmentsDir,
		paths.RestartsDir,
		paths.LogsDir,
		paths.ExtensionDataRoot,
		paths.GatewayDir,
		paths.GatewayCredentialsDir,
		paths.BinDir,
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

	paths, err := ResolveHomePathsFrom("~/compozy-test-home")
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if paths.HomeDir != filepath.Join(userHome, "compozy-test-home") {
		t.Fatalf(
			"ResolveHomePathsFrom() HomeDir = %q, want %q",
			paths.HomeDir,
			filepath.Join(userHome, "compozy-test-home"),
		)
	}
	if got, want := paths.MemoryDir, filepath.Join(
		userHome,
		"compozy-test-home",
		ProfilesDirName,
		DefaultProfileDirName,
		MemoryDirName,
	); got != want {
		t.Fatalf("ResolveHomePathsFrom() MemoryDir = %q, want %q", got, want)
	}
	if got, want := paths.SkillsDir, filepath.Join(userHome, "compozy-test-home", SkillsDirName); got != want {
		t.Fatalf("ResolveHomePathsFrom() SkillsDir = %q, want %q", got, want)
	}
	if got, want := paths.LoopsDir, filepath.Join(userHome, "compozy-test-home", LoopsDirName); got != want {
		t.Fatalf("ResolveHomePathsFrom() LoopsDir = %q, want %q", got, want)
	}
	if got, want := paths.RestartsDir, filepath.Join(userHome, "compozy-test-home", RestartsDirName); got != want {
		t.Fatalf("ResolveHomePathsFrom() RestartsDir = %q, want %q", got, want)
	}
	if got, want := paths.NetworkAuditFile, filepath.Join(
		userHome,
		"compozy-test-home",
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
