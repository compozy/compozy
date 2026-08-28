package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type roleResolverFunc func(context.Context, string, compozyconfig.RoleName) (ResolvedRole, error)

func (f roleResolverFunc) Resolve(
	ctx context.Context,
	workspaceID string,
	role compozyconfig.RoleName,
) (ResolvedRole, error) {
	return f(ctx, workspaceID, role)
}

func resolvedRoleResolver(resolved *ResolvedRole) RoleResolver {
	return roleResolverFunc(func(context.Context, string, compozyconfig.RoleName) (ResolvedRole, error) {
		return *resolved, nil
	})
}

func TestRoleResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve default builtin and inherited identities", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		resolver := newRoleResolver(&cfg, nil, nil)

		dream, err := resolver.Resolve(t.Context(), "", compozyconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(dream) error = %v", err)
		}
		if !dream.Builtin || dream.AgentName != compozyconfig.BuiltinDreamingCuratorAgentName ||
			strings.TrimSpace(dream.AgentDef.Prompt) == "" {
			t.Fatalf("Resolve(dream) = %#v, want embedded dreaming-curator", dream)
		}
		if dream.Provenance["agent"] != compozyconfig.RoleFieldSourceDefault ||
			dream.Provenance["model"] != compozyconfig.RoleFieldSourceDefault {
			t.Fatalf("Resolve(dream) provenance = %#v, want default fields", dream.Provenance)
		}

		coordinator, err := resolver.Resolve(t.Context(), "", compozyconfig.RoleCoordinator)
		if err != nil {
			t.Fatalf("Resolve(coordinator) error = %v", err)
		}
		if coordinator.Enabled || !coordinator.Builtin ||
			coordinator.AgentName != compozyconfig.BuiltinCoordinatorAgentName {
			t.Fatalf("Resolve(coordinator) = %#v, want disabled builtin coordinator", coordinator)
		}

		for _, role := range []compozyconfig.RoleName{compozyconfig.RoleAutoTitle, compozyconfig.RoleMemoryExtractor} {
			resolved, resolveErr := resolver.Resolve(t.Context(), "", role)
			if resolveErr != nil {
				t.Fatalf("Resolve(%s) error = %v", role, resolveErr)
			}
			if !resolved.Inherit || resolved.AgentName != "" || resolved.AgentDef.Name != "" ||
				resolved.AgentDef.Prompt != "" {
				t.Fatalf("Resolve(%s) = %#v, want invocation-time inheritance", role, resolved)
			}
		}
	})

	t.Run("Should resolve custom catalog agents", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Roles.Dream.Agent = "my-curator"
		agent := compozyconfig.AgentDef{Name: "my-curator", Prompt: "curate exactly", Tools: []string{"read"}}
		resolver := newRoleResolver(&cfg, nil, roleAgentResolverStub{
			agents: map[string]compozyconfig.AgentDef{"my-curator": agent},
		})

		resolved, err := resolver.Resolve(t.Context(), "", compozyconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(dream) error = %v", err)
		}
		if resolved.Builtin || resolved.Inherit || resolved.AgentName != "my-curator" {
			t.Fatalf("Resolve(dream) = %#v, want custom catalog identity", resolved)
		}
		if resolved.AgentDef.Prompt != agent.Prompt || len(resolved.AgentDef.Tools) != 1 ||
			resolved.AgentDef.Tools[0] != "read" {
			t.Fatalf("Resolve(dream) AgentDef = %#v, want untouched catalog definition", resolved.AgentDef)
		}

		cfg.Roles.AutoTitle.Agent = "my-curator"
		resolved, err = newRoleResolver(&cfg, nil, roleAgentResolverStub{
			agents: map[string]compozyconfig.AgentDef{"my-curator": agent},
		}).Resolve(t.Context(), "", compozyconfig.RoleAutoTitle)
		if err != nil {
			t.Fatalf("Resolve(auto_title) error = %v", err)
		}
		if resolved.Inherit || resolved.AgentName != "my-curator" {
			t.Fatalf("Resolve(auto_title) = %#v, want explicit catalog identity", resolved)
		}
	})

	t.Run("Should resolve inherited role overrides through the invoking agent provider chain", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name          string
			role          compozyconfig.RoleName
			agentProvider string
			wantProvider  string
		}{
			{
				name:          "Should prefer the invoking agent provider",
				role:          compozyconfig.RoleAutoTitle,
				agentProvider: "agent-provider",
				wantProvider:  "agent-provider",
			},
			{
				name:         "Should use the default provider when the invoking agent inherits it",
				role:         compozyconfig.RoleMemoryExtractor,
				wantProvider: "default-provider",
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := roleResolverConfig()
				cfg.Defaults.Provider = "default-provider"
				cfg.Providers["agent-provider"] = compozyconfig.ProviderConfig{Command: "agent-acp"}
				cfg.Providers["default-provider"] = compozyconfig.ProviderConfig{Command: "default-acp"}
				switch testCase.role {
				case compozyconfig.RoleAutoTitle:
					cfg.Roles.AutoTitle.Model = "role-model"
				case compozyconfig.RoleMemoryExtractor:
					cfg.Roles.MemoryExtractor.Model = "role-model"
				}
				resolver := newRoleResolver(&cfg, nil, roleAgentResolverStub{agents: map[string]compozyconfig.AgentDef{
					"invoking-agent": {
						Name:     "invoking-agent",
						Provider: testCase.agentProvider,
						Prompt:   "Handle the parent session.",
					},
				}})
				ctx := withRoleInvocationCorrelation(t.Context(), roleInvocationCorrelation{
					AgentName: "invoking-agent",
				})

				resolved, err := resolver.Resolve(ctx, "", testCase.role)
				if err != nil {
					t.Fatalf("Resolve(%s) error = %v", testCase.role, err)
				}
				if !resolved.Inherit || resolved.AgentName != "invoking-agent" ||
					resolved.Provider != testCase.wantProvider || resolved.Model != "role-model" {
					t.Fatalf(
						"Resolve(%s) = %#v, want inherited invoking-agent routed through %q/role-model",
						testCase.role,
						resolved,
						testCase.wantProvider,
					)
				}
			})
		}
	})

	t.Run("Should reject missing catalog agents and unknown roles deterministically", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Roles.Dream.Agent = "ghost"
		resolver := newRoleResolver(&cfg, nil, roleAgentResolverStub{})

		_, err := resolver.Resolve(t.Context(), "", compozyconfig.RoleDream)
		resolutionErr, resolutionErrMatched := errors.AsType[*RoleResolutionError](err)
		if !resolutionErrMatched || resolutionErr.Code != roleErrorAgentNotFound ||
			resolutionErr.Agent != "ghost" {
			t.Fatalf("Resolve(ghost) error = %v, want role_agent_not_found naming ghost", err)
		}

		_, err = resolver.Resolve(t.Context(), "", compozyconfig.RoleName("unknown"))
		resolutionErr, resolutionErrMatched = errors.AsType[*RoleResolutionError](err)
		if !resolutionErrMatched || resolutionErr.Code != roleErrorUnknown {
			t.Fatalf("Resolve(unknown) error = %v, want role_unknown", err)
		}
	})

	t.Run("Should apply role agent and provider model precedence", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name          string
			roleModel     string
			agentModel    string
			providerModel string
			want          string
		}{
			{name: "role model", roleModel: "role-model", agentModel: "agent-model", providerModel: "provider-model", want: "role-model"},
			{name: "agent model", agentModel: "agent-model", providerModel: "provider-model", want: "agent-model"},
			{name: "provider default", providerModel: "provider-model", want: "provider-model"},
		} {
			t.Run("Should prefer "+testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := roleResolverConfig()
				cfg.Providers["mock"] = compozyconfig.ProviderConfig{
					Command: "mock-acp",
					Models:  compozyconfig.ProviderModelsConfig{Default: testCase.providerModel},
				}
				cfg.Roles.Dream.Agent = "my-curator"
				cfg.Roles.Dream.Model = testCase.roleModel
				resolver := newRoleResolver(&cfg, nil, roleAgentResolverStub{
					agents: map[string]compozyconfig.AgentDef{
						"my-curator": {
							Name: "my-curator", Provider: "mock", Model: testCase.agentModel, Prompt: "Curate memory.",
						},
					},
				})

				resolved, err := resolver.Resolve(t.Context(), "", compozyconfig.RoleDream)
				if err != nil {
					t.Fatalf("Resolve(dream) error = %v", err)
				}
				if resolved.Model != testCase.want {
					t.Fatalf("Resolve(dream) model = %q, want %q", resolved.Model, testCase.want)
				}
			})
		}
	})

	t.Run("Should reject providers that require an unresolved runtime model", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		const providerName = "model-required"
		cfg.Providers[providerName] = compozyconfig.ProviderConfig{
			Command: "pi-acp", Harness: compozyconfig.ProviderHarnessPiACP,
		}
		cfg.Roles.Dream.Provider = providerName
		resolver := newRoleResolver(&cfg, nil, nil)

		_, err := resolver.Resolve(t.Context(), "", compozyconfig.RoleDream)
		if err == nil || !errors.Is(err, compozyconfig.ErrRuntimeModelRequired) {
			t.Fatalf("Resolve(dream) error = %v, want missing runtime model", err)
		}
	})

	t.Run("Should report per-field workspace and global provenance", func(t *testing.T) {
		t.Parallel()

		global := roleResolverConfig()
		global.Providers["mock"] = compozyconfig.ProviderConfig{Command: "mock-acp"}
		global.Roles.Dream.Provider = "mock"
		global.Roles.Dream.Model = "global-model"
		global.RoleSources[compozyconfig.RoleDream][compozyconfig.RoleFieldProvider] = compozyconfig.RoleFieldSourceGlobal
		global.RoleSources[compozyconfig.RoleDream][compozyconfig.RoleFieldModel] = compozyconfig.RoleFieldSourceGlobal
		workspaceA := global
		workspaceA.Roles.Dream.Model = "workspace-model"
		workspaceA.RoleSources = compozyconfig.CloneRoleFieldSources(global.RoleSources)
		workspaceA.RoleSources[compozyconfig.RoleDream][compozyconfig.RoleFieldModel] = compozyconfig.RoleFieldSourceWorkspace
		workspaceB := global
		resolver := newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]compozyconfig.Config{
			"ws-a": workspaceA,
			"ws-b": workspaceB,
		}}, nil)

		resolvedA, err := resolver.Resolve(t.Context(), "ws-a", compozyconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(ws-a) error = %v", err)
		}
		resolvedB, err := resolver.Resolve(t.Context(), "ws-b", compozyconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(ws-b) error = %v", err)
		}
		if resolvedA.Model != "workspace-model" ||
			resolvedA.Provenance["model"] != compozyconfig.RoleFieldSourceWorkspace ||
			resolvedB.Model != "global-model" || resolvedB.Provenance["model"] != compozyconfig.RoleFieldSourceGlobal {
			t.Fatalf("workspace provenance = a:%#v b:%#v", resolvedA, resolvedB)
		}
		if resolvedA.Provenance["agent"] != compozyconfig.RoleFieldSourceDefault ||
			resolvedB.Provenance["agent"] != compozyconfig.RoleFieldSourceDefault {
			t.Fatalf(
				"agent provenance = a:%q b:%q, want default",
				resolvedA.Provenance["agent"],
				resolvedB.Provenance["agent"],
			)
		}
	})

	t.Run("Should return a disabled role", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Roles.Dream.Enabled = false
		cfg.Roles.Dream.Agent = "missing-curator"
		cfg.Roles.Dream.Provider = "model-required"
		cfg.Providers["model-required"] = compozyconfig.ProviderConfig{
			Command: "pi-acp", Harness: compozyconfig.ProviderHarnessPiACP,
		}
		resolved, err := newRoleResolver(&cfg, nil, nil).Resolve(t.Context(), "", compozyconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(dream) error = %v", err)
		}
		if resolved.Enabled {
			t.Fatalf("Resolve(dream) Enabled = true, want false")
		}
		if resolved.AgentName != "" || resolved.Model != "" {
			t.Fatalf("Resolve(dream) = %#v, want no resolved route for disabled role", resolved)
		}
	})

	t.Run("Should allow direct ACP roles without a runtime model", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Providers["direct-acp"] = compozyconfig.ProviderConfig{
			Command: "direct-agent acp", Harness: compozyconfig.ProviderHarnessACP,
		}
		cfg.Roles.Dream.Provider = "direct-acp"
		resolved, err := newRoleResolver(&cfg, nil, nil).Resolve(t.Context(), "", compozyconfig.RoleDream)
		if err != nil {
			t.Fatalf("Resolve(dream) error = %v", err)
		}
		if resolved.Provider != "direct-acp" || resolved.Model != "" {
			t.Fatalf("Resolve(dream) = %#v, want direct ACP provider without a model", resolved)
		}
	})
}

type roleAgentResolverStub struct {
	agents map[string]compozyconfig.AgentDef
}

func (s roleAgentResolverStub) ResolveAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	agent, ok := s.agents[name]
	if !ok {
		return compozyconfig.AgentDef{}, workspacepkg.ErrAgentNotAvailable
	}
	return compozyconfig.CloneAgentDef(agent), nil
}

type roleWorkspaceResolverStub struct {
	configs map[string]compozyconfig.Config
}

func (s roleWorkspaceResolverStub) Resolve(
	_ context.Context,
	idOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	cfg, ok := s.configs[idOrPath]
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: idOrPath},
		Config:    cfg,
	}, nil
}

func (s roleWorkspaceResolverStub) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	return s.Resolve(ctx, path)
}

func roleResolverConfig() compozyconfig.Config {
	return compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
}
