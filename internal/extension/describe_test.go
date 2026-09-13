package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/diagnosticcontract"
)

func TestDescribeExtension(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 18, 30, 0, 0, time.UTC)
	tests := []struct {
		name       string
		extension  *Extension
		active     bool
		now        time.Time
		wantType   string
		wantState  string
		wantHealth string
		wantUptime int64
	}{
		{
			name: "Should report active subprocess runtime",
			extension: &Extension{
				Info: ExtensionInfo{
					Name:    "telegram-adapter",
					Version: "1.2.3",
					Source:  SourceUser,
					Enabled: true,
					Capabilities: CapabilitiesConfig{
						Provides: []string{"bridge.adapter"},
					},
					Permissions: PermissionsConfig{
						Requires: []string{"bridges/messages/ingest"},
					},
				},
				Status: ExtensionStatus{
					Active:        true,
					Healthy:       true,
					PID:           4242,
					LastStartedAt: now.Add(-15 * time.Minute),
				},
			},
			active:     true,
			now:        now,
			wantType:   "subprocess",
			wantState:  "active",
			wantHealth: "healthy",
			wantUptime: 900,
		},
		{
			name: "Should report registered resource health",
			extension: &Extension{
				Info: ExtensionInfo{
					Name:    "workspace-review",
					Version: "0.1.0",
					Source:  SourceWorkspace,
					Enabled: true,
				},
				Status: ExtensionStatus{
					Registered: true,
				},
			},
			active:     true,
			now:        now,
			wantType:   "resource",
			wantState:  "registered",
			wantHealth: "healthy",
			wantUptime: 0,
		},
		{
			name: "Should report missing environment requirements",
			extension: &Extension{
				Info: ExtensionInfo{
					Name:    "env-ext",
					Version: "0.1.0",
					Source:  SourceUser,
					Enabled: true,
				},
				Manifest: &Manifest{
					RequiresEnv: []string{"PRESENT_TOKEN", "MISSING_TOKEN"},
				},
				Status: ExtensionStatus{
					Registered:        true,
					MissingEnv:        []string{"MISSING_TOKEN"},
					MissingEnvChecked: true,
				},
			},
			active:     true,
			now:        now,
			wantType:   "resource",
			wantState:  "registered",
			wantHealth: "healthy",
			wantUptime: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := DescribeExtension(tt.extension, tt.active, tt.now)
			if payload.Type != tt.wantType {
				t.Fatalf("DescribeExtension().Type = %q, want %q", payload.Type, tt.wantType)
			}
			if payload.State != tt.wantState {
				t.Fatalf("DescribeExtension().State = %q, want %q", payload.State, tt.wantState)
			}
			if payload.Health != tt.wantHealth {
				t.Fatalf("DescribeExtension().Health = %q, want %q", payload.Health, tt.wantHealth)
			}
			if payload.UptimeSeconds != tt.wantUptime {
				t.Fatalf("DescribeExtension().UptimeSeconds = %d, want %d", payload.UptimeSeconds, tt.wantUptime)
			}
			if tt.name == "Should report missing environment requirements" {
				if got, want := payload.RequiresEnv, []string{"PRESENT_TOKEN", "MISSING_TOKEN"}; len(got) != len(want) {
					t.Fatalf("DescribeExtension().RequiresEnv = %#v, want %#v", got, want)
				}
				if got, want := payload.MissingEnv, []string{
					"MISSING_TOKEN",
				}; len(got) != len(want) ||
					got[0] != want[0] {
					t.Fatalf("DescribeExtension().MissingEnv = %#v, want %#v", got, want)
				}
			}
		})
	}
}

func TestDescribeExtensionProjectsFormatAndOrderedDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should project recorded diagnostics before provenance warnings", func(t *testing.T) {
		t.Parallel()

		recorded := diagnosticcontract.DiagnosticItem{ID: "recorded", Code: "recorded"}
		provenance := diagnosticcontract.DiagnosticItem{ID: "provenance", Code: "provenance"}
		payload := DescribeExtension(&Extension{Info: ExtensionInfo{
			Name: "portable", Format: FormatAgentPlugin,
			IngestDiagnostics: []diagnosticcontract.DiagnosticItem{recorded},
			Provenance:        ExtensionProvenance{Warnings: []diagnosticcontract.DiagnosticItem{provenance}},
		}}, false, time.Now())

		if payload.Format != string(FormatAgentPlugin) {
			t.Fatalf("DescribeExtension().Format = %q, want %q", payload.Format, FormatAgentPlugin)
		}
		if want := []diagnosticcontract.DiagnosticItem{
			recorded,
			provenance,
		}; !reflect.DeepEqual(
			payload.Diagnostics,
			want,
		) {
			t.Fatalf("DescribeExtension().Diagnostics = %#v, want %#v", payload.Diagnostics, want)
		}
	})

	t.Run("Should default legacy zero values to the native format", func(t *testing.T) {
		t.Parallel()

		payload := DescribeExtension(&Extension{Info: ExtensionInfo{Name: "native"}}, false, time.Now())
		if payload.Format != string(FormatCompozy) {
			t.Fatalf("DescribeExtension().Format = %q, want %q", payload.Format, FormatCompozy)
		}
	})
}

