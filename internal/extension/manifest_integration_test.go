//go:build integration

package extensionpkg

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
	registrypkg "github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
)

// Invariant: authored plugins reach the durable listing through the real source, cache and install loader.
// Owner: plugin catalog integration; canonical suite: TestPluginCatalogInstallability.
func TestPluginCatalogInstallability(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		skills, servers int
	}{
		{name: "open-design", servers: 1},
		{name: "loop-engineering", skills: 7},
	} {
		t.Run(
			"Should derive "+tc.name+" installability from cached authored bytes through the install loader",
			func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				fixture := filepath.Join("testdata", "client-plugins", tc.name)
				if err := os.CopyFS(filepath.Join(root, "good"), os.DirFS(fixture)); err != nil {
					t.Fatal(err)
				}
				broken := filepath.Join(root, "broken", ".claude-plugin")
				if err := os.MkdirAll(broken, 0o700); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(broken, "plugin.json"), `{"name":`)
				writeFile(
					t,
					filepath.Join(root, "marketplace.json"),
					`{"plugins":[{"name":"good","source":"./good"},{"name":"broken","source":"./broken"}]}`,
				)
				resolver := &pluginsource.Resolver{
					Cache:   &pluginsource.PackageCache{Root: t.TempDir()},
					Sources: pluginsource.Sources{TempDir: t.TempDir()},
				}
				projector, err := marketplace.NewPluginProjector(resolver, InspectPluginPackage)
				if err != nil {
					t.Fatal(err)
				}
				source, err := marketplace.NewPluginSource(
					marketplace.ResolvedSource{Name: "team", Ref: root, Kind: "custom", Enabled: true},
					&resolver.Sources, projector,
				)
				if err != nil {
					t.Fatal(err)
				}
				doc, err := source.Fetch(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				entries, diagnostics := doc.Entries, doc.Diagnostics
				if len(entries) != 2 || len(diagnostics) != 1 || !entries[0].Installable ||
					entries[1].Installable || entries[1].InstallBlocker != "load_failed" || entries[1].Layout != "claude-plugin" {
					t.Fatalf("real loader projection = %+v, diagnostics %+v, %v", entries, diagnostics, err)
				}
				detail, err := marketplace.ProjectEntry(entries[0])
				if err != nil || detail.Extension.Contents.Skills != tc.skills ||
					detail.Extension.Contents.MCPServers != tc.servers {
					t.Fatalf("real package contents = %+v, %v", detail, err)
				}
				if detail.SourceRef != doc.SourceRef ||
					detail.Extension.Acquisition.DigestSHA256 != entries[0].DigestSHA256 {
					t.Fatalf("projected acquisition does not identify the inspected bytes: %+v", detail)
				}
				db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "catalog.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := db.Close(testutil.Context(t)); err != nil {
						t.Error(err)
					}
				})
				catalog, err := marketplace.NewSQLiteStore(db)
				if err != nil {
					t.Fatal(err)
				}
				doc.FetchedAt = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
				if err := catalog.ReplaceSource(t.Context(), "team", 0, doc); err != nil {
					t.Fatal(err)
				}
				page, err := catalog.BrowseSource(t.Context(), "team", "", 0, 10)
				if err != nil || page.Total != 2 || page.State.Installable != 1 ||
					page.State.SourceRef != doc.SourceRef || page.State.DocumentDigest != doc.DocumentDigest ||
					page.State.DocumentPath != doc.DocumentPath || !page.State.GeneratedAt.IsZero() ||
					len(page.State.Diagnostics) != 1 || page.Entries[0].InstallBlocker != "load_failed" ||
					!page.Entries[1].Installable || page.Entries[1].Description != "" {
					t.Fatalf("durable plugin projection = %+v, %v", page, err)
				}
				remaining, err := os.ReadDir(resolver.Sources.TempDir)
				if err != nil || len(remaining) != 0 {
					t.Fatalf("inspection retained temporary trees: %v, %v", remaining, err)
				}
				before, err := ComputeDirectoryChecksum(fixture)
				if err != nil {
					t.Fatal(err)
				}
				after, err := ComputeDirectoryChecksum(filepath.Join(root, "good"))
				if err != nil || before != after {
					t.Fatalf("projection changed authored package bytes: %s != %s, %v", before, after, err)
				}
			},
		)
	}
}

