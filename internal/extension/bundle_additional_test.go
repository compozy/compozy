package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/windowmanager"
)

func TestLoadBundleSpecsLoadsMixedFormatsAndSorts(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	bundlesDir := filepath.Join(rootDir, "bundles")
	if err := os.MkdirAll(bundlesDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(bundlesDir, "zeta.toml"), []byte(`
name = " Zeta "
description = " Team bundle "

[[profiles]]
name = " default "

[profiles.channels]
primary = " ops "

[[profiles.channels.items]]
name = " ops "
description = " Operations "
`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(zeta.toml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundlesDir, "alpha.json"), []byte(`{
		"bundle": {
			"name": " Alpha ",
			"description": " Alerts bundle ",
			"profiles": [{
				"name": " default ",
				"channels": {
					"primary": " alerts ",
					"items": [{
						"name": " alerts ",
						"description": " Alerts channel "
					}]
				}
			}]
		}
	}`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(alpha.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundlesDir, "ignore.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(ignore.txt) error = %v", err)
	}

	bundles, err := LoadBundleSpecs(context.Background(), rootDir, &Manifest{
		Name: "bundle-loader",
		Resources: ResourcesConfig{
			Bundles: []string{"bundles"},
		},
	})
	if err != nil {
		t.Fatalf("LoadBundleSpecs() error = %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("len(bundles) = %d, want 2", len(bundles))
	}

	gotNames := []string{bundles[0].Name, bundles[1].Name}
	if !slices.Equal(gotNames, []string{"Alpha", "Zeta"}) {
		t.Fatalf("bundle names = %#v, want sorted trimmed names", gotNames)
	}
	if bundles[0].Profiles[0].Channels.Primary != "alerts" {
		t.Fatalf("alpha primary channel = %q, want alerts", bundles[0].Profiles[0].Channels.Primary)
	}
	if bundles[1].Profiles[0].Channels.Items[0].Description != "Operations" {
		t.Fatalf(
			"zeta channel description = %q, want Operations",
			bundles[1].Profiles[0].Channels.Items[0].Description,
		)
	}
}

func TestLoadBundleSpecsLoadsProfileAgentsWithSidecars(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "bundles"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(bundles) error = %v", err)
	}
	agentDir := filepath.Join(rootDir, "agents", "marketer")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(agentDir) error = %v", err)
	}
	writeFile(t, filepath.Join(rootDir, "bundles", "marketing.toml"), `
name = "marketing"

[[profiles]]
name = "default"

[[profiles.agents]]
path = "agents/marketer"
`)
	writeFile(t, filepath.Join(agentDir, "AGENT.md"), `---
name: marketer
provider: claude
model: sonnet
---

Run marketing workflows.
`)
	writeFile(t, filepath.Join(agentDir, "mcp.json"), `{
  "mcpServers": {
    "linear": {
      "command": "linear-mcp"
    }
  }
}`)
	writeFile(t, filepath.Join(agentDir, "capabilities.toml"), `
[[capabilities]]
id = "plan-campaign"
summary = "Plan a campaign."
outcome = "A campaign plan."
version = "1.0.0"
execution_outline = ["inspect", "plan"]
requirements = ["workspace-write"]
`)
	writeFile(t, filepath.Join(agentDir, "SOUL.md"), "Lead with campaign context.")
	writeFile(t, filepath.Join(agentDir, "HEARTBEAT.md"), "Inspect campaigns and use AGH task APIs.")

	bundles, err := LoadBundleSpecs(context.Background(), rootDir, &Manifest{
		Name: "bundle-loader",
		Resources: ResourcesConfig{
			Bundles: []string{"bundles"},
		},
	})
	if err != nil {
		t.Fatalf("LoadBundleSpecs() error = %v", err)
	}
	if got, want := len(bundles), 1; got != want {
		t.Fatalf("len(bundles) = %d, want %d", got, want)
	}
	agents := bundles[0].Profiles[0].Agents
	if got, want := len(agents), 1; got != want {
		t.Fatalf("len(profile.Agents) = %d, want %d", got, want)
	}
	agent := agents[0]
	if got, want := agent.Path, "agents/marketer"; got != want {
		t.Fatalf("agent.Path = %q, want %q", got, want)
	}
	if got, want := agent.Agent.Name, "marketer"; got != want {
		t.Fatalf("agent.Agent.Name = %q, want %q", got, want)
	}
	if got, want := len(agent.Agent.MCPServers), 1; got != want {
		t.Fatalf("len(agent.Agent.MCPServers) = %d, want %d", got, want)
	}
	if agent.Agent.Capabilities == nil || len(agent.Agent.Capabilities.Capabilities) != 1 {
		t.Fatalf("agent.Agent.Capabilities = %#v, want one capability", agent.Agent.Capabilities)
	}
	if agent.Soul == nil || agent.Soul.SourcePath != "agents/marketer/SOUL.md" {
		t.Fatalf("agent.Soul = %#v, want packaged SOUL sidecar", agent.Soul)
	}
	if agent.Heartbeat == nil || agent.Heartbeat.SourcePath != "agents/marketer/HEARTBEAT.md" {
		t.Fatalf("agent.Heartbeat = %#v, want packaged HEARTBEAT sidecar", agent.Heartbeat)
	}
}

func TestLoadBundleSpecsLoadsStrictProfileLayouts(t *testing.T) {
	t.Parallel()

	t.Run("Should load and canonicalize strict JSON profile layouts", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(rootDir, "bundles"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(bundles) error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(rootDir, "layouts"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(layouts) error = %v", err)
		}
		writeBundleWithLayouts(t, rootDir, []string{"layouts/two-up.json"})
		writeJSONFile(t, filepath.Join(rootDir, "layouts", "two-up.json"), testBundleLayoutResource("two-up"))

		bundles, err := LoadBundleSpecs(context.Background(), rootDir, &Manifest{
			Name: "bundle-loader",
			Resources: ResourcesConfig{
				Bundles: []string{"bundles"},
			},
		})
		if err != nil {
			t.Fatalf("LoadBundleSpecs() error = %v", err)
		}
		layouts := bundles[0].Profiles[0].Layouts
		if got, want := len(layouts), 1; got != want {
			t.Fatalf("len(profile.Layouts) = %d, want %d", got, want)
		}
		if got, want := layouts[0].Path, "layouts/two-up.json"; got != want {
			t.Fatalf("layout.Path = %q, want %q", got, want)
		}
		if got, want := layouts[0].Layout.ID, "two-up"; got != want {
			t.Fatalf("layout.ID = %q, want %q", got, want)
		}
		if got, want := layouts[0].Layout.AspectVariant, windowmanager.LayoutAspectAny; got != want {
			t.Fatalf("layout.AspectVariant = %q, want %q", got, want)
		}
	})
}

func TestLoadBundleSpecsRejectsEscapingOrNonStrictProfileLayouts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		layoutPath string
		want       string
		setup      func(*testing.T, string, string)
	}{
		{
			name:       "Should reject lexical traversal outside the extension root",
			layoutPath: "../outside.json",
			want:       "escapes extension root",
			setup: func(t *testing.T, _ string, containerDir string) {
				t.Helper()
				writeJSONFile(t, filepath.Join(containerDir, "outside.json"), testBundleLayoutResource("outside"))
			},
		},
		{
			name:       "Should reject symlinks that resolve outside the extension root",
			layoutPath: "layouts/escape.json",
			want:       "escapes extension root",
			setup: func(t *testing.T, rootDir string, containerDir string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(rootDir, "layouts"), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(layouts) error = %v", err)
				}
				outside := filepath.Join(containerDir, "outside.json")
				writeJSONFile(t, outside, testBundleLayoutResource("outside"))
				if err := os.Symlink(outside, filepath.Join(rootDir, "layouts", "escape.json")); err != nil {
					t.Fatalf("os.Symlink() error = %v", err)
				}
			},
		},
		{
			name:       "Should reject unknown JSON fields",
			layoutPath: "layouts/unknown.json",
			want:       "unknown field",
			setup: func(t *testing.T, rootDir string, _ string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(rootDir, "layouts"), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(layouts) error = %v", err)
				}
				writeFile(
					t,
					filepath.Join(rootDir, "layouts", "unknown.json"),
					`{"version":1,"id":"unknown","display_name":"Unknown",`+
						`"aspect_variant":"any","overflow_policy":"stack",`+
						`"document":{"version":2,"workspace_id":"","desktops":[],`+
						`"windows":{},"overrides":{}},"legacy":true}`,
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			containerDir := t.TempDir()
			rootDir := filepath.Join(containerDir, "extension")
			if err := os.MkdirAll(filepath.Join(rootDir, "bundles"), 0o755); err != nil {
				t.Fatalf("os.MkdirAll(bundles) error = %v", err)
			}
			tc.setup(t, rootDir, containerDir)
			writeBundleWithLayouts(t, rootDir, []string{tc.layoutPath})

			_, err := LoadBundleSpecs(context.Background(), rootDir, &Manifest{
				Name: "bundle-loader",
				Resources: ResourcesConfig{
					Bundles: []string{"bundles"},
				},
			})
			if !errors.Is(err, ErrBundleInvalid) {
				t.Fatalf("LoadBundleSpecs() error = %v, want ErrBundleInvalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadBundleSpecs() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBundleJobValidateUsesNormalizedParticipation(t *testing.T) {
	t.Parallel()

	mode := participation.ModeLive
	strategy := participation.ChannelStrategy(" named ")
	channel := " ops "
	job := BundleJob{
		Name: "daily-digest", AgentName: "planner", Prompt: "Summarize incidents.",
		Schedule: automationpkg.ScheduleSpec{
			Mode: automationpkg.ScheduleModeEvery, Interval: "1m",
		},
		Task: &automationpkg.JobTaskConfig{NetworkParticipation: &participation.Request{
			Mode: &mode, ChannelStrategy: &strategy, ChannelID: &channel,
		}},
		Retry: automationpkg.DefaultRetryConfig(), FireLimit: automationpkg.DefaultFireLimitConfig(),
	}

	err := job.Validate("operations", "default", map[string]struct{}{"alerts": {}})
	if !errors.Is(err, ErrBundleInvalid) || !strings.Contains(err.Error(), "undeclared channel \"ops\"") {
		t.Fatalf("BundleJob.Validate() error = %v, want normalized undeclared channel", err)
	}
}

func TestLoadBundleSpecsRejectsInvalidProfileAgents(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{
			name: "Should reject missing AGENT.md",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(rootDir, "agents", "missing"), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(missing) error = %v", err)
				}
				writeBundleWithAgents(t, rootDir, []string{"agents/missing"})
			},
		},
		{
			name: "Should reject absolute profile agent path",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				writeBundleWithAgents(t, rootDir, []string{filepath.Join(rootDir, "agents", "absolute")})
			},
		},
		{
			name: "Should reject duplicate profile agent names",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				writeProfileAgent(t, rootDir, "agents/alpha", "shared", "")
				writeProfileAgent(t, rootDir, "agents/beta", "shared", "")
				writeBundleWithAgents(t, rootDir, []string{"agents/alpha", "agents/beta"})
			},
		},
		{
			name: "Should reject invalid packaged sidecar",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				writeProfileAgent(
					t,
					rootDir,
					"agents/invalid-sidecar",
					"invalid-sidecar",
					"---\nprovider: hidden\n---\nNope.",
				)
				writeBundleWithAgents(t, rootDir, []string{"agents/invalid-sidecar"})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(rootDir, "bundles"), 0o755); err != nil {
				t.Fatalf("os.MkdirAll(bundles) error = %v", err)
			}
			tc.setup(t, rootDir)

			_, err := LoadBundleSpecs(context.Background(), rootDir, &Manifest{
				Name: "bundle-loader",
				Resources: ResourcesConfig{
					Bundles: []string{"bundles"},
				},
			})
			if !errors.Is(err, ErrBundleInvalid) {
				t.Fatalf("LoadBundleSpecs() error = %v, want ErrBundleInvalid", err)
			}
		})
	}
}

