package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	profilepkg "github.com/compozy/compozy/internal/profile"
)

var _ managedInstallRegistry = (*recordingManagedInstallRegistry)(nil)

// Invariant: install planning names only declarations that will be created,
// carries their credential asks, and apply binds pre-existing profiles without
// seeding them or changing activation.
// Owner: extension managed-install declared-profile pipeline.
// Canonical suite: managed install tests.
func TestDeclaredProfileInstallPlanAndApply(t *testing.T) {
	t.Parallel()

	manager := &declaredProfileManagerStub{
		profiles: map[string]profilepkg.Profile{
			"existing": {ID: "pf-existing", Name: "existing", Color: "#8e8eb5", Icon: "circle", State: profilepkg.StateActive},
		},
		markers: make(map[string]bool),
	}
	manifest := &Manifest{Name: "growth-kit", Profiles: []ManifestProfile{
		{
			Name: "growth", Color: "#5fbf85", Icon: "chart-line",
			Defaults:    ManifestProfileDefaults{Agent: "growth-analyst"},
			Credentials: []ManifestProfileCredential{{Provider: "openai", Slot: "api_key"}},
		},
		{Name: "existing", Color: "#e0635a", Icon: "flame"},
	}}
	plan, err := BuildDeclaredProfilePlan(t.Context(), manager, manifest)
	if err != nil {
		t.Fatalf("BuildDeclaredProfilePlan() error = %v", err)
	}
	if len(plan.Profiles) != 2 || !plan.Profiles[0].Create || len(plan.Profiles[0].NeedsSetup) != 1 || plan.Profiles[1].Create {
		t.Fatalf("BuildDeclaredProfilePlan() = %#v, want create growth and bind existing", plan)
	}
	results, err := ApplyDeclaredProfiles(t.Context(), manager, manifest)
	if err != nil {
		t.Fatalf("ApplyDeclaredProfiles() error = %v", err)
	}
	if len(results) != 2 || !results[0].Created || results[0].Bound || results[1].Created || !results[1].Bound {
		t.Fatalf("ApplyDeclaredProfiles() = %#v, want created then bound", results)
	}
	if manager.profiles["existing"].Color != "#8e8eb5" || manager.profiles["existing"].Icon != "circle" {
		t.Fatalf("existing profile mutated to %#v", manager.profiles["existing"])
	}
	if manager.lastSeed.Defaults.Agent != "growth-analyst" || len(manager.lastSeed.CredentialAsks) != 1 {
		t.Fatalf("created seed = %#v, want defaults and credential ask", manager.lastSeed)
	}
}

type declaredProfileManagerStub struct {
	profiles map[string]profilepkg.Profile
	markers  map[string]bool
	lastSeed profilepkg.DeclaredSeed
}

func (s *declaredProfileManagerStub) CreateDeclared(
	_ context.Context,
	in profilepkg.DeclaredInput,
) (profilepkg.Profile, error) {
	key := in.Extension + "\x00" + in.Name
	if s.markers[key] {
		return s.profiles[in.Name], nil
	}
	s.markers[key] = true
	if existing, ok := s.profiles[in.Name]; ok {
		return existing, nil
	}
	s.lastSeed = in.Seed
	created := profilepkg.Profile{
		ID: "pf-" + in.Name, Name: in.Name, Color: in.Seed.Color, Icon: in.Seed.Icon,
		State: profilepkg.StateActive, CreatedAt: time.Now().UTC(),
	}
	s.profiles[in.Name] = created
	return created, nil
}

func (s *declaredProfileManagerStub) GetByName(_ context.Context, name string) (profilepkg.Profile, error) {
	profile, ok := s.profiles[name]
	if !ok {
		return profilepkg.Profile{}, profilepkg.ErrNotFound
	}
	return profile, nil
}

func (s *declaredProfileManagerStub) HasDeclaredMarker(
	_ context.Context,
	extension, name string,
) (bool, error) {
	return s.markers[extension+"\x00"+name], nil
}

type managedInstallRegistryStub struct {
	getFn     func(string) (*ExtensionInfo, error)
	installFn func(*Manifest, string, string, ...InstallOption) error
}

type managedInstallReconcileRegistryStub struct {
	infos []ExtensionInfo
	err   error
}

func (s managedInstallReconcileRegistryStub) List() ([]ExtensionInfo, error) {
	return append([]ExtensionInfo(nil), s.infos...), s.err
}

func (s managedInstallRegistryStub) Get(name string) (*ExtensionInfo, error) {
	if s.getFn != nil {
		return s.getFn(name)
	}
	return nil, ErrExtensionNotFound
}

