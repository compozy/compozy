package extensionpkg

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/extension/agentplugin"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/oklog/ulid/v2"
)

const legacyRegistryTestExtensionsTableSchema = `CREATE TABLE IF NOT EXISTS extensions (
	name          TEXT PRIMARY KEY,
	version       TEXT NOT NULL,
	source        TEXT NOT NULL,
	enabled       BOOLEAN NOT NULL DEFAULT 1,
	manifest_path TEXT NOT NULL,
	installed_at  TEXT NOT NULL,
	capabilities  TEXT NOT NULL DEFAULT '{}',
	permissions       TEXT NOT NULL DEFAULT '{}',
	checksum      TEXT NOT NULL
);`

func TestRegistryInstallPersistsExtension(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "alpha-registry", registryManifestOptions{
		capabilities: []string{"tool.provider", "view.provider"},
		permissions:  []string{"sessions/list", "observe/health"},
		extraFiles: map[string]string{
			"skills/alpha.md": "# alpha\n",
		},
	})

	if err := env.registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Name != manifest.Name {
		t.Fatalf("Name = %q, want %q", got.Name, manifest.Name)
	}
	if got.Version != manifest.Version {
		t.Fatalf("Version = %q, want %q", got.Version, manifest.Version)
	}
	if got.Source != SourceUser {
		t.Fatalf("Source = %v, want %v", got.Source, SourceUser)
	}
	if !got.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if got.ManifestPath != filepath.Join(dir, manifestTOMLFileName) {
		t.Fatalf("ManifestPath = %q, want %q", got.ManifestPath, filepath.Join(dir, manifestTOMLFileName))
	}
	if got.Format != FormatCompozy || len(got.IngestDiagnostics) != 0 {
		t.Fatalf(
			"native format state = %q/%#v, want compozy with no ingest diagnostics",
			got.Format,
			got.IngestDiagnostics,
		)
	}
	if !got.InstalledAt.Equal(env.installedAt) {
		t.Fatalf("InstalledAt = %v, want %v", got.InstalledAt, env.installedAt)
	}
	if !reflect.DeepEqual(got.Capabilities, normalizeCapabilitiesConfig(manifest.Capabilities)) {
		t.Fatalf("Capabilities = %#v, want %#v", got.Capabilities, normalizeCapabilitiesConfig(manifest.Capabilities))
	}
	if !reflect.DeepEqual(got.Permissions, normalizePermissionsConfig(manifest.Permissions)) {
		t.Fatalf("Permissions = %#v, want %#v", got.Permissions, normalizePermissionsConfig(manifest.Permissions))
	}
	if got.Checksum != checksum {
		t.Fatalf("Checksum = %q, want %q", got.Checksum, checksum)
	}
	if got.RegistrySlug != nil || got.RegistryName != nil || got.RemoteVersion != nil {
		t.Fatalf(
			"local install registry metadata = (%v, %v, %v), want all nil",
			got.RegistrySlug,
			got.RegistryName,
			got.RemoteVersion,
		)
	}
}

// Invariant: a transient global-store writer cannot make extension installation fail.
// Owner: extension registry persistence.
// Canonical suite: extension registry tests.
func TestRegistryInstallRetriesBusyPersistence(t *testing.T) {
	t.Parallel()

	t.Run("Should persist after a competing writer releases the database", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		dir, manifest, checksum := createRegistryTestExtension(
			t,
			"busy-persist-registry",
			registryManifestOptions{},
		)
		holdTransientRegistryWriteLock(t, env.db)
		if err := env.registry.Install(manifest, dir, checksum); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		if _, err := env.registry.Get(manifest.Name); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
	})
}

// Invariant: delayed cleanup from an older extension lifecycle cannot disable
// a replacement lifecycle for the same registry name.
// Owner: extension registry persistence.
// Canonical suite: extension registry tests.
func TestRegistryConditionalDisableFencesOlderLifecycle(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	t.Run("Should reject stale cleanup and accept the current lifecycle owner", func(t *testing.T) {
		env := newRegistryTestEnv(t)
		dir, manifest, checksum := createRegistryTestExtension(t, "lifecycle-fence", registryManifestOptions{})
		if err := env.registry.Install(manifest, dir, checksum); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		older, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(older) error = %v", err)
		}
		if older.lifecycleToken == "" {
			t.Fatal("older lifecycle token is empty")
		}
		if err := env.registry.Enable(manifest.Name); err != nil {
			t.Fatalf("Enable(replacement lifecycle) error = %v", err)
		}
		replacement, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(replacement) error = %v", err)
		}
		if replacement.lifecycleToken == older.lifecycleToken {
			t.Fatalf("replacement lifecycle token = %q, want a new owner token", replacement.lifecycleToken)
		}
		disabled, err := env.registry.DisableIfLifecycleToken(manifest.Name, older.lifecycleToken)
		if err != nil {
			t.Fatalf("DisableIfLifecycleToken(older) error = %v", err)
		}
		if disabled {
			t.Fatal("DisableIfLifecycleToken(older) disabled the replacement")
		}
		current, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(after stale disable) error = %v", err)
		}
		if !current.Enabled {
			t.Fatal("replacement Enabled = false after stale disable")
		}
		disabled, err = env.registry.DisableIfLifecycleToken(manifest.Name, replacement.lifecycleToken)
		if err != nil {
			t.Fatalf("DisableIfLifecycleToken(replacement) error = %v", err)
		}
		if !disabled {
			t.Fatal("DisableIfLifecycleToken(replacement) disabled = false, want true")
		}
	})
}

// Invariant: format and recorded ingestion diagnostics round-trip on the row
// that owns each global or workspace extension instance.
// Owner: extension registry persistence.
// Canonical suite: extension registry tests.
func TestRegistryPersistsAgentPluginInstanceMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should persist global format and component skips from the verified artifact", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		root := writeAgentPluginManifestFixture(t)
		writeFile(t, filepath.Join(root, "mcp.json"), fmt.Sprintf(`{
  "$schema": %q,
  "mcpServers": {
    "legacy": {"type":"sse","url":"https://example.com/events"}
  }
}`, agentplugin.MCPSchemaID))
		manifest, err := LoadManifest(root)
		if err != nil {
			t.Fatalf("LoadManifest() error = %v", err)
		}
		var sseSkips int
		for _, diagnostic := range manifest.IngestDiagnostics {
			if diagnostic.Evidence["scope"] == "mcp:legacy" &&
				strings.Contains(diagnostic.Message, "sse transport is not supported") {
				sseSkips++
			}
		}
		if sseSkips != 1 {
			t.Fatalf("LoadManifest().IngestDiagnostics = %#v, want one SSE skip", manifest.IngestDiagnostics)
		}
		checksum, err := ComputeDirectoryChecksum(root)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum() error = %v", err)
		}
		if err := env.registry.Install(manifest, root, checksum); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		got, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Format != FormatAgentPlugin || !reflect.DeepEqual(got.IngestDiagnostics, manifest.IngestDiagnostics) {
			t.Fatalf(
				"persisted metadata = %q/%#v, want %#v",
				got.Format,
				got.IngestDiagnostics,
				manifest.IngestDiagnostics,
			)
		}
	})

	t.Run("Should replace dev diagnostics atomically with the generation", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		firstDiagnostics := AgentPluginDiagnostics("portable", []agentplugin.Diagnostic{{
			Scope: "skill:first", Message: "description is required",
		}})
		first, err := env.registry.LinkDev(DevLinkRequest{
			Name: "portable", WorkspaceID: "workspace-a", OriginPath: "/tmp/portable",
			GenerationHash: strings.Repeat("a", 64), Format: FormatAgentPlugin,
			IngestDiagnostics: firstDiagnostics,
		})
		if err != nil {
			t.Fatalf("LinkDev(first) error = %v", err)
		}
		if first.Format != FormatAgentPlugin || !reflect.DeepEqual(first.IngestDiagnostics, firstDiagnostics) {
			t.Fatalf("LinkDev(first) = %#v, want portable metadata", first)
		}

		secondDiagnostics := AgentPluginDiagnostics("portable", []agentplugin.Diagnostic{{
			Scope: "mcp:second", Message: "invalid mcp server entry",
		}})
		second, err := env.registry.LinkDev(DevLinkRequest{
			Name: "portable", WorkspaceID: "workspace-a", OriginPath: "/tmp/portable",
			GenerationHash: strings.Repeat("b", 64), Format: FormatAgentPlugin,
			IngestDiagnostics: secondDiagnostics,
		})
		if err != nil {
			t.Fatalf("LinkDev(second) error = %v", err)
		}
		if second.BundleGeneration != strings.Repeat("b", 64) ||
			!reflect.DeepEqual(second.IngestDiagnostics, secondDiagnostics) {
			t.Fatalf("LinkDev(second) = %#v, want generation and replacement diagnostics", second)
		}
	})
}

func TestRegistryInstallRejectsDuplicateName(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "duplicate-registry", registryManifestOptions{})

	if err := env.registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install(first) error = %v", err)
	}

	err := env.registry.Install(manifest, dir, checksum)
	if err == nil {
		t.Fatal("Install(duplicate) error = nil, want ErrExtensionExists")
	}
	if !errors.Is(err, ErrExtensionExists) {
		t.Fatalf("Install(duplicate) error = %v, want ErrExtensionExists", err)
	}

	existsErr, existsErrMatched := errors.AsType[*ExtensionExistsError](err)
	if !existsErrMatched {
		t.Fatalf("Install(duplicate) error type = %T, want *ExtensionExistsError", err)
	}
	if existsErr.Name != manifest.Name {
		t.Fatalf("ExtensionExistsError.Name = %q, want %q", existsErr.Name, manifest.Name)
	}
}