func TestLoadManifestBridgeMetadataRoundTrip(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, manifestTOMLFileName), `[extension]
name = "slack-bridge"
version = "0.1.0"
min_compozy_version = "0.5.0"

[capabilities]
provides = ["bridge.adapter"]

[bridge]
platform = "slack"
display_name = "Slack"

[[bridge.secret_slots]]
name = "bot_token"
description = "Bot OAuth token"
required = true

[[bridge.secret_slots]]
name = "signing_secret"
description = "Request signing secret"
required = true

[bridge.config_schema]
schema = "compozy.bridge.slack"
version = "v1"
`)

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if got, want := manifest.Capabilities.Provides, []string{
		extensionprotocol.CapabilityProvideBridgeAdapter,
	}; len(got) != len(want) ||
		got[0] != want[0] {
		t.Fatalf("manifest.Capabilities.Provides = %#v, want %#v", got, want)
	}
	if got, want := manifest.Bridge.Platform, "slack"; got != want {
		t.Fatalf("manifest.Bridge.Platform = %q, want %q", got, want)
	}
	if got, want := manifest.Bridge.DisplayName, "Slack"; got != want {
		t.Fatalf("manifest.Bridge.DisplayName = %q, want %q", got, want)
	}
	if got, want := manifest.Bridge.SecretSlots, []bridgepkg.BridgeSecretSlot{
		{Name: "bot_token", Description: "Bot OAuth token", Required: true},
		{Name: "signing_secret", Description: "Request signing secret", Required: true},
	}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("manifest.Bridge.SecretSlots = %#v, want %#v", got, want)
	}
	if manifest.Bridge.ConfigSchema == nil {
		t.Fatal("manifest.Bridge.ConfigSchema = nil, want value")
	}
	if got, want := *manifest.Bridge.ConfigSchema, (bridgepkg.BridgeProviderConfigSchema{Schema: "compozy.bridge.slack", Version: "v1"}); got != want {
		t.Fatalf("manifest.Bridge.ConfigSchema = %#v, want %#v", got, want)
	}
}

func TestClientPluginManagedInstall(t *testing.T) {
	for _, tc := range []struct {
		name            string
		skills, servers int
	}{{"open-design", 0, 1}, {"loop-engineering", 7, 0}} {
		t.Run("Should install inspect and remove the authored "+tc.name+" package", func(t *testing.T) {
			t.Parallel()
			fixture := filepath.Join("testdata", "client-plugins", tc.name)
			before, err := ComputeDirectoryChecksum(fixture)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{}
			err = filepath.WalkDir(fixture, func(file string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				relative, err := filepath.Rel(fixture, file)
				if err != nil {
					return err
				}
				content, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				files[tc.name+"/"+filepath.ToSlash(relative)] = string(content)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			source := newLifecycleSourceNamed(t, tc.name, "github", "1.0.0")
			source.archives["1.0.0"] = lifecycleTarGzFiles(t, files)
			loader := func(context.Context) ([]registrypkg.Source, error) { return []registrypkg.Source{source}, nil }
			home, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatal(err)
			}
			env := newRegistryTestEnv(t)
			installed, err := InstallMarketplaceManaged(
				t.Context(),
				home,
				env.registry,
				loader,
				MarketplaceInstallRequest{
					Slug:                   "acme/" + tc.name,
					SourceFilter:           "github",
					PolicyAllowsUnverified: true,
					AllowUnverified:        true,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if installed.Provenance.Layout != "claude-plugin" || installed.Format != FormatAgentPlugin {
				t.Fatalf("installed identity = %#v", installed)
			}
			persisted, err := env.registry.Get(installed.Name)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.Provenance.Layout != "claude-plugin" {
				t.Fatalf("persisted layout = %q", persisted.Provenance.Layout)
			}
			installRoot := PackageRootFromManifest(persisted.ManifestPath)
			actual, err := ComputeDirectoryChecksum(installRoot)
			if err != nil {
				t.Fatal(err)
			}
			if actual != before {
				t.Fatalf("installed bytes differ: %s != %s", actual, before)
			}
			manifest, err := loadManifestAtPath(persisted.ManifestPath)
			if err != nil {
				t.Fatal(err)
			}
			contents, err := InspectPackageContents(t.Context(), &Extension{Info: *persisted, Manifest: manifest}, "")
			if err != nil {
				t.Fatal(err)
			}
			if contents.Skills != tc.skills || contents.MCPServers != tc.servers || contents.Hooks != 0 {
				t.Fatalf("loaded contents = %#v", contents)
			}
			if _, err := RemoveManagedExtension(t.Context(), home, env.registry, installed.Name, nil, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(installRoot); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("package root survived removal: %v", err)
			}
			after, err := ComputeDirectoryChecksum(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("source fixture changed")
			}
		})
	}
}