func (s managedInstallRegistryStub) Install(
	manifest *Manifest,
	path string,
	checksum string,
	opts ...InstallOption,
) error {
	if s.installFn != nil {
		return s.installFn(manifest, path, checksum, opts...)
	}
	return nil
}

func TestManagedInstallHelpers(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if got := ManagedInstallRoot(homePaths); got == "" {
		t.Fatal("ManagedInstallRoot() returned empty path")
	}
	if got, want := ManagedInstallPath(
		homePaths,
		" test-ext ",
	), filepath.Join(
		homePaths.HomeDir,
		managedInstallDirName,
		"test-ext",
	); got != want {
		t.Fatalf("ManagedInstallPath() = %q, want %q", got, want)
	}
	if got, err := ManagedInstallPathChecked(homePaths, " test-ext "); err != nil || got != filepath.Join(
		homePaths.HomeDir,
		managedInstallDirName,
		"test-ext",
	) {
		t.Fatalf("ManagedInstallPathChecked() = %q, %v; want contained path", got, err)
	}
	for _, name := range []string{"../escape", "nested/name", `nested\name`, ".", "..", "marketplace", "/abs"} {
		t.Run("Should reject unsafe name "+name, func(t *testing.T) {
			t.Parallel()

			if got, err := ManagedInstallPathChecked(homePaths, name); err == nil {
				t.Fatalf("ManagedInstallPathChecked(%q) = %q, nil; want error", name, got)
			}
		})
	}

	stagingDir, err := NewManagedInstallStagingDir(homePaths)
	if err != nil {
		t.Fatalf("NewManagedInstallStagingDir() error = %v", err)
	}
	if _, err := os.Stat(stagingDir); err != nil {
		t.Fatalf("os.Stat(stagingDir) error = %v", err)
	}
	if err := os.RemoveAll(stagingDir); err != nil {
		t.Fatalf("os.RemoveAll(stagingDir) error = %v", err)
	}
}

func TestReconcileManagedExtensionArtifacts(t *testing.T) {
	t.Parallel()

	t.Run("Should restore committed artifacts and remove managed residues", func(t *testing.T) {
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		root := ManagedInstallRoot(homePaths)
		registered := filepath.Join(root, "registered")
		backup := registered + managedInstallBackupMarker + "1755280000000000000"
		orphan := filepath.Join(root, "orphan")
		staging := filepath.Join(root, managedInstallStagingPrefix+"interrupted")
		for _, path := range []string{registered, backup, orphan, staging, homePaths.ExtensionDataRoot} {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("MkdirAll(%q) error = %v", path, err)
			}
		}
		writeFile(t, filepath.Join(registered, manifestTOMLFileName), "candidate")
		writeFile(t, filepath.Join(backup, manifestTOMLFileName), "committed")
		writeFile(t, filepath.Join(orphan, manifestTOMLFileName), "orphan")
		writeFile(t, filepath.Join(staging, "partial"), "staged")
		dataSentinel := filepath.Join(homePaths.ExtensionDataRoot, "registered", "state")
		if err := os.MkdirAll(filepath.Dir(dataSentinel), 0o755); err != nil {
			t.Fatalf("MkdirAll(data sentinel) error = %v", err)
		}
		writeFile(t, dataSentinel, "preserve")
		quarantine := filepath.Join(homePaths.ExtensionDataRoot, "removed.compozy-quarantine-1755280000000000000")
		if err := os.MkdirAll(quarantine, 0o755); err != nil {
			t.Fatalf("MkdirAll(quarantine) error = %v", err)
		}
		writeFile(t, filepath.Join(quarantine, "state"), "residue")
		committedChecksum, err := ComputeDirectoryChecksum(backup)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(backup) error = %v", err)
		}
		registry := managedInstallReconcileRegistryStub{infos: []ExtensionInfo{{
			Name:         "registered",
			ManifestPath: filepath.Join(registered, manifestTOMLFileName),
			Checksum:     committedChecksum,
		}}}

		if err := ReconcileManagedExtensionArtifacts(homePaths, registry); err != nil {
			t.Fatalf("ReconcileManagedExtensionArtifacts() error = %v", err)
		}
		content, err := os.ReadFile(filepath.Join(registered, manifestTOMLFileName))
		if err != nil || string(content) != "committed" {
			t.Fatalf("registered artifact = %q, %v; want restored committed bytes", content, err)
		}
		for _, removed := range []string{backup, orphan, staging} {
			if _, statErr := os.Lstat(removed); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("orphan %q stat error = %v, want not exists", removed, statErr)
			}
		}
		if _, statErr := os.Lstat(quarantine); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("quarantine stat error = %v, want startup sweep removal", statErr)
		}
		data, err := os.ReadFile(dataSentinel)
		if err != nil || string(data) != "preserve" {
			t.Fatalf("extension data = %q, %v; want preserved", data, err)
		}
	})
}