func TestDescribeExtensionProjectsCuratedArchiveIdentity(t *testing.T) {
	t.Parallel()
	t.Run("Should project distinct curated archive and installed tree identity", func(t *testing.T) {
		t.Parallel()

		payload := DescribeExtension(&Extension{Info: ExtensionInfo{
			Name: "curated", Version: "1.0.0", Source: SourceMarketplace,
			Provenance: ExtensionProvenance{
				Slug: "acme/curated", CatalogEntryID: "extension.acme.curated",
				InstalledFrom:  ExtensionInstalledFromMarketplace,
				ChecksumSHA256: "tree-digest", ArchiveDigestSHA256: "archive-digest",
				ChecksumVerified: true, RegistryTier: ExtensionRegistryTierOfficial,
			},
		}}, false, time.Now())
		if payload.Provenance == nil {
			t.Fatal("DescribeExtension().Provenance = nil")
		}
		if payload.Provenance.CatalogEntryID != "extension.acme.curated" ||
			payload.Provenance.ArchiveDigestSHA256 != "archive-digest" ||
			payload.Provenance.ChecksumSHA256 != "tree-digest" {
			t.Fatalf(
				"DescribeExtension().Provenance = %#v, want separate catalog/archive/tree identity",
				payload.Provenance,
			)
		}
		if payload.Provenance.Trust == nil || payload.Provenance.Trust.Decision != ExtensionTrustDecisionVerified {
			t.Fatalf("DescribeExtension().Provenance.Trust = %#v, want verified", payload.Provenance.Trust)
		}

		allowed := extensionProvenancePayload(ExtensionProvenance{
			InstalledFrom:   ExtensionInstalledFromLocalPath,
			RegistryTier:    ExtensionRegistryTierUnverified,
			AllowUnverified: true,
		})
		if allowed == nil || allowed.Trust == nil || allowed.Trust.Decision != ExtensionTrustDecisionAllowedUnverified {
			t.Fatalf("allowed side-load trust = %#v, want allowed_unverified", allowed)
		}
		blocked := extensionProvenancePayload(ExtensionProvenance{
			InstalledFrom: ExtensionInstalledFromLocalPath,
			RegistryTier:  ExtensionRegistryTierUnverified,
		})
		if blocked == nil || blocked.Trust == nil || blocked.Trust.Decision != ExtensionTrustDecisionBlocked {
			t.Fatalf("blocked side-load trust = %#v, want blocked", blocked)
		}
	})
}

