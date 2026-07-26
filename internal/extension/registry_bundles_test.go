package extensionpkg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestRegistryBlocksDisableAndUninstallWithActiveBundles(t *testing.T) {
	t.Parallel()

	env := newRegistryTestEnvWithBundleActivations(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "bundle-guard", registryManifestOptions{})
	if err := env.registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if _, err := env.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO resource_records (
			kind, id, version, scope_kind, scope_id, owner_kind, owner_id,
			source_kind, source_id, spec_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bundle.activation",
		"act_guard",
		1,
		"global",
		nil,
		"daemon",
		"bundle-test",
		"daemon",
		"bundle-test",
		`{"extension_name":"`+manifest.Name+`","bundle_name":"bundle","profile_name":"default"}`,
		store.FormatTimestamp(env.installedAt),
		store.FormatTimestamp(env.installedAt),
	); err != nil {
		t.Fatalf("insert bundle activation error = %v", err)
	}

	if err := env.registry.Disable(manifest.Name); !errors.Is(err, ErrExtensionHasActiveBundles) {
		t.Fatalf("Disable() error = %v, want ErrExtensionHasActiveBundles", err)
	}
	if err := env.registry.Uninstall(manifest.Name); !errors.Is(err, ErrExtensionHasActiveBundles) {
		t.Fatalf("Uninstall() error = %v, want ErrExtensionHasActiveBundles", err)
	}
}

func newRegistryTestEnvWithBundleActivations(t *testing.T) registryTestEnv {
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

func TestLoadBundleSpecsRejectsCaseInsensitiveDuplicateBundleNames(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	bundlesDir := filepath.Join(rootDir, "bundles")
	if err := os.MkdirAll(bundlesDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundlesDir, "alpha.json"), []byte(`{
		"name": "Marketing",
		"profiles": [{
			"name": "default",
			"channels": {
				"primary": "ops",
				"items": [{"name": "ops"}]
			}
		}]
	}`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(alpha.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundlesDir, "beta.json"), []byte(`{
		"name": "marketing",
		"profiles": [{
			"name": "default",
			"channels": {
				"primary": "ops",
				"items": [{"name": "ops"}]
			}
		}]
	}`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(beta.json) error = %v", err)
	}

	_, err := LoadBundleSpecs(context.Background(), rootDir, &Manifest{
		Name: "bundle-guard",
		Resources: ResourcesConfig{
			Bundles: []string{"bundles"},
		},
	})
	if !errors.Is(err, ErrBundleInvalid) {
		t.Fatalf("LoadBundleSpecs() error = %v, want ErrBundleInvalid", err)
	}
}

func TestBundleSpecValidateRejectsCaseInsensitiveDuplicateProfilesAndInvalidDeliveryDefaults(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		spec        BundleSpec
		wantMessage string
	}{
		{
			name: "Should reject case-insensitive duplicate profile names",
			spec: BundleSpec{
				Name: "marketing",
				Profiles: []BundleProfile{
					{
						Name: "Default",
						Channels: BundleChannelsConfig{
							Primary: "ops",
							Items:   []BundleChannel{{Name: "ops"}},
						},
					},
					{
						Name: "default",
						Channels: BundleChannelsConfig{
							Primary: "ops-2",
							Items:   []BundleChannel{{Name: "ops-2"}},
						},
					},
				},
			},
		},
		{
			name: "Should reject invalid bridge delivery default JSON",
			spec: BundleSpec{
				Name: "marketing",
				Profiles: []BundleProfile{{
					Name: "default",
					Channels: BundleChannelsConfig{
						Primary: "ops",
						Items:   []BundleChannel{{Name: "ops"}},
					},
					Bridges: []BundleBridgePreset{{
						Name:             "telegram-main",
						Platform:         "telegram",
						DisplayName:      "Marketing Bridge",
						RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
						DeliveryDefaults: []byte(`{invalid`),
					}},
				}},
			},
		},
		{
			name:        "Should reject case-insensitive duplicate window layout IDs",
			wantMessage: `layout "two-up" is duplicated`,
			spec: BundleSpec{
				Name: "marketing",
				Profiles: []BundleProfile{{
					Name: "default",
					Layouts: []BundleLayout{
						{Path: "layouts/alpha.json", Layout: testBundleLayoutResource("Two-Up")},
						{Path: "layouts/beta.json", Layout: testBundleLayoutResource("two-up")},
					},
				}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.spec.Validate(&Manifest{
				Name: "bundle-guard",
				Bridge: BridgeConfig{
					Platform:    "telegram",
					DisplayName: "Telegram",
				},
				Capabilities: CapabilitiesConfig{
					Provides: []string{"bridge.adapter"},
				},
			})
			if !errors.Is(err, ErrBundleInvalid) {
				t.Fatalf("BundleSpec.Validate() error = %v, want ErrBundleInvalid", err)
			}
			if tc.wantMessage != "" && !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("BundleSpec.Validate() error = %v, want message containing %q", err, tc.wantMessage)
			}
		})
	}
}