func TestReconcileManagedExtensionArtifactsRejectsEscapedRoot(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("directory symlink setup is platform-specific")
	}
	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	writeFile(t, sentinel, "preserve")
	if err := os.Symlink(outside, ManagedInstallRoot(homePaths)); err != nil {
		t.Fatalf("Symlink(managed root) error = %v", err)
	}
	if err := ReconcileManagedExtensionArtifacts(homePaths, managedInstallReconcileRegistryStub{}); err == nil ||
		!strings.Contains(err.Error(), "escapes home") {
		t.Fatalf("ReconcileManagedExtensionArtifacts(symlink root) error = %v, want containment failure", err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "preserve" {
		t.Fatalf("outside sentinel = %q, %v; want preserved", data, err)
	}
}

func TestReconcileManagedExtensionArtifactsRejectsBackupLookalikes(t *testing.T) {
	t.Parallel()

	t.Run("Should not restore a non-generated backup suffix", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		root := ManagedInstallRoot(homePaths)
		registered := filepath.Join(root, "registered")
		lookalike := registered + managedInstallBackupMarker + "interrupted"
		for _, path := range []string{registered, lookalike} {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("MkdirAll(%q) error = %v", path, err)
			}
		}
		writeFile(t, filepath.Join(registered, manifestTOMLFileName), "candidate")
		writeFile(t, filepath.Join(lookalike, manifestTOMLFileName), "lookalike")
		if err := ReconcileManagedExtensionArtifacts(homePaths, managedInstallReconcileRegistryStub{
			infos: []ExtensionInfo{{
				Name: "registered", ManifestPath: filepath.Join(registered, manifestTOMLFileName), Checksum: "expected",
			}},
		}); err != nil {
			t.Fatalf("ReconcileManagedExtensionArtifacts() error = %v", err)
		}
		content, err := os.ReadFile(filepath.Join(registered, manifestTOMLFileName))
		if err != nil || string(content) != "candidate" {
			t.Fatalf("registered artifact = %q, %v; want candidate untouched", content, err)
		}
		if _, err := os.Lstat(lookalike); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("lookalike stat error = %v, want removed as an orphan", err)
		}
	})
}

func TestRemoveManagedInstallContainment(t *testing.T) {
	t.Parallel()

	t.Run("Should remove a contained managed install", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		target, err := ManagedInstallPathChecked(homePaths, "test-ext")
		if err != nil {
			t.Fatalf("ManagedInstallPathChecked() error = %v", err)
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(target) error = %v", err)
		}
		if err := RemoveManagedInstall(homePaths, "test-ext"); err != nil {
			t.Fatalf("RemoveManagedInstall() error = %v", err)
		}
		if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Lstat(target) error = %v, want not exists", err)
		}
	})

	t.Run("Should reject a managed install symlink that resolves outside the root", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		root := ManagedInstallRoot(homePaths)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(root) error = %v", err)
		}
		outside := t.TempDir()
		sentinel := filepath.Join(outside, "sentinel")
		if err := os.WriteFile(sentinel, []byte("preserve"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(sentinel) error = %v", err)
		}
		target, err := ManagedInstallPathChecked(homePaths, "test-ext")
		if err != nil {
			t.Fatalf("ManagedInstallPathChecked() error = %v", err)
		}
		if err := os.Symlink(outside, target); err != nil {
			t.Fatalf("os.Symlink(outside, target) error = %v", err)
		}
		if err := RemoveManagedInstall(homePaths, "test-ext"); err == nil {
			t.Fatal("RemoveManagedInstall() error = nil, want containment refusal")
		}
		if got, err := os.ReadFile(sentinel); err != nil || string(got) != "preserve" {
			t.Fatalf("os.ReadFile(sentinel) = %q, %v; want preserved", got, err)
		}
	})
}

func TestInstallLocalManagedRejectsUnsafeManifestName(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	registry := managedInstallRegistryStub{
		installFn: func(*Manifest, string, string, ...InstallOption) error {
			t.Fatal("Install should not be called for unsafe managed extension names")
			return nil
		},
	}

	err = InstallLocalManaged(
		homePaths,
		registry,
		&Manifest{Name: "../escape", Version: "1.0.0"},
		filepath.Join(t.TempDir(), "missing-source"),
		"sha256:unused",
	)
	if err == nil {
		t.Fatal("InstallLocalManaged() error = nil, want unsafe name rejection")
	}
	escapedPath := filepath.Join(homePaths.HomeDir, "..", "escape")
	if _, statErr := os.Stat(escapedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("os.Stat(%q) error = %v, want not exists", escapedPath, statErr)
	}
}

