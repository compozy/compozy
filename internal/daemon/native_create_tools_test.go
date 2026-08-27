package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestNativeNetworkChannelCreate(t *testing.T) {
	t.Parallel()

	var created store.NetworkChannelEntry
	createCalls := 0
	netStore := apitest.StubNetworkStore{
		CreateNetworkChannelFn: func(_ context.Context, entry store.NetworkChannelEntry) error {
			createCalls++
			created = entry
			return nil
		},
	}
	registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
		Network:      &nativeNetworkStub{},
		NetworkStore: netStore,
		Workspaces:   nativeNetworkTestWorkspaceService(t),
		Sessions:     nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
	}, nativeApproveAllPolicyInputs())

	t.Run("Should register a channel with purpose through the network store", func(t *testing.T) {
		result, err := registry.Call(t.Context(), toolspkg.Scope{
			ProfileID: store.DefaultProfileID,
			Operator:  true,
		}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDNetworkChannelCreate,
			Input: json.RawMessage(
				`{"workspace":"ws-native-network","channel":"design","purpose":"UI reviews"}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(network_channel_create) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"design"`))
		if createCalls != 1 {
			t.Fatalf("CreateNetworkChannel calls = %d, want 1", createCalls)
		}
		if created.Channel != "design" ||
			created.ProfileID != store.DefaultProfileID ||
			created.WorkspaceID != nativeNetworkTestWorkspaceID ||
			created.Purpose != "UI reviews" {
			t.Fatalf("created entry = %#v, want default-owned design/native-workspace/UI reviews", created)
		}
	})

	t.Run("Should persist the registered workspace id when Compozy identity differs", func(t *testing.T) {
		registryWorkspaceID := "ws-native-network"
		identityWorkspaceID := "01KSGVKVZVS4WP4HVMFE08J96Y"
		var stored store.NetworkChannelEntry
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network: &nativeNetworkStub{},
			NetworkStore: apitest.StubNetworkStore{
				CreateNetworkChannelFn: func(_ context.Context, entry store.NetworkChannelEntry) error {
					stored = entry
					return nil
				},
			},
			Workspaces: apitest.StubWorkspaceService{
				ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if err := ctx.Err(); err != nil {
						return workspacepkg.ResolvedWorkspace{}, err
					}
					if ref != registryWorkspaceID && ref != identityWorkspaceID {
						t.Fatalf(
							"Resolve() ref = %q, want registry %q or identity %q",
							ref,
							registryWorkspaceID,
							identityWorkspaceID,
						)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      registryWorkspaceID,
							RootDir: t.TempDir(),
							Name:    "native-network",
						},
						WorkspaceID: identityWorkspaceID,
					}, nil
				},
			},
			Sessions: nativeNetworkTestSessionManager(registryWorkspaceID),
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(t.Context(), toolspkg.Scope{
			ProfileID: store.DefaultProfileID,
			Operator:  true,
		}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDNetworkChannelCreate,
			Input: json.RawMessage(
				"{\"workspace\":\"ws-native-network\",\"channel\":\"general\",\"purpose\":\"Announcements\"}",
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(network_channel_create) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte("\"general\""))
		if stored.WorkspaceID != registryWorkspaceID {
			t.Fatalf(
				"stored workspace_id = %q, want registry id %q; identity id %q must not be persisted",
				stored.WorkspaceID,
				registryWorkspaceID,
				identityWorkspaceID,
			)
		}
	})

	t.Run("Should create a durable channel when registry and identity ids differ", func(t *testing.T) {
		registryWorkspaceID := "ws-native-network"
		identityWorkspaceID := "01KSGVKVZVS4WP4HVMFE08J96Y"
		root := t.TempDir()
		db, err := openDaemonTestGlobalDBAtPath(t.Context(), filepath.Join(t.TempDir(), store.GlobalDatabaseName))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if closeErr := db.Close(cleanupCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})
		workspace := workspacepkg.Workspace{
			ID:      registryWorkspaceID,
			RootDir: root,
			Name:    "native-network",
		}
		if err := db.InsertWorkspace(t.Context(), workspace); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Network:      &nativeNetworkStub{},
			NetworkStore: db,
			Workspaces: apitest.StubWorkspaceService{
				ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if err := ctx.Err(); err != nil {
						return workspacepkg.ResolvedWorkspace{}, err
					}
					if ref != registryWorkspaceID && ref != identityWorkspaceID {
						t.Fatalf(
							"Resolve() ref = %q, want registry %q or identity %q",
							ref,
							registryWorkspaceID,
							identityWorkspaceID,
						)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspace,
						WorkspaceID: identityWorkspaceID,
					}, nil
				},
			},
			Sessions: nativeNetworkTestSessionManager(registryWorkspaceID),
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(t.Context(), toolspkg.Scope{
			ProfileID: store.DefaultProfileID,
			Operator:  true,
		}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDNetworkChannelCreate,
			Input: json.RawMessage(
				"{\"workspace\":\"ws-native-network\",\"channel\":\"durable\",\"purpose\":\"Durable coordination\"}",
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(network_channel_create) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte("\"durable\""))
		entry, err := db.GetNetworkChannel(t.Context(), store.ReadScope{AllProfiles: true}, store.NetworkChannelRef{
			WorkspaceID: registryWorkspaceID,
			Channel:     "durable",
		})
		if err != nil {
			t.Fatalf("GetNetworkChannel() error = %v", err)
		}
		if entry.Purpose != "Durable coordination" {
			t.Fatalf("entry purpose = %q, want Durable coordination", entry.Purpose)
		}
	})

	t.Run("Should reject an invalid channel name", func(t *testing.T) {
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDNetworkChannelCreate,
			Input: json.RawMessage(
				`{"workspace":"ws-native-network","channel":"Bad Name","purpose":"x"}`,
			),
		})
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
	})

	t.Run("Should require a purpose", func(t *testing.T) {
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDNetworkChannelCreate,
			Input: json.RawMessage(
				`{"workspace":"ws-native-network","channel":"general","purpose":"   "}`,
			),
		})
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
	})
}

