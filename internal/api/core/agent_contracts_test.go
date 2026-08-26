package core_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestAgentListPayloadsDistinguishesInactiveShadows(t *testing.T) {
	t.Parallel()

	t.Run("Should distinguish the active workspace definition from its inactive user shadow", func(t *testing.T) {
		t.Parallel()
		home, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		shadowPath := filepath.Join(home.AgentsDir, "reviewer", compozyconfig.AgentDefinitionFileName)
		if _, err := compozyconfig.CreateAgentDefFile(shadowPath, compozyconfig.AgentDefinitionDraft{
			Name: "reviewer", Description: "Global review", Provider: "codex", Prompt: "Review globally.",
		}, false); err != nil {
			t.Fatalf("CreateAgentDefFile(shadow) error = %v", err)
		}
		entries := []core.AgentCatalogEntry{{
			Def: compozyconfig.AgentDef{
				Name: "reviewer", Description: "Workspace review", Provider: "claude", Prompt: "Review locally.",
				SourceLayer: "project", ShadowedDefinitions: []compozyconfig.AgentDefinitionRef{{
					Layer: "user", Path: shadowPath,
				}},
			},
			Origin: contract.AgentOriginWorkspace, WorkspaceID: "ws-1",
		}}
		payloads := core.AgentListPayloads(entries, nil, home, "ws-1")
		if got, want := len(payloads), 2; got != want {
			t.Fatalf("AgentListPayloads() length = %d, want %d", got, want)
		}
		active, shadowed := payloads[0], payloads[1]
		if active.Shadowed {
			t.Fatal("active.Shadowed = true, want false")
		}
		if active.Scope != "workspace" || active.Description != "Workspace review" {
			t.Fatalf("active payload = %#v", active)
		}
		if !shadowed.Shadowed {
			t.Fatal("shadowed.Shadowed = false, want true")
		}
		if shadowed.Scope != "shadowed" || shadowed.Layer != "user" {
			t.Fatalf("shadowed ownership = scope %q layer %q", shadowed.Scope, shadowed.Layer)
		}
		if shadowed.Description != "Global review" {
			t.Fatalf("shadowed.Description = %q, want Global review", shadowed.Description)
		}
		if shadowed.DefinitionDigest == "" {
			t.Fatal("shadowed.DefinitionDigest is empty")
		}
	})
}

func TestAgentPayloadDoesNotExposeMCPSecretBindings(t *testing.T) {
	t.Run("Should project MCP bindings as redacted presence without Vault refs", func(t *testing.T) {
		t.Parallel()

		payload := core.AgentPayloadFromDef(compozyconfig.AgentDef{
			Name:     "reviewer",
			Provider: "codex",
			MCPServers: []compozyconfig.MCPServer{{
				Name:      "github",
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.github.example/mcp",
				SecretEnv: map[string]string{"GITHUB_TOKEN": "vault:mcp/global/github/env/GITHUB_TOKEN"},
				Auth: compozyconfig.MCPAuthConfig{
					Registration:    compozyconfig.MCPAuthRegistrationPreRegistered,
					IssuerURL:       "https://auth.github.example",
					ClientID:        "compozy-client",
					ClientSecretRef: "vault:mcp/global/github/oauth/client-secret",
				},
			}},
		})

		if got, want := payload.MCPServers[0].SecretEnv["GITHUB_TOKEN"], compozyconfig.RedactedValue(); got != want {
			t.Fatalf("MCP secret env projection = %q, want %q", got, want)
		}
		if !payload.MCPServers[0].Auth.ClientSecretConfigured {
			t.Fatal("MCP OAuth client secret configured = false, want presence metadata")
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal(agent payload) error = %v", err)
		}
		if strings.Contains(string(encoded), "vault:mcp/") {
			t.Fatalf("agent payload exposed MCP Vault binding: %s", encoded)
		}
	})
}

func TestAgentPayloadEffectiveRuntimeUsesRequestedWorkspaceConfig(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should project every effective runtime field and provenance from the requested workspace",
		testAgentPayloadEffectiveRuntime,
	)
}

