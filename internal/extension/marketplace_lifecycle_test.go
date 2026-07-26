package extensionpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	registrypkg "github.com/compozy/agh/internal/registry"
)

type lifecycleSource struct {
	name          string
	extensionName string
	slug          string
	latestVersion string
	archives      map[string][]byte
	closeErr      error
	closeCount    int
}

var _ registrypkg.Source = (*lifecycleSource)(nil)

func (s *lifecycleSource) Name() string {
	if strings.TrimSpace(s.name) == "" {
		return "github"
	}
	return s.name
}

func (s *lifecycleSource) Capabilities() registrypkg.SourceCaps {
	return registrypkg.SourceCaps{Search: true}
}

func (s *lifecycleSource) Search(
	context.Context,
	string,
	registrypkg.SearchOpts,
) ([]registrypkg.Listing, error) {
	return []registrypkg.Listing{
		{
			Slug:    s.packageSlug(),
			Name:    s.packageName(),
			Version: s.latestVersion,
			Source:  s.Name(),
			Type:    registrypkg.PackageTypeExtension,
		},
	}, nil
}

func (s *lifecycleSource) Info(context.Context, string) (*registrypkg.Detail, error) {
	return &registrypkg.Detail{
		Listing: registrypkg.Listing{
			Slug:    s.packageSlug(),
			Name:    s.packageName(),
			Version: s.latestVersion,
			Source:  s.Name(),
			Type:    registrypkg.PackageTypeExtension,
		},
	}, nil
}

func (s *lifecycleSource) Download(
	_ context.Context,
	_ string,
	opts registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = s.latestVersion
	}
	archive := s.archives[version]
	if archive == nil {
		return nil, fmt.Errorf("test source missing version %q", version)
	}
	return &registrypkg.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(archive)),
		Slug:        s.packageSlug(),
		Version:     version,
		ContentSize: -1,
		ContentType: "application/gzip",
	}, nil
}

func (s *lifecycleSource) Close() error {
	s.closeCount++
	return s.closeErr
}

func (s *lifecycleSource) packageName() string {
	if strings.TrimSpace(s.extensionName) == "" {
		return "lifecycle-ext"
	}
	return s.extensionName
}

func (s *lifecycleSource) packageSlug() string {
	if strings.TrimSpace(s.slug) == "" {
		return "acme/" + s.packageName()
	}
	return s.slug
}