// Invariant: installed contents count loaded package resources, independent of enabled state.
// Owner: extension snapshot projection; canonical suite: describe_test.go (UT-023).
func TestExtensionContents(t *testing.T) {
	t.Parallel()
	t.Run("Should count the illustrative bridge kit without counting resource directories", func(t *testing.T) {
		t.Parallel()
		manifest := &Manifest{
			Bridge:    BridgeConfig{Platform: "herdr"},
			Resources: ResourcesConfig{Skills: []ManifestResourcePath{{Path: "skills"}}},
		}
		kit := []KitItem{
			{Kind: skillspkg.SkillResourceKind, Name: "orchestration"},
			{Kind: skillspkg.SkillResourceKind, Name: "terminal"},
		}
		want := contract.ExtensionContentsPayload{Skills: 2, Bridges: 1}
		if got := ContentsFor(manifest, kit); got != want {
			t.Fatalf("ContentsFor() = %#v, want %#v", got, want)
		}
	})
	t.Run("Should count the current catalog package instead of the illustrative fixture", func(t *testing.T) {
		t.Parallel()
		manifest, err := LoadManifest(filepath.Join("..", "..", "catalog", "packages", "herdr-bridge"))
		if err != nil {
			t.Fatal(err)
		}
		want := contract.ExtensionContentsPayload{Hooks: 18}
		if got := ContentsFor(manifest, nil); got != want {
			t.Fatalf("ContentsFor() = %#v, want %#v", got, want)
		}
	})
	t.Run("Should report zero contents for an empty manifest", func(t *testing.T) {
		t.Parallel()
		for _, manifest := range []*Manifest{nil, {}} {
			if got := ContentsFor(manifest, nil); got != (contract.ExtensionContentsPayload{}) {
				t.Fatalf("ContentsFor() = %#v", got)
			}
		}
	})
	t.Run("Should count each shipped resource once across inventory placements", func(t *testing.T) {
		t.Parallel()
		kit := []KitItem{
			{
				Kind: skillspkg.SkillResourceKind,
				Name: "review",
			},
			{Kind: skillspkg.SkillResourceKind, Name: "review", Live: true},
			{Kind: looppkg.ResourceKind, Name: "review"},
			{Kind: compozyconfig.AgentResourceKind, Name: "review"},
			{Kind: resources.ResourceKind("agent.soul"), Name: "review"},
		}
		manifest := &Manifest{
			Resources: ResourcesConfig{
				MCPServers: map[string]MCPServerConfig{"git": {}},
				Hooks:      []HookConfig{{Name: "turn"}},
			},
		}
		want := contract.ExtensionContentsPayload{Skills: 1, Loops: 1, Agents: 1, MCPServers: 1, Hooks: 1}
		if got := ContentsFor(manifest, kit); got != want {
			t.Fatalf("ContentsFor() = %#v, want %#v", got, want)
		}
	})
	t.Run("Should project loaded resources for a disabled installed extension", func(t *testing.T) {
		t.Parallel()
		ext := &Extension{Info: ExtensionInfo{Name: "kit"}, Manifest: &Manifest{},
			Skills: []*skillspkg.Skill{{Meta: skillspkg.SkillMeta{Name: "review"}}},
			Loops:  []looppkg.ResourceSpec{{Name: "review"}}, Agents: []compozyconfig.AgentDef{{Name: "review"}},
		}
		want := contract.ExtensionContentsPayload{Skills: 1, Loops: 1, Agents: 1}
		payload := DescribeExtension(ext, true, time.Time{})
		if payload.Contents != want || payload.Enabled {
			t.Fatalf("DescribeExtension() = %#v", payload)
		}
	})
}

