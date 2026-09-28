package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestNativeAgentCreate(t *testing.T) {
	t.Parallel()

	t.Run("Should report the registered target workspace", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := compozyconfig.DefaultWithHome(homePaths)
		cfg.Defaults.Provider = "claude"
		cfg.Providers["claude"] = compozyconfig.ProviderConfig{Command: "claude"}
		targetRoot := t.TempDir()
		resolveTarget := func(ref string) (workspacepkg.ResolvedWorkspace, error) {
			switch ref {
			case "target-alias", "ws-target", "identity-target":
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID: "ws-target", RootDir: targetRoot, Name: "target",
					},
					ProfileID: store.DefaultProfileID, ProfileName: daemonDefaultProfileName,
					WorkspaceID: "identity-target", Config: cfg,
				}, nil
			default:
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
			}
		}
		workspaces := apitest.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return resolveTarget(ref)
			},
			ResolveForProfileFn: func(
				_ context.Context,
				ref string,
				profileName string,
			) (workspacepkg.ResolvedWorkspace, error) {
				if profileName != daemonDefaultProfileName {
					t.Fatalf("ResolveForProfile() profile = %q, want %q", profileName, daemonDefaultProfileName)
				}
				return resolveTarget(ref)
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths:   homePaths,
			Config:      cfg,
			Workspaces:  workspaces,
			AgentSkills: agentSkillPublisherFunc(func(context.Context) error { return nil }),
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true, WorkspaceID: "ws-scope"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentCreate,
				Input: json.RawMessage(
					`{"scope":"workspace","workspace":"target-alias","name":"operator-target","provider":"claude","prompt":"Use the selected workspace."}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(agent_create target workspace) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"workspace_id":"ws-target"`))
		requireNativeStructuredExcludes(t, result, []byte(`"workspace_id":"identity-target"`))
		requireNativeStructuredExcludes(t, result, []byte(`"workspace_id":"target-alias"`))
		requireNativeStructuredExcludes(t, result, []byte(`"workspace_id":"ws-scope"`))
	})

	t.Run("Should reject a missing sync dependency before persistence", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths:  homePaths,
			Workspaces: nativeTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input: json.RawMessage(
				`{"scope":"global","name":"missing-sync","provider":"claude","prompt":"No write."}`,
			),
		})
		if err == nil {
			t.Fatal("Registry.Call(agent_create) error = nil, want unavailable sync error")
		}
		path := filepath.Join(homePaths.AgentsDir, "missing-sync", compozyconfig.AgentDefinitionFileName)
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(unwritten agent) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should resolve a publisher wired after native registry construction", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := compozyconfig.DefaultWithHome(homePaths)
		cfg.Defaults.Provider = "claude"
		cfg.Providers["claude"] = compozyconfig.ProviderConfig{Command: "claude"}
		var publisher agentSkillPublisher
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths:  homePaths,
			Config:     cfg,
			Workspaces: nativeTestWorkspaceService(t),
			AgentSkillsRuntime: func() agentSkillPublisher {
				return publisher
			},
		}, nativeApproveAllPolicyInputs())
		publisher = agentSkillPublisherFunc(func(context.Context) error { return nil })

		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input: json.RawMessage(
				`{"scope":"global","name":"late-publisher","provider":"claude","prompt":"Late publisher."}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(agent_create late publisher) error = %v", err)
		}
		path := filepath.Join(homePaths.AgentsDir, "late-publisher", compozyconfig.AgentDefinitionFileName)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("os.Stat(late-publisher agent) error = %v", err)
		}
	})

	t.Run("Should roll back a failed sync and allow a clean retry", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		syncErr := errors.New("catalog unavailable")
		syncCalls := 0
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			HomePaths:  homePaths,
			Workspaces: nativeTestWorkspaceService(t),
			AgentSkills: agentSkillPublisherFunc(func(context.Context) error {
				syncCalls++
				if syncCalls == 1 {
					return syncErr
				}
				return nil
			}),
		}, nativeApproveAllPolicyInputs())
		request := toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input:  json.RawMessage(`{"scope":"global","name":"retryable","provider":"claude","prompt":"Retry."}`),
		}
		if _, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, request); !errors.Is(err, syncErr) {
			t.Fatalf("first Registry.Call(agent_create) error = %v, want sync failure", err)
		}
		path := filepath.Join(homePaths.AgentsDir, "retryable", compozyconfig.AgentDefinitionFileName)
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(rolled back agent) error = %v, want os.ErrNotExist", statErr)
		}
		if _, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, request); err != nil {
			t.Fatalf("second Registry.Call(agent_create) error = %v", err)
		}
		if syncCalls != 3 {
			t.Fatalf("AgentSkills.Sync() calls = %d, want failure, reconciliation, and retry", syncCalls)
		}
	})

	homePaths := testHomePaths(t)
	cfg := compozyconfig.DefaultWithHome(homePaths)
	cfg.Defaults.Provider = "claude"
	cfg.Providers["claude"] = compozyconfig.ProviderConfig{
		Command: "claude",
		Models:  compozyconfig.ProviderModelsConfig{Default: "claude-sonnet"},
	}
	registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
		HomePaths:   homePaths,
		Config:      cfg,
		Workspaces:  nativeTestWorkspaceService(t),
		AgentSkills: agentSkillPublisherFunc(func(context.Context) error { return nil }),
	}, nativeApproveAllPolicyInputs())

	t.Run("Should author one global AGENT.md", func(t *testing.T) {
		result, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input: json.RawMessage(
				`{"scope":"global","name":"scout","provider":"claude","model":"claude-opus-4-8","reasoning_effort":"max","speed":"fast","acp_options":[{"id":"context","value_id":"1m"},{"id":"thinking","bool_value":true}],"prompt":"You scout the codebase."}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(agent_create) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"scout"`))
		path := filepath.Join(homePaths.AgentsDir, "scout", "AGENT.md")
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("agent definition not written at %q: %v", path, statErr)
		}
		agent, loadErr := compozyconfig.LoadAgentDefFile(path)
		if loadErr != nil {
			t.Fatalf("LoadAgentDefFile() error = %v", loadErr)
		}
		if got, want := agent.ReasoningEffort, "max"; got != want {
			t.Fatalf("agent.ReasoningEffort = %q, want %q", got, want)
		}
		agentOptions := agent.ACPOptionsValue()
		if agent.SpeedValue() != "fast" || len(agentOptions) != 2 || agentOptions[0].ID != "context" ||
			agentOptions[0].ValueID != "1m" || agentOptions[1].ID != "thinking" ||
			agentOptions[1].BoolValue == nil || !*agentOptions[1].BoolValue {
			t.Fatalf("agent runtime defaults = speed %q options %#v", agent.SpeedValue(), agentOptions)
		}
	})

	t.Run("Should conflict when the agent already exists", func(t *testing.T) {
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input: json.RawMessage(
				`{"scope":"global","name":"scout","provider":"claude","prompt":"Duplicate."}`,
			),
		})
		requireToolReason(t, err, toolspkg.ErrToolConflict, toolspkg.ReasonConflictedID)
	})

	t.Run("Should reject a reserved agent name through the shared authoring path", func(t *testing.T) {
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input: json.RawMessage(
				`{"scope":"global","name":"coordinator","provider":"claude","prompt":"Reserved."}`,
			),
		})
		if !errors.Is(err, compozyconfig.ErrAgentNameReserved) {
			t.Fatalf("Registry.Call(agent_create reserved) error = %v, want ErrAgentNameReserved", err)
		}
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched || toolErr.Code != toolspkg.ErrorCodeAgentNameReserved {
			t.Fatalf("Registry.Call(agent_create reserved) error = %#v, want agent_name_reserved", err)
		}
		path := filepath.Join(homePaths.AgentsDir, "coordinator")
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(reserved agent directory) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should inherit the configured runtime when the request omits overrides", func(t *testing.T) {
		result, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input:  json.RawMessage(`{"scope":"global","name":"inherited","prompt":"Use project defaults."}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(agent_create inherited) error = %v", err)
		}
		requireNativeStructuredContains(
			t,
			result,
			[]byte(`"effective_runtime":{"provider":"claude","model":"claude-sonnet"`),
		)
		agent, loadErr := compozyconfig.LoadAgentDefFile(filepath.Join(
			homePaths.AgentsDir,
			"inherited",
			compozyconfig.AgentDefinitionFileName,
		))
		if loadErr != nil {
			t.Fatalf("LoadAgentDefFile(inherited) error = %v", loadErr)
		}
		if agent.Provider != "" || agent.Model != "" || agent.ReasoningEffort != "" {
			t.Fatalf(
				"authored runtime = (%q, %q, %q), want inherited fields omitted",
				agent.Provider,
				agent.Model,
				agent.ReasoningEffort,
			)
		}
	})

	t.Run("Should allow onboarding as an ordinary agent name", func(t *testing.T) {
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAgentCreate,
			Input: json.RawMessage(
				"{\"scope\":\"global\",\"name\":\"onboarding\",\"provider\":\"claude\",\"prompt\":\"Operator-authored.\"}",
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(agent_create onboarding) error = %v", err)
		}
	})

	t.Run("Should apply ordinary scope policy when onboarding is the caller", func(t *testing.T) {
		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{AgentName: "onboarding", Operator: true},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAgentCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"escalated","provider":"claude","prompt":"injected"}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(agent_create from onboarding) error = %v", err)
		}
	})
}

