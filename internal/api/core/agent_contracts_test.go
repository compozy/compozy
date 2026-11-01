package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

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

func TestAgentPayloadProjectsFallbackChain(t *testing.T) { // UT-018
	t.Run("Should fingerprint exactly the routes that declare a command", func(t *testing.T) {
		t.Parallel()

		const command = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
		payload := core.AgentPayloadFromDef(compozyconfig.AgentDef{
			Name:     "reviewer",
			Provider: "claude",
			Prompt:   "Review.",
			FallbackChain: []compozyconfig.RoleFallback{
				{Provider: "claude", Model: "opus-4-8", Command: command},
				{Provider: "cursor", Model: "grok-4.6", ReasoningEffort: "high"},
			},
		})
		want := []contract.RoleFallbackStatus{
			{
				Provider: "claude", Model: "opus-4-8",
				Command: command, CommandFingerprint: compozyconfig.CommandFingerprint(command),
			},
			{Provider: "cursor", Model: "grok-4.6", ReasoningEffort: "high"},
		}
		if len(payload.FallbackChain) != len(want) ||
			payload.FallbackChain[0].Command != want[0].Command ||
			payload.FallbackChain[0].CommandFingerprint != want[0].CommandFingerprint ||
			!strings.HasPrefix(payload.FallbackChain[0].CommandFingerprint, "sha256:") ||
			payload.FallbackChain[1].CommandFingerprint != "" ||
			payload.FallbackChain[1].ReasoningEffort != "high" {
			t.Fatalf("AgentPayload.FallbackChain = %#v, want %#v", payload.FallbackChain, want)
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

func TestAgentAggregateOwnershipAndPagination(t *testing.T) {
	t.Parallel()
	t.Run("Should page same-named agents with metrics belonging to each workspace", func(t *testing.T) {
		t.Parallel()
		totals := map[string]int{workspaceaccess.UserSelector: 1, "ws-a": 2, "ws-b": 3}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				MetricsByAgentFn: func(_ context.Context, _ store.ReadScope, workspace string) (map[string]session.AgentSessionMetrics, error) {
					return map[string]session.AgentSessionMetrics{"same": {Total: totals[workspace]}}, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
					return []workspacepkg.Workspace{
						{ID: "ws-a", Name: "A"},
						{ID: "ws-b", Name: "B"},
						{ID: "ws-missing", RootMissing: true},
					}, nil
				},
				ResolveFn: func(_ context.Context, id string) (workspacepkg.ResolvedWorkspace, error) {
					if id == "ws-missing" {
						return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
					}
					return workspacepkg.ResolvedWorkspace{
						ID:          id,
						WorkspaceID: id,
						Agents:      []compozyconfig.AgentDef{{Name: "same", Provider: "codex"}},
					}, nil
				},
			},
		)
		path := filepath.Join(fixture.HomePaths.AgentsDir, "same", compozyconfig.AgentDefinitionFileName)
		if _, err := compozyconfig.CreateAgentDefFile(
			path,
			compozyconfig.AgentDefinitionDraft{Name: "same", Provider: "codex", Prompt: "test"},
			false,
		); err != nil {
			t.Fatal(err)
		}
		cursor := ""
		for i, workspace := range []string{"", "ws-a", "ws-b"} {
			response := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/agents/aggregate?all_workspaces=true&name=same&limit=1&cursor="+cursor,
				nil,
			)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var page contract.AgentCatalogResponse
			decodeJSON(t, response.Body.Bytes(), &page)
			if page.SessionsAvailable || len(page.Diagnostics) != 1 ||
				page.Diagnostics[0].Code != "workspace_root_missing" ||
				page.Diagnostics[0].Evidence["workspace_id"] != "ws-missing" {
				t.Fatalf("diagnostics = %#v", page.Diagnostics)
			}
			if len(page.Agents) != 1 || page.Agents[0].Agent.WorkspaceID != workspace ||
				page.Agents[0].Sessions == nil ||
				page.Agents[0].Sessions.Total != i+1 {
				t.Fatalf("page %d = %#v", i, page)
			}
			if page.Page.HasMore != (i < 2) {
				t.Fatalf("page has_more = %v", page.Page.HasMore)
			}
			cursor = page.Page.NextCursor
		}
	})
}

func TestAgentMemoryRoutes(t *testing.T) {
	t.Run("Should take ownership from the catalog when the workspace scan found the agent first", func(t *testing.T) {
		t.Parallel()
		def := compozyconfig.AgentDef{Name: "triage", Kind: compozyconfig.AgentKindStaysOn}
		resolve := func(context.Context, string, string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{ID: "ws-one", Agents: []compozyconfig.AgentDef{def}}, nil
		}
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, testutil.StubWorkspaceService{
			ResolveForProfileFn: resolve,
			ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return resolve(ctx, ref, "")
			},
		})
		fixture.Handlers.AgentMemory = agentmemory.NewStore(t.TempDir())
		fixture.Handlers.AgentCatalog = stubAgentCatalog{listForWorkspace: func(*workspacepkg.ResolvedWorkspace) ([]core.AgentCatalogEntry, error) {
			return []core.AgentCatalogEntry{{Def: def, MemoryOrigin: "user", PackageOwned: true}}, nil
		}}
		fixture.Handlers.RegisterAgentMemoryRoutes(fixture.Engine.Group("/agents"))
		response := testutil.PerformRequest(t, fixture.Engine, "GET", "/agents/triage/memory?workspace=ws-one", nil)
		if response.Code != 200 {
			t.Fatalf("status %d body %s", response.Code, response.Body.String())
		}
	})

	t.Run("Should persist notes through shared routes and reject stale edits", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		fixture.Handlers.AgentMemory = agentmemory.NewStore(t.TempDir())
		fixture.Handlers.AgentCatalog = stubAgentCatalog{listForWorkspace: func(*workspacepkg.ResolvedWorkspace) ([]core.AgentCatalogEntry, error) {
			return []core.AgentCatalogEntry{{Def: compozyconfig.AgentDef{Name: "triage", Kind: compozyconfig.AgentKindStaysOn}, MemoryOrigin: "user"}, {Def: compozyconfig.AgentDef{Name: "ordinary"}, MemoryOrigin: "user"}}, nil
		}}
		fixture.Handlers.RegisterAgentMemoryRoutes(fixture.Engine.Group("/agents"))
		for _, tc := range []struct {
			method, path, body string
			status             int
			contains           string
		}{
			{"GET", "/agents/triage/memory", "", 200, "max_lines"},
			{"POST", "/agents/triage/memory/entries", `{"text":"preview deploys"}`, 200, "preview deploys"},
			{"GET", "/agents/triage/memory", "", 200, "preview deploys"},
			{"PUT", "/agents/triage/memory", `{"content":"stale","expected_digest":"old"}`, 409, "agent_memory_conflict"},
			{"POST", "/agents/triage/memory/search", `{"query":"preview"}`, 200, "preview deploys"},
			{"PATCH", "/agents/triage/memory/entries", `{"match":"preview","text":"production"}`, 200, "production deploys"},
			{"GET", "/agents/triage/memory/history", "", 200, "revisions"},
			{"POST", "/agents/triage/memory/validate", `{"content":"valid"}`, 200, `"valid":true`},
			{"DELETE", "/agents/triage/memory/entries", `{"match":"production"}`, 200, `"removed":1`},
			{"GET", "/agents/ordinary/memory", "", 422, "agent_not_stays_on"},
			{"GET", "/agents/missing/memory", "", 404, "error"},
		} {
			response := testutil.PerformRequest(t, fixture.Engine, tc.method, tc.path, []byte(tc.body))
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.contains) {
				t.Fatalf("%s %s: status %d body %s", tc.method, tc.path, response.Code, response.Body.String())
			}
		}
	})
}