func TestRegistryInstallPersistsMarketplaceMetadata(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "marketplace-registry", registryManifestOptions{})

	if err := env.registry.Install(
		manifest,
		dir,
		checksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata("acme/marketplace-registry", "github", "v1.4.0"),
	); err != nil {
		t.Fatalf("Install(marketplace) error = %v", err)
	}

	got, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Source != SourceMarketplace {
		t.Fatalf("Source = %v, want %v", got.Source, SourceMarketplace)
	}
	if got.RegistrySlug == nil || *got.RegistrySlug != "acme/marketplace-registry" {
		t.Fatalf("RegistrySlug = %#v, want acme/marketplace-registry", got.RegistrySlug)
	}
	if got.RegistryName == nil || *got.RegistryName != "github" {
		t.Fatalf("RegistryName = %#v, want github", got.RegistryName)
	}
	if got.RemoteVersion == nil || *got.RemoteVersion != "v1.4.0" {
		t.Fatalf("RemoteVersion = %#v, want v1.4.0", got.RemoteVersion)
	}
}

func TestRegistryInstallRejectsChecksumMismatch(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, _ := createRegistryTestExtension(t, "checksum-registry", registryManifestOptions{
		extraFiles: map[string]string{
			"bin/extension": "#!/bin/sh\n",
		},
	})

	err := env.registry.Install(manifest, dir, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("Install(checksum mismatch) error = nil, want ErrExtensionChecksumMismatch")
	}
	if !errors.Is(err, ErrExtensionChecksumMismatch) {
		t.Fatalf("Install(checksum mismatch) error = %v, want ErrExtensionChecksumMismatch", err)
	}

	mismatchErr, mismatchErrMatched := errors.AsType[*ExtensionChecksumMismatchError](err)
	if !mismatchErrMatched {
		t.Fatalf("Install(checksum mismatch) error type = %T, want *ExtensionChecksumMismatchError", err)
	}
	if mismatchErr.ExpectedChecksum != strings.Repeat("0", 64) {
		t.Fatalf("ExpectedChecksum = %q, want %q", mismatchErr.ExpectedChecksum, strings.Repeat("0", 64))
	}

	got, err := env.registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(List()) = %d, want 0", len(got))
	}
}

func TestRegistryGetReturnsNotFound(t *testing.T) {
	env := newRegistryTestEnv(t)

	_, err := env.registry.Get("missing")
	if err == nil {
		t.Fatal("Get(missing) error = nil, want ErrExtensionNotFound")
	}
	if !errors.Is(err, ErrExtensionNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrExtensionNotFound", err)
	}

	notFoundErr, notFoundErrMatched := errors.AsType[*ExtensionNotFoundError](err)
	if !notFoundErrMatched {
		t.Fatalf("Get(missing) error type = %T, want *ExtensionNotFoundError", err)
	}
	if notFoundErr.Name != "missing" {
		t.Fatalf("ExtensionNotFoundError.Name = %q, want %q", notFoundErr.Name, "missing")
	}
}

func TestRegistryListReturnsAllInstalledExtensions(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	alphaDir, alphaManifest, alphaChecksum := createRegistryTestExtension(t, "alpha", registryManifestOptions{
		capabilities: []string{"tool.provider"},
		permissions:  []string{"sessions/list"},
	})
	betaDir, betaManifest, betaChecksum := createRegistryTestExtension(t, "beta", registryManifestOptions{
		capabilities: []string{"tool.provider"},
		permissions:  []string{"observe/health"},
	})

	if err := env.registry.Install(alphaManifest, alphaDir, alphaChecksum); err != nil {
		t.Fatalf("Install(alpha) error = %v", err)
	}
	if err := env.registry.Install(
		betaManifest,
		betaDir,
		betaChecksum,
		WithInstallSource(SourceMarketplace),
	); err != nil {
		t.Fatalf("Install(beta) error = %v", err)
	}

	got, err := env.registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(got))
	}

	names := []string{got[0].Name, got[1].Name}
	if !slices.Equal(names, []string{"alpha", "beta"}) {
		t.Fatalf("names = %v, want %v", names, []string{"alpha", "beta"})
	}
	if got[0].Source != SourceUser {
		t.Fatalf("alpha source = %v, want %v", got[0].Source, SourceUser)
	}
	if got[1].Source != SourceMarketplace {
		t.Fatalf("beta source = %v, want %v", got[1].Source, SourceMarketplace)
	}
}

func TestRegistryListReturnsEmptySlice(t *testing.T) {
	env := newRegistryTestEnv(t)

	got, err := env.registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got == nil {
		t.Fatal("List() = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("len(List()) = %d, want 0", len(got))
	}
}

func TestRegistryEnabledBundledNamesCache(t *testing.T) {
	// withDaemonVersion mutates the package-level build version, so this suite is intentionally serial.
	withDaemonVersion(t, "0.6.0")

	t.Run("Should return cloned snapshots and invalidate every registry mutation", func(t *testing.T) {
		env := newRegistryTestEnv(t)
		bundledDir, bundledManifest, bundledChecksum := createRegistryTestExtension(
			t,
			"bundled-cache",
			registryManifestOptions{},
		)
		userDir, userManifest, userChecksum := createRegistryTestExtension(
			t,
			"user-cache",
			registryManifestOptions{},
		)
		if err := env.registry.Install(
			bundledManifest,
			bundledDir,
			bundledChecksum,
			WithInstallSource(SourceBundled),
		); err != nil {
			t.Fatalf("Install(bundled) error = %v", err)
		}
		if err := env.registry.Install(userManifest, userDir, userChecksum); err != nil {
			t.Fatalf("Install(user) error = %v", err)
		}

		names, err := env.registry.EnabledBundledNames()
		if err != nil {
			t.Fatalf("EnabledBundledNames() error = %v", err)
		}
		if !slices.Equal(names, []string{bundledManifest.Name}) {
			t.Fatalf("EnabledBundledNames() = %#v, want bundled name", names)
		}
		names[0] = "mutated-by-caller"
		cached, err := env.registry.EnabledBundledNames()
		if err != nil {
			t.Fatalf("EnabledBundledNames(cached) error = %v", err)
		}
		if !slices.Equal(cached, []string{bundledManifest.Name}) {
			t.Fatalf("cached bundled names = %#v, want immutable snapshot", cached)
		}

		if err := env.registry.Disable(bundledManifest.Name); err != nil {
			t.Fatalf("Disable(bundled) error = %v", err)
		}
		disabled, err := env.registry.EnabledBundledNames()
		if err != nil {
			t.Fatalf("EnabledBundledNames(disabled) error = %v", err)
		}
		if len(disabled) != 0 {
			t.Fatalf("disabled bundled names = %#v, want empty", disabled)
		}
		if err := env.registry.Enable(bundledManifest.Name); err != nil {
			t.Fatalf("Enable(bundled) error = %v", err)
		}
		enabled, err := env.registry.EnabledBundledNames()
		if err != nil {
			t.Fatalf("EnabledBundledNames(enabled) error = %v", err)
		}
		if !slices.Equal(enabled, []string{bundledManifest.Name}) {
			t.Fatalf("enabled bundled names = %#v, want bundled name", enabled)
		}
		if err := env.registry.Uninstall(bundledManifest.Name); err != nil {
			t.Fatalf("Uninstall(bundled) error = %v", err)
		}
		uninstalled, err := env.registry.EnabledBundledNames()
		if err != nil {
			t.Fatalf("EnabledBundledNames(uninstalled) error = %v", err)
		}
		if len(uninstalled) != 0 {
			t.Fatalf("uninstalled bundled names = %#v, want empty", uninstalled)
		}
	})
}

// Invariant: EnabledBundledNames, the source of bundled tool trust, excludes a tombstoned built-in
// even when a bundled row still exists, and the cache follows tombstone writes and clears.
// Owner: extension bundled enrollment and tool trust.
// Canonical suite: registry bundled cache tests in registry_test.go.
func TestRegistryEnabledBundledNamesExcludesTombstonedBuiltin(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	t.Run("Should exclude a tombstoned name that has no row", func(t *testing.T) {
		env := newRegistryTestEnv(t)
		ctx := testutil.Context(t)
		if _, err := env.registry.Tombstone(ctx, "spec-cycle", "operator"); err != nil {
			t.Fatalf("Tombstone() error = %v", err)
		}

		names, err := env.registry.EnabledBundledNames()
		if err != nil {
			t.Fatalf("EnabledBundledNames() error = %v", err)
		}
		if len(names) != 0 {
			t.Fatalf("EnabledBundledNames() = %#v, want no tombstoned name", names)
		}
	})

	t.Run(
		"Should drop a tombstoned bundled row from the cache and restore it once the tombstone clears",
		func(t *testing.T) {
			env := newRegistryTestEnv(t)
			ctx := testutil.Context(t)
			bundledDir, bundledManifest, bundledChecksum := createRegistryTestExtension(
				t,
				"spec-cycle",
				registryManifestOptions{},
			)
			if err := env.registry.Install(
				bundledManifest,
				bundledDir,
				bundledChecksum,
				WithInstallSource(SourceBundled),
			); err != nil {
				t.Fatalf("Install(bundled) error = %v", err)
			}

			before, err := env.registry.EnabledBundledNames()
			if err != nil {
				t.Fatalf("EnabledBundledNames(before) error = %v", err)
			}
			if !slices.Equal(before, []string{"spec-cycle"}) {
				t.Fatalf("EnabledBundledNames(before) = %#v, want spec-cycle", before)
			}

			if _, err := env.registry.Tombstone(ctx, "spec-cycle", "operator"); err != nil {
				t.Fatalf("Tombstone() error = %v", err)
			}
			afterTombstone, err := env.registry.EnabledBundledNames()
			if err != nil {
				t.Fatalf("EnabledBundledNames(tombstoned) error = %v", err)
			}
			if len(afterTombstone) != 0 {
				t.Fatalf("EnabledBundledNames(tombstoned) = %#v, want the cache to drop spec-cycle", afterTombstone)
			}

			cleared, err := env.registry.Clear(ctx, "spec-cycle")
			if err != nil || !cleared {
				t.Fatalf("Clear() = %v, %v; want true, nil", cleared, err)
			}
			restored, err := env.registry.EnabledBundledNames()
			if err != nil {
				t.Fatalf("EnabledBundledNames(cleared) error = %v", err)
			}
			if !slices.Equal(restored, []string{"spec-cycle"}) {
				t.Fatalf("EnabledBundledNames(cleared) = %#v, want spec-cycle", restored)
			}
		},
	)
}

func TestRegistryEnableAndDisable(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "toggle-registry", registryManifestOptions{})

	if err := env.registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if err := env.registry.Disable(manifest.Name); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	disabled, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get(disabled) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatal("Enabled after Disable() = true, want false")
	}

	if err := env.registry.Enable(manifest.Name); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	enabled, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get(enabled) error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("Enabled after Enable() = false, want true")
	}
}

