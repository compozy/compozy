package extensionpkg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

var _ managedInstallRegistry = (*recordingManagedInstallRegistry)(nil)

type managedInstallRegistryStub struct {
	getFn     func(string) (*ExtensionInfo, error)
	installFn func(*Manifest, string, string, ...InstallOption) error
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

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
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

func TestInstallLocalManagedRejectsUnsafeManifestName(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
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
		[]byte("{\"name\":\"@agh/extension-sdk\"}\n"),
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

	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", "@agh"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(node_modules/@agh) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", ".bin"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(node_modules/.bin) error = %v", err)
	}
	if err := os.Symlink(
		filepath.Join(sourceDir, "vendor", "extension-sdk"),
		filepath.Join(sourceDir, "node_modules", "@agh", "extension-sdk"),
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

	copiedDir := filepath.Join(targetDir, "node_modules", "@agh", "extension-sdk")
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

func TestCopyInstallTreeCopiesDeclaredRuntimeNodeModulesOnly(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules", "@agh"), 0o755); err != nil {
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
			"{\"dependencies\":{\"@agh/extension-sdk\":\"workspace:*\"},\"devDependencies\":{\"@types/node\":\"^25.5.2\",\"typescript\":\"^6.0.2\"}}\n",
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
		[]byte("{\"name\":\"@agh/extension-sdk\",\"main\":\"./dist/index.js\"}\n"),
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
		filepath.Join(sourceDir, "node_modules", "@agh", "extension-sdk"),
	); err != nil {
		t.Skipf("os.Symlink(runtime dependency) unavailable: %v", err)
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

	copiedRuntimeDir := filepath.Join(targetDir, "node_modules", "@agh", "extension-sdk")
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

func TestInstallLocalManagedUsesInstalledChecksumForMaterializedSymlinks(t *testing.T) {
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

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
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
}

func TestInstallLocalManagedNormalizesProvidedChecksum(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(source) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "extension.toml"),
		[]byte("name = \"checksum-ext\"\nversion = \"1.0.0\"\nmin_agh_version = \"0.1.0\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}

	sourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
	}

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
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

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	existingSourceDir := filepath.Join(t.TempDir(), "existing-source")
	if err := os.MkdirAll(existingSourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(existing source) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(existingSourceDir, "extension.toml"),
		[]byte("name = \"existing-ext\"\nversion = \"1.0.0\"\nmin_agh_version = \"0.1.0\"\n"),
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
		[]byte("name = \"failing-ext\"\nversion = \"1.0.0\"\nmin_agh_version = \"0.1.0\"\n"),
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

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
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
			[]byte("name = \"wrapped-ext\"\nversion = \"1.0.0\"\nmin_agh_version = \"0.1.0\"\n"),
			0o644,
		); err != nil {
			t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
		}

		sourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(source) error = %v", err)
		}
		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
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
	installErr        error
}

func (*recordingManagedInstallRegistry) Get(string) (*ExtensionInfo, error) {
	return nil, ErrExtensionNotFound
}

func (r *recordingManagedInstallRegistry) Install(_ *Manifest, _ string, checksum string, _ ...InstallOption) error {
	r.installedChecksum = checksum
	if r.installErr != nil {
		return r.installErr
	}
	return nil
}