func TestCopyInstallTreeRejectsEmptyPaths(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an empty source directory", func(t *testing.T) {
		t.Parallel()

		err := copyInstallTree(" ", filepath.Join(t.TempDir(), "target"))
		if err == nil || !strings.Contains(err.Error(), "source directory is required") {
			t.Fatalf("copyInstallTree(empty source) error = %v, want required source", err)
		}
	})

	t.Run("Should reject an empty target directory", func(t *testing.T) {
		t.Parallel()

		err := copyInstallTree(t.TempDir(), " ")
		if err == nil || !strings.Contains(err.Error(), "target directory is required") {
			t.Fatalf("copyInstallTree(empty target) error = %v, want required target", err)
		}
	})
}

func TestCopyInstallTreeMaterializesSymlinkTargets(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source) error = %v", err)
	}

	internalDir := filepath.Join(sourceDir, "vendor", "extension-sdk")
	if err := os.MkdirAll(filepath.Join(internalDir, "bin"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(internal) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(internalDir, "package.json"),
		[]byte("{\"name\":\"@compozy/extension-sdk\"}\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(package.json) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(internalDir, "bin", "tsc"),
		[]byte("#!/usr/bin/env node\n"),
		0o755,
	); err != nil {
		t.Fatalf("os.WriteFile(tsc) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", "@compozy"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(node_modules/@compozy) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", ".bin"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(node_modules/.bin) error = %v", err)
	}
	if err := os.Symlink(
		filepath.Join(sourceDir, "vendor", "extension-sdk"),
		filepath.Join(sourceDir, "node_modules", "@compozy", "extension-sdk"),
	); err != nil {
		t.Skipf("os.Symlink(directory) unavailable: %v", err)
	}
	if err := os.Symlink(
		filepath.Join(sourceDir, "vendor", "extension-sdk", "bin", "tsc"),
		filepath.Join(sourceDir, "node_modules", ".bin", "tsc"),
	); err != nil {
		t.Skipf("os.Symlink(file) unavailable: %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "target")
	if err := copyInstallTree(sourceDir, targetDir); err != nil {
		t.Fatalf("copyInstallTree() error = %v", err)
	}

	copiedDir := filepath.Join(targetDir, "node_modules", "@compozy", "extension-sdk")
	info, err := os.Lstat(copiedDir)
	if err != nil {
		t.Fatalf("os.Lstat(%q) error = %v", copiedDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copied sdk dir mode = %v, want materialized directory", info.Mode())
	}
	if !info.IsDir() {
		t.Fatalf("copied sdk dir IsDir() = false, want true")
	}
	if _, err := os.Stat(filepath.Join(copiedDir, "package.json")); err != nil {
		t.Fatalf("os.Stat(copied package.json) error = %v", err)
	}

	copiedFile := filepath.Join(targetDir, "node_modules", ".bin", "tsc")
	fileInfo, err := os.Lstat(copiedFile)
	if err != nil {
		t.Fatalf("os.Lstat(%q) error = %v", copiedFile, err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copied tsc mode = %v, want materialized file", fileInfo.Mode())
	}
	if fileInfo.IsDir() {
		t.Fatalf("copied tsc IsDir() = true, want file")
	}
	content, err := os.ReadFile(copiedFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", copiedFile, err)
	}
	if string(content) != "#!/usr/bin/env node\n" {
		t.Fatalf("copied tsc content = %q, want script payload", string(content))
	}
}

func TestCopyInstallTreeRetainsResolvedSymlinkAuthority(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source) error = %v", err)
	}
	safePath := filepath.Join(sourceDir, "safe.txt")
	if err := os.WriteFile(safePath, []byte("safe"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(safe) error = %v", err)
	}
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside-secret"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(outside) error = %v", err)
	}
	linkPath := filepath.Join(sourceDir, "link.txt")
	if err := os.Symlink("safe.txt", linkPath); err != nil {
		t.Skipf("os.Symlink(internal) unavailable: %v", err)
	}

	var swapErr error
	targetDir := filepath.Join(t.TempDir(), "target")
	err := copyInstallTreeWithHooks(sourceDir, targetDir, installCopyHooks{
		afterSymlinkResolve: func() {
			if err := os.Remove(linkPath); err != nil {
				swapErr = fmt.Errorf("remove resolved symlink: %w", err)
				return
			}
			if err := os.Symlink(outsidePath, linkPath); err != nil {
				swapErr = fmt.Errorf("replace resolved symlink: %w", err)
			}
		},
	})
	if swapErr != nil {
		t.Fatalf("symlink swap error = %v", swapErr)
	}
	if err != nil {
		t.Fatalf("copyInstallTreeWithHooks() error = %v", err)
	}
	copied, err := os.ReadFile(filepath.Join(targetDir, "link.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(copied link) error = %v", err)
	}
	if string(copied) != "safe" {
		t.Fatalf("copied link = %q, want originally resolved in-root bytes", copied)
	}
}

func TestCopyInstallTreeCopiesDeclaredRuntimeNodeModulesOnly(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", "@compozy"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source node_modules) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", "@types"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source @types) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", ".bin"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source .bin) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "package.json"),
		[]byte(
			"{\"dependencies\":{\"@compozy/extension-sdk\":\"workspace:*\",\"@compozy/extension-utils\":\"workspace:*\"},\"devDependencies\":{\"@types/node\":\"^25.5.2\",\"typescript\":\"^6.0.2\"}}\n",
		),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(source package.json) error = %v", err)
	}

	runtimePackageDir := filepath.Join(sourceDir, "vendor", "extension-sdk")
	if err := os.MkdirAll(filepath.Join(runtimePackageDir, "dist"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(runtime package) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(runtimePackageDir, "package.json"),
		[]byte("{\"name\":\"@compozy/extension-sdk\",\"main\":\"./dist/index.js\"}\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(runtime package.json) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(runtimePackageDir, "dist", "index.js"),
		[]byte("export const runtime = true;\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(runtime dist) error = %v", err)
	}
	runtimeUtilsDir := filepath.Join(sourceDir, "vendor", "extension-utils")
	if err := os.MkdirAll(runtimeUtilsDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(runtime utils) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(runtimeUtilsDir, "package.json"),
		[]byte("{\"name\":\"@compozy/extension-utils\"}\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(runtime utils package.json) error = %v", err)
	}

	typescriptDir := filepath.Join(t.TempDir(), "typescript")
	if err := os.MkdirAll(filepath.Join(typescriptDir, "bin"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(typescript) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(typescriptDir, "bin", "tsc"),
		[]byte("#!/usr/bin/env node\n"),
		0o755,
	); err != nil {
		t.Fatalf("os.WriteFile(tsc) error = %v", err)
	}

	nodeTypesDir := filepath.Join(t.TempDir(), "node-types")
	if err := os.MkdirAll(nodeTypesDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(node types) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeTypesDir, "index.d.ts"), []byte("export {};\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(node types) error = %v", err)
	}

	if err := os.Symlink(
		runtimePackageDir,
		filepath.Join(sourceDir, "node_modules", "@compozy", "extension-sdk"),
	); err != nil {
		t.Skipf("os.Symlink(runtime dependency) unavailable: %v", err)
	}
	if err := os.Symlink(
		runtimeUtilsDir,
		filepath.Join(sourceDir, "node_modules", "@compozy", "extension-utils"),
	); err != nil {
		t.Skipf("os.Symlink(second runtime dependency) unavailable: %v", err)
	}
	if err := os.Symlink(typescriptDir, filepath.Join(sourceDir, "node_modules", "typescript")); err != nil {
		t.Skipf("os.Symlink(dev dependency) unavailable: %v", err)
	}
	if err := os.Symlink(nodeTypesDir, filepath.Join(sourceDir, "node_modules", "@types", "node")); err != nil {
		t.Skipf("os.Symlink(dev dependency) unavailable: %v", err)
	}
	if err := os.Symlink(
		filepath.Join(typescriptDir, "bin", "tsc"),
		filepath.Join(sourceDir, "node_modules", ".bin", "tsc"),
	); err != nil {
		t.Skipf("os.Symlink(dev binary) unavailable: %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "target")
	if err := copyInstallTree(sourceDir, targetDir); err != nil {
		t.Fatalf("copyInstallTree() error = %v", err)
	}

	copiedRuntimeDir := filepath.Join(targetDir, "node_modules", "@compozy", "extension-sdk")
	info, err := os.Lstat(copiedRuntimeDir)
	if err != nil {
		t.Fatalf("os.Lstat(%q) error = %v", copiedRuntimeDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copied runtime dir mode = %v, want materialized directory", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(copiedRuntimeDir, "package.json")); err != nil {
		t.Fatalf("os.Stat(copied runtime package.json) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(copiedRuntimeDir, "dist", "index.js")); err != nil {
		t.Fatalf("os.Stat(copied runtime dist) error = %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(targetDir, "node_modules", "@compozy", "extension-utils", "package.json"),
	); err != nil {
		t.Fatalf("os.Stat(copied second scoped runtime package) error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "node_modules", "typescript")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(copied dev dependency) error = %v, want not exists", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "node_modules", "@types")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(copied dev types) error = %v, want not exists", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "node_modules", ".bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(copied dev bin) error = %v, want not exists", err)
	}
}

func TestCopyInstallTreeRejectsRuntimeDependencySymlinkOutsideSourceRoot(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source node_modules) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "package.json"),
		[]byte("{\"dependencies\":{\"escape\":\"1.0.0\"}}\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(source package.json) error = %v", err)
	}

	outsideDir := filepath.Join(t.TempDir(), "escape")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(outside) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "package.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(outside package.json) error = %v", err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(sourceDir, "node_modules", "escape")); err != nil {
		t.Skipf("os.Symlink(runtime dependency) unavailable: %v", err)
	}

	err := copyInstallTree(sourceDir, filepath.Join(t.TempDir(), "target"))
	if err == nil {
		t.Fatal("copyInstallTree() error = nil, want symlink escape rejection")
	}
	if !strings.Contains(err.Error(), "reject runtime dependency symlink") {
		t.Fatalf("copyInstallTree() error = %v, want runtime dependency symlink rejection", err)
	}
}

func TestInstallLocalManaged(t *testing.T) {
	t.Parallel()

	for _, boundary := range []managedInstallBoundary{
		managedInstallBoundaryStaged,
		managedInstallBoundaryFinalMoved,
	} {
		t.Run("Should leave no visible state after interruption at "+string(boundary), func(t *testing.T) {
			t.Parallel()

			sourceDir := t.TempDir()
			writeFile(t, filepath.Join(sourceDir, manifestTOMLFileName), "[extension]\nname = \"interrupted-ext\"\n")
			checksum, err := ComputeDirectoryChecksum(sourceDir)
			if err != nil {
				t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
			}
			homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			installCalls := 0
			registry := managedInstallRegistryStub{installFn: func(
				*Manifest,
				string,
				string,
				...InstallOption,
			) error {
				installCalls++
				return nil
			}}
			interruption := errors.New("injected interruption")
			err = InstallLocalManaged(
				homePaths,
				registry,
				&Manifest{Name: "interrupted-ext"},
				sourceDir,
				checksum,
				withManagedInstallBoundaryObserver(func(current managedInstallBoundary) error {
					if current == boundary {
						return interruption
					}
					return nil
				}),
			)
			if !errors.Is(err, interruption) {
				t.Fatalf("InstallLocalManaged(interrupted) error = %v, want %v", err, interruption)
			}
			if installCalls != 0 {
				t.Fatalf("registry install calls after interruption = %d, want zero", installCalls)
			}
			finalDir := ManagedInstallPath(homePaths, "interrupted-ext")
			if _, statErr := os.Stat(finalDir); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("final dir after interruption stat error = %v, want not exists", statErr)
			}

			if err := InstallLocalManaged(
				homePaths,
				registry,
				&Manifest{Name: "interrupted-ext"},
				sourceDir,
				checksum,
			); err != nil {
				t.Fatalf("InstallLocalManaged(retry) error = %v", err)
			}
			if installCalls != 1 {
				t.Fatalf("registry install calls after retry = %d, want one", installCalls)
			}
		})
	}

	t.Run("Should use the installed checksum for materialized symlinks", func(t *testing.T) {
		t.Parallel()

		sourceDir := filepath.Join(t.TempDir(), "source")
		if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(source) error = %v", err)
		}

		internalFile := filepath.Join(sourceDir, "vendor", "external.js")
		if err := os.MkdirAll(filepath.Dir(internalFile), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(vendor) error = %v", err)
		}
		if err := os.WriteFile(internalFile, []byte("export const value = 1;\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(internal) error = %v", err)
		}
		if err := os.Symlink(internalFile, filepath.Join(sourceDir, "node_modules", "external.js")); err != nil {
			t.Skipf("os.Symlink(file) unavailable: %v", err)
		}

		sourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
		}
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		registry := &recordingManagedInstallRegistry{}
		manifest := &Manifest{Name: "symlink-ext"}

		if err := InstallLocalManaged(homePaths, registry, manifest, sourceDir, sourceChecksum); err != nil {
			t.Fatalf("InstallLocalManaged() error = %v", err)
		}

		finalDir := ManagedInstallPath(homePaths, manifest.Name)
		finalChecksum, err := ComputeDirectoryChecksum(finalDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(final) error = %v", err)
		}
		if got := registry.installedChecksum; got != finalChecksum {
			t.Fatalf("registry installed checksum = %q, want %q", got, finalChecksum)
		}
		if finalChecksum == sourceChecksum {
			t.Fatalf(
				"final checksum = %q, want checksum different from source symlink tree %q",
				finalChecksum,
				sourceChecksum,
			)
		}
	})

	t.Run("Should keep managed installs enabled by default", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		writeFile(t, filepath.Join(sourceDir, manifestTOMLFileName), "[extension]\nname = \"inert-ext\"\n")
		sourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
		}
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		registry := &recordingManagedInstallRegistry{}

		if err := InstallLocalManaged(
			homePaths,
			registry,
			&Manifest{Name: "inert-ext"},
			sourceDir,
			sourceChecksum,
		); err != nil {
			t.Fatalf("InstallLocalManaged() error = %v", err)
		}
		if !registry.installedEnabled {
			t.Fatal("registry installed enabled = false, want default-on")
		}
	})

	t.Run("Should honor an explicit enabled state", func(t *testing.T) {
		t.Parallel()

		sourceDir := t.TempDir()
		writeFile(t, filepath.Join(sourceDir, manifestTOMLFileName), "[extension]\nname = \"enabled-ext\"\n")
		sourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
		}
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		registry := &recordingManagedInstallRegistry{}

		if err := InstallLocalManaged(
			homePaths,
			registry,
			&Manifest{Name: "enabled-ext"},
			sourceDir,
			sourceChecksum,
			WithInstallEnabled(true),
		); err != nil {
			t.Fatalf("InstallLocalManaged() error = %v", err)
		}
		if !registry.installedEnabled {
			t.Fatal("registry installed enabled = false, want explicit enabled state")
		}
	})
}