// Invariant: a transient global-store writer cannot make extension enablement fail.
// Owner: extension registry enablement persistence.
// Canonical suite: extension registry tests.
func TestRegistrySetEnabledForProfileRetriesBusyPersistence(t *testing.T) {
	t.Parallel()

	t.Run("Should disable after a competing writer releases the database", func(t *testing.T) {
		t.Parallel()

		env := newRegistryTestEnv(t)
		dir, manifest, checksum := createRegistryTestExtension(
			t,
			"busy-enablement-registry",
			registryManifestOptions{},
		)
		if err := env.registry.Install(manifest, dir, checksum); err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		holdTransientRegistryWriteLock(t, env.db)
		if err := env.registry.Disable(manifest.Name); err != nil {
			t.Fatalf("Disable() error = %v", err)
		}
		enabled, err := env.registry.IsEnabledForProfile(manifest.Name, store.DefaultProfileID)
		if err != nil {
			t.Fatalf("IsEnabledForProfile() error = %v", err)
		}
		if enabled {
			t.Fatal("IsEnabledForProfile() = true, want false")
		}
	})
}

// Invariant: extension enablement is an exception per profile; absence means
// enabled and a disable never changes another profile.
// Owner: extension registry enablement persistence.
// Canonical suite: extension registry tests.
func TestRegistryProfileEnablementUsesDisabledExceptions(t *testing.T) {
	t.Parallel()
	t.Run("Should keep profile enablement exceptions isolated", func(t *testing.T) {
		t.Parallel()
		testRegistryProfileEnablementUsesDisabledExceptions(t)
	})
}

// Invariant: profile placement catalogs include both published manifests and
// workspace development links because profile rename planning must see every
// owned declaration.
// Owner: extension registry placement catalog.
// Canonical suite: extension registry tests.
func TestRegistryPlacementsForProfileIncludesDevelopmentLinks(t *testing.T) {
	t.Parallel()
	t.Run("Should include profile-bound resources from a development link", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		dir := t.TempDir()
		const extensionName = "dev-placement"
		writeFile(t, filepath.Join(dir, manifestTOMLFileName), `[extension]
name = "dev-placement"
version = "0.2.1"
min_compozy_version = "0.5.0"

[capabilities]
provides = ["tool.provider"]

[permissions]
requires = ["sessions/list"]

[[resources.skills]]
path = "skills/review"
profile = "finance"
`)
		if _, err := env.registry.LinkDev(DevLinkRequest{
			Name: extensionName, WorkspaceID: "workspace-a", OriginPath: dir,
			GenerationHash: strings.Repeat("d", 64),
		}); err != nil {
			t.Fatalf("LinkDev() error = %v", err)
		}
		placements, err := env.registry.PlacementsForProfile(testutil.Context(t), "finance")
		if err != nil {
			t.Fatalf("PlacementsForProfile() error = %v", err)
		}
		want := []profilepkg.PlacementRef{{Extension: extensionName, Resource: "skills/review", ProfileName: "finance"}}
		if !slices.Equal(placements, want) {
			t.Fatalf("PlacementsForProfile() = %#v, want %#v", placements, want)
		}
	})
}

func testRegistryProfileEnablementUsesDisabledExceptions(t *testing.T) {
	t.Helper()
	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "profile-toggle", registryManifestOptions{})
	if err := env.registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	financeID := insertActiveRegistryProfile(t, env, "finance")

	initial, err := env.registry.IsEnabledForProfile(manifest.Name, financeID)
	if err != nil || !initial {
		t.Fatalf("IsEnabledForProfile(initial) = %t, %v, want true, nil", initial, err)
	}
	if err := env.registry.SetEnabledForProfile(manifest.Name, financeID, false); err != nil {
		t.Fatalf("SetEnabledForProfile(disable) error = %v", err)
	}
	disabled, err := env.registry.IsEnabledForProfile(manifest.Name, financeID)
	if err != nil || disabled {
		t.Fatalf("IsEnabledForProfile(disabled) = %t, %v, want false, nil", disabled, err)
	}
	defaultEnabled, err := env.registry.IsEnabledForProfile(manifest.Name, store.DefaultProfileID)
	if err != nil || !defaultEnabled {
		t.Fatalf("IsEnabledForProfile(default) = %t, %v, want true, nil", defaultEnabled, err)
	}
	if err := env.registry.SetEnabledForProfile(manifest.Name, financeID, true); err != nil {
		t.Fatalf("SetEnabledForProfile(enable) error = %v", err)
	}
	var exceptions int
	if err := env.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM extension_profile_enablement
		WHERE extension_name = ? AND profile_id = ?`, manifest.Name, financeID).Scan(&exceptions); err != nil {
		t.Fatalf("count enablement exceptions: %v", err)
	}
	if exceptions != 0 {
		t.Fatalf("enablement exception rows = %d, want 0", exceptions)
	}
}

// Invariant: dev-only links share the durable per-profile enablement authority,
// and an exception survives while either a published install or any workspace
// link still owns that extension name.
// Owner: extension registry enablement persistence.
// Canonical suite: extension registry tests.
func TestRegistryDevLinksComposeWithProfileEnablement(t *testing.T) {
	t.Parallel()

	t.Run("Should persist a dev-only disable until the last workspace link is removed [IT-055]", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		const name = "dev-profile-toggle"
		for index, workspaceID := range []string{"workspace-a", "workspace-b"} {
			if _, err := env.registry.LinkDev(DevLinkRequest{
				Name: name, WorkspaceID: workspaceID, OriginPath: "/tmp/dev-profile-toggle",
				GenerationHash: strings.Repeat(string(rune('a'+index)), 64),
			}); err != nil {
				t.Fatalf("LinkDev(%s) error = %v", workspaceID, err)
			}
		}
		if err := env.registry.SetEnabledForProfile(name, store.DefaultProfileID, false); err != nil {
			t.Fatalf("SetEnabledForProfile(dev-only disabled) error = %v", err)
		}
		enabled, err := env.registry.IsEnabledForProfile(name, store.DefaultProfileID)
		if err != nil || enabled {
			t.Fatalf("IsEnabledForProfile(dev-only disabled) = %t, %v, want false, nil", enabled, err)
		}
		if err := env.registry.UnlinkDev(name, "workspace-a"); err != nil {
			t.Fatalf("UnlinkDev(workspace-a) error = %v", err)
		}
		enabled, err = env.registry.IsEnabledForProfile(name, store.DefaultProfileID)
		if err != nil || enabled {
			t.Fatalf("IsEnabledForProfile(remaining link) = %t, %v, want false, nil", enabled, err)
		}
		if err := env.registry.UnlinkDev(name, "workspace-b"); err != nil {
			t.Fatalf("UnlinkDev(workspace-b) error = %v", err)
		}
		if _, err := env.registry.IsEnabledForProfile(
			name,
			store.DefaultProfileID,
		); !errors.Is(
			err,
			ErrExtensionNotFound,
		) {
			t.Fatalf("IsEnabledForProfile(unlinked) error = %v, want ErrExtensionNotFound", err)
		}
		var exceptions int
		if err := env.db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM extension_profile_enablement WHERE extension_name = ?`, name,
		).Scan(&exceptions); err != nil {
			t.Fatalf("count dev-only exceptions error = %v", err)
		}
		if exceptions != 0 {
			t.Fatalf("dev-only exception rows = %d, want 0", exceptions)
		}
	})

	t.Run("Should retain an exception when uninstall leaves a development link", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		dir, manifest, checksum := createRegistryTestExtension(t, "published-and-dev", registryManifestOptions{})
		if err := env.registry.Install(manifest, dir, checksum); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		if _, err := env.registry.LinkDev(DevLinkRequest{
			Name: manifest.Name, WorkspaceID: "workspace-a", OriginPath: dir,
			GenerationHash: strings.Repeat("c", 64),
		}); err != nil {
			t.Fatalf("LinkDev() error = %v", err)
		}
		if err := env.registry.SetEnabledForProfile(manifest.Name, store.DefaultProfileID, false); err != nil {
			t.Fatalf("SetEnabledForProfile(disabled) error = %v", err)
		}
		if err := env.registry.Uninstall(manifest.Name); err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}
		enabled, err := env.registry.IsEnabledForProfile(manifest.Name, store.DefaultProfileID)
		if err != nil || enabled {
			t.Fatalf("IsEnabledForProfile(dev remains) = %t, %v, want false, nil", enabled, err)
		}
	})
}