func TestBundleDocumentToBundleSpecNormalizesValuesAndDefaults(t *testing.T) {
	t.Parallel()

	disabled := false
	doc := bundleDocument{
		Bundle: bundleRawSpec{
			Name:        " Marketing ",
			Description: " Team bundle ",
			Profiles: []bundleRawProfile{{
				Name:        " default ",
				Description: " Primary profile ",
				Channels: BundleChannelsConfig{
					Primary: " ops ",
					Items: []BundleChannel{{
						Name:        " ops ",
						Description: " Operations ",
					}},
				},
				Jobs: []bundleRawJob{{
					Name:      " daily-digest ",
					AgentName: " planner ",
					Prompt:    " summarize incidents ",
					Schedule: automationpkg.ScheduleSpec{
						Mode:     automationpkg.ScheduleModeEvery,
						Interval: "1m",
					},
					Task: &automationpkg.JobTaskConfig{
						NetworkParticipation: bundleNamedParticipation("ops"),
					},
					Retry:     automationpkg.DefaultRetryConfig(),
					FireLimit: automationpkg.DefaultFireLimitConfig(),
				}, {
					Name:      " disabled-job ",
					AgentName: " planner ",
					Prompt:    " summarize incidents ",
					Enabled:   &disabled,
					Schedule: automationpkg.ScheduleSpec{
						Mode:     automationpkg.ScheduleModeEvery,
						Interval: "5m",
					},
					Retry:     automationpkg.DefaultRetryConfig(),
					FireLimit: automationpkg.DefaultFireLimitConfig(),
				}},
				Triggers: []bundleRawTrigger{{
					Name:         " mention-alert ",
					AgentName:    " planner ",
					Prompt:       " triage this ",
					Event:        "message.created",
					Filter:       map[string]string{"team": "ops"},
					Retry:        automationpkg.DefaultRetryConfig(),
					FireLimit:    automationpkg.DefaultFireLimitConfig(),
					EndpointSlug: " /alerts ",
				}},
				Bridges: []BundleBridgePreset{{
					Name:             " telegram-main ",
					ExtensionName:    " bundled.bridge ",
					DisplayName:      " Marketing Bridge ",
					DeliveryDefaults: json.RawMessage(`{"mode":"safe"}`),
					SecretSlots: []BundleBridgeSecretSlot{{
						Name:        " bot_token ",
						Kind:        " api_token ",
						Description: " Bot token ",
					}},
				}},
			}},
		},
	}

	spec, err := doc.toBundleSpec(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("toBundleSpec() error = %v", err)
	}
	if spec.Name != "Marketing" {
		t.Fatalf("spec.Name = %q, want Marketing", spec.Name)
	}
	if spec.Description != "Team bundle" {
		t.Fatalf("spec.Description = %q, want Team bundle", spec.Description)
	}

	profile := spec.Profiles[0]
	if profile.Name != "default" {
		t.Fatalf("profile.Name = %q, want default", profile.Name)
	}
	if profile.Channels.Primary != "ops" {
		t.Fatalf("profile.Channels.Primary = %q, want ops", profile.Channels.Primary)
	}
	if !profile.Jobs[0].Enabled {
		t.Fatalf("jobs[0].Enabled = false, want true default")
	}
	if profile.Jobs[1].Enabled {
		t.Fatalf("jobs[1].Enabled = true, want explicit false")
	}
	if !profile.Triggers[0].Enabled {
		t.Fatalf("triggers[0].Enabled = false, want true default")
	}
	if profile.Triggers[0].EndpointSlug != "/alerts" {
		t.Fatalf("triggers[0].EndpointSlug = %q, want /alerts", profile.Triggers[0].EndpointSlug)
	}
	if profile.Bridges[0].SecretSlots[0].Kind != "api_token" {
		t.Fatalf("bridges[0].SecretSlots[0].Kind = %q, want api_token", profile.Bridges[0].SecretSlots[0].Kind)
	}

	*profile.Jobs[0].Task.NetworkParticipation.ChannelID = "changed"
	if got := *doc.Bundle.Profiles[0].Jobs[0].Task.NetworkParticipation.ChannelID; got != "ops" {
		t.Fatalf("raw job task mutated to %#v", doc.Bundle.Profiles[0].Jobs[0].Task)
	}

	profile.Triggers[0].Filter["team"] = "security"
	if doc.Bundle.Profiles[0].Triggers[0].Filter["team"] != "ops" {
		t.Fatalf("raw trigger filter mutated to %#v", doc.Bundle.Profiles[0].Triggers[0].Filter)
	}

	profile.Bridges[0].SecretSlots[0].Name = "changed"
	if doc.Bundle.Profiles[0].Bridges[0].SecretSlots[0].Name != " bot_token " {
		t.Fatalf("raw bridge secret slot mutated to %#v", doc.Bundle.Profiles[0].Bridges[0].SecretSlots)
	}
}