func testAgentPayloadEffectiveRuntime(t *testing.T) {
	t.Helper()
	fast := true
	agent := compozyconfig.AgentDef{
		Name:   "reviewer",
		Prompt: "Review changes.",
	}
	agent.SetSpeed("fast")
	agent.SetACPOptions([]compozyconfig.ACPOptionSelection{
		{ID: "context", ValueID: "1m"},
		{ID: "thinking", BoolValue: &fast},
	})
	entry := core.AgentCatalogEntry{Def: agent, Origin: contract.AgentOriginGlobal}
	codexConfig := compozyconfig.Config{Defaults: compozyconfig.DefaultsConfig{Provider: "codex"}}
	claudeConfig := compozyconfig.Config{Defaults: compozyconfig.DefaultsConfig{Provider: "claude"}}

	codexPayload := core.AgentPayloadFromEntryWithConfig(entry, &codexConfig)
	claudePayload := core.AgentPayloadFromEntryWithConfig(entry, &claudeConfig)
	if codexPayload.EffectiveRuntime == nil || claudePayload.EffectiveRuntime == nil {
		t.Fatal("EffectiveRuntime = nil, want projections for both workspaces")
	}
	if got, want := codexPayload.EffectiveRuntime.Provider, "codex"; got != want {
		t.Fatalf("codex EffectiveRuntime.Provider = %q, want %q", got, want)
	}
	if got, want := claudePayload.EffectiveRuntime.Provider, "claude"; got != want {
		t.Fatalf("claude EffectiveRuntime.Provider = %q, want %q", got, want)
	}
	if codexPayload.Provider != "" || claudePayload.Provider != "" {
		t.Fatalf("authored Provider = %q/%q, want empty", codexPayload.Provider, claudePayload.Provider)
	}
	for name, payload := range map[string]contract.AgentPayload{
		"codex":  codexPayload,
		"claude": claudePayload,
	} {
		if payload.Speed != contract.SpeedFast || len(payload.ACPOptions) != 2 {
			t.Fatalf("%s authored runtime defaults = speed %q options %#v", name, payload.Speed, payload.ACPOptions)
		}
		if payload.ACPOptions[0].ID != "context" || payload.ACPOptions[0].ValueID != "1m" ||
			payload.ACPOptions[0].BoolValue != nil || payload.ACPOptions[1].ID != "thinking" ||
			payload.ACPOptions[1].BoolValue == nil || !*payload.ACPOptions[1].BoolValue {
			t.Fatalf("%s authored ACP options = %#v", name, payload.ACPOptions)
		}
		if payload.EffectiveRuntime == nil || payload.EffectiveRuntime.Speed != contract.SpeedFast ||
			len(payload.EffectiveRuntime.ACPOptions) != 2 {
			t.Fatalf("%s effective runtime defaults = %#v", name, payload.EffectiveRuntime)
		}
	}
	if got, want := codexPayload.EffectiveRuntime.Sources.Provider,
		contract.AgentRuntimeValueSourceProjectDefault; got != want {
		t.Fatalf("codex provider source = %q, want %q", got, want)
	}
	for name, projection := range map[string]struct {
		payload *contract.AgentEffectiveRuntimePayload
		config  *compozyconfig.Config
	}{
		"codex":  {payload: codexPayload.EffectiveRuntime, config: &codexConfig},
		"claude": {payload: claudePayload.EffectiveRuntime, config: &claudeConfig},
	} {
		resolved, err := projection.config.ResolveAgent(agent)
		if err != nil {
			t.Fatalf("%s ResolveAgent() error = %v", name, err)
		}
		if got, want := projection.payload.Model, resolved.Model; got != want {
			t.Fatalf("%s model = %q, want %q", name, got, want)
		}
		if got, want := string(projection.payload.ReasoningEffort), resolved.ReasoningEffort; got != want {
			t.Fatalf("%s reasoning = %q, want %q", name, got, want)
		}
		if got, want := string(projection.payload.Sources.Provider),
			resolved.RuntimeSources.Provider.String(); got != want {
			t.Fatalf("%s provider source = %q, want %q", name, got, want)
		}
		if got, want := string(projection.payload.Sources.Model),
			resolved.RuntimeSources.Model.String(); got != want {
			t.Fatalf("%s model source = %q, want %q", name, got, want)
		}
		if got, want := string(projection.payload.Sources.ReasoningEffort),
			resolved.RuntimeSources.ReasoningEffort.String(); got != want {
			t.Fatalf("%s reasoning source = %q, want %q", name, got, want)
		}
		if got, want := string(projection.payload.Sources.Speed),
			resolved.RuntimeSources.Speed.String(); got != want {
			t.Fatalf("%s speed source = %q, want %q", name, got, want)
		}
	}
}