func TestInstallLocalManagedNormalizesProvidedChecksum(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "extension.toml"),
		[]byte("name = \"checksum-ext\"\nversion = \"1.0.0\"\nmin_compozy_version = \"0.1.0\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}

	sourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
	}

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	registry := &recordingManagedInstallRegistry{}
	manifest := &Manifest{Name: "checksum-ext"}

	if err := InstallLocalManaged(
		homePaths,
		registry,
		manifest,
		sourceDir,
		"  "+strings.ToUpper(sourceChecksum)+"  ",
	); err != nil {
		t.Fatalf("InstallLocalManaged(normalized checksum) error = %v", err)
	}
}

func TestInstallLocalManagedRejectsExistingOrFailedInstall(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	existingSourceDir := filepath.Join(t.TempDir(), "existing-source")
	if err := os.MkdirAll(existingSourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(existing source) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(existingSourceDir, "extension.toml"),
		[]byte("name = \"existing-ext\"\nversion = \"1.0.0\"\nmin_compozy_version = \"0.1.0\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(existing extension.toml) error = %v", err)
	}

	err = InstallLocalManaged(homePaths, managedInstallRegistryStub{
		getFn: func(string) (*ExtensionInfo, error) {
			return &ExtensionInfo{Name: "existing-ext"}, nil
		},
	}, &Manifest{Name: "existing-ext"}, existingSourceDir, "checksum-ignored")
	if err == nil {
		t.Fatal("InstallLocalManaged(existing) error = nil, want non-nil")
	}

	failingSourceDir := filepath.Join(t.TempDir(), "failing-source")
	if err := os.MkdirAll(failingSourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(failing source) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(failingSourceDir, "extension.toml"),
		[]byte("name = \"failing-ext\"\nversion = \"1.0.0\"\nmin_compozy_version = \"0.1.0\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(failing extension.toml) error = %v", err)
	}
	sourceChecksum, err := ComputeDirectoryChecksum(failingSourceDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(failing source) error = %v", err)
	}

	installErr := errors.New("install failed")
	err = InstallLocalManaged(homePaths, managedInstallRegistryStub{
		getFn: func(string) (*ExtensionInfo, error) {
			return nil, ErrExtensionNotFound
		},
		installFn: func(*Manifest, string, string, ...InstallOption) error {
			return installErr
		},
	}, &Manifest{Name: "failing-ext"}, failingSourceDir, sourceChecksum)
	if !errors.Is(err, installErr) {
		t.Fatalf("InstallLocalManaged(failing) error = %v, want %v", err, installErr)
	}
	if _, statErr := os.Stat(ManagedInstallPath(homePaths, "failing-ext")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed install path stat error = %v, want not exists", statErr)
	}
}