func bundleNamedParticipation(channelID string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channelID,
	}
}

func writeBundleWithAgents(t *testing.T, rootDir string, paths []string) {
	t.Helper()

	var builder strings.Builder
	builder.WriteString("name = \"marketing\"\n\n[[profiles]]\nname = \"default\"\n")
	for _, path := range paths {
		builder.WriteString("\n[[profiles.agents]]\n")
		builder.WriteString("path = ")
		encoded, err := json.Marshal(path)
		if err != nil {
			t.Fatalf("json.Marshal(path) error = %v", err)
		}
		builder.WriteString(string(encoded))
		builder.WriteByte('\n')
	}
	writeFile(t, filepath.Join(rootDir, "bundles", "marketing.toml"), builder.String())
}

func writeBundleWithLayouts(t *testing.T, rootDir string, paths []string) {
	t.Helper()

	var builder strings.Builder
	builder.WriteString("name = \"marketing\"\n\n[[profiles]]\nname = \"default\"\n")
	for _, path := range paths {
		builder.WriteString("\n[[profiles.layouts]]\n")
		builder.WriteString("path = ")
		encoded, err := json.Marshal(path)
		if err != nil {
			t.Fatalf("json.Marshal(path) error = %v", err)
		}
		builder.WriteString(string(encoded))
		builder.WriteByte('\n')
	}
	writeFile(t, filepath.Join(rootDir, "bundles", "marketing.toml"), builder.String())
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
}