// Invariant: runtime eligibility is the union of active profile enablement;
// disabling one profile cannot stop resources still enabled for another.
// Owner: extension registry runtime eligibility projection.
// Canonical suite: extension registry tests.
func TestRegistryRuntimeEligibilityUsesEveryActiveProfile(t *testing.T) {
	t.Parallel()
	t.Run("Should keep runtime eligible while any active profile remains enabled", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		dir, manifest, checksum := createRegistryTestExtension(t, "runtime-eligibility", registryManifestOptions{})
		if err := env.registry.Install(manifest, dir, checksum); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		financeID := insertActiveRegistryProfile(t, env, "finance")

		if err := env.registry.SetEnabledForProfile(manifest.Name, store.DefaultProfileID, false); err != nil {
			t.Fatalf("SetEnabledForProfile(default disabled) error = %v", err)
		}
		eligible, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(with finance enabled) error = %v", err)
		}
		if !eligible.Enabled {
			t.Fatal("runtime eligible with finance enabled = false, want true")
		}

		if err := env.registry.SetEnabledForProfile(manifest.Name, financeID, false); err != nil {
			t.Fatalf("SetEnabledForProfile(finance disabled) error = %v", err)
		}
		ineligible, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(all active profiles disabled) error = %v", err)
		}
		if ineligible.Enabled {
			t.Fatal("runtime eligible with every active profile disabled = true, want false")
		}

		if _, err := env.db.ExecContext(
			t.Context(),
			`UPDATE profiles SET state = 'archived', archived_at = ? WHERE id = ?`,
			store.FormatTimestamp(env.installedAt.Add(time.Minute)), financeID); err != nil {
			t.Fatalf("archive finance profile: %v", err)
		}
		if err := env.registry.SetEnabledForProfile(manifest.Name, store.DefaultProfileID, true); err != nil {
			t.Fatalf("SetEnabledForProfile(default enabled) error = %v", err)
		}
		eligible, err = env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(archived finance ignored) error = %v", err)
		}
		if !eligible.Enabled {
			t.Fatal("runtime eligible with default enabled and finance archived = false, want true")
		}
	})
}

func TestRegistryUninstallRemovesExtension(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "remove-registry", registryManifestOptions{})

	if err := env.registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if err := env.registry.Uninstall(manifest.Name); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	_, err := env.registry.Get(manifest.Name)
	if !errors.Is(err, ErrExtensionNotFound) {
		t.Fatalf("Get(after uninstall) error = %v, want ErrExtensionNotFound", err)
	}
}

func TestRegistryUninstallMissingReturnsNotFound(t *testing.T) {
	env := newRegistryTestEnv(t)

	err := env.registry.Uninstall("missing")
	if err == nil {
		t.Fatal("Uninstall(missing) error = nil, want ErrExtensionNotFound")
	}
	if !errors.Is(err, ErrExtensionNotFound) {
		t.Fatalf("Uninstall(missing) error = %v, want ErrExtensionNotFound", err)
	}
}

// TestRegistryBuiltinTombstoneRoundTrip covers UT-005. Invariant: a tombstone stores the trimmed name,
// lists in name order with removed_at and removed_by, and rejects blank names and blank actors.
func TestRegistryBuiltinTombstoneRoundTrip(t *testing.T) {
	removedAt := time.Date(2026, 11, 17, 14, 2, 11, 0, time.UTC)
	newTombstoneEnv := func(t *testing.T) registryTestEnv {
		t.Helper()
		env := newRegistryTestEnv(t)
		env.registry.now = func() time.Time { return removedAt }
		return env
	}

	t.Run("Should store the trimmed name, return the record, and refuse a second tombstone", func(t *testing.T) {
		env := newTombstoneEnv(t)
		ctx := testutil.Context(t)

		got, err := env.registry.Tombstone(ctx, "  spec-cycle  ", "operator")
		if err != nil {
			t.Fatalf("Tombstone() error = %v", err)
		}
		if got.Name != "spec-cycle" || got.RemovedBy != "operator" || !got.RemovedAt.Equal(removedAt) {
			t.Fatalf("Tombstone() = %#v, want name spec-cycle, removed_by operator, removed_at %s", got, removedAt)
		}
		tombstoned, err := env.registry.Tombstoned(ctx, "spec-cycle")
		if err != nil || !tombstoned {
			t.Fatalf("Tombstoned(trimmed) = %v, %v; want true, nil", tombstoned, err)
		}
		if _, err := env.registry.Tombstone(ctx, "spec-cycle", "operator"); err == nil {
			t.Fatal("second Tombstone() error = nil, want a duplicate-key refusal")
		}
	})

	t.Run("Should list tombstones in name order with removed_at and removed_by", func(t *testing.T) {
		env := newTombstoneEnv(t)
		ctx := testutil.Context(t)
		for _, name := range []string{"spec-cycle", "open-design"} {
			if _, err := env.registry.Tombstone(ctx, name, "operator"); err != nil {
				t.Fatalf("Tombstone(%q) error = %v", name, err)
			}
		}

		got, err := env.registry.ListTombstones(ctx)
		if err != nil {
			t.Fatalf("ListTombstones() error = %v", err)
		}
		want := []BuiltinTombstone{
			{Name: "open-design", RemovedAt: removedAt, RemovedBy: "operator"},
			{Name: "spec-cycle", RemovedAt: removedAt, RemovedBy: "operator"},
		}
		if len(got) != len(want) {
			t.Fatalf("ListTombstones() = %#v, want %#v", got, want)
		}
		for i := range want {
			if got[i].Name != want[i].Name || got[i].RemovedBy != want[i].RemovedBy ||
				!got[i].RemovedAt.Equal(want[i].RemovedAt) {
				t.Fatalf("ListTombstones()[%d] = %#v, want %#v", i, got[i], want[i])
			}
		}
	})

	t.Run("Should reject blank names and a blank actor without writing", func(t *testing.T) {
		env := newTombstoneEnv(t)
		ctx := testutil.Context(t)

		for _, name := range []string{"", "   "} {
			if _, err := env.registry.Tombstone(ctx, name, "operator"); err == nil {
				t.Fatalf("Tombstone(%q) error = nil, want rejection", name)
			}
			if _, err := env.registry.Tombstoned(ctx, name); err == nil {
				t.Fatalf("Tombstoned(%q) error = nil, want rejection", name)
			}
		}
		if _, err := env.registry.Tombstone(ctx, "spec-cycle", "  "); err == nil {
			t.Fatal("Tombstone(blank removed_by) error = nil, want rejection")
		}
		got, err := env.registry.ListTombstones(ctx)
		if err != nil || len(got) != 0 {
			t.Fatalf("ListTombstones() after rejected writes = %#v, %v; want empty", got, err)
		}
	})

	t.Run("Should clear a tombstone once and report whether it existed", func(t *testing.T) {
		env := newTombstoneEnv(t)
		ctx := testutil.Context(t)
		if _, err := env.registry.Tombstone(ctx, "spec-cycle", "operator"); err != nil {
			t.Fatalf("Tombstone() error = %v", err)
		}

		cleared, err := env.registry.Clear(ctx, " spec-cycle ")
		if err != nil || !cleared {
			t.Fatalf("Clear(existing) = %v, %v; want true, nil", cleared, err)
		}
		cleared, err = env.registry.Clear(ctx, "spec-cycle")
		if err != nil || cleared {
			t.Fatalf("Clear(missing) = %v, %v; want false, nil", cleared, err)
		}
		tombstoned, err := env.registry.Tombstoned(ctx, "spec-cycle")
		if err != nil || tombstoned {
			t.Fatalf("Tombstoned(after clear) = %v, %v; want false, nil", tombstoned, err)
		}
	})
}

func TestRegistryInstallConcurrentDuplicateReturnsSingleExistsError(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "concurrent-registry", registryManifestOptions{})

	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			errs <- env.registry.Install(manifest, dir, checksum)
		}()
	}
	close(start)

	var (
		successes   int
		existsCount int
	)
	for range 2 {
		err := <-errs
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrExtensionExists):
			existsCount++
		default:
			t.Fatalf("concurrent Install() error = %v, want nil or ErrExtensionExists", err)
		}
	}

	if successes != 1 || existsCount != 1 {
		t.Fatalf("concurrent Install() outcomes = successes:%d exists:%d, want 1 and 1", successes, existsCount)
	}
}

func TestRegistryInstallReplaceExistingUpdatesMarketplaceRecord(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "replace-registry", registryManifestOptions{})

	if err := env.registry.Install(
		manifest,
		dir,
		checksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata("acme/replace-registry", "github", "1.0.0"),
	); err != nil {
		t.Fatalf("Install(first) error = %v", err)
	}

	writeFile(
		t,
		filepath.Join(dir, manifestTOMLFileName),
		strings.Replace(
			registryManifestTOML("replace-registry", registryManifestOptions{}),
			`version = "0.2.1"`,
			`version = "0.3.0"`,
			1,
		),
	)
	updatedManifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(updated) error = %v", err)
	}
	updatedChecksum, err := ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(updated) error = %v", err)
	}

	if err := env.registry.Install(
		updatedManifest,
		dir,
		updatedChecksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata("acme/replace-registry", "github", "1.1.0"),
		WithInstallReplaceExisting(),
	); err != nil {
		t.Fatalf("Install(replace) error = %v", err)
	}

	got, err := env.registry.Get("replace-registry")
	if err != nil {
		t.Fatalf("Get(updated) error = %v", err)
	}
	if got.Version != "0.3.0" {
		t.Fatalf("Version = %q, want %q", got.Version, "0.3.0")
	}
	if got.RemoteVersion == nil || *got.RemoteVersion != "1.1.0" {
		t.Fatalf("RemoteVersion = %#v, want 1.1.0", got.RemoteVersion)
	}
}