func TestNativeNetworkChannelUpdate(t *testing.T) {
	t.Parallel()

	entry := store.NetworkChannelEntry{
		ProfileID:    store.DefaultProfileID,
		WorkspaceID:  nativeNetworkTestWorkspaceID,
		Channel:      "design",
		Purpose:      "UI reviews",
		FanoutPolicy: store.NetworkFanoutPolicyCapabilityMatch,
		CreatedBy:    "test",
	}
	patchCalls := 0
	netStore := apitest.StubNetworkStore{
		GetNetworkChannelFn: func(_ context.Context, ref store.NetworkChannelRef) (store.NetworkChannelEntry, error) {
			if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "design" {
				return store.NetworkChannelEntry{}, fmt.Errorf(
					"GetNetworkChannel() ref = %#v, want native design channel",
					ref,
				)
			}
			return entry, nil
		},
		PatchNetworkChannelFn: func(
			_ context.Context,
			readScope store.ReadScope,
			ref store.NetworkChannelRef,
			patch store.NetworkChannelPatch,
		) error {
			if readScope.ProfileID != store.DefaultProfileID {
				return fmt.Errorf("PatchNetworkChannel() scope = %#v, want default profile", readScope)
			}
			if ref.WorkspaceID != nativeNetworkTestWorkspaceID || ref.Channel != "design" {
				return fmt.Errorf("PatchNetworkChannel() ref = %#v, want native design channel", ref)
			}
			patchCalls++
			entry = patch.Apply(entry)
			if err := entry.Validate(); err != nil {
				return fmt.Errorf("patched entry Validate(): %w", err)
			}
			return nil
		},
		WriteNetworkChannelFn: func(context.Context, store.NetworkChannelEntry) error {
			return fmt.Errorf("WriteNetworkChannel() should not be called for a partial update")
		},
	}
	registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
		Network:      &nativeNetworkStub{},
		NetworkStore: netStore,
		Workspaces:   nativeNetworkTestWorkspaceService(t),
		Sessions:     nativeNetworkTestSessionManager(nativeNetworkTestWorkspaceID),
	}, nativeApproveAllPolicyInputs())

	t.Run("Should return the public snake case channel payload", func(t *testing.T) {
		t.Parallel()

		result, err := registry.Call(t.Context(), toolspkg.Scope{
			ProfileID: store.DefaultProfileID,
			Operator:  true,
		}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDNetworkChannelUpdate,
			Input: json.RawMessage(
				`{"workspace":"ws-native-network","channel":"design","purpose":"Pair reviews","fanout_policy":"coordinator","coordinator_peer_id":"reviewer.sess-a"}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(network_channel_update) error = %v", err)
		}
		if patchCalls != 1 {
			t.Fatalf("PatchNetworkChannel calls = %d, want 1", patchCalls)
		}
		requireNativeStructuredContains(t, result, []byte(`"workspace_id":"ws-native-network"`))
		requireNativeStructuredContains(t, result, []byte(`"fanout_policy":"coordinator"`))
		requireNativeStructuredContains(t, result, []byte(`"coordinator_peer_id":"reviewer.sess-a"`))
		requireNativeStructuredExcludes(t, result, []byte(`"WorkspaceID"`))
	})
}

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
				_ string,
			) (workspacepkg.ResolvedWorkspace, error) {
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
			Workspaces: nativeNetworkTestWorkspaceService(t),
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
			Workspaces: nativeNetworkTestWorkspaceService(t),
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
			Workspaces: nativeNetworkTestWorkspaceService(t),
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
		Workspaces:  nativeNetworkTestWorkspaceService(t),
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
		if agent.Speed != "fast" || len(agent.ACPOptions) != 2 || agent.ACPOptions[0].ID != "context" ||
			agent.ACPOptions[0].ValueID != "1m" || agent.ACPOptions[1].ID != "thinking" ||
			agent.ACPOptions[1].BoolValue == nil || !*agent.ACPOptions[1].BoolValue {
			t.Fatalf("agent runtime defaults = speed %q options %#v", agent.Speed, agent.ACPOptions)
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

		const workspaceID = "ws-native-network"
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
							Name:    "native-network",
						},
						WorkspaceID: workspaceID,
						Agents: []compozyconfig.AgentDef{
							{Name: compozyconfig.DefaultAgentName, Provider: "codex", Prompt: "General."},
							{Name: "onboarding", Provider: "codex", Prompt: "Onboarding."},
						},
					}, nil
				},
			},
			Sessions: nativeNetworkTestSessionManager(workspaceID),
			AgentCatalog: nativeAgentCatalogStub{agents: []compozyconfig.AgentDef{
				{Name: "catalog-visible", Provider: "codex", Prompt: "Catalog visible."},
				{Name: "onboarding", Provider: "codex", Prompt: "Catalog onboarding."},
			}, workspaceAgents: []compozyconfig.AgentDef{
				{Name: "dev-linked", Provider: "codex", Prompt: "Workspace extension agent."},
			}},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWorkspaceDescribe,
			Input:  json.RawMessage("{\"workspace\":\"ws-native-network\"}"),
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