func TestMarketplaceLifecycleInstallsUpdatesAndRemovesManagedExtensions(t *testing.T) {
	t.Run("Should install update and remove managed marketplace extensions", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		source := newLifecycleSource(t, "1.0.0", "2.0.0")
		loader := func(context.Context) ([]registrypkg.Source, error) {
			return []registrypkg.Source{source}, nil
		}

		source.latestVersion = "1.0.0"
		installed, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceInstallRequest{
				Slug: "acme/lifecycle-ext", SourceFilter: "github",
				PolicyAllowsUnverified: true, AllowUnverified: true,
			},
		)
		if err != nil {
			t.Fatalf("InstallMarketplaceManaged() error = %v", err)
		}
		if installed.Source != SourceMarketplace ||
			dereferenceOptionalString(installed.RegistrySlug) != "acme/lifecycle-ext" ||
			dereferenceOptionalString(installed.RegistryName) != "github" ||
			dereferenceOptionalString(installed.RemoteVersion) != "1.0.0" {
			t.Fatalf("installed metadata = %#v, want marketplace provenance", installed)
		}
		requireFileContains(t, filepath.Join(ManagedInstallPath(homePaths, "lifecycle-ext"), "VERSION.txt"), "1.0.0")

		source.latestVersion = "2.0.0"
		checks, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{Names: []string{"lifecycle-ext"}, CheckOnly: true},
			nil,
		)
		if err != nil {
			t.Fatalf("UpdateMarketplaceManaged(check) error = %v", err)
		}
		if len(checks) != 1 || checks[0].Status != MarketplaceUpdateStatusAvailable {
			t.Fatalf("UpdateMarketplaceManaged(check) = %#v, want available update", checks)
		}
		requireFileContains(t, filepath.Join(ManagedInstallPath(homePaths, "lifecycle-ext"), "VERSION.txt"), "1.0.0")

		reloads := 0
		_, err = UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{Names: []string{"lifecycle-ext"}},
			nil,
		)
		if !errors.Is(err, ErrExtensionUnverifiedPolicyBlocked) {
			t.Fatalf(
				"UpdateMarketplaceManaged(policy blocked) error = %v, want ErrExtensionUnverifiedPolicyBlocked",
				err,
			)
		}

		updates, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{
				Names: []string{"lifecycle-ext"}, PolicyAllowsUnverified: true, AllowUnverified: true,
			},
			func(context.Context) error {
				reloads++
				return nil
			},
		)
		if err != nil {
			t.Fatalf("UpdateMarketplaceManaged(apply) error = %v", err)
		}
		if len(updates) != 1 || updates[0].Status != MarketplaceUpdateStatusUpdated || reloads != 1 {
			t.Fatalf("UpdateMarketplaceManaged(apply) = %#v reloads=%d, want updated with one reload", updates, reloads)
		}
		requireFileContains(t, filepath.Join(ManagedInstallPath(homePaths, "lifecycle-ext"), "VERSION.txt"), "2.0.0")

		removeErr := errors.New("reload failed")
		_, err = RemoveManagedExtension(t.Context(), env.registry, "lifecycle-ext", func(context.Context) error {
			return removeErr
		})
		if !errors.Is(err, removeErr) {
			t.Fatalf("RemoveManagedExtension(reload failure) error = %v, want reload failure", err)
		}
		if _, err := env.registry.Get("lifecycle-ext"); err != nil {
			t.Fatalf("registry.Get(after remove rollback) error = %v, want restored row", err)
		}
		requireFileContains(t, filepath.Join(ManagedInstallPath(homePaths, "lifecycle-ext"), "VERSION.txt"), "2.0.0")

		removed, err := RemoveManagedExtension(t.Context(), env.registry, "lifecycle-ext", nil)
		if err != nil {
			t.Fatalf("RemoveManagedExtension() error = %v", err)
		}
		if removed.Status != "removed" {
			t.Fatalf("RemoveManagedExtension() = %#v, want removed status", removed)
		}
		if _, err := env.registry.Get("lifecycle-ext"); !errors.Is(err, ErrExtensionNotFound) {
			t.Fatalf("registry.Get(after remove) error = %v, want ErrExtensionNotFound", err)
		}
		if _, err := os.Stat(ManagedInstallPath(homePaths, "lifecycle-ext")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(managed dir after remove) error = %v, want not exist", err)
		}
	})

	t.Run("Should close the registry source before moving an installation into place", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		closeErr := errors.New("registry close failed")
		source := newLifecycleSource(t, "1.0.0")
		source.closeErr = closeErr
		_, err = InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			func(context.Context) ([]registrypkg.Source, error) {
				return []registrypkg.Source{source}, nil
			},
			MarketplaceInstallRequest{
				Slug: "acme/lifecycle-ext", PolicyAllowsUnverified: true, AllowUnverified: true,
			},
		)
		if !errors.Is(err, closeErr) {
			t.Fatalf("InstallMarketplaceManaged() error = %v, want source close failure", err)
		}
		if got, want := source.closeCount, 1; got != want {
			t.Fatalf("source close count = %d, want %d", got, want)
		}
		if _, statErr := os.Stat(ManagedInstallPath(homePaths, "lifecycle-ext")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(managed install) error = %v, want not-exist", statErr)
		}
	})

	t.Run("Should keep removal committed when backup cleanup partially fails", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		source := newLifecycleSource(t, "1.0.0")
		installed, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			func(context.Context) ([]registrypkg.Source, error) { return []registrypkg.Source{source}, nil },
			MarketplaceInstallRequest{
				Slug: "acme/lifecycle-ext", PolicyAllowsUnverified: true, AllowUnverified: true,
			},
		)
		if err != nil {
			t.Fatalf("InstallMarketplaceManaged() error = %v", err)
		}
		installDir, err := InstalledExtensionDir(*installed)
		if err != nil {
			t.Fatalf("InstalledExtensionDir() error = %v", err)
		}
		change, err := stageExtensionDirRemoval(installDir)
		if err != nil {
			t.Fatalf("stageExtensionDirRemoval() error = %v", err)
		}
		if err := env.registry.Uninstall(installed.Name); err != nil {
			t.Fatalf("registry.Uninstall() error = %v", err)
		}
		manifestPath := filepath.Join(change.backupDir, manifestTOMLFileName)
		cleanupErr := errors.New("forced partial backup cleanup failure")
		change.removeBackup = func(string) error {
			if removeErr := os.Remove(manifestPath); removeErr != nil {
				return errors.Join(cleanupErr, removeErr)
			}
			return cleanupErr
		}

		result := finalizeManagedExtensionRemoval(installed.Name, installDir, change)
		if result.Status != "removed" || result.Name != installed.Name {
			t.Fatalf("finalizeManagedExtensionRemoval() = %#v, want committed removal", result)
		}
		if len(result.Warnings) != 1 ||
			result.Warnings[0].Code != diagnosticcontract.CodeExtensionRemoveCleanupFailed {
			t.Fatalf("removal warnings = %#v, want cleanup warning", result.Warnings)
		}
		if _, getErr := env.registry.Get(installed.Name); !errors.Is(getErr, ErrExtensionNotFound) {
			t.Fatalf("registry.Get(removed extension) error = %v, want ErrExtensionNotFound", getErr)
		}
		if _, statErr := os.Stat(installDir); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(removed install dir) error = %v, want not exist", statErr)
		}
		if _, statErr := os.Stat(change.backupDir); statErr != nil {
			t.Fatalf("os.Stat(backup residue) error = %v, want retained residue", statErr)
		}
		if _, statErr := os.Stat(manifestPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(partially removed manifest) error = %v, want not exist", statErr)
		}
	})
}

func TestMarketplaceLifecycleRollsBackFailedUpdateReload(t *testing.T) {
	t.Run("Should roll back failed update reloads", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		source := newLifecycleSource(t, "1.0.0", "2.0.0")
		loader := func(context.Context) ([]registrypkg.Source, error) {
			return []registrypkg.Source{source}, nil
		}

		source.latestVersion = "1.0.0"
		if _, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceInstallRequest{
				Slug: "acme/lifecycle-ext", PolicyAllowsUnverified: true, AllowUnverified: true,
			},
		); err != nil {
			t.Fatalf("InstallMarketplaceManaged() error = %v", err)
		}
		source.latestVersion = "2.0.0"

		reloadErr := errors.New("reload rejected update")
		_, err = UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{
				Names: []string{"lifecycle-ext"}, PolicyAllowsUnverified: true, AllowUnverified: true,
			},
			func(context.Context) error {
				return reloadErr
			},
		)
		if !errors.Is(err, reloadErr) {
			t.Fatalf("UpdateMarketplaceManaged(reload failure) error = %v, want reload rejection", err)
		}

		info, err := env.registry.Get("lifecycle-ext")
		if err != nil {
			t.Fatalf("registry.Get(after update rollback) error = %v", err)
		}
		if info.Version != "1.0.0" || dereferenceOptionalString(info.RemoteVersion) != "1.0.0" {
			t.Fatalf("registry info after rollback = %#v, want original version", info)
		}
		requireFileContains(t, filepath.Join(ManagedInstallPath(homePaths, "lifecycle-ext"), "VERSION.txt"), "1.0.0")
	})
}