func TestNativeWorkspaceDescribeIncludesOrdinaryOnboardingAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should include ordinary onboarding alongside workspace and catalog agents", func(t *testing.T) {
		t.Parallel()

		const workspaceID = "ws-native"
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: apitest.StubWorkspaceService{
				ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if err := ctx.Err(); err != nil {
						return workspacepkg.ResolvedWorkspace{}, err
					}
					if ref != workspaceID {
						t.Fatalf("Resolve() ref = %q, want %q", ref, workspaceID)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      workspaceID,
							RootDir: t.TempDir(),
							Name:    "native",
						},
						WorkspaceID: workspaceID,
						Agents: []compozyconfig.AgentDef{
							{Name: compozyconfig.DefaultAgentName, Provider: "codex", Prompt: "General."},
							{Name: "onboarding", Provider: "codex", Prompt: "Onboarding."},
						},
					}, nil
				},
			},
			Sessions: nativeTestSessionManager(workspaceID),
			AgentCatalog: nativeAgentCatalogStub{agents: []compozyconfig.AgentDef{
				{Name: "catalog-visible", Provider: "codex", Prompt: "Catalog visible."},
				{Name: "onboarding", Provider: "codex", Prompt: "Catalog onboarding."},
			}, workspaceAgents: []compozyconfig.AgentDef{
				{Name: "dev-linked", Provider: "codex", Prompt: "Workspace extension agent."},
			}},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWorkspaceDescribe,
			Input:  json.RawMessage("{\"workspace\":\"ws-native\"}"),
		})
		if err != nil {
			t.Fatalf("Registry.Call(workspace_describe) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte("\"general\""))
		requireNativeStructuredContains(t, result, []byte("\"catalog-visible\""))
		requireNativeStructuredContains(t, result, []byte("\"dev-linked\""))
		requireNativeStructuredContains(t, result, []byte("\"onboarding\""))
	})
}

