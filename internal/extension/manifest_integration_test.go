//go:build integration

package extensionpkg

import (
	"path/filepath"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/version"
)

func TestLoadManifestBridgeMetadataRoundTrip(t *testing.T) {
	t.Cleanup(version.OverrideVersionForTesting("0.6.0"))

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, manifestTOMLFileName), `[extension]
name = "slack-bridge"
version = "0.1.0"
min_agh_version = "0.5.0"

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
schema = "agh.bridge.slack"
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
	if got, want := *manifest.Bridge.ConfigSchema, (bridgepkg.BridgeProviderConfigSchema{Schema: "agh.bridge.slack", Version: "v1"}); got != want {
		t.Fatalf("manifest.Bridge.ConfigSchema = %#v, want %#v", got, want)
	}
}