func TestRegistryInstallReplaceExistingPreservesEnabledState(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "disabled-replace-registry", registryManifestOptions{})

	if err := env.registry.Install(
		manifest,
		dir,
		checksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata("acme/disabled-replace-registry", "github", "1.0.0"),
	); err != nil {
		t.Fatalf("Install(first) error = %v", err)
	}
	if err := env.registry.Disable(manifest.Name); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	writeFile(
		t,
		filepath.Join(dir, manifestTOMLFileName),
		strings.Replace(
			registryManifestTOML("disabled-replace-registry", registryManifestOptions{}),
			`version = "0.2.1"`,
			`version = "0.3.0"`,
			1,
		),
	)
	updatedManifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(updated) error = %v", err)
	}
	updatedChecksum, err := ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(updated) error = %v", err)
	}

	if err := env.registry.Install(
		updatedManifest,
		dir,
		updatedChecksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata("acme/disabled-replace-registry", "github", "1.1.0"),
		WithInstallReplaceExisting(),
	); err != nil {
		t.Fatalf("Install(replace) error = %v", err)
	}

	got, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get(updated) error = %v", err)
	}
	if got.Enabled {
		t.Fatal("Enabled = true after replace, want disabled state preserved")
	}
	if got.Version != "0.3.0" {
		t.Fatalf("Version = %q, want %q", got.Version, "0.3.0")
	}
}

func TestRegistryInstallReplaceExistingWrapsPersistErrors(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	dbPath := filepath.Join(t.TempDir(), "compozy-registry-legacy.db")
	db, err := store.OpenSQLiteDatabase(testutil.Context(t), dbPath, func(ctx context.Context, db *sql.DB) error {
		_, execErr := db.ExecContext(ctx, legacyRegistryTestExtensionsTableSchema)
		return execErr
	})
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	registry := NewRegistry(db)
	dir, manifest, checksum := createRegistryTestExtension(t, "legacy-persist-registry", registryManifestOptions{})

	err = registry.Install(manifest, dir, checksum, WithInstallReplaceExisting())
	if err == nil {
		t.Fatal("Install(replace existing on legacy schema) error = nil, want failure")
	}
	if !strings.Contains(err.Error(), `extension: persist "legacy-persist-registry"`) {
		t.Fatalf("Install(replace existing on legacy schema) error = %v, want wrapped persist context", err)
	}
}

func TestRegistryInstallClearsRemoteMetadataForNonMarketplaceSources(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "non-marketplace-registry", registryManifestOptions{})

	if err := env.registry.Install(
		manifest,
		dir,
		checksum,
		WithInstallRegistryMetadata("acme/non-marketplace", "github", "1.0.0"),
	); err != nil {
		t.Fatalf("Install(non-marketplace metadata) error = %v", err)
	}

	got, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.RegistrySlug != nil || got.RegistryName != nil || got.RemoteVersion != nil {
		t.Fatalf(
			"non-marketplace registry metadata = (%v, %v, %v), want all nil",
			got.RegistrySlug,
			got.RegistryName,
			got.RemoteVersion,
		)
	}
}

func TestRegistryInstallAcceptsManifestFilePathAndExplicitSource(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "file-path-install", registryManifestOptions{})
	manifestPath := filepath.Join(dir, manifestTOMLFileName)

	if err := env.registry.Install(manifest, manifestPath, checksum, WithInstallSource(SourceBundled)); err != nil {
		t.Fatalf("Install(file manifest) error = %v", err)
	}

	got, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Source != SourceBundled {
		t.Fatalf("Source = %v, want %v", got.Source, SourceBundled)
	}
	if got.ManifestPath != manifestPath {
		t.Fatalf("ManifestPath = %q, want %q", got.ManifestPath, manifestPath)
	}
}

func TestRegistryInstallRejectsInvalidSourceAndBlankChecksum(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "invalid-source", registryManifestOptions{})

	if err := env.registry.Install(manifest, dir, checksum, WithInstallSource(ExtensionSource(99))); err == nil {
		t.Fatal("Install(invalid source) error = nil, want non-nil")
	}
	if err := env.registry.Install(manifest, dir, ""); err == nil {
		t.Fatal("Install(blank checksum) error = nil, want non-nil")
	}
}

func TestRegistryInstallRejectsOnDiskManifestIdentityMismatch(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "identity-registry", registryManifestOptions{})

	spoofed := *manifest
	spoofed.Name = "spoofed-registry"

	err := env.registry.Install(&spoofed, dir, checksum, WithInstallSource(SourceUser))
	if err == nil {
		t.Fatal("Install(mismatched manifest) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "does not match provided identity") {
		t.Fatalf("Install(mismatched manifest) error = %v, want identity mismatch detail", err)
	}
}