func TestCopyInstallTreeRejectsSymlinkDirectoryCycles(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source) error = %v", err)
	}
	if err := os.Symlink(".", filepath.Join(sourceDir, "loop")); err != nil {
		t.Skipf("os.Symlink(directory) unavailable: %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "target")
	err := copyInstallTree(sourceDir, targetDir)
	if err == nil {
		t.Fatal("copyInstallTree() error = nil, want symlink cycle failure")
	}
	if !strings.Contains(err.Error(), "symlink directory cycle detected") {
		t.Fatalf("copyInstallTree() error = %v, want symlink cycle context", err)
	}
}

func TestCopyInstallTreeRejectsSymlinkTargetsOutsideSourceRoot(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectExternalDirectoryTargets", func(t *testing.T) {
		t.Parallel()

		sourceDir := filepath.Join(t.TempDir(), "source")
		if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(source) error = %v", err)
		}

		externalDir := filepath.Join(t.TempDir(), "external-sdk")
		if err := os.MkdirAll(externalDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(external) error = %v", err)
		}
		if err := os.Symlink(externalDir, filepath.Join(sourceDir, "node_modules", "sdk")); err != nil {
			t.Skipf("os.Symlink(directory) unavailable: %v", err)
		}

		err := copyInstallTree(sourceDir, filepath.Join(t.TempDir(), "target"))
		if err == nil {
			t.Fatal("copyInstallTree() error = nil, want symlink escape failure")
		}
		if !strings.Contains(err.Error(), "escapes source root") {
			t.Fatalf("copyInstallTree() error = %v, want escape context", err)
		}
	})

	t.Run("ShouldRejectExternalFileTargets", func(t *testing.T) {
		t.Parallel()

		sourceDir := filepath.Join(t.TempDir(), "source")
		if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(source) error = %v", err)
		}

		externalFile := filepath.Join(t.TempDir(), "external.js")
		if err := os.WriteFile(externalFile, []byte("export const value = 1;\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(external) error = %v", err)
		}
		if err := os.Symlink(externalFile, filepath.Join(sourceDir, "node_modules", "external.js")); err != nil {
			t.Skipf("os.Symlink(file) unavailable: %v", err)
		}

		err := copyInstallTree(sourceDir, filepath.Join(t.TempDir(), "target"))
		if err == nil {
			t.Fatal("copyInstallTree() error = nil, want symlink escape failure")
		}
		if !strings.Contains(err.Error(), "escapes source root") {
			t.Fatalf("copyInstallTree() error = %v, want escape context", err)
		}
	})
}