func testBundleLayoutResource(id string) windowmanager.LayoutResource {
	return windowmanager.LayoutResource{
		Version:          windowmanager.LayoutResourceVersion,
		ID:               id,
		DisplayName:      "Two up",
		ParticipantSlots: []windowmanager.WindowID{"primary", "secondary"},
		Document: windowmanager.LayoutDocument{
			Version: windowmanager.SnapshotVersion,
			Desktops: []windowmanager.Desktop{{
				ID:       "desktop-default",
				Name:     "Desktop 1",
				Purpose:  windowmanager.DesktopPurposeStandard,
				Groups:   []windowmanager.LayoutGroup{},
				Floating: []windowmanager.WindowID{},
			}},
			Windows: map[windowmanager.WindowID]windowmanager.Window{},
		},
	}
}

func writeProfileAgent(t *testing.T, rootDir string, relPath string, name string, soulBody string) {
	t.Helper()

	agentDir := filepath.Join(rootDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(agentDir) error = %v", err)
	}
	writeFile(t, filepath.Join(agentDir, "AGENT.md"), `---
name: `+name+`
---

Run profile work.
`)
	if soulBody != "" {
		writeFile(t, filepath.Join(agentDir, "SOUL.md"), soulBody)
	}
}

func TestBundleDocumentToBundleSpecRejectsConflictingProfileDeclarations(t *testing.T) {
	t.Parallel()

	_, err := (bundleDocument{
		Profiles: []bundleRawProfile{{Name: "root"}},
		Bundle: bundleRawSpec{
			Profiles: []bundleRawProfile{{Name: "bundle"}},
		},
	}).toBundleSpec(context.Background(), t.TempDir())
	if !errors.Is(err, ErrBundleInvalid) {
		t.Fatalf("toBundleSpec() error = %v, want ErrBundleInvalid", err)
	}
}