func TestRegistryUtilityHelpers(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	t.Run("Should capability denied error formatting", func(t *testing.T) {
		var nilDenied *ErrCapabilityDenied
		if got := nilDenied.Error(); got != "" {
			t.Fatalf("nil ErrCapabilityDenied.Error() = %q, want empty", got)
		}

		denied := &ErrCapabilityDenied{}
		if got := denied.Error(); got != "capability_denied" {
			t.Fatalf("ErrCapabilityDenied{}.Error() = %q, want %q", got, "capability_denied")
		}

		denied = &ErrCapabilityDenied{Data: CapabilityDeniedData{Method: "sessions/list"}}
		if got := denied.Error(); got != "capability_denied: sessions/list" {
			t.Fatalf("ErrCapabilityDenied(method).Error() = %q", got)
		}

		denied = &ErrCapabilityDenied{
			Data: CapabilityDeniedData{
				Method:   "sessions/list",
				Required: []string{"session.read"},
				Granted:  []string{"observe.read"},
			},
		}
		if got := denied.Error(); !strings.Contains(got, "requires") {
			t.Fatalf("ErrCapabilityDenied(required).Error() = %q, want requires text", got)
		}
	})

	t.Run("Should typed registry errors expose sentinels", func(t *testing.T) {
		notFound := &ExtensionNotFoundError{Name: "missing"}
		if got := notFound.Error(); got != "extension: extension not found: missing" {
			t.Fatalf("ExtensionNotFoundError.Error() = %q", got)
		}
		if !errors.Is(notFound, ErrExtensionNotFound) {
			t.Fatalf("errors.Is(notFound, ErrExtensionNotFound) = false")
		}

		existsErr := &ExtensionExistsError{Name: "duplicate"}
		if got := existsErr.Error(); got != "extension: extension already exists: duplicate" {
			t.Fatalf("ExtensionExistsError.Error() = %q", got)
		}
		if !errors.Is(existsErr, ErrExtensionExists) {
			t.Fatalf("errors.Is(existsErr, ErrExtensionExists) = false")
		}

		var nilChecksum *ExtensionChecksumMismatchError
		if got := nilChecksum.Error(); got != "extension: checksum mismatch" {
			t.Fatalf("nil ExtensionChecksumMismatchError.Error() = %q", got)
		}

		mismatch := &ExtensionChecksumMismatchError{ExpectedChecksum: "abc", ActualChecksum: "def"}
		if got := mismatch.Error(); !strings.Contains(got, "expected abc, got def") {
			t.Fatalf("ExtensionChecksumMismatchError.Error() = %q", got)
		}
		if !errors.Is(mismatch, ErrExtensionChecksumMismatch) {
			t.Fatalf("errors.Is(mismatch, ErrExtensionChecksumMismatch) = false")
		}
		if got := (&ExtensionNotFoundError{}).Error(); got != ErrExtensionNotFound.Error() {
			t.Fatalf("ExtensionNotFoundError{}.Error() = %q, want %q", got, ErrExtensionNotFound.Error())
		}
		if got := (&ExtensionExistsError{}).Error(); got != ErrExtensionExists.Error() {
			t.Fatalf("ExtensionExistsError{}.Error() = %q, want %q", got, ErrExtensionExists.Error())
		}
	})

	t.Run("Should parse extension source", func(t *testing.T) {
		for input, want := range map[string]ExtensionSource{
			"bundled":     SourceBundled,
			"user":        SourceUser,
			"workspace":   SourceWorkspace,
			"marketplace": SourceMarketplace,
		} {
			got, err := parseExtensionSource(input)
			if err != nil {
				t.Fatalf("parseExtensionSource(%q) error = %v", input, err)
			}
			if got != want {
				t.Fatalf("parseExtensionSource(%q) = %v, want %v", input, got, want)
			}
		}
		if _, err := parseExtensionSource("unknown"); err == nil {
			t.Fatal("parseExtensionSource(unknown) error = nil, want non-nil")
		}
	})

	t.Run("Should check ready validates receiver", func(t *testing.T) {
		var nilRegistry *Registry
		if err := nilRegistry.checkReady("list"); err == nil {
			t.Fatal("nil registry checkReady() error = nil, want non-nil")
		}

		registry := &Registry{}
		if err := registry.checkReady("list"); err == nil {
			t.Fatal("nil db checkReady() error = nil, want non-nil")
		}
	})

	t.Run("Should resolve install artifact supports directory and manifest file", func(t *testing.T) {
		dir := t.TempDir()
		jsonPath := filepath.Join(dir, manifestJSONFileName)
		writeFile(t, jsonPath, `{
  "extension": {
    "name": "json-only",
    "version": "0.2.1",
    "min_compozy_version": "0.5.0"
  }
}`)

		artifactRoot, manifestPath, err := resolveInstallArtifact(dir)
		if err != nil {
			t.Fatalf("resolveInstallArtifact(dir) error = %v", err)
		}
		if artifactRoot != dir {
			t.Fatalf("artifactRoot = %q, want %q", artifactRoot, dir)
		}
		if manifestPath != jsonPath {
			t.Fatalf("manifestPath = %q, want %q", manifestPath, jsonPath)
		}

		fileRoot, fileManifest, err := resolveInstallArtifact(jsonPath)
		if err != nil {
			t.Fatalf("resolveInstallArtifact(file) error = %v", err)
		}
		if fileRoot != dir || fileManifest != jsonPath {
			t.Fatalf("resolveInstallArtifact(file) = (%q, %q)", fileRoot, fileManifest)
		}

		otherFile := filepath.Join(dir, "notes.txt")
		writeFile(t, otherFile, "notes")
		if _, _, err := resolveInstallArtifact(otherFile); err == nil {
			t.Fatal("resolveInstallArtifact(notes.txt) error = nil, want non-nil")
		}
		if _, _, err := resolveInstallArtifact(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("resolveInstallArtifact(missing) error = nil, want non-nil")
		}
		if _, _, err := resolveInstallArtifact(""); err == nil {
			t.Fatal("resolveInstallArtifact(blank) error = nil, want non-nil")
		}
	})

	t.Run("Should checksum helper handles errors and hashes symlinks deterministically", func(t *testing.T) {
		if _, err := ComputeDirectoryChecksum(""); err == nil {
			t.Fatal("ComputeDirectoryChecksum(blank) error = nil, want non-nil")
		}

		filePath := filepath.Join(t.TempDir(), "not-a-dir.txt")
		writeFile(t, filePath, "payload")
		if _, err := ComputeDirectoryChecksum(filePath); err == nil {
			t.Fatal("ComputeDirectoryChecksum(file) error = nil, want non-nil")
		}

		dir := t.TempDir()
		targetPath := filepath.Join(dir, "payload.txt")
		writeFile(t, targetPath, "payload")
		linkPath := filepath.Join(dir, "payload-link.txt")
		if err := os.Symlink("payload.txt", linkPath); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}
		packageDir := filepath.Join(dir, "packages", "typescript", "bin")
		if err := os.MkdirAll(packageDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", packageDir, err)
		}
		writeFile(t, filepath.Join(packageDir, "tsc"), "#!/usr/bin/env node\n")
		nodeModulesDir := filepath.Join(dir, "node_modules", ".bin")
		if err := os.MkdirAll(nodeModulesDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", nodeModulesDir, err)
		}
		if err := os.Symlink(
			filepath.Join("..", "..", "packages", "typescript"),
			filepath.Join(dir, "node_modules", "typescript"),
		); err != nil {
			t.Skipf("os.Symlink(directory) unavailable: %v", err)
		}
		if err := os.Symlink(
			filepath.Join("..", "typescript", "bin", "tsc"),
			filepath.Join(nodeModulesDir, "tsc"),
		); err != nil {
			t.Skipf("os.Symlink(file) unavailable: %v", err)
		}

		first, err := ComputeDirectoryChecksum(dir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(dir with symlink) error = %v", err)
		}
		second, err := ComputeDirectoryChecksum(dir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(dir with repeated symlink hash) error = %v", err)
		}
		if first != second {
			t.Fatalf(
				"ComputeDirectoryChecksum(dir with symlink) = %q on first pass, %q on second pass, want stable checksum",
				first,
				second,
			)
		}

		if err := os.Remove(filepath.Join(nodeModulesDir, "tsc")); err != nil {
			t.Fatalf("os.Remove(tsc symlink) error = %v", err)
		}
		if err := os.Symlink(
			filepath.Join("..", "..", "payload.txt"),
			filepath.Join(nodeModulesDir, "tsc"),
		); err != nil {
			t.Fatalf("os.Symlink(updated file target) error = %v", err)
		}

		updated, err := ComputeDirectoryChecksum(dir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(dir after symlink target change) error = %v", err)
		}
		if updated == first {
			t.Fatalf(
				"ComputeDirectoryChecksum(dir after symlink target change) = %q, want checksum change from %q",
				updated,
				first,
			)
		}
	})

	t.Run("Should constraint mapper preserves passthrough errors", func(t *testing.T) {
		original := errors.New("boom")
		if err := mapRegistryConstraintError(original, "duplicate"); !errors.Is(err, original) {
			t.Fatalf("mapRegistryConstraintError(boom) = %v, want wrapped boom", err)
		}
	})

	t.Run("Should string and json helpers normalize optional values", func(t *testing.T) {
		t.Parallel()

		if got := optionalInstallString("   "); got != nil {
			t.Fatalf("optionalInstallString(blank) = %#v, want nil", got)
		}

		got := optionalInstallString(" extension ")
		if got == nil || *got != "extension" {
			t.Fatalf("optionalInstallString(non-blank) = %#v, want %q", got, "extension")
		}

		var decoded map[string]any
		if err := decodeRegistryJSON("", &decoded); err != nil {
			t.Fatalf("decodeRegistryJSON(blank) error = %v", err)
		}
		if len(decoded) != 0 {
			t.Fatalf("decodeRegistryJSON(blank) = %#v, want empty map", decoded)
		}
		if err := decodeRegistryJSON("{", &decoded); err == nil {
			t.Fatal("decodeRegistryJSON(invalid) error = nil, want non-nil")
		}
	})

	t.Run("Should manifest resolution prefers toml and reports missing manifests", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		jsonPath := filepath.Join(dir, manifestJSONFileName)
		tomlPath := filepath.Join(dir, manifestTOMLFileName)
		writeFile(t, jsonPath, `{"extension":{"name":"json-only","version":"0.2.1","min_compozy_version":"0.5.0"}}`)
		writeFile(t, tomlPath, `[extension]
name = "toml-first"
version = "0.2.1"
min_compozy_version = "0.5.0"
`)

		gotPath, err := resolveManifestPath(dir)
		if err != nil {
			t.Fatalf("resolveManifestPath(toml+json) error = %v", err)
		}
		if gotPath != tomlPath {
			t.Fatalf("resolveManifestPath(toml+json) = %q, want %q", gotPath, tomlPath)
		}

		trimmedName, err := normalizeExtensionName(" bridge ")
		if err != nil {
			t.Fatalf("normalizeExtensionName(valid) error = %v", err)
		}
		if trimmedName != "bridge" {
			t.Fatalf("normalizeExtensionName(valid) = %q, want %q", trimmedName, "bridge")
		}
		if _, err := normalizeExtensionName("   "); err == nil {
			t.Fatal("normalizeExtensionName(blank) error = nil, want non-nil")
		}

		missingDir := t.TempDir()
		_, err = resolveManifestPath(missingDir)
		if err == nil {
			t.Fatal("resolveManifestPath(missing) error = nil, want non-nil")
		}
		if _, ok := errors.AsType[*ManifestNotFoundError](err); !ok {
			t.Fatalf("resolveManifestPath(missing) error = %T, want *ManifestNotFoundError", err)
		}
	})

	t.Run("Should rows affected helper and checksum string handle edge cases", func(t *testing.T) {
		t.Parallel()

		if err := rowsAffectedNotFound(registryTestResult{rowsAffected: 1}, "existing"); err != nil {
			t.Fatalf("rowsAffectedNotFound(existing) error = %v, want nil", err)
		}

		err := rowsAffectedNotFound(registryTestResult{rowsAffected: 0}, "missing")
		if err == nil {
			t.Fatal("rowsAffectedNotFound(missing) error = nil, want non-nil")
		}
		if _, ok := errors.AsType[*ExtensionNotFoundError](err); !ok {
			t.Fatalf("rowsAffectedNotFound(missing) error = %T, want *ExtensionNotFoundError", err)
		}

		boom := errors.New("boom")
		if err := rowsAffectedNotFound(registryTestResult{err: boom}, "broken"); !errors.Is(err, boom) {
			t.Fatalf("rowsAffectedNotFound(result error) = %v, want wrapped boom", err)
		}

		if err := writeChecksumString(sha256.New(), "payload"); err != nil {
			t.Fatalf("writeChecksumString(valid) error = %v", err)
		}
		if err := writeChecksumString(failingHash{}, "payload"); err == nil {
			t.Fatal("writeChecksumString(failing hash) error = nil, want non-nil")
		}
	})

	t.Run("Should write checksum entry covers regular files symlinks and errors", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "payload.txt"), "payload")

		if err := writeChecksumEntry(sha256.New(), dir, "payload.txt"); err != nil {
			t.Fatalf("writeChecksumEntry(regular file) error = %v", err)
		}
		if err := writeChecksumEntry(failingHash{}, dir, "payload.txt"); err == nil {
			t.Fatal("writeChecksumEntry(failing hash) error = nil, want non-nil")
		}
		if err := writeChecksumEntry(sha256.New(), dir, "missing.txt"); err == nil {
			t.Fatal("writeChecksumEntry(missing) error = nil, want non-nil")
		}
		if err := writeChecksumEntry(sha256.New(), dir, "."); err == nil {
			t.Fatal("writeChecksumEntry(directory) error = nil, want non-nil")
		}

		linkPath := filepath.Join(dir, "payload-link.txt")
		if err := os.Symlink("payload.txt", linkPath); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}
		if err := writeChecksumEntry(sha256.New(), dir, "payload-link.txt"); err != nil {
			t.Fatalf("writeChecksumEntry(symlink) error = %v", err)
		}
	})
}

type registryTestEnv struct {
	db          *sql.DB
	registry    *Registry
	installedAt time.Time
}

type registryTestResult struct {
	rowsAffected int64
	err          error
}

func (r registryTestResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r registryTestResult) RowsAffected() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.rowsAffected, nil
}

type failingHash struct{}

func (failingHash) Write(_ []byte) (int, error) { return 0, errors.New("hash failed") }
func (failingHash) Sum(b []byte) []byte         { return b }
func (failingHash) Reset()                      {}
func (failingHash) Size() int                   { return 0 }
func (failingHash) BlockSize() int              { return 0 }

type registryManifestOptions struct {
	capabilities []string
	permissions  []string
	extraFiles   map[string]string
}