func TestInstallLocalManagedWrapsPhaseErrors(t *testing.T) {
	t.Parallel()

	t.Run("ShouldWrapSourceChecksumFailures", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		err = InstallLocalManaged(
			homePaths,
			&recordingManagedInstallRegistry{},
			&Manifest{Name: "missing-ext"},
			filepath.Join(t.TempDir(), "missing"),
			"checksum",
		)
		if err == nil || !strings.Contains(err.Error(), "extension: compute source checksum") {
			t.Fatalf("InstallLocalManaged() error = %v, want wrapped source checksum failure", err)
		}
	})

	t.Run("ShouldWrapRegistryInstallFailures", func(t *testing.T) {
		t.Parallel()

		sourceDir := filepath.Join(t.TempDir(), "source")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(source) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(sourceDir, "extension.toml"),
			[]byte("name = \"wrapped-ext\"\nversion = \"1.0.0\"\nmin_compozy_version = \"0.1.0\"\n"),
			0o644,
		); err != nil {
			t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
		}

		sourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
		}
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		registry := &recordingManagedInstallRegistry{installErr: errors.New("registry boom")}
		err = InstallLocalManaged(homePaths, registry, &Manifest{Name: "wrapped-ext"}, sourceDir, sourceChecksum)
		if err == nil || !strings.Contains(err.Error(), `extension: persist managed extension "wrapped-ext"`) {
			t.Fatalf("InstallLocalManaged() error = %v, want wrapped registry install failure", err)
		}
	})
}

type recordingManagedInstallRegistry struct {
	installedChecksum string
	installedEnabled  bool
	installErr        error
}

func (*recordingManagedInstallRegistry) Get(string) (*ExtensionInfo, error) {
	return nil, ErrExtensionNotFound
}

func (r *recordingManagedInstallRegistry) Install(
	_ *Manifest,
	_ string,
	checksum string,
	opts ...InstallOption,
) error {
	r.installedChecksum = checksum
	config := installConfig{enabled: true}
	applyInstallOptions(&config, opts...)
	r.installedEnabled = config.enabled
	if r.installErr != nil {
		return r.installErr
	}
	return nil
}