func TestMarketplaceLifecycleReportsCommittedBatchUpdatesBeforeLaterFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should return committed updates in a typed partial failure", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		good := newLifecycleSourceNamed(t, "a-good", "good-registry", "1.0.0", "2.0.0")
		bad := newLifecycleSourceNamed(t, "z-bad", "bad-registry", "1.0.0", "2.0.0")
		loader := func(context.Context) ([]registrypkg.Source, error) {
			return []registrypkg.Source{good, bad}, nil
		}

		for _, source := range []*lifecycleSource{good, bad} {
			source.latestVersion = "1.0.0"
			if _, err := InstallMarketplaceManaged(
				t.Context(),
				homePaths,
				env.registry,
				loader,
				MarketplaceInstallRequest{
					Slug:                   source.packageSlug(),
					SourceFilter:           source.Name(),
					PolicyAllowsUnverified: true,
					AllowUnverified:        true,
				},
			); err != nil {
				t.Fatalf("InstallMarketplaceManaged(%s) error = %v", source.packageName(), err)
			}
			source.latestVersion = "2.0.0"
		}
		bad.archives["2.0.0"] = lifecycleTarGzNamed(t, "wrong-identity", "2.0.0")

		updates, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{All: true, PolicyAllowsUnverified: true, AllowUnverified: true},
			nil,
		)
		var batchErr *MarketplaceUpdateBatchError
		if !errors.As(err, &batchErr) {
			t.Fatalf("UpdateMarketplaceManaged() error = %T %v, want *MarketplaceUpdateBatchError", err, err)
		}
		if len(updates) != 1 || updates[0].Name != "a-good" || updates[0].Status != MarketplaceUpdateStatusUpdated {
			t.Fatalf("UpdateMarketplaceManaged() updates = %#v, want committed a-good update", updates)
		}
		if batchErr.FailedName != "z-bad" || len(batchErr.Completed) != 1 ||
			!reflect.DeepEqual(batchErr.Completed[0], updates[0]) {
			t.Fatalf("MarketplaceUpdateBatchError = %#v, want z-bad after committed a-good", batchErr)
		}
		for name, version := range map[string]string{"a-good": "2.0.0", "z-bad": "1.0.0"} {
			info, getErr := env.registry.Get(name)
			if getErr != nil {
				t.Fatalf("registry.Get(%s) error = %v", name, getErr)
			}
			if info.Version != version {
				t.Fatalf("registry.Get(%s).Version = %q, want %q", name, info.Version, version)
			}
			requireFileContains(t, filepath.Join(ManagedInstallPath(homePaths, name), "VERSION.txt"), version)
		}
	})
}

func TestMarketplaceLifecycleReportsPostCommitCleanupFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		target      string
		wantBackups int
		configure   func(*MarketplaceUpdateRequest, error)
	}{
		{
			name:   "Should preserve backup residue after backup cleanup fails",
			target: marketplaceUpdateCleanupBackup, wantBackups: 1,
			configure: func(req *MarketplaceUpdateRequest, cleanupErr error) {
				req.commitChange = func(*stagedExtensionDirChange) error { return cleanupErr }
			},
		},
		{
			name:   "Should preserve the committed update after staging cleanup fails",
			target: marketplaceUpdateCleanupStaging,
			configure: func(req *MarketplaceUpdateRequest, cleanupErr error) {
				req.removeStaging = func(string) error { return cleanupErr }
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertMarketplacePostCommitCleanupFailure(t, tt.target, tt.wantBackups, tt.configure)
		})
	}
}