// Invariant: only a complete recorded acquisition identity classifies an installed origin.
// Owner: extension snapshot projection; canonical suite: describe_test.go.
func TestDescribeExtensionOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		provenance ExtensionProvenance
		want       *contract.MarketplaceOriginPayload
	}{
		{
			name:       "Should leave sideloads unclassified",
			provenance: ExtensionProvenance{Slug: "compozy/git", CatalogEntryID: "git"},
		},
		{
			name:       "Should leave incomplete identity unclassified",
			provenance: ExtensionProvenance{SourceName: "team", EntryID: "git"},
		},
		{
			name:       "Should preserve origin independently of the instance and display names",
			provenance: ExtensionProvenance{SourceName: "team", SourceRef: "github:team/plugins", EntryID: "git"},
			want: &contract.MarketplaceOriginPayload{
				Source:    "team",
				SourceRef: "github:team/plugins",
				EntryID:   "git",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DescribeExtension(
				&Extension{Info: ExtensionInfo{Name: "other", Provenance: tt.provenance}},
				true,
				time.Time{},
			).Origin
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Origin = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// Invariant: server metadata respects profile placement and never includes launch secrets.
// Owner: extension payload projection; canonical suite: describe_test.go (UT-023).
func TestDescribeExtensionServers(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should project sorted owned servers with safe launch summaries for the selected profile",
		func(t *testing.T) {
			t.Parallel()
			ext := &Extension{Info: ExtensionInfo{Name: "kit", Enabled: true},
				Status: ExtensionStatus{WorkspaceID: "workspace-a"},
				Manifest: &Manifest{Resources: ResourcesConfig{
					MCPServers: map[string]MCPServerConfig{
						"remote": {
							Profile: "work",
							URL:     "https://user:password@example.com/private-token?key=query-secret",
							Headers: map[string]string{"Authorization": "header-secret"},
							Auth: &MCPServerAuthConfig{
								Method:       "oauth",
								Registration: "dynamic",
								IssuerURL:    "https://issuer.example.com",
								Scopes:       []string{"read"},
							},
						},
						"local": {
							Profile: "work",
							Command: "/opt/tools/server",
							Args:    []string{"--token=argument-secret"},
							Env:     map[string]string{"TOKEN": "env-secret"},
						},
						"personal": {Profile: "personal", Command: "hidden-server"},
					}, Hooks: []HookConfig{{Name: "work-hook", Profile: "work"}, {Name: "personal-hook", Profile: "personal"}},
				}},
			}
			payload := DescribeExtensionForProfile(ext, true, time.Time{}, "work")
			if payload.Contents.MCPServers != 2 || payload.Contents.Hooks != 1 || len(payload.MCPServers) != 2 {
				t.Fatalf("contents=%#v servers=%#v", payload.Contents, payload.MCPServers)
			}
			if payload.MCPServers[0].Name != "local" || payload.MCPServers[0].Launch != "server" ||
				payload.MCPServers[1].Launch != "https://example.com" {
				t.Fatalf("server summaries=%#v", payload.MCPServers)
			}
			for _, server := range payload.MCPServers {
				if server.Owner != "extension:kit" || server.Scope != "workspace" || server.Status != "unknown" {
					t.Fatalf("server=%#v", server)
				}
			}
			if auth := payload.MCPServers[1].Auth; auth == nil || auth.Method != "oauth" ||
				auth.Registration != "dynamic" ||
				auth.IssuerURL != "https://issuer.example.com" ||
				!reflect.DeepEqual(auth.Scopes, []string{"read"}) {
				t.Fatalf("auth metadata missing: %#v", auth)
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"password", "private-token", "query-secret", "header-secret", "argument-secret", "env-secret", "hidden-server"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("payload leaked %q", secret)
				}
			}
			if payload.Inputs == nil || payload.MissingInputs == nil {
				t.Fatal("unconfigured inputs must be empty arrays")
			}
			ext.Info.Enabled = false
			disabled := DescribeExtensionForProfile(ext, true, time.Time{}, "work")
			if disabled.MCPServers[0].Status != "disabled" {
				t.Fatalf("disabled servers=%#v", disabled.MCPServers)
			}
			if len(ext.Manifest.Resources.MCPServers) != 3 || len(ext.Manifest.Resources.Hooks) != 2 {
				t.Fatal("projection mutated the original manifest")
			}
		},
	)
}

// Invariant: offline contents count actual declared package resources and honor profile placement.
// Owner: extension package inspection; canonical suite: describe_test.go (UT-023).
func TestExtensionContentsOffline(t *testing.T) {
	t.Parallel()
	t.Run("Should inspect disabled package contents without executing its missing runtime command", func(t *testing.T) {
		t.Parallel()
		fixture := createManagerTestExtension(t, `[extension]
name = "offline-kit"
version = "1.0.0"
description = "Offline package"
min_compozy_version = "0.5.0"
[entrypoint]
command = "this-command-does-not-exist"
[[resources.skills]]
path = "skills/shared"
[[resources.skills]]
path = "skills/work"
profile = "work"
[[resources.agents]]
path = "agents"
[[resources.loops]]
path = "loops"
`, map[string]string{
			"skills/shared/one/SKILL.md": managerSkillFile("one", "Shared"),
			"skills/shared/two/SKILL.md": managerSkillFile("two", "Shared"),
			"skills/work/three/SKILL.md": managerSkillFile("three", "Work"),
			"agents/coder/AGENT.md":      managerAgentFile("coder"),
			"loops/review/LOOP.yaml":     extensionLoopYAML("review"),
		})
		ext := &Extension{Info: ExtensionInfo{Name: "offline-kit", Enabled: false,
			ManifestPath: filepath.Join(fixture.dir, manifestTOMLFileName)}, Manifest: fixture.manifest}
		for _, profile := range []string{"default", "work"} {
			got, err := InspectPackageContents(t.Context(), ext, profile)
			if err != nil {
				t.Fatal(err)
			}
			wantSkills := 2
			if profile == "work" {
				wantSkills = 3
			}
			want := contract.ExtensionContentsPayload{Skills: wantSkills, Agents: 1, Loops: 1}
			if got != want {
				t.Fatalf("profile=%s contents=%#v want=%#v", profile, got, want)
			}
		}
		if len(ext.Skills) != 0 || len(ext.Manifest.Resources.Skills) != 2 {
			t.Fatal("inspection mutated the snapshot")
		}
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := InspectPackageContents(canceled, ext, "default"); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled error=%v", err)
		}
		if err := os.WriteFile(
			filepath.Join(fixture.dir, "skills/shared/one/SKILL.md"),
			[]byte("invalid skill"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectPackageContents(t.Context(), ext, "default"); err == nil {
			t.Fatal("corrupt declared skill was silently counted")
		}
	})
}
