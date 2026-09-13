//go:build integration

package extensionpkg

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	registrypkg "github.com/compozy/compozy/internal/registry"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
)

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