func assertMarketplacePostCommitCleanupFailure(
	t *testing.T,
	target string,
	wantBackups int,
	configure func(*MarketplaceUpdateRequest, error),
) {
	t.Helper()
	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	env := newRegistryTestEnv(t)
	source := newLifecycleSource(t, "1.0.0", "2.0.0")
	loader := func(context.Context) ([]registrypkg.Source, error) {
		return []registrypkg.Source{source}, nil
	}
	source.latestVersion = "1.0.0"
	if _, err := InstallMarketplaceManaged(
		t.Context(),
		homePaths,
		env.registry,
		loader,
		MarketplaceInstallRequest{
			Slug: "acme/lifecycle-ext", PolicyAllowsUnverified: true, AllowUnverified: true,
		},
	); err != nil {
		t.Fatalf("InstallMarketplaceManaged() error = %v", err)
	}
	source.latestVersion = "2.0.0"
	cleanupErr := errors.New("post-commit cleanup denied")
	reloads := 0
	req := MarketplaceUpdateRequest{
		Names: []string{"lifecycle-ext"}, PolicyAllowsUnverified: true, AllowUnverified: true,
	}
	configure(&req, cleanupErr)

	updates, err := UpdateMarketplaceManaged(
		t.Context(),
		homePaths,
		env.registry,
		loader,
		req,
		func(context.Context) error {
			reloads++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("UpdateMarketplaceManaged() error = %v, want committed update with warning", err)
	}
	if len(updates) != 1 || updates[0].Status != MarketplaceUpdateStatusUpdated {
		t.Fatalf("UpdateMarketplaceManaged() = %#v, want updated result", updates)
	}
	if len(updates[0].Warnings) != 1 ||
		updates[0].Warnings[0].Code != diagnosticcontract.CodeExtensionUpdateCleanupFailed ||
		updates[0].Warnings[0].Evidence["cleanup_target"] != target {
		t.Fatalf("UpdateMarketplaceManaged().Warnings = %#v, want %s cleanup diagnostic", updates[0].Warnings, target)
	}
	if reloads != 1 {
		t.Fatalf("reload calls = %d, want 1 successful activation", reloads)
	}
	info, err := env.registry.Get("lifecycle-ext")
	if err != nil {
		t.Fatalf("registry.Get() error = %v", err)
	}
	if info.Version != "2.0.0" {
		t.Fatalf("registry version = %q, want committed 2.0.0", info.Version)
	}
	installDir := ManagedInstallPath(homePaths, "lifecycle-ext")
	requireFileContains(t, filepath.Join(installDir, "VERSION.txt"), "2.0.0")
	backups, err := filepath.Glob(installDir + ".agh-backup-*")
	if err != nil {
		t.Fatalf("filepath.Glob(backup) error = %v", err)
	}
	if len(backups) != wantBackups {
		t.Fatalf("backup paths = %#v, want %d", backups, wantBackups)
	}
}

func TestMarketplaceLifecycleValidatesSourcesAndInputs(t *testing.T) {
	t.Run("Should validate marketplace sources and lifecycle inputs", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		source := newLifecycleSource(t, "1.0.0")

		primary := newLifecycleSource(t, "1.0.0")
		secondary := newLifecycleSource(t, "1.0.0")
		secondary.name = "secondary"
		filtered, err := LoadMarketplaceSources(t.Context(), func(context.Context) ([]registrypkg.Source, error) {
			return []registrypkg.Source{primary, secondary}, nil
		}, "github")
		if err != nil {
			t.Fatalf("LoadMarketplaceSources(filter) error = %v", err)
		}
		if len(filtered) != 1 || filtered[0].Name() != "github" {
			t.Fatalf("LoadMarketplaceSources(filter) = %#v, want github source", filtered)
		}
		if secondary.closeCount != 1 {
			t.Fatalf("secondary source close count = %d, want 1", secondary.closeCount)
		}

		if _, err := LoadMarketplaceSources(t.Context(), func(context.Context) ([]registrypkg.Source, error) {
			return []registrypkg.Source{primary}, nil
		}, "missing"); err == nil {
			t.Fatal("LoadMarketplaceSources(missing filter) error = nil, want failure")
		}
		closeErr := errors.New("close failed")
		closing := newLifecycleSource(t, "1.0.0")
		closing.closeErr = closeErr
		if _, err := LoadMarketplaceSources(t.Context(), func(context.Context) ([]registrypkg.Source, error) {
			return []registrypkg.Source{closing}, errors.New("loader failed")
		}, ""); !errors.Is(err, closeErr) {
			t.Fatalf("LoadMarketplaceSources(loader close failure) error = %v, want close failure joined", err)
		}

		if _, err := InstalledExtensionDir(
			ExtensionInfo{Name: "bad", ManifestPath: "relative/extension.toml"},
		); err == nil {
			t.Fatal("InstalledExtensionDir(relative) error = nil, want failure")
		}
		if _, err := InstalledExtensionDir(
			ExtensionInfo{Name: "bad", ManifestPath: filepath.Join(t.TempDir(), "README.md")},
		); err == nil {
			t.Fatal("InstalledExtensionDir(non-manifest) error = nil, want failure")
		}

		loader := func(context.Context) ([]registrypkg.Source, error) {
			return []registrypkg.Source{source}, nil
		}
		source.latestVersion = "1.0.0"
		if _, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceInstallRequest{Slug: "acme/lifecycle-ext", AllowUnverified: true},
		); !errors.Is(err, ErrExtensionUnverifiedPolicyBlocked) {
			t.Fatalf(
				"InstallMarketplaceManaged(policy blocked) error = %v, want ErrExtensionUnverifiedPolicyBlocked",
				err,
			)
		}
		if _, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceInstallRequest{Slug: "acme/lifecycle-ext", PolicyAllowsUnverified: true},
		); !errors.Is(err, ErrExtensionChecksumUnverified) {
			t.Fatalf("InstallMarketplaceManaged(unconfirmed) error = %v, want ErrExtensionChecksumUnverified", err)
		}
		if _, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceInstallRequest{
				Slug: "acme/lifecycle-ext", PolicyAllowsUnverified: true, AllowUnverified: true,
			},
		); err != nil {
			t.Fatalf("InstallMarketplaceManaged() error = %v", err)
		}
		if _, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceInstallRequest{
				Slug: "acme/lifecycle-ext", PolicyAllowsUnverified: true, AllowUnverified: true,
			},
		); !errors.Is(err, ErrExtensionExists) {
			t.Fatalf("InstallMarketplaceManaged(duplicate) error = %v, want ErrExtensionExists", err)
		}

		current, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{Names: []string{"lifecycle-ext"}},
			func(context.Context) error {
				return errors.New("reload should not run for current extension")
			},
		)
		if err != nil {
			t.Fatalf("UpdateMarketplaceManaged(current) error = %v", err)
		}
		if len(current) != 1 || current[0].Status != MarketplaceUpdateStatusCurrent {
			t.Fatalf("UpdateMarketplaceManaged(current) = %#v, want current status", current)
		}
		allCurrent, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{All: true},
			nil,
		)
		if err != nil {
			t.Fatalf("UpdateMarketplaceManaged(all current) error = %v", err)
		}
		if len(allCurrent) != 1 || allCurrent[0].Status != MarketplaceUpdateStatusCurrent {
			t.Fatalf("UpdateMarketplaceManaged(all current) = %#v, want one current marketplace extension", allCurrent)
		}

		source.latestVersion = "2.0.0"
		source.archives["2.0.0"] = lifecycleTarGzNamed(t, "renamed-ext", "2.0.0")
		if _, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{
				Names: []string{"lifecycle-ext"}, PolicyAllowsUnverified: true, AllowUnverified: true,
			},
			nil,
		); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
			t.Fatalf("UpdateMarketplaceManaged(identity mismatch) error = %v, want identity mismatch", err)
		}

		if _, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{},
			nil,
		); err == nil {
			t.Fatal("UpdateMarketplaceManaged(missing name) error = nil, want failure")
		}
	})
}