func TestCoordinatorConfigPayloadFromConfig(t *testing.T) {
	t.Parallel()

	baseConfig := compozyconfig.ResolvedCoordinatorRole{
		Enabled:                       true,
		AgentName:                     " coordinator ",
		Provider:                      " codex ",
		Model:                         " gpt-5.4 ",
		TTL:                           90 * time.Minute,
		MaxChildren:                   5,
		MaxActiveSessionsPerWorkspace: 5,
	}

	tests := []struct {
		name        string
		cfg         compozyconfig.ResolvedCoordinatorRole
		source      contract.CoordinatorConfigSource
		workspaceID string
		assert      func(*testing.T, contract.CoordinatorConfigPayload)
	}{
		{
			name:        "Should trim coordinator string fields",
			cfg:         baseConfig,
			source:      contract.CoordinatorConfigSourceWorkspace,
			workspaceID: " ws-1 ",
			assert: func(t *testing.T, payload contract.CoordinatorConfigPayload) {
				t.Helper()
				if payload.AgentName != "coordinator" || payload.Provider != "codex" || payload.Model != "gpt-5.4" {
					t.Fatalf("trimmed fields = %q/%q/%q", payload.AgentName, payload.Provider, payload.Model)
				}
			},
		},
		{
			name:        "Should convert default TTL to seconds",
			cfg:         baseConfig,
			source:      contract.CoordinatorConfigSourceWorkspace,
			workspaceID: "ws-1",
			assert: func(t *testing.T, payload contract.CoordinatorConfigPayload) {
				t.Helper()
				if payload.DefaultTTLSeconds != 5400 {
					t.Fatalf("DefaultTTLSeconds = %d, want 5400", payload.DefaultTTLSeconds)
				}
			},
		},
		{
			name:        "Should map coordinator limits",
			cfg:         baseConfig,
			source:      contract.CoordinatorConfigSourceWorkspace,
			workspaceID: "ws-1",
			assert: func(t *testing.T, payload contract.CoordinatorConfigPayload) {
				t.Helper()
				if payload.MaxChildren != 5 || payload.MaxActiveSessionsPerWorkspace != 5 {
					t.Fatalf(
						"limits = %d/%d, want 5/5",
						payload.MaxChildren,
						payload.MaxActiveSessionsPerWorkspace,
					)
				}
			},
		},
		{
			name:        "Should preserve disabled configs",
			cfg:         compozyconfig.ResolvedCoordinatorRole{Enabled: false},
			source:      contract.CoordinatorConfigSourceDefault,
			workspaceID: "",
			assert: func(t *testing.T, payload contract.CoordinatorConfigPayload) {
				t.Helper()
				if payload.Enabled {
					t.Fatal("Enabled = true, want false")
				}
			},
		},
		{
			name:        "Should trim source workspace id",
			cfg:         baseConfig,
			source:      contract.CoordinatorConfigSourceWorkspace,
			workspaceID: " ws-1 ",
			assert: func(t *testing.T, payload contract.CoordinatorConfigPayload) {
				t.Helper()
				if payload.Source != contract.CoordinatorConfigSourceWorkspace || payload.WorkspaceID != "ws-1" {
					t.Fatalf("source/workspace = %q/%q, want workspace/ws-1", payload.Source, payload.WorkspaceID)
				}
			},
		},
		{
			name:        "Should keep empty workspace id empty",
			cfg:         baseConfig,
			source:      contract.CoordinatorConfigSourceDefault,
			workspaceID: " ",
			assert: func(t *testing.T, payload contract.CoordinatorConfigPayload) {
				t.Helper()
				if payload.WorkspaceID != "" {
					t.Fatalf("WorkspaceID = %q, want empty", payload.WorkspaceID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := core.CoordinatorConfigPayloadFromConfig(tt.cfg, tt.source, tt.workspaceID)
			tt.assert(t, payload)
		})
	}
}
