//go:build integration

// Suite: Slack manifest CLI integration
// Invariant: a real persisted bridge instance round-trips through CLI, UDS, BaseHandlers, and manifest generation.
// Boundary IN: integration daemon, SQLite registry, UDS client, and Cobra command.
// Boundary OUT: live Slack APIs, owned by provider-control integration tests.
package cli

import (
	"encoding/json"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestBridgeSlackManifestIntegration(t *testing.T) {
	t.Run("Should round trip a generated manifest through the daemon UDS API", func(t *testing.T) {
		t.Parallel()

		h := newIntegrationHarness(t)
		h.runner.bridgeProviders = []bridgepkg.BridgeProvider{{
			Platform:      "slack",
			ExtensionName: "slack-reference",
			DisplayName:   "Slack",
			Description:   "Connect Slack messages, commands, actions, and reactions to AGH.",
		}}
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
		defer func() {
			_, stderr, stopErr := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
			if stopErr != nil {
				t.Errorf("daemon stop error = %v; stderr=%s", stopErr, stderr)
			}
			if waitErr := h.runner.waitForExit(); waitErr != nil {
				t.Errorf("integration daemon wait error = %v", waitErr)
			}
		}()

		createOut := mustExecuteRoot(
			t,
			h.deps,
			"bridge", "create",
			"--scope", "global",
			"--platform", "slack",
			"--extension", "slack-reference",
			"--display-name", "Support agent",
			"--enabled=false",
			"--provider-config", `{"webhook":{"public_url":"https://bridge.example.com/slack/support"}}`,
			"-o", "json",
		)
		var created BridgeRecord
		if err := json.Unmarshal([]byte(createOut), &created); err != nil {
			t.Fatalf("json.Unmarshal(bridge create) error = %v", err)
		}

		manifestOut := mustExecuteRoot(
			t,
			h.deps,
			"bridge", "manifest", "slack",
			"--instance", created.ID,
		)
		var manifest SlackManifestRecord
		if err := json.Unmarshal([]byte(manifestOut), &manifest); err != nil {
			t.Fatalf("json.Unmarshal(bridge manifest) error = %v; output=%s", err, manifestOut)
		}
		if manifest.DisplayInformation.Name != "Support agent" ||
			manifest.Settings.EventSubscriptions.RequestURL !=
				"https://bridge.example.com/slack/support" {
			t.Fatalf("manifest = %#v", manifest)
		}
		if manifest.Settings.SocketModeEnabled {
			t.Fatal("socket_mode_enabled = true, want false")
		}
	})
}