func TestMarketplaceLifecycleVerifiesCuratedArchiveDigest(t *testing.T) {
	t.Parallel()

	t.Run("Should install and update a curated HTTPS artifact without registry fallback", func(t *testing.T) {
		t.Parallel()

		archives := map[string][]byte{
			"/repository-orientation-v1.0.0.tar.gz": lifecycleTarGzNamed(t, "repository-orientation", "1.0.0"),
			"/repository-orientation-v1.1.0.tar.gz": lifecycleTarGzNamed(t, "repository-orientation", "1.1.0"),
		}
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			archive, ok := archives[request.URL.Path]
			if !ok {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", "application/gzip")
			if _, err := writer.Write(archive); err != nil {
				t.Fatalf("writer.Write() error = %v", err)
			}
		}))
		t.Cleanup(server.Close)

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		loader := func(context.Context) ([]registrypkg.Source, error) {
			return nil, errors.New("registry fallback must not run for a curated artifact")
		}
		v1 := &MarketplaceTrustEvidence{
			CatalogEntryID:      "repository-orientation",
			Version:             "1.0.0",
			ArchiveDigestSHA256: lifecycleArchiveDigest(archives["/repository-orientation-v1.0.0.tar.gz"]),
			RegistryTier:        ExtensionRegistryTierOfficial,
			ArtifactURL:         server.URL + "/repository-orientation-v1.0.0.tar.gz",
			Repository:          "https://github.com/compozy/agh",
		}
		installed, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceInstallRequest{
				Slug: "compozy/repository-orientation", Trust: v1, ArtifactHTTPClient: server.Client(),
			},
		)
		if err != nil {
			t.Fatalf("InstallMarketplaceManaged() error = %v", err)
		}
		if got := dereferenceOptionalString(installed.RegistryName); got != MarketplaceCatalogRegistryName {
			t.Fatalf("installed registry = %q, want %q", got, MarketplaceCatalogRegistryName)
		}
		if installed.Provenance.SourceURL != v1.Repository {
			t.Fatalf("installed source URL = %q, want %q", installed.Provenance.SourceURL, v1.Repository)
		}

		v2 := &MarketplaceTrustEvidence{
			CatalogEntryID:      "repository-orientation",
			Version:             "1.1.0",
			ArchiveDigestSHA256: lifecycleArchiveDigest(archives["/repository-orientation-v1.1.0.tar.gz"]),
			RegistryTier:        ExtensionRegistryTierOfficial,
			ArtifactURL:         server.URL + "/repository-orientation-v1.1.0.tar.gz",
			Repository:          v1.Repository,
		}
		updates, err := UpdateMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			loader,
			MarketplaceUpdateRequest{
				Names: []string{"repository-orientation"}, ArtifactHTTPClient: server.Client(),
				ResolveTrust: func(_ context.Context, slug string, version string) (*MarketplaceTrustEvidence, error) {
					if slug != "compozy/repository-orientation" || version != "" {
						t.Fatalf("ResolveTrust(%q, %q), want catalog latest lookup", slug, version)
					}
					return v2, nil
				},
			},
			nil,
		)
		if err != nil {
			t.Fatalf("UpdateMarketplaceManaged() error = %v", err)
		}
		if len(updates) != 1 || updates[0].Status != MarketplaceUpdateStatusUpdated ||
			updates[0].LatestVersion != "1.1.0" {
			t.Fatalf("UpdateMarketplaceManaged() = %#v, want curated v1.1.0 update", updates)
		}
		info, err := env.registry.Get("repository-orientation")
		if err != nil {
			t.Fatalf("registry.Get() error = %v", err)
		}
		if info.Version != "1.1.0" || info.Provenance.ArchiveDigestSHA256 != v2.ArchiveDigestSHA256 {
			t.Fatalf("updated extension = %#v, want catalog-pinned v1.1.0", info)
		}
	})

	t.Run("Should install a matching curated archive without unverified consent", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		source := newLifecycleSource(t, "1.0.0")
		archiveDigest := lifecycleArchiveDigest(source.archives["1.0.0"])

		installed, err := InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			func(context.Context) ([]registrypkg.Source, error) { return []registrypkg.Source{source}, nil },
			MarketplaceInstallRequest{
				Slug: "acme/lifecycle-ext",
				Trust: &MarketplaceTrustEvidence{
					CatalogEntryID:      "lifecycle-ext-v1",
					Version:             "1.0.0",
					ArchiveDigestSHA256: archiveDigest,
					RegistryTier:        ExtensionRegistryTierOfficial,
				},
			},
		)
		if err != nil {
			t.Fatalf("InstallMarketplaceManaged() error = %v", err)
		}
		if !installed.Provenance.ChecksumVerified || installed.Provenance.AllowUnverified {
			t.Fatalf("installed provenance = %#v, want verified without unverified consent", installed.Provenance)
		}
		if installed.Provenance.CatalogEntryID != "lifecycle-ext-v1" ||
			installed.Provenance.ArchiveDigestSHA256 != archiveDigest ||
			installed.Provenance.RegistryTier != ExtensionRegistryTierOfficial {
			t.Fatalf("installed provenance = %#v, want curated catalog identity", installed.Provenance)
		}
		if len(installed.Provenance.Warnings) != 0 {
			t.Fatalf("installed warnings = %#v, want none", installed.Provenance.Warnings)
		}
	})

	t.Run("Should reject a curated digest mismatch even with unverified consent", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		env := newRegistryTestEnv(t)
		source := newLifecycleSource(t, "1.0.0")

		_, err = InstallMarketplaceManaged(
			t.Context(),
			homePaths,
			env.registry,
			func(context.Context) ([]registrypkg.Source, error) { return []registrypkg.Source{source}, nil },
			MarketplaceInstallRequest{
				Slug:            "acme/lifecycle-ext",
				AllowUnverified: true,
				Trust: &MarketplaceTrustEvidence{
					CatalogEntryID:      "lifecycle-ext-v1",
					Version:             "1.0.0",
					ArchiveDigestSHA256: strings.Repeat("0", sha256.Size*2),
					RegistryTier:        ExtensionRegistryTierOfficial,
				},
			},
		)
		if !errors.Is(err, ErrExtensionArchiveDigestMismatch) {
			t.Fatalf("InstallMarketplaceManaged() error = %v, want ErrExtensionArchiveDigestMismatch", err)
		}
		if !strings.Contains(err.Error(), "lifecycle-ext-v1") || !strings.Contains(err.Error(), "1.0.0") {
			t.Fatalf("InstallMarketplaceManaged() error = %v, want entry and version identity", err)
		}
		if _, statErr := os.Stat(ManagedInstallPath(homePaths, "lifecycle-ext")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(managed install) error = %v, want not-exist", statErr)
		}
	})

	t.Run("Should enforce unverified catalog publisher policy and consent on install", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			policyAllows bool
			allow        bool
			wantErr      error
		}{
			{
				name:    "Should block without policy or consent",
				wantErr: ErrExtensionUnverifiedPolicyBlocked,
			},
			{
				name:    "Should block despite consent when policy forbids",
				allow:   true,
				wantErr: ErrExtensionUnverifiedPolicyBlocked,
			},
			{
				name:         "Should require consent when policy allows",
				policyAllows: true,
				wantErr:      ErrExtensionChecksumUnverified,
			},
			{
				name:         "Should install with policy and consent",
				policyAllows: true,
				allow:        true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
				if err != nil {
					t.Fatalf("ResolveHomePathsFrom() error = %v", err)
				}
				env := newRegistryTestEnv(t)
				source := newLifecycleSource(t, "1.0.0")
				archiveDigest := lifecycleArchiveDigest(source.archives["1.0.0"])

				installed, err := InstallMarketplaceManaged(
					t.Context(),
					homePaths,
					env.registry,
					func(context.Context) ([]registrypkg.Source, error) { return []registrypkg.Source{source}, nil },
					MarketplaceInstallRequest{
						Slug:                   "acme/lifecycle-ext",
						PolicyAllowsUnverified: tt.policyAllows,
						AllowUnverified:        tt.allow,
						Trust: &MarketplaceTrustEvidence{
							CatalogEntryID:      "lifecycle-ext-v1",
							Version:             "1.0.0",
							ArchiveDigestSHA256: archiveDigest,
							RegistryTier:        ExtensionRegistryTierUnverified,
						},
					},
				)
				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("InstallMarketplaceManaged() error = %v, want %v", err, tt.wantErr)
					}
					_, statErr := os.Stat(ManagedInstallPath(homePaths, "lifecycle-ext"))
					if !errors.Is(statErr, os.ErrNotExist) {
						t.Fatalf("os.Stat(managed install) error = %v, want not-exist", statErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("InstallMarketplaceManaged() error = %v", err)
				}
				if installed == nil {
					t.Fatal("InstallMarketplaceManaged() installed = nil")
				}
				assertUnverifiedCatalogProvenance(t, installed.Provenance)
			})
		}
	})

	t.Run("Should enforce unverified catalog publisher policy and consent on update", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			policyAllows bool
			allow        bool
			wantErr      error
		}{
			{
				name:    "Should block without policy or consent",
				wantErr: ErrExtensionUnverifiedPolicyBlocked,
			},
			{
				name:    "Should block despite consent when policy forbids",
				allow:   true,
				wantErr: ErrExtensionUnverifiedPolicyBlocked,
			},
			{
				name:         "Should require consent when policy allows",
				policyAllows: true,
				wantErr:      ErrExtensionChecksumUnverified,
			},
			{
				name:         "Should update with policy and consent",
				policyAllows: true,
				allow:        true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
				if err != nil {
					t.Fatalf("ResolveHomePathsFrom() error = %v", err)
				}
				env := newRegistryTestEnv(t)
				source := newLifecycleSource(t, "1.0.0", "2.0.0")
				loader := func(context.Context) ([]registrypkg.Source, error) {
					return []registrypkg.Source{source}, nil
				}
				source.latestVersion = "1.0.0"
				if _, err := InstallMarketplaceManaged(
					t.Context(),
					homePaths,
					env.registry,
					loader,
					MarketplaceInstallRequest{
						Slug: "acme/lifecycle-ext",
						Trust: &MarketplaceTrustEvidence{
							CatalogEntryID:      "lifecycle-ext-v1",
							Version:             "1.0.0",
							ArchiveDigestSHA256: lifecycleArchiveDigest(source.archives["1.0.0"]),
							RegistryTier:        ExtensionRegistryTierOfficial,
						},
					},
				); err != nil {
					t.Fatalf("InstallMarketplaceManaged() error = %v", err)
				}
				source.latestVersion = "2.0.0"

				updates, err := UpdateMarketplaceManaged(
					t.Context(),
					homePaths,
					env.registry,
					loader,
					MarketplaceUpdateRequest{
						Names:                  []string{"lifecycle-ext"},
						PolicyAllowsUnverified: tt.policyAllows,
						AllowUnverified:        tt.allow,
						ResolveTrust: func(context.Context, string, string) (*MarketplaceTrustEvidence, error) {
							return &MarketplaceTrustEvidence{
								CatalogEntryID:      "lifecycle-ext-v2",
								Version:             "2.0.0",
								ArchiveDigestSHA256: lifecycleArchiveDigest(source.archives["2.0.0"]),
								RegistryTier:        ExtensionRegistryTierUnverified,
							}, nil
						},
					},
					nil,
				)
				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("UpdateMarketplaceManaged() error = %v, want %v", err, tt.wantErr)
					}
					info, getErr := env.registry.Get("lifecycle-ext")
					if getErr != nil {
						t.Fatalf("registry.Get() error = %v", getErr)
					}
					if info.Version != "1.0.0" {
						t.Fatalf("registry version = %q, want 1.0.0", info.Version)
					}
					return
				}
				if err != nil {
					t.Fatalf("UpdateMarketplaceManaged() error = %v", err)
				}
				if len(updates) != 1 || updates[0].Status != MarketplaceUpdateStatusUpdated {
					t.Fatalf("UpdateMarketplaceManaged() = %#v, want one updated result", updates)
				}
				info, err := env.registry.Get("lifecycle-ext")
				if err != nil {
					t.Fatalf("registry.Get() error = %v", err)
				}
				assertUnverifiedCatalogProvenance(t, info.Provenance)
			})
		}
	})
}

