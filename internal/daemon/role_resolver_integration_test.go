//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

func TestRoleResolverIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve every role from built-in defaults without a user config", func(t *testing.T) {
		t.Parallel()

		homePaths := e2etest.NewHomePaths(t)
		cfg, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		resolver := newRoleResolver(&cfg, nil, nil)

		for _, testCase := range []struct {
			role    compozyconfig.RoleName
			agent   string
			enabled bool
			builtin bool
			inherit bool
			model   string
		}{
			{role: compozyconfig.RoleCoordinator, agent: compozyconfig.BuiltinCoordinatorAgentName, builtin: true},
			{role: compozyconfig.RoleAutoTitle, enabled: true, inherit: true},
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
			Providers: map[string]compozyconfig.ProviderConfig{
				"mock": {Command: "mock-acp"},
			},
			Mutate: func(cfg *compozyconfig.Config) {
				cfg.Roles.Coordinator.Enabled = true
				cfg.Roles.Coordinator.Provider = "mock"
				cfg.Roles.Coordinator.Model = "global-model"
			},
		})
		workspaceA := e2etest.SeedWorkspace(t, e2etest.WorkspaceSeedOptions{Files: map[string]string{
			".compozy/config.toml": "[roles.coordinator]\nmodel = \"workspace-model\"\n",
		}})
		workspaceB := e2etest.SeedWorkspace(t, e2etest.WorkspaceSeedOptions{})
		workspaceC := e2etest.SeedWorkspace(t, e2etest.WorkspaceSeedOptions{Files: map[string]string{
			".compozy/config.toml": "[roles.coordinator]\nmodel = \"global-model\"\n",
		}})

		global, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(global) error = %v", err)
		}
		configA, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceA))
		if err != nil {
			t.Fatalf("LoadForHome(workspace A) error = %v", err)
		}
		configB, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceB))
		if err != nil {
			t.Fatalf("LoadForHome(workspace B) error = %v", err)
		}
		configC, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceC))
		if err != nil {
			t.Fatalf("LoadForHome(workspace C) error = %v", err)
		}
		resolver := newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]compozyconfig.Config{
			"workspace-a": configA,
			"workspace-b": configB,
			"workspace-c": configC,
		}}, nil)

		resolvedA, err := resolver.Resolve(t.Context(), "workspace-a", compozyconfig.RoleCoordinator)
		if err != nil {
			t.Fatalf("Resolve(workspace A) error = %v", err)
		}
		resolvedB, err := resolver.Resolve(t.Context(), "workspace-b", compozyconfig.RoleCoordinator)
		if err != nil {
			t.Fatalf("Resolve(workspace B) error = %v", err)
		}
		if resolvedA.Model != "workspace-model" || resolvedA.Provenance["model"] != "workspace" {
			t.Fatalf("Resolve(workspace A) = %#v, want workspace model provenance", resolvedA)
		}
		if resolvedB.Model != "global-model" || resolvedB.Provenance["model"] != "global" {
			t.Fatalf("Resolve(workspace B) = %#v, want global model provenance", resolvedB)
		}
		resolvedC, err := resolver.Resolve(t.Context(), "workspace-c", compozyconfig.RoleCoordinator)
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
			Providers: map[string]compozyconfig.ProviderConfig{
				"mock": {Command: "mock-acp"},
			},
			AgentDefs: []e2etest.AgentSeed{{
				Name:     "my-curator",
				Provider: "mock",
				Model:    "agent-model",
				Prompt:   "Coordinate workspace work.",
			}},
			Mutate: func(cfg *compozyconfig.Config) {
				cfg.Roles.Coordinator.Enabled = true
				cfg.Roles.Coordinator.Agent = "my-curator"
			},
		})
		cfg, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		agents, err := compozyconfig.LoadWorkspaceAgentDefs("", nil, homePaths, "")
		if err != nil {
			t.Fatalf("LoadWorkspaceAgentDefs() error = %v", err)
		}
		catalog := make(map[string]compozyconfig.AgentDef, len(agents))
		for _, agent := range agents {
			catalog[agent.Name] = agent
		}

		resolved, err := newRoleResolver(&cfg, nil, roleAgentResolverStub{agents: catalog}).Resolve(
			t.Context(),
			"",
			compozyconfig.RoleCoordinator,
		)
		if err != nil {
			t.Fatalf("Resolve(coordinator) error = %v", err)
		}
		if resolved.AgentName != "my-curator" || resolved.Model != "agent-model" ||
			resolved.AgentDef.Prompt != "Coordinate workspace work." {
			t.Fatalf("Resolve(coordinator) = %#v, want disk-backed my-curator route", resolved)
		}

		cfg.Roles.Coordinator.Model = "role-model"
		resolved, err = newRoleResolver(&cfg, nil, roleAgentResolverStub{agents: catalog}).Resolve(
			t.Context(),
			"",
			compozyconfig.RoleCoordinator,
		)
		if err != nil {
			t.Fatalf("Resolve(coordinator role override) error = %v", err)
		}
		if resolved.Model != "role-model" {
			t.Fatalf("Resolve(coordinator role override) model = %q, want role-model", resolved.Model)
		}
	})

	t.Run("Should boot while excluding a pre-existing reserved agent definition", func(t *testing.T) {
		t.Parallel()

		homePaths := e2etest.NewHomePaths(t)
		e2etest.WriteAgentDef(t, homePaths, e2etest.AgentSeed{
			Name:     compozyconfig.BuiltinCoordinatorAgentName,
			Provider: "fake",
			Model:    "shadow-model",
			Prompt:   "Shadow the builtin coordinator.",
		})
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{HomePaths: homePaths})
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		workspace := url.QueryEscape(harness.WorkspaceRoot)
		var agents compozycontract.AgentsResponse
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

		var catalog compozycontract.AgentCatalogResponse
		if err := harness.UDSJSON(
			ctx,
			http.MethodGet,
			"/api/agents/catalog?workspace="+workspace,
			nil,
			&catalog,
		); err != nil {
			t.Fatalf("list agent catalog after reserved definition boot error = %v", err)
		}
		catalogAgents := make([]compozycontract.AgentPayload, 0, len(catalog.Agents))
		for _, item := range catalog.Agents {
			catalogAgents = append(catalogAgents, item.Agent)
		}
		assertBuiltinIdentityAbsent(t, catalogAgents)
	})

	t.Run("Should expose one truthful role projection across HTTP UDS and CLI", func(t *testing.T) {
		t.Parallel()

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
				cfg.Roles.Coordinator.Enabled = true
				cfg.Roles.AutoTitle.Enabled = true
				cfg.Roles.AutoTitle.Provider = "claude"
				cfg.Roles.AutoTitle.Model = "title-model"
			}},
			Workspace: e2etest.WorkspaceSeedOptions{Files: map[string]string{
				".compozy/config.toml": `[roles.coordinator]
agent = "missing-curator"

`,
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		path := "/api/roles?workspace=" + url.QueryEscape(harness.WorkspaceID)

		var httpRoles compozycontract.RolesResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpRoles); err != nil {
			t.Fatalf("HTTP GET roles error = %v", err)
		}
		var udsRoles compozycontract.RolesResponse
		if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &udsRoles); err != nil {
			t.Fatalf("UDS GET roles error = %v", err)
		}
		if !reflect.DeepEqual(httpRoles, udsRoles) {
			t.Fatalf("HTTP roles = %#v, want UDS parity %#v", httpRoles, udsRoles)
		}
		if len(httpRoles.Roles) != 2 {
			t.Fatalf("len(roles) = %d, want 2", len(httpRoles.Roles))
		}
		for index := 1; index < len(httpRoles.Roles); index++ {
			if httpRoles.Roles[index-1].Role >= httpRoles.Roles[index].Role {
				t.Fatalf("roles are not sorted: %#v", httpRoles.Roles)
			}
		}

		var coordinator compozycontract.RoleStatus
		for _, role := range httpRoles.Roles {
			if role.Role == string(compozyconfig.RoleCoordinator) {
				coordinator = role
			}
			if role.ResolutionMode == compozycontract.RoleResolutionModeInherit && role.Agent != nil {
				t.Fatalf("inherit role fabricated an agent identity: %#v", role)
			}
		}
		if coordinator.Role == "" || len(coordinator.Diagnostics) != 1 ||
			coordinator.Diagnostics[0].Code != compozycontract.CodeRoleAgentNotFound {
			t.Fatalf("coordinator role = %#v, want role_agent_not_found diagnostic", coordinator)
		}

		var cliCoordinator compozycontract.RoleStatus
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliCoordinator,
			"roles",
			"show",
			"coordinator",
			"--workspace",
			harness.WorkspaceRoot,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI roles show coordinator error = %v", err)
		}
		if !reflect.DeepEqual(cliCoordinator, coordinator) {
			t.Fatalf("CLI coordinator = %#v, want HTTP/UDS parity %#v", cliCoordinator, coordinator)
		}
		var cliRoles []compozycontract.RoleStatus
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliRoles,
			"roles",
			"list",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI roles list error = %v", err)
		}
		if !reflect.DeepEqual(cliRoles, httpRoles.Roles) {
			t.Fatalf("CLI roles=%#v, want HTTP/UDS parity %#v", cliRoles, httpRoles)
		}
		var title compozycontract.RoleStatus
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&title,
			"roles",
			"show",
			"auto_title",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI title role error = %v", err)
		}
		if !title.Enabled || title.Provider == nil || *title.Provider != "claude" ||
			title.Model == nil || *title.Model != "title-model" || len(title.Diagnostics) != 0 ||
			title.Provenance[compozyconfig.RoleFieldEnabled] != compozyconfig.RoleFieldSourceGlobal {
			t.Fatalf("title role=%#v, want configured role availability", title)
		}
		titleIndex := slices.IndexFunc(cliRoles, func(role compozycontract.RoleStatus) bool {
			return role.Role == title.Role
		})
		if titleIndex < 0 {
			t.Fatalf("auto_title missing from CLI/HTTP/UDS roster: %#v", cliRoles)
		}
		if !reflect.DeepEqual(cliRoles[titleIndex], title) {
			t.Fatalf("title list=%#v, show=%#v", cliRoles[titleIndex], title)
		}
		stdout, stderr, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, "roles", "show", "auto_title")
		if err != nil || !strings.Contains(strings.Join(strings.Fields(stdout), " "), "Enabled: true") {
			t.Fatalf("human title status stdout=%q stderr=%q error=%v", stdout, stderr, err)
		}
		t.Logf(
			"auto_title: enabled=%t provider=%s model=%s provenance=%s; CLI/HTTP/UDS agree",
			title.Enabled,
			*title.Provider,
			*title.Model,
			title.Provenance[compozyconfig.RoleFieldEnabled],
		)

		workspace := url.QueryEscape(harness.WorkspaceRoot)
		var agents compozycontract.AgentsResponse
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

func assertBuiltinIdentityAbsent(t *testing.T, agents []compozycontract.AgentPayload) {
	t.Helper()

	for _, agent := range agents {
		if compozyconfig.IsReservedAgentName(agent.Name) {
			t.Fatalf("agent catalog exposed reserved identity %q: %#v", agent.Name, agent)
		}
	}
}