func newRegistryTestEnv(t *testing.T) registryTestEnv {
	t.Helper()

	db := openExtensionTestGlobalSQLDB(t)

	installedAt := time.Date(2026, 4, 10, 15, 30, 0, 0, time.UTC)
	registry := NewRegistry(db)
	registry.now = func() time.Time { return installedAt }

	return registryTestEnv{
		db:          db,
		registry:    registry,
		installedAt: installedAt,
	}
}

func holdTransientRegistryWriteLock(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := testutil.Context(t)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 1`); err != nil {
		t.Fatalf("set registry busy timeout: %v", err)
	}

	var sequence int
	var databaseName string
	var databasePath string
	if err := db.QueryRowContext(ctx, `PRAGMA database_list`).Scan(
		&sequence,
		&databaseName,
		&databasePath,
	); err != nil {
		t.Fatalf("read registry database path: %v", err)
	}
	locker, err := store.OpenSQLiteDatabase(ctx, databasePath, nil)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase(locker) error = %v", err)
	}
	t.Cleanup(func() {
		if err := locker.Close(); err != nil {
			t.Errorf("locker.Close() error = %v", err)
		}
	})
	lockConn, err := locker.Conn(ctx)
	if err != nil {
		t.Fatalf("locker.Conn() error = %v", err)
	}
	t.Cleanup(func() {
		if err := lockConn.Close(); err != nil {
			t.Errorf("lockConn.Close() error = %v", err)
		}
	})
	if _, err := lockConn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		t.Fatalf("BEGIN IMMEDIATE locker error = %v", err)
	}

	releaseDone := make(chan error, 1)
	timer := time.AfterFunc(100*time.Millisecond, func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
		defer cancel()
		_, commitErr := lockConn.ExecContext(releaseCtx, `COMMIT`)
		releaseDone <- commitErr
	})
	t.Cleanup(func() {
		if timer.Stop() {
			if _, err := lockConn.ExecContext(testutil.Context(t), `COMMIT`); err != nil {
				t.Errorf("manual lock release error = %v", err)
			}
			return
		}
		if err := <-releaseDone; err != nil {
			t.Errorf("timed lock release error = %v", err)
		}
	})
}

func insertActiveRegistryProfile(t *testing.T, env registryTestEnv, name string) string {
	t.Helper()
	profileIDValue, err := ulid.New(ulid.Timestamp(time.Now().UTC()), rand.Reader)
	if err != nil {
		t.Fatalf("ulid.New(active profile) error = %v", err)
	}
	profileID := profileIDValue.String()
	insertRegistryProfile(t, env, profileID, name, "#112233", "briefcase")
	return profileID
}

func insertRegistryProfile(t *testing.T, env registryTestEnv, id, name, color, icon string) {
	t.Helper()
	if _, err := env.db.ExecContext(t.Context(), `INSERT INTO profiles (
		id, name, color, icon, state, created_at
	) VALUES (?, ?, ?, ?, 'active', ?)`, id, name, color, icon, store.FormatTimestamp(env.installedAt)); err != nil {
		t.Fatalf("insert %s profile: %v", name, err)
	}
}

func TestRegistryDBReturnsBackingHandleAndNilSafe(t *testing.T) {
	t.Parallel()

	var nilRegistry *Registry
	if got := nilRegistry.DB(); got != nil {
		t.Fatalf("(*Registry)(nil).DB() = %#v, want nil", got)
	}

	env := newRegistryTestEnv(t)
	if got, want := env.registry.DB(), env.db; got != want {
		t.Fatalf("registry.DB() = %#v, want %#v", got, want)
	}
}

func createRegistryTestExtension(t *testing.T, name string, opts registryManifestOptions) (string, *Manifest, string) {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, manifestTOMLFileName), registryManifestTOML(name, opts))

	for relPath, content := range opts.extraFiles {
		absPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(absPath), err)
		}
		writeFile(t, absPath, content)
	}

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", dir, err)
	}
	checksum, err := ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", dir, err)
	}

	return dir, manifest, checksum
}

func registryManifestTOML(name string, opts registryManifestOptions) string {
	capabilities := append([]string(nil), opts.capabilities...)
	if len(capabilities) == 0 {
		capabilities = []string{"tool.provider"}
	}

	permissions := append([]string(nil), opts.permissions...)
	if len(permissions) == 0 {
		permissions = []string{"sessions/list"}
	}

	return fmt.Sprintf(
		`[extension]
name = %q
version = "0.2.1"
description = "Registry test extension"
min_compozy_version = "0.5.0"

[capabilities]
provides = %s

[permissions]
requires = %s
`,
		name,
		tomlStringArray(capabilities),
		tomlStringArray(permissions),
	)
}

func tomlStringArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// Invariant: one package can have isolated durable attachments without widening a scoped install or losing rollback state.
// Owner: extension registry. Canonical suite: registry_test.go.
func TestRegistryInstallationScopes(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve exact workspace and profile attachments before inherited scopes", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		profileID := insertActiveRegistryProfile(t, env, "marketing")
		for _, id := range []string{"ws-a", "ws-b"} {
			if _, err := env.db.ExecContext(t.Context(), `INSERT INTO workspaces
   (id, root_dir, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, id, "/"+id, id,
				store.FormatTimestamp(env.installedAt), store.FormatTimestamp(env.installedAt)); err != nil {
				t.Fatal(err)
			}
		}
		dir, manifest, checksum := createRegistryTestExtension(t, "attached", registryManifestOptions{})
		globalProfile := InstallationScope{ProfileID: profileID}
		workspaceProfile := InstallationScope{ProfileID: profileID, WorkspaceID: "ws-a"}
		workspaceAll := InstallationScope{WorkspaceID: "ws-a"}
		if err := env.registry.Install(manifest, dir, checksum, WithInstallScope(globalProfile)); err != nil {
			t.Fatal(err)
		}
		initial, err := env.registry.Installations(t.Context(), manifest.Name)
		if err != nil || len(initial) != 1 || initial[0].Scope != globalProfile {
			t.Fatalf("initial installations = %#v, %v", initial, err)
		}
		for _, scope := range []InstallationScope{workspaceAll, workspaceProfile} {
			if err := env.registry.AttachInstallation(t.Context(), manifest.Name, scope); err != nil {
				t.Fatal(err)
			}
		}
		before, err := env.registry.Installations(t.Context(), manifest.Name)
		if err != nil {
			t.Fatal(err)
		}
		env.registry.now = func() time.Time { return env.installedAt.Add(time.Hour) }
		if err := env.registry.AttachInstallation(t.Context(), manifest.Name, workspaceProfile); err != nil {
			t.Fatal(err)
		}
		if err := env.registry.Install(manifest, dir, checksum, WithInstallReplaceExisting()); err != nil {
			t.Fatal(err)
		}
		after, err := env.registry.Installations(t.Context(), manifest.Name)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("retry/update installations = %#v, want %#v, err=%v", after, before, err)
		}
		for _, tc := range []struct {
			read, want InstallationScope
			missing    bool
		}{
			{read: workspaceProfile, want: workspaceProfile},
			{read: InstallationScope{ProfileID: store.DefaultProfileID, WorkspaceID: "ws-a"}, want: workspaceAll},
			{read: InstallationScope{ProfileID: profileID, WorkspaceID: "ws-b"}, want: globalProfile},
			{read: InstallationScope{ProfileID: store.DefaultProfileID, WorkspaceID: "ws-b"}, missing: true},
			{read: InstallationScope{ProfileID: store.DefaultProfileID}, missing: true},
		} {
			resolved, err := env.registry.ResolveInstallation(t.Context(), manifest.Name, tc.read)
			if tc.missing {
				if !errors.Is(err, ErrExtensionNotFound) {
					t.Fatalf("foreign scope %#v: %v", tc.read, err)
				}
			} else if err != nil || resolved.Scope != tc.want {
				t.Fatalf("resolve %#v = %#v, %v; want %#v", tc.read, resolved, err, tc.want)
			}
		}
		snapshot, err := env.registry.SnapshotRemovalState(t.Context(), manifest.Name)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.registry.Uninstall(manifest.Name); err != nil {
			t.Fatal(err)
		}
		if err := env.registry.Install(manifest, dir, checksum); err != nil {
			t.Fatal(err)
		}
		if err := env.registry.RestoreRemovalState(t.Context(), manifest.Name, snapshot); err != nil {
			t.Fatal(err)
		}
		restored, err := env.registry.Installations(t.Context(), manifest.Name)
		if err != nil || !reflect.DeepEqual(before, restored) {
			t.Fatalf("restored = %#v, want %#v, err=%v", restored, before, err)
		}
		if err := env.registry.DetachInstallation(t.Context(), manifest.Name, workspaceProfile); err != nil {
			t.Fatal(err)
		}
		resolved, err := env.registry.ResolveInstallation(t.Context(), manifest.Name, workspaceProfile)
		if err != nil || resolved.Scope != workspaceAll {
			t.Fatalf("after exact detach = %#v, %v", resolved, err)
		}
		if _, err := env.registry.Get(manifest.Name); err != nil {
			t.Fatalf("detach removed package: %v", err)
		}
	})
	t.Run("Should not restore an attachment after its profile or workspace was deleted", func(t *testing.T) {
		t.Parallel()
		for _, removed := range []string{"profile", "workspace"} {
			env := newRegistryTestEnv(t)
			profileID := insertActiveRegistryProfile(t, env, "marketing")
			if _, err := env.db.ExecContext(t.Context(), `INSERT INTO workspaces
    (id, root_dir, name, created_at, updated_at) VALUES ('ws-owned', '/owned', 'owned', ?, ?)`,
				store.FormatTimestamp(env.installedAt), store.FormatTimestamp(env.installedAt)); err != nil {
				t.Fatal(err)
			}
			dir, manifest, checksum := createRegistryTestExtension(t, "owned", registryManifestOptions{})
			scope := InstallationScope{ProfileID: profileID, WorkspaceID: "ws-owned"}
			if err := env.registry.Install(manifest, dir, checksum, WithInstallScope(scope)); err != nil {
				t.Fatal(err)
			}
			snapshot, err := env.registry.SnapshotRemovalState(t.Context(), manifest.Name)
			if err != nil {
				t.Fatal(err)
			}
			if removed == "profile" {
				if _, err := env.db.ExecContext(
					t.Context(),
					`DELETE FROM profiles WHERE id = ?`,
					profileID,
				); err != nil {
					t.Fatal(err)
				}
			} else if _, err := env.db.ExecContext(t.Context(), `DELETE FROM workspaces WHERE id = 'ws-owned'`); err != nil {
				t.Fatal(err)
			}
			rows, err := env.registry.Installations(t.Context(), manifest.Name)
			if err != nil || len(rows) != 0 {
				t.Fatalf("deleted %s retained attachment: %#v, %v", removed, rows, err)
			}
			if err := env.registry.Uninstall(manifest.Name); err != nil {
				t.Fatal(err)
			}
			if err := env.registry.Install(manifest, dir, checksum); err != nil {
				t.Fatal(err)
			}
			if err := env.registry.RestoreRemovalState(t.Context(), manifest.Name, snapshot); err != nil {
				t.Fatal(err)
			}
			restored, err := env.registry.Installations(t.Context(), manifest.Name)
			if err != nil || len(restored) != 0 {
				t.Fatalf("compensation widened deleted %s scope: %#v, %v", removed, restored, err)
			}
		}
	})
	t.Run("Should roll back the package insert when an attachment owner is missing", func(t *testing.T) {
		t.Parallel()
		for _, scope := range []InstallationScope{{ProfileID: "missing"}, {WorkspaceID: "missing"}} {
			env := newRegistryTestEnv(t)
			dir, manifest, checksum := createRegistryTestExtension(t, "refused", registryManifestOptions{})
			if err := env.registry.Install(manifest, dir, checksum, WithInstallScope(scope)); err == nil {
				t.Fatal("accepted missing attachment owner")
			}
			if _, err := env.registry.Get(manifest.Name); !errors.Is(err, ErrExtensionNotFound) {
				t.Fatalf("partial package persisted: %v", err)
			}
			rows, err := env.registry.Installations(t.Context(), manifest.Name)
			if err != nil || len(rows) != 0 {
				t.Fatalf("partial attachments = %#v, %v", rows, err)
			}
		}
	})
}