func assertUnverifiedCatalogProvenance(t *testing.T, provenance ExtensionProvenance) {
	t.Helper()

	if !provenance.ChecksumVerified || provenance.RegistryTier != ExtensionRegistryTierUnverified ||
		!provenance.AllowUnverified {
		t.Fatalf("provenance = %#v, want digest-verified unverified-publisher consent", provenance)
	}
	if len(provenance.Warnings) != 1 ||
		provenance.Warnings[0].Code != diagnosticcontract.CodeExtensionRegistryTierUnverified {
		t.Fatalf("provenance warnings = %#v, want unverified publisher diagnostic", provenance.Warnings)
	}
	if decision := extensionTrustDecision(provenance); decision != ExtensionTrustDecisionAllowedUnverified {
		t.Fatalf("extensionTrustDecision() = %q, want %q", decision, ExtensionTrustDecisionAllowedUnverified)
	}
}

func TestMarketplaceTrustDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should project pre-install trust from registry tier and live policy", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name            string
			tier            string
			policyAllows    bool
			wantDecision    string
			wantWarningCode string
		}{
			{
				name:         "Should trust a digest-pinned official catalog entry",
				tier:         ExtensionRegistryTierOfficial,
				wantDecision: ExtensionTrustDecisionVerified,
			},
			{
				name:            "Should block an unverified tier when policy forbids it",
				tier:            ExtensionRegistryTierUnverified,
				wantDecision:    ExtensionTrustDecisionBlocked,
				wantWarningCode: diagnosticcontract.CodeExtensionRegistryTierUnverified,
			},
			{
				name:            "Should require confirmation for an unverified tier when policy allows it",
				tier:            ExtensionRegistryTierUnverified,
				policyAllows:    true,
				wantDecision:    ExtensionTrustDecisionAllowedUnverified,
				wantWarningCode: diagnosticcontract.CodeExtensionRegistryTierUnverified,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				report, err := MarketplaceEntryTrustReport(MarketplaceTrustEvidence{
					CatalogEntryID: "extension.acme.catalog",
					Version:        "1.0.0",
					ArchiveDigestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					RegistryTier: tt.tier,
				}, tt.policyAllows)
				if err != nil {
					t.Fatalf("MarketplaceEntryTrustReport() error = %v", err)
				}
				if report.Decision != tt.wantDecision || report.RegistryTier != tt.tier ||
					report.ChecksumVerified || report.AllowUnverified != tt.policyAllows {
					t.Fatalf("MarketplaceEntryTrustReport() = %#v", report)
				}
				if tt.wantWarningCode == "" {
					if len(report.Warnings) != 0 {
						t.Fatalf("MarketplaceEntryTrustReport().Warnings = %#v, want none", report.Warnings)
					}
					return
				}
				if len(report.Warnings) != 1 || report.Warnings[0].Code != tt.wantWarningCode {
					t.Fatalf(
						"MarketplaceEntryTrustReport().Warnings = %#v, want code %q",
						report.Warnings,
						tt.wantWarningCode,
					)
				}
			})
		}
	})

	t.Run("Should preserve the typed policy diagnostic across error inspection", func(t *testing.T) {
		t.Parallel()

		err := ValidateUnverifiedSideLoad(" acme/blocked ", " local ", false, true)
		if !errors.Is(err, ErrExtensionUnverifiedPolicyBlocked) {
			t.Fatalf("ValidateUnverifiedSideLoad() error = %v, want ErrExtensionUnverifiedPolicyBlocked", err)
		}
		var blocked *ExtensionPolicyBlockedError
		if !errors.As(err, &blocked) {
			t.Fatalf("ValidateUnverifiedSideLoad() error = %T, want *ExtensionPolicyBlockedError", err)
		}
		if blocked.Error() != "extension: unverified install blocked by policy: acme/blocked" {
			t.Fatalf("ExtensionPolicyBlockedError.Error() = %q", blocked.Error())
		}
		item := blocked.DiagnosticItem()
		if item.Code != diagnosticcontract.CodeExtensionUnverifiedPolicyBlocked ||
			item.SuggestedCommand != "agh config set extensions.marketplace.allow_unverified true" {
			t.Fatalf("ExtensionPolicyBlockedError.DiagnosticItem() = %#v", item)
		}
		if got := item.Evidence["settings_path"]; got != "/settings/extensions" {
			t.Fatalf("ExtensionPolicyBlockedError.DiagnosticItem().Evidence[settings_path] = %v", got)
		}
		if err := ValidateUnverifiedSideLoad("acme/allowed", "local", true, true); err != nil {
			t.Fatalf("ValidateUnverifiedSideLoad(allowed) error = %v", err)
		}
	})

	t.Run("Should reject incomplete curated evidence before download", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			trust MarketplaceTrustEvidence
			want  string
		}{
			{name: "Should require catalog entry identity", want: "entry id is required"},
			{
				name: "Should require version", want: "version is required",
				trust: MarketplaceTrustEvidence{CatalogEntryID: "extension.acme.curated"},
			},
			{
				name: "Should require archive digest", want: "archive digest is required",
				trust: MarketplaceTrustEvidence{CatalogEntryID: "extension.acme.curated", Version: "1.0.0"},
			},
			{
				name: "Should require registry tier", want: "registry tier is required",
				trust: MarketplaceTrustEvidence{
					CatalogEntryID: "extension.acme.curated", Version: "1.0.0", ArchiveDigestSHA256: "digest",
				},
			},
			{
				name: "Should reject unsupported registry tier", want: `registry tier "partner" is unsupported`,
				trust: MarketplaceTrustEvidence{
					CatalogEntryID: "extension.acme.curated", Version: "1.0.0",
					ArchiveDigestSHA256: "digest", RegistryTier: "partner",
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if err := tt.trust.validate("acme/curated"); err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("MarketplaceTrustEvidence.validate() error = %v, want %q", err, tt.want)
				}
			})
		}
	})

	t.Run("Should preserve curated digest mismatch identity and diagnostics", func(t *testing.T) {
		t.Parallel()

		cause := &registrypkg.ArchiveDigestMismatchError{ExpectedSHA256: "expected", ActualSHA256: "actual"}
		err := wrapCuratedDigestMismatch(cause, &MarketplaceTrustEvidence{
			CatalogEntryID: " extension.acme.curated ", Version: " 1.0.0 ",
		})
		if !errors.Is(err, ErrExtensionArchiveDigestMismatch) || !errors.Is(err, registrypkg.ErrArchiveDigestMismatch) {
			t.Fatalf("wrapCuratedDigestMismatch() error = %v, want both digest mismatch identities", err)
		}
		var mismatch *ExtensionArchiveDigestMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("wrapCuratedDigestMismatch() error = %T, want *ExtensionArchiveDigestMismatchError", err)
		}
		if errors.Unwrap(mismatch) != cause {
			t.Fatalf(
				"errors.Unwrap(ExtensionArchiveDigestMismatchError) = %v, want original mismatch",
				errors.Unwrap(mismatch),
			)
		}
		if !strings.Contains(mismatch.Error(), `entry "extension.acme.curated" version "1.0.0"`) {
			t.Fatalf("ExtensionArchiveDigestMismatchError.Error() = %q", mismatch.Error())
		}
		if item := mismatch.DiagnosticItem(); item.Code != diagnosticcontract.CodeExtensionArchiveDigestMismatch {
			t.Fatalf("ExtensionArchiveDigestMismatchError.DiagnosticItem() = %#v", item)
		}
		var nilMismatch *ExtensionArchiveDigestMismatchError
		if !errors.Is(nilMismatch, ErrExtensionArchiveDigestMismatch) || errors.Is(nilMismatch, cause) {
			t.Fatalf("errors.Is(nil mismatch) did not preserve nil-safe sentinel matching")
		}
	})
}

