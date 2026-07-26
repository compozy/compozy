//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

func TestRoleResolverIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve every role from built-in defaults without a user config", func(t *testing.T) {
		t.Parallel()

		homePaths := e2etest.NewHomePaths(t)
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		resolver := newRoleResolver(&cfg, nil, nil)
		defaults := aghconfig.DefaultRolesConfig()

		for _, testCase := range []struct {
			role    aghconfig.RoleName
			agent   string
			enabled bool
			builtin bool
			inherit bool
			model   string
		}{
			{role: aghconfig.RoleCoordinator, agent: aghconfig.BuiltinCoordinatorAgentName, builtin: true},
			{role: aghconfig.RoleDream, agent: aghconfig.BuiltinDreamingCuratorAgentName, enabled: true, builtin: true},
			{role: aghconfig.RoleCheckpointSummary, agent: aghconfig.BuiltinDreamingCuratorAgentName, enabled: true, builtin: true},
			{role: aghconfig.RoleMemoryExtractor, enabled: true, inherit: true},
			{role: aghconfig.RoleAutoTitle, enabled: true, inherit: true},
			{role: aghconfig.RoleMemoryController, enabled: defaults.MemoryController.Enabled, model: defaults.MemoryController.Model},
		} {
			t.Run("Should resolve "+string(testCase.role), func(t *testing.T) {
				t.Parallel()

				resolved, resolveErr := resolver.Resolve(t.Context(), "", testCase.role)
				if resolveErr != nil {
					t.Fatalf("Resolve(%s) error = %v", testCase.role, resolveErr)
				}
				if resolved.Enabled != testCase.enabled || resolved.AgentName != testCase.agent ||
					resolved.Builtin != testCase.builtin || resolved.Inherit != testCase.inherit ||
					resolved.Model != testCase.model {
					t.Fatalf("Resolve(%s) = %#v, want enabled=%t agent=%q builtin=%t inherit=%t model=%q",
						testCase.role,
						resolved,
						testCase.enabled,
						testCase.agent,
						testCase.builtin,
						testCase.inherit,
						testCase.model,
					)
				}
			})
		}
	})

	t.Run("Should isolate workspace role overlays and report their provenance", func(t *testing.T) {
		t.Parallel()

		homePaths := e2etest.NewHomePaths(t)
		e2etest.SeedConfig(t, homePaths, e2etest.ConfigSeedOptions{
			Providers: map[string]aghconfig.ProviderConfig{
				"mock": {Command: "mock-acp"},
			},
			Mutate: func(cfg *aghconfig.Config) {
				cfg.Roles.Dream.Provider = "mock"
				cfg.Roles.Dream.Model = "global-model"
			},
		})
		workspaceA := e2etest.SeedWorkspace(t, e2etest.WorkspaceSeedOptions{Files: map[string]string{
			".agh/config.toml": "[roles.dream]\nmodel = \"workspace-model\"\n",
		}})
		workspaceB := e2etest.SeedWorkspace(t, e2etest.WorkspaceSeedOptions{})
		workspaceC := e2etest.SeedWorkspace(t, e2etest.WorkspaceSeedOptions{Files: map[string]string{
			".agh/config.toml": "[roles.dream]\nmodel = \"global-model\"\n",
		}})

		global, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(global) error = %v", err)
		}
		configA, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(workspaceA))
		if err != nil {
			t.Fatalf("LoadForHome(workspace A) error = %v", err)
		}
		configB, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(workspaceB))
		if err != nil {
			t.Fatalf("LoadForHome(workspace B) error = %v", err)
		}
		configC, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(workspaceC))
		if err != nil {
			t.Fatalf("LoadForHome(workspace C) error = %v", err)
		}
		resolver := newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]aghconfig.Config{
			"workspace-a": configA,
			"workspace-b": configB,
			"workspace-c": configC,
		}}, nil)

		resolvedA, err := resolver.Resolve(t.Context(), "workspace-a", aghconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(workspace A) error = %v", err)
		}
		resolvedB, err := resolver.Resolve(t.Context(), "workspace-b", aghconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(workspace B) error = %v", err)
		}
		if resolvedA.Model != "workspace-model" || resolvedA.Provenance["model"] != "workspace" {
			t.Fatalf("Resolve(workspace A) = %#v, want workspace model provenance", resolvedA)
		}
		if resolvedB.Model != "global-model" || resolvedB.Provenance["model"] != "global" {
			t.Fatalf("Resolve(workspace B) = %#v, want global model provenance", resolvedB)
		}
		resolvedC, err := resolver.Resolve(t.Context(), "workspace-c", aghconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(workspace C) error = %v", err)
		}
		if resolvedC.Model != "global-model" || resolvedC.Provenance["model"] != "workspace" {
			t.Fatalf("Resolve(workspace C) = %#v, want explicit same-value workspace provenance", resolvedC)
		}
	})

	t.Run("Should route through an agent definition loaded from disk", func(t *testing.T) {
		t.Parallel()

		homePaths := e2etest.NewHomePaths(t)
		e2etest.SeedConfig(t, homePaths, e2etest.ConfigSeedOptions{
			Providers: map[string]aghconfig.ProviderConfig{
				"mock": {Command: "mock-acp"},
			},
			AgentDefs: []e2etest.AgentSeed{{
				Name:     "my-curator",
				Provider: "mock",
				Model:    "agent-model",
				Prompt:   "Curate durable memories.",
			}},
			Mutate: func(cfg *aghconfig.Config) {
				cfg.Roles.Dream.Agent = "my-curator"
			},
		})
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		agents, err := aghconfig.LoadWorkspaceAgentDefs("", nil, homePaths)
		if err != nil {
			t.Fatalf("LoadWorkspaceAgentDefs() error = %v", err)
		}
		catalog := make(map[string]aghconfig.AgentDef, len(agents))
		for _, agent := range agents {
			catalog[agent.Name] = agent
		}

		resolved, err := newRoleResolver(&cfg, nil, roleAgentResolverStub{agents: catalog}).Resolve(
			t.Context(),
			"",
			aghconfig.RoleDream,
		)
		if err != nil {
			t.Fatalf("Resolve(dream) error = %v", err)
		}
		if resolved.AgentName != "my-curator" || resolved.Model != "agent-model" ||
			resolved.AgentDef.Prompt != "Curate durable memories." {
			t.Fatalf("Resolve(dream) = %#v, want disk-backed my-curator route", resolved)
		}

		cfg.Roles.Dream.Model = "role-model"
		resolved, err = newRoleResolver(&cfg, nil, roleAgentResolverStub{agents: catalog}).Resolve(
			t.Context(),
			"",
			aghconfig.RoleDream,
		)
		if err != nil {
			t.Fatalf("Resolve(dream role override) error = %v", err)
		}
		if resolved.Model != "role-model" {
			t.Fatalf("Resolve(dream role override) model = %q, want role-model", resolved.Model)
		}
	})

	t.Run("Should boot while excluding a pre-existing reserved agent definition", func(t *testing.T) {
		t.Parallel()

		homePaths := e2etest.NewHomePaths(t)
		e2etest.WriteAgentDef(t, homePaths, e2etest.AgentSeed{
			Name:     aghconfig.BuiltinCoordinatorAgentName,
			Provider: "fake",
			Model:    "shadow-model",
			Prompt:   "Shadow the builtin coordinator.",
		})
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{HomePaths: homePaths})
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		workspace := url.QueryEscape(harness.WorkspaceRoot)
		var agents aghcontract.AgentsResponse
		if err := harness.UDSJSON(
			ctx,
			http.MethodGet,
			"/api/agents?workspace="+workspace,
			nil,
			&agents,
		); err != nil {
			t.Fatalf("list agents after reserved definition boot error = %v", err)
		}
		assertBuiltinIdentityAbsent(t, agents.Agents)

		var catalog aghcontract.AgentCatalogResponse
		if err := harness.UDSJSON(
			ctx,
			http.MethodGet,
			"/api/agents/catalog?workspace="+workspace,
			nil,
			&catalog,
		); err != nil {
			t.Fatalf("list agent catalog after reserved definition boot error = %v", err)
		}
		catalogAgents := make([]aghcontract.AgentPayload, 0, len(catalog.Agents))
		for _, item := range catalog.Agents {
			catalogAgents = append(catalogAgents, item.Agent)
		}
		assertBuiltinIdentityAbsent(t, catalogAgents)
	})

	t.Run("Should expose one truthful role projection across HTTP UDS and CLI", func(t *testing.T) {
		t.Parallel()

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			Workspace: e2etest.WorkspaceSeedOptions{Files: map[string]string{
				".agh/config.toml": `[roles.dream]
agent = "missing-curator"
`,
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		path := "/api/roles?workspace=" + url.QueryEscape(harness.WorkspaceID)

		var httpRoles aghcontract.RolesResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpRoles); err != nil {
			t.Fatalf("HTTP GET roles error = %v", err)
		}
		var udsRoles aghcontract.RolesResponse
		if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &udsRoles); err != nil {
			t.Fatalf("UDS GET roles error = %v", err)
		}
		if !reflect.DeepEqual(httpRoles, udsRoles) {
			t.Fatalf("HTTP roles = %#v, want UDS parity %#v", httpRoles, udsRoles)
		}
		if len(httpRoles.Roles) != 6 {
			t.Fatalf("len(roles) = %d, want 6", len(httpRoles.Roles))
		}
		for index := 1; index < len(httpRoles.Roles); index++ {
			if httpRoles.Roles[index-1].Role >= httpRoles.Roles[index].Role {
				t.Fatalf("roles are not sorted: %#v", httpRoles.Roles)
			}
		}

		var dream aghcontract.RoleStatus
		for _, role := range httpRoles.Roles {
			if role.Role == string(aghconfig.RoleDream) {
				dream = role
			}
			if role.ResolutionMode == aghcontract.RoleResolutionModeInherit && role.Agent != nil {
				t.Fatalf("inherit role fabricated an agent identity: %#v", role)
			}
		}
		if dream.Role == "" || len(dream.Diagnostics) != 1 ||
			dream.Diagnostics[0].Code != aghcontract.CodeRoleAgentNotFound {
			t.Fatalf("dream role = %#v, want role_agent_not_found diagnostic", dream)
		}

		var cliDream aghcontract.RoleStatus
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliDream,
			"roles",
			"show",
			"dream",
			"--workspace",
			harness.WorkspaceRoot,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI roles show dream error = %v", err)
		}
		if !reflect.DeepEqual(cliDream, dream) {
			t.Fatalf("CLI dream = %#v, want HTTP/UDS parity %#v", cliDream, dream)
		}

		workspace := url.QueryEscape(harness.WorkspaceRoot)
		var agents aghcontract.AgentsResponse
		if err := harness.UDSJSON(
			ctx,
			http.MethodGet,
			"/api/agents?workspace="+workspace,
			nil,
			&agents,
		); err != nil {
			t.Fatalf("UDS GET agents error = %v", err)
		}
		assertBuiltinIdentityAbsent(t, agents.Agents)
	})
}

func assertBuiltinIdentityAbsent(t *testing.T, agents []aghcontract.AgentPayload) {
	t.Helper()

	for _, agent := range agents {
		if aghconfig.IsReservedAgentName(agent.Name) {
			t.Fatalf("agent catalog exposed reserved identity %q: %#v", agent.Name, agent)
		}
	}
}