type nativeAgentCatalogStub struct {
	agents          []compozyconfig.AgentDef
	workspaceAgents []compozyconfig.AgentDef
}

func (s nativeAgentCatalogStub) ListAgents(context.Context) ([]core.AgentCatalogEntry, error) {
	entries := make([]core.AgentCatalogEntry, 0, len(s.agents))
	for _, agent := range s.agents {
		entries = append(entries, core.AgentCatalogEntry{
			Def:    compozyconfig.CloneAgentDef(agent),
			Origin: contract.AgentOriginGlobal,
		})
	}
	return entries, nil
}

func (s nativeAgentCatalogStub) ListAgentsForWorkspace(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) ([]core.AgentCatalogEntry, error) {
	entries, err := s.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	for _, agent := range s.workspaceAgents {
		entries = append(entries, core.AgentCatalogEntry{
			Def:         compozyconfig.CloneAgentDef(agent),
			Origin:      contract.AgentOriginWorkspace,
			WorkspaceID: workspace.ID,
		})
	}
	return entries, nil
}

func (s nativeAgentCatalogStub) GetAgent(_ context.Context, name string) (core.AgentCatalogEntry, error) {
	for _, agent := range s.agents {
		if agent.Name == name {
			return core.AgentCatalogEntry{Def: agent, Origin: contract.AgentOriginGlobal}, nil
		}
	}
	return core.AgentCatalogEntry{}, os.ErrNotExist
}