func newLifecycleSource(t *testing.T, versions ...string) *lifecycleSource {
	t.Helper()
	return newLifecycleSourceNamed(t, "lifecycle-ext", "github", versions...)
}

func newLifecycleSourceNamed(
	t *testing.T,
	extensionName string,
	registryName string,
	versions ...string,
) *lifecycleSource {
	t.Helper()

	archives := make(map[string][]byte, len(versions))
	for _, version := range versions {
		archives[version] = lifecycleTarGzNamed(t, extensionName, version)
	}
	return &lifecycleSource{
		name:          registryName,
		extensionName: extensionName,
		slug:          "acme/" + extensionName,
		latestVersion: versions[len(versions)-1],
		archives:      archives,
	}
}

func lifecycleTarGzNamed(t *testing.T, name string, version string) []byte {
	t.Helper()

	files := map[string]string{
		filepath.Join(name, "extension.toml"): strings.Replace(
			registryManifestTOML(name, registryManifestOptions{}),
			`version = "0.2.1"`,
			fmt.Sprintf(`version = %q`, version),
			1,
		),
		filepath.Join(name, "VERSION.txt"): version + "\n",
	}

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("tarWriter.WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("tarWriter.Write(%q) error = %v", name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}
	return buffer.Bytes()
}

func lifecycleArchiveDigest(archive []byte) string {
	digest := sha256.Sum256(archive)
	return hex.EncodeToString(digest[:])
}

func requireFileContains(t *testing.T, path string, want string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("file %q = %q, want contains %q", path, string(content), want)
	}
}