func TestRegistrySupersededBundledRetention(t *testing.T) {
	t.Parallel()
	t.Run("Should keep predecessor state when enrolling its successor", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		oldDir, oldManifest, oldChecksum := createRegistryTestExtension(t, "dev-cycle", registryManifestOptions{})
		if err := env.registry.Install(oldManifest, oldDir, oldChecksum, WithInstallSource(SourceBundled)); err != nil {
			t.Fatal(err)
		}
		if err := env.registry.SetEnabledForProfile("dev-cycle", store.DefaultProfileID, false); err != nil {
			t.Fatal(err)
		}
		dir, manifest, checksum := createRegistryTestExtension(t, "spec-cycle", registryManifestOptions{})
		if err := env.registry.Install(manifest, dir, checksum, WithInstallSource(SourceBundled)); err != nil {
			t.Fatal(err)
		}
		if _, err := env.registry.Get("dev-cycle"); err != nil {
			t.Fatalf("predecessor lost: %v", err)
		}
		oldEnabled, err := env.registry.IsEnabledForProfile("dev-cycle", store.DefaultProfileID)
		if err != nil || oldEnabled {
			t.Fatalf("predecessor enablement = %v, %v", oldEnabled, err)
		}
		enabled, err := env.registry.IsEnabledForProfile("spec-cycle", store.DefaultProfileID)
		if err != nil || !enabled {
			t.Fatalf("successor inherited retired settings: %v, %v", enabled, err)
		}
		if _, err := os.Stat(filepath.Join(oldDir, manifestTOMLFileName)); err != nil {
			t.Fatalf("predecessor files lost: %v", err)
		}
	})
}

// builtinPermanentTestName is a bundled built-in that the permanent-removal tests enroll.
const builtinPermanentTestName = "spec-cycle"

// seedBundledRegistryRow enrolls the test built-in through the same path boot uses.
func seedBundledRegistryRow(t *testing.T, env registryTestEnv) compozyconfig.HomePaths {
	t.Helper()
	withDaemonVersion(t, "0.6.0")
	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	bundledFS := fstest.MapFS{
		manifestTOMLFileName: {Data: []byte(registryManifestTOML(builtinPermanentTestName, registryManifestOptions{}))},
	}
	if err := InstallBundledExtension(
		homePaths,
		env.registry,
		BundledInstallSpec{Name: builtinPermanentTestName, FS: bundledFS},
	); err != nil {
		t.Fatalf("InstallBundledExtension() error = %v", err)
	}
	return homePaths
}

// seedMarketplaceRegistryRow installs a non-bundled row with a real package directory and returns that directory.
func seedMarketplaceRegistryRow(t *testing.T, env registryTestEnv, name string) string {
	t.Helper()
	dir := t.TempDir()
	manifestData := []byte(registryManifestTOML(name, registryManifestOptions{}))
	if err := os.WriteFile(filepath.Join(dir, manifestTOMLFileName), manifestData, 0o600); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	checksum, err := ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum() error = %v", err)
	}
	if err := env.registry.Install(manifest, dir, checksum, WithInstallSource(SourceMarketplace)); err != nil {
		t.Fatalf("Install(marketplace) error = %v", err)
	}
	return dir
}

func TestRegistryUninstallBuiltinTombstonedIsAtomic(t *testing.T) {
	t.Run("Should delete the bundled row and insert its tombstone in one transaction", func(t *testing.T) {
		env := newRegistryTestEnv(t)
		seedBundledRegistryRow(t, env)

		got, err := env.registry.uninstallBuiltinTombstoned(builtinPermanentTestName, " operator ")
		if err != nil {
			t.Fatalf("uninstallBuiltinTombstoned() error = %v", err)
		}
		if got.Name != builtinPermanentTestName || got.RemovedBy != "operator" {
			t.Fatalf(
				"uninstallBuiltinTombstoned() = %#v, want name %q removed_by operator",
				got,
				builtinPermanentTestName,
			)
		}
		if _, err := env.registry.Get(builtinPermanentTestName); !errors.Is(err, ErrExtensionNotFound) {
			t.Fatalf("Get(after permanent removal) error = %v, want ErrExtensionNotFound", err)
		}
		tombstoned, err := env.registry.Tombstoned(t.Context(), builtinPermanentTestName)
		if err != nil || !tombstoned {
			t.Fatalf("Tombstoned() = %v, %v; want true, nil", tombstoned, err)
		}
	})

	t.Run("Should roll back the row delete when the tombstone insert conflicts", func(t *testing.T) {
		env := newRegistryTestEnv(t)
		seedBundledRegistryRow(t, env)
		// A tombstone that already exists makes the in-transaction insert fail after the row delete.
		if _, err := env.registry.Tombstone(t.Context(), builtinPermanentTestName, "seed"); err != nil {
			t.Fatalf("seed Tombstone() error = %v", err)
		}

		if _, err := env.registry.uninstallBuiltinTombstoned(builtinPermanentTestName, "operator"); err == nil {
			t.Fatal("uninstallBuiltinTombstoned(conflicting tombstone) error = nil, want a refusal")
		}
		info, err := env.registry.Get(builtinPermanentTestName)
		if err != nil {
			t.Fatalf("Get(after failed permanent removal) error = %v, want the row kept", err)
		}
		if info.Source != SourceBundled {
			t.Fatalf("Source after failed permanent removal = %v, want bundled", info.Source)
		}
		tombstones, err := env.registry.ListTombstones(t.Context())
		if err != nil {
			t.Fatalf("ListTombstones() error = %v", err)
		}
		if len(tombstones) != 1 || tombstones[0].RemovedBy != "seed" {
			t.Fatalf("ListTombstones() = %#v, want only the seeded tombstone", tombstones)
		}
	})

	t.Run("Should refuse a non-bundled row without changing it or writing a tombstone", func(t *testing.T) {
		env := newRegistryTestEnv(t)
		seedMarketplaceRegistryRow(t, env, "dev-cycle")

		_, err := env.registry.uninstallBuiltinTombstoned("dev-cycle", "operator")
		if !errors.Is(err, ErrExtensionNotBuiltin) {
			t.Fatalf("uninstallBuiltinTombstoned(marketplace) error = %v, want ErrExtensionNotBuiltin", err)
		}
		info, err := env.registry.Get("dev-cycle")
		if err != nil || info.Source != SourceMarketplace {
			t.Fatalf("Get(dev-cycle) = %#v, %v; want the marketplace row unchanged", info, err)
		}
		tombstoned, err := env.registry.Tombstoned(t.Context(), "dev-cycle")
		if err != nil || tombstoned {
			t.Fatalf("Tombstoned(dev-cycle) = %v, %v; want false, nil", tombstoned, err)
		}
	})

	t.Run("Should refuse an unknown name without writing a tombstone", func(t *testing.T) {
		env := newRegistryTestEnv(t)

		_, err := env.registry.uninstallBuiltinTombstoned("missing-ext", "operator")
		if !errors.Is(err, ErrExtensionNotFound) {
			t.Fatalf("uninstallBuiltinTombstoned(unknown) error = %v, want ErrExtensionNotFound", err)
		}
		tombstoned, err := env.registry.Tombstoned(t.Context(), "missing-ext")
		if err != nil || tombstoned {
			t.Fatalf("Tombstoned(missing-ext) = %v, %v; want false, nil", tombstoned, err)
		}
	})
}
