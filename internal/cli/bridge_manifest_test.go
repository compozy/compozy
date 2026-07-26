// Suite: Slack manifest CLI
// Invariant: the manifest command emits an agent-readable artifact or writes it with one exact operator next step.
// Boundary IN: Cobra flags plus the shared daemon client manifest response.
// Boundary OUT: daemon manifest construction and provider APIs, owned by bridge and route suites.
package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
)

func TestBridgeSlackManifestCommand(t *testing.T) {
	t.Run("Should emit valid JSON on stdout without prose by default", func(t *testing.T) {
		t.Parallel()

		manifest := testSlackManifestRecord()
		deps := newTestDeps(t, &stubClient{
			slackBridgeManifestFn: func(_ context.Context, instanceID string) (SlackManifestRecord, error) {
				if instanceID != "brg-slack" {
					t.Fatalf("SlackBridgeManifest() instance = %q, want brg-slack", instanceID)
				}
				return manifest, nil
			},
		})

		stdout, stderr, err := executeRootCommand(
			t,
			deps,
			"bridge",
			"manifest",
			"slack",
			"--instance",
			"brg-slack",
		)
		if err != nil {
			t.Fatalf("bridge manifest slack error = %v; stderr=%s", err, stderr)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}

		var decoded SlackManifestRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%s", err, stdout)
		}
		if decoded.Settings.EventSubscriptions.RequestURL !=
			"https://bridge.example.com/slack/support" {
			t.Fatalf("decoded manifest = %#v", decoded)
		}
		if strings.Contains(stdout, "paste into") {
			t.Fatalf("stdout contains write-only prose: %s", stdout)
		}
	})

	t.Run("Should write the requested output file and print the exact epilogue", func(t *testing.T) {
		t.Parallel()

		manifest := testSlackManifestRecord()
		deps := newTestDeps(t, &stubClient{
			slackBridgeManifestFn: func(context.Context, string) (SlackManifestRecord, error) {
				return manifest, nil
			},
		})
		outputPath := filepath.Join(t.TempDir(), "custom-slack-manifest.json")

		stdout, stderr, err := executeRootCommand(
			t,
			deps,
			"bridge",
			"manifest",
			"slack",
			"--instance",
			"brg-slack",
			"--write",
			"--out",
			outputPath,
		)
		if err != nil {
			t.Fatalf("bridge manifest slack --write error = %v; stderr=%s", err, stderr)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("stdout = %q, want empty in write mode", stdout)
		}
		if !strings.Contains(stderr, "paste into api.slack.com → From an app manifest") {
			t.Fatalf("stderr = %q, want exact paste epilogue", stderr)
		}

		payload, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", outputPath, err)
		}
		var decoded SlackManifestRecord
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(written manifest) error = %v; payload=%s", err, payload)
		}
		if decoded.DisplayInformation.Name != manifest.DisplayInformation.Name {
			t.Fatalf("written manifest = %#v, want name %q", decoded, manifest.DisplayInformation.Name)
		}
	})

	t.Run("Should write to the AGH home when no output override is set", func(t *testing.T) {
		t.Parallel()

		manifest := testSlackManifestRecord()
		deps := newTestDeps(t, &stubClient{
			slackBridgeManifestFn: func(context.Context, string) (SlackManifestRecord, error) {
				return manifest, nil
			},
		})
		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		deps.resolveHome = func() (aghconfig.HomePaths, error) { return homePaths, nil }
		deps.ensureHome = aghconfig.EnsureHomeLayout

		stdout, stderr, err := executeRootCommand(
			t,
			deps,
			"bridge",
			"manifest",
			"slack",
			"--instance",
			"brg-slack",
			"--write",
		)
		if err != nil {
			t.Fatalf("bridge manifest slack --write error = %v; stderr=%s", err, stderr)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("stdout = %q, want empty in write mode", stdout)
		}
		defaultPath := filepath.Join(homePaths.HomeDir, "slack-manifest.json")
		payload, err := os.ReadFile(defaultPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", defaultPath, err)
		}
		if !json.Valid(payload) {
			t.Fatalf("written default manifest is not valid JSON: %s", payload)
		}
		if !strings.Contains(stderr, defaultPath) ||
			!strings.Contains(stderr, "paste into api.slack.com → From an app manifest") {
			t.Fatalf("stderr = %q, want default path and exact epilogue", stderr)
		}
	})

	t.Run("Should reject an output override without write mode", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(
			t,
			deps,
			"bridge",
			"manifest",
			"slack",
			"--instance",
			"brg-slack",
			"--out",
			filepath.Join(t.TempDir(), "manifest.json"),
		)
		if err == nil || !strings.Contains(err.Error(), "--out requires --write") {
			t.Fatalf("bridge manifest slack --out error = %v, want --write remediation", err)
		}
	})
}

func testSlackManifestRecord() SlackManifestRecord {
	return bridgepkg.SlackAppManifest{
		Metadata: bridgepkg.SlackManifestMetadata{MajorVersion: 1, MinorVersion: 1},
		DisplayInformation: bridgepkg.SlackManifestDisplayInformation{
			Name:            "Support agent",
			Description:     "Connect Slack messages to AGH.",
			BackgroundColor: "#E8572A",
		},
		Features: bridgepkg.SlackManifestFeatures{
			BotUser: bridgepkg.SlackManifestBotUser{DisplayName: "Support agent"},
			SlashCommands: []bridgepkg.SlackManifestSlashCommand{{
				Command:     "/agh",
				URL:         "https://bridge.example.com/slack/support",
				Description: "Send a command to AGH",
			}},
		},
		OAuthConfig: bridgepkg.SlackManifestOAuthConfig{
			Scopes: bridgepkg.SlackManifestOAuthScopes{Bot: []string{"chat:write"}},
		},
		Settings: bridgepkg.SlackManifestSettings{
			EventSubscriptions: bridgepkg.SlackManifestEventSubscriptions{
				RequestURL: "https://bridge.example.com/slack/support",
				BotEvents:  []string{"message.channels"},
			},
			Interactivity: bridgepkg.SlackManifestInteractivity{
				IsEnabled:  true,
				RequestURL: "https://bridge.example.com/slack/support",
			},
		},
	}
}
