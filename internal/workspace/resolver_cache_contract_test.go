package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/resources"
)

func TestWorkspaceContractResolverCacheDependencies(t *testing.T) {
	t.Parallel()

	t.Run("Should invalidate resolver cache when workspace dotenv appears", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		workspaceConfig := filepath.Join(root, compozyconfig.DirName, compozyconfig.ConfigName)
		agentFile := filepath.Join(
			root,
			compozyconfig.DirName,
			compozyconfig.AgentsDirName,
			"coder",
			agentDefinitionFile,
		)
		writeFile(t, workspaceConfig, "[http]\nport = 4242\n")
		writeAgentDef(t, agentFile, "coder", "v1")

		ws := Workspace{ID: "ws_dotenv_cache", RootDir: root, Name: "repo"}
		loader := &countingConfigLoader{cfg: validConfig(homePaths)}
		currentTime := time.Unix(1_700_010_000, 0).UTC()
		resolver := newTestResolver(t, newMockWorkspaceStore(ws),
			WithHomePaths(homePaths),
			WithConfigLoader(loader.Load),
			withNow(func() time.Time { return currentTime }),
			WithCacheTTL(10*time.Minute),
		)

		if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
			t.Fatalf("Resolve(first) error = %v", err)
		}
		if got := loader.Calls(); got != 1 {
			t.Fatalf("config loader calls after first resolve = %d, want 1", got)
		}

		workspaceEnv := compozyconfig.WorkspaceDotEnvFile(root)
		writeFile(t, workspaceEnv, "COMPOZY_TEST_CACHE_MARKER=one\n")
		touchPath(t, workspaceEnv, time.Unix(1_700_010_100, 0).UTC())
		currentTime = currentTime.Add(time.Minute)
		if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
			t.Fatalf("Resolve(after dotenv creation) error = %v", err)
		}
		if got := loader.Calls(); got != 2 {
			t.Fatalf("config loader calls after dotenv creation = %d, want 2", got)
		}
	})

	t.Run("Should invalidate resolver cache when agent capability catalog appears", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		workspaceConfig := filepath.Join(root, compozyconfig.DirName, compozyconfig.ConfigName)
		agentFile := filepath.Join(
			root,
			compozyconfig.DirName,
			compozyconfig.AgentsDirName,
			"coder",
			agentDefinitionFile,
		)
		capabilityFile := filepath.Join(filepath.Dir(agentFile), "capabilities.toml")
		writeFile(t, workspaceConfig, "[http]\nport = 4242\n")
		writeAgentDef(t, agentFile, "coder", "v1")

		ws := Workspace{ID: "ws_capability_cache", RootDir: root, Name: "repo"}
		loader := &countingConfigLoader{cfg: validConfig(homePaths)}
		currentTime := time.Unix(1_700_020_000, 0).UTC()
		resolver := newTestResolver(t, newMockWorkspaceStore(ws),
			WithHomePaths(homePaths),
			WithConfigLoader(loader.Load),
			withNow(func() time.Time { return currentTime }),
			WithCacheTTL(10*time.Minute),
		)

		if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
			t.Fatalf("Resolve(first) error = %v", err)
		}
		if got := loader.Calls(); got != 1 {
			t.Fatalf("config loader calls after first resolve = %d, want 1", got)
		}

		writeFile(t, capabilityFile, strings.Join([]string{
			"[[capabilities]]",
			"id = \"review-copy\"",
			"summary = \"Review copy.\"",
			"outcome = \"Prioritized review.\"",
			"",
		}, "\n"))
		touchPath(t, capabilityFile, time.Unix(1_700_020_100, 0).UTC())
		currentTime = currentTime.Add(time.Minute)
		afterCapabilities, err := resolver.Resolve(ctx, ws.ID)
		if err != nil {
			t.Fatalf("Resolve(after capability catalog creation) error = %v", err)
		}
		if got := loader.Calls(); got != 2 {
			t.Fatalf("config loader calls after capability catalog creation = %d, want 2", got)
		}
		if got, want := agentCapabilityIDsForContract(
			afterCapabilities.Agents,
			"coder",
		), []string{
			"review-copy",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("agent capability IDs after catalog creation = %#v, want %#v", got, want)
		}
	})
}

func TestWorkspaceContractProfileConfigCacheDependencies(t *testing.T) {
	t.Parallel()

	t.Run("Should invalidate the profile cache when a profile config appears", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		ws := Workspace{ID: "ws_profile_config_cache", RootDir: root, Name: "repo"}
		configCalls := 0
		resolver := newTestResolver(t, newMockWorkspaceStore(ws),
			WithHomePaths(homePaths),
			WithProfileConfigLoader(func(_ string, _ string) (compozyconfig.Config, error) {
				configCalls++
				return validConfig(homePaths), nil
			}),
		)

		if _, err := resolver.ResolveForProfile(t.Context(), ws.ID, "marketing"); err != nil {
			t.Fatalf("ResolveForProfile(first) error = %v", err)
		}
		if _, err := resolver.ResolveForProfile(t.Context(), ws.ID, "marketing"); err != nil {
			t.Fatalf("ResolveForProfile(cache hit) error = %v", err)
		}
		if got, want := configCalls, 1; got != want {
			t.Fatalf("profile config loader calls before dependency change = %d, want %d", got, want)
		}

		profileConfig := filepath.Join(homePaths.ProfilesDir, "marketing", compozyconfig.ConfigName)
		writeFile(t, profileConfig, "[http]\nport = 4243\n")
		if _, err := resolver.ResolveForProfile(t.Context(), ws.ID, "marketing"); err != nil {
			t.Fatalf("ResolveForProfile(after profile config) error = %v", err)
		}
		if got, want := configCalls, 2; got != want {
			t.Fatalf("profile config loader calls after dependency change = %d, want %d", got, want)
		}
	})

	t.Run("Should invalidate the profile cache when an existing profile layer changes", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name string
			path func(compozyconfig.HomePaths, string) string
		}{
			{
				name: "personal config",
				path: func(home compozyconfig.HomePaths, _ string) string {
					return filepath.Join(home.ProfilesDir, "marketing", compozyconfig.ConfigName)
				},
			},
			{
				name: "personal MCP sidecar",
				path: func(home compozyconfig.HomePaths, _ string) string {
					return filepath.Join(home.ProfilesDir, "marketing", compozyconfig.MCPJSONName)
				},
			},
			{
				name: "workspace profile config",
				path: func(_ compozyconfig.HomePaths, root string) string {
					return filepath.Join(
						root, compozyconfig.DirName, compozyconfig.ProfilesDirName,
						"marketing", compozyconfig.ConfigName,
					)
				},
			},
			{
				name: "workspace profile MCP sidecar",
				path: func(_ compozyconfig.HomePaths, root string) string {
					return filepath.Join(
						root, compozyconfig.DirName, compozyconfig.ProfilesDirName,
						"marketing", compozyconfig.MCPJSONName,
					)
				},
			},
		} {
			t.Run("Should invalidate after editing "+test.name, func(t *testing.T) {
				t.Parallel()

				homePaths := newTestHomePaths(t)
				root := t.TempDir()
				dependency := test.path(homePaths, root)
				writeFile(t, dependency, "first\n")
				ws := Workspace{
					ID:      "ws_profile_layer_" + strings.ReplaceAll(test.name, " ", "_"),
					RootDir: root,
					Name:    "repo",
				}
				configCalls := 0
				resolver := newTestResolver(t, newMockWorkspaceStore(ws),
					WithHomePaths(homePaths),
					WithProfileConfigLoader(func(_ string, _ string) (compozyconfig.Config, error) {
						configCalls++
						return validConfig(homePaths), nil
					}),
				)

				if _, err := resolver.ResolveForProfile(t.Context(), ws.ID, "marketing"); err != nil {
					t.Fatalf("ResolveForProfile(first) error = %v", err)
				}
				if _, err := resolver.ResolveForProfile(t.Context(), ws.ID, "marketing"); err != nil {
					t.Fatalf("ResolveForProfile(cache hit) error = %v", err)
				}
				writeFile(t, dependency, "second revision\n")
				if _, err := resolver.ResolveForProfile(t.Context(), ws.ID, "marketing"); err != nil {
					t.Fatalf("ResolveForProfile(after edit) error = %v", err)
				}
				if got, want := configCalls, 2; got != want {
					t.Fatalf("profile config loader calls after %s edit = %d, want %d", test.name, got, want)
				}
			})
		}
	})
}

func TestWorkspaceContractConfigClone(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve Loop Goal and task runtime config", func(t *testing.T) {
		t.Parallel()

		original := compozyconfig.Config{
			Loops: compozyconfig.LoopsConfig{
				Defaults: compozyconfig.LoopsDefaultsConfig{
					Delivery: compozyconfig.LoopDefaultConfig{
						RuntimeDefaults: dsl.RuntimeDefaults{
							Judge: dsl.RuntimeSpec{Model: "judge-v1", Reasoning: "high"},
						},
					},
				},
			},
			Goals: compozyconfig.GoalsConfig{MaxTurns: 7, ContextNudgeRatio: 0.4},
			Task: compozyconfig.TaskConfig{
				Orchestration: compozyconfig.TaskOrchestrationConfig{SummaryMaxBytes: 4096},
			},
		}

		cloned := cloneConfig(&original)
		if cloned.Loops.Defaults.Delivery.RuntimeDefaults.Judge.Model != "judge-v1" ||
			cloned.Loops.Defaults.Delivery.RuntimeDefaults.Judge.Reasoning != "high" ||
			cloned.Goals.MaxTurns != 7 || cloned.Goals.ContextNudgeRatio != 0.4 ||
			cloned.Task.Orchestration.SummaryMaxBytes != 4096 {
			t.Fatalf("cloned runtime config = %#v", cloned)
		}
		cloned.Goals.MaxTurns = 9
		if original.Goals.MaxTurns != 7 {
			t.Fatalf("original Goals.MaxTurns = %d, want 7", original.Goals.MaxTurns)
		}
	})

	t.Run("Should deep copy mutable memory extensions and automation config", func(t *testing.T) {
		t.Parallel()

		original := compozyconfig.Config{
			Memory: compozyconfig.MemoryConfig{
				Controller: compozyconfig.MemoryControllerConfig{
					Policy: compozyconfig.MemoryControllerPolicyConfig{
						AllowOrigins: []string{"agent"},
					},
				},
			},
			Extensions: compozyconfig.ExtensionsConfig{
				Resources: compozyconfig.ExtensionsResourcesConfig{
					AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
				},
			},
			Automation: compozyconfig.AutomationConfig{
				Triggers: []compozyconfig.AutomationTrigger{{
					Name:   "github-push",
					Filter: map[string]string{"branch": "main"},
				}},
			},
		}

		cloned := cloneConfig(&original)
		cloned.Memory.Controller.Policy.AllowOrigins[0] = "operator"
		cloned.Extensions.Resources.AllowedKinds[0] = resources.ResourceKind("task")
		cloned.Automation.Triggers[0].Filter["branch"] = "release"

		if got, want := original.Memory.Controller.Policy.AllowOrigins, []string{"agent"}; !slices.Equal(got, want) {
			t.Fatalf("original Memory.Controller.Policy.AllowOrigins = %#v, want %#v", got, want)
		}
		if got, want := original.Extensions.Resources.AllowedKinds, []resources.ResourceKind{
			resources.ResourceKind("tool"),
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("original Extensions.Resources.AllowedKinds = %#v, want %#v", got, want)
		}
		if got, want := original.Automation.Triggers[0].Filter["branch"], "main"; got != want {
			t.Fatalf("original Automation.Triggers[0].Filter[branch] = %q, want %q", got, want)
		}
	})
}

func agentCapabilityIDsForContract(agents []compozyconfig.AgentDef, name string) []string {
	for _, agent := range agents {
		if agent.Name != name || agent.Capabilities == nil {
			continue
		}

		ids := make([]string, 0, len(agent.Capabilities.Capabilities))
		for _, capability := range agent.Capabilities.Capabilities {
			ids = append(ids, capability.ID)
		}
		return ids
	}
	return nil
}

func TestWorkspaceContractAgentConfigResolution(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate caller mutations from cold and warm narrow cache results", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		additionalDir := mustCanonicalRoot(t, t.TempDir())
		ws := Workspace{
			ID: "ws_agent_config_copies", RootDir: t.TempDir(), Name: "repo",
			AdditionalDirs: []string{additionalDir},
		}
		cfg := validConfig(homePaths)
		cfg.Memory.Controller.Policy.AllowOrigins = []string{"agent"}
		cfg.Automation.Triggers = []compozyconfig.AutomationTrigger{{
			Name: "github-push", Filter: map[string]string{"branch": "main"},
		}}
		resolver := newTestResolver(t, newMockWorkspaceStore(ws),
			WithHomePaths(homePaths),
			WithConfigLoader(func(string) (compozyconfig.Config, error) { return cfg, nil }),
			withNow(func() time.Time { return time.Unix(1_700_050_000, 0).UTC() }),
		)
		writeFile(t,
			filepath.Join(ws.RootDir, compozyconfig.DirName, compozyconfig.AgentsDirName, "coder", agentDefinitionFile),
			strings.Join([]string{
				"---", "name: coder", "provider: claude", "model: original",
				"tools: [compozy__session_list]", "---", "", "Prompt for coder.", "",
			}, "\n"),
		)

		for attempt := range 3 {
			resolved, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, "")
			if err != nil {
				t.Fatalf("ResolveAgentConfig(attempt %d) error = %v", attempt, err)
			}
			if got := resolved.Config.Memory.Controller.Policy.AllowOrigins; !slices.Equal(got, []string{"agent"}) {
				t.Fatalf("config origins on attempt %d = %#v, want original agent slice", attempt, got)
			}
			if got := resolved.Config.Automation.Triggers; len(got) != 1 || got[0].Filter["branch"] != "main" {
				t.Fatalf("config automation on attempt %d = %#v, want original main filter", attempt, got)
			}
			if got := resolved.Agents; len(got) != 1 || got[0].Model != "original" ||
				!slices.Equal(got[0].Tools, []string{"compozy__session_list"}) {
				t.Fatalf("agents on attempt %d = %#v, want original coder model and tools", attempt, got)
			}
			if got := resolved.AdditionalDirs; !slices.Equal(got, []string{additionalDir}) {
				t.Fatalf("additional directories on attempt %d = %#v, want %#v", attempt, got, []string{additionalDir})
			}
			if attempt == 2 {
				continue
			}
			resolved.Config.Memory.Controller.Policy.AllowOrigins[0] = "caller-origin"
			resolved.Config.Automation.Triggers[0].Filter["branch"] = "caller-branch"
			resolved.Agents[0].Model = "caller-model"
			resolved.Agents[0].Tools[0] = "caller-tool"
			resolved.AdditionalDirs[0] = "caller-directory"
		}
	})

	t.Run("Should resolve config and agents without traversing an invalid skills directory", func(t *testing.T) {
		t.Parallel()

		resolver, ws, _ := newAgentConfigContractResolver(t)
		workspaceDir := filepath.Join(ws.RootDir, compozyconfig.DirName)
		writeFile(t, filepath.Join(workspaceDir, compozyconfig.ConfigName), "[defaults]\nagent = \"coder\"\n")
		writeAgentDef(
			t,
			filepath.Join(workspaceDir, compozyconfig.AgentsDirName, "coder", agentDefinitionFile),
			"coder",
			"selected-model",
		)
		writeFile(t, filepath.Join(workspaceDir, compozyconfig.SkillsDirName), "not a resource directory")

		if _, err := resolver.Resolve(t.Context(), ws.ID); err == nil {
			t.Fatal("Resolve(invalid skills directory) error = nil, want rejection")
		} else if !strings.Contains(err.Error(), "scan skills directory") {
			t.Fatalf("Resolve(invalid skills directory) error = %v, want skill traversal failure", err)
		}
		resolved, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, "")
		if err != nil {
			t.Fatalf("ResolveAgentConfig(invalid skills directory) error = %v", err)
		}
		if resolved.Config.Defaults.Agent != "coder" || agentModel(resolved.Agents, "coder") != "selected-model" {
			t.Fatalf("narrow resolution with unrelated invalid skills = %#v, want configured coder", resolved)
		}
	})

	for _, profileName := range []string{"", "marketing"} {
		t.Run(
			"Should preserve full resolution values and skills after narrow resolution for "+profileName,
			func(t *testing.T) {
				t.Parallel()

				availability := &agentConfigContractProfileAvailability{id: "profile-marketing"}
				resolver, ws, homePaths := newAgentConfigContractResolver(
					t,
					WithProfileAvailabilityChecker(availability),
				)
				writeAgentDef(t, filepath.Join(homePaths.AgentsDir, "coder", agentDefinitionFile), "coder", "global")
				workspaceDir := filepath.Join(ws.RootDir, compozyconfig.DirName)
				writeFile(t, filepath.Join(workspaceDir, compozyconfig.ConfigName), "[defaults]\nagent = \"coder\"\n")
				writeAgentDef(
					t,
					filepath.Join(workspaceDir, compozyconfig.AgentsDirName, "coder", agentDefinitionFile),
					"coder",
					"workspace",
				)
				personalDir := filepath.Join(homePaths.ProfilesDir, "marketing")
				writeAgentDef(
					t,
					filepath.Join(personalDir, compozyconfig.AgentsDirName, "personal", agentDefinitionFile),
					"personal",
					"personal",
				)
				profileDir := filepath.Join(workspaceDir, compozyconfig.ProfilesDirName, "marketing")
				writeFile(t, filepath.Join(profileDir, compozyconfig.ConfigName), "[defaults]\nagent = \"personal\"\n")
				writeAgentDef(
					t,
					filepath.Join(profileDir, compozyconfig.AgentsDirName, "coder", agentDefinitionFile),
					"coder",
					"profile",
				)
				writeSkill(t, filepath.Join(workspaceDir, compozyconfig.SkillsDirName, "first"))

				narrow, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, profileName)
				if err != nil {
					t.Fatalf("ResolveAgentConfig() error = %v", err)
				}
				var full ResolvedWorkspace
				if profileName == "" {
					full, err = resolver.Resolve(t.Context(), ws.ID)
				} else {
					full, err = resolver.ResolveForProfile(t.Context(), ws.ID, profileName)
				}
				if err != nil {
					t.Fatalf("full resolution after ResolveAgentConfig() error = %v", err)
				}
				if !reflect.DeepEqual(narrow.Workspace, full.Workspace) || narrow.WorkspaceID == "" ||
					narrow.WorkspaceID != full.WorkspaceID || narrow.ProfileID != full.ProfileID ||
					narrow.ProfileName != full.ProfileName {
					t.Fatalf("narrow workspace/profile identity = %#v, want full identity %#v", narrow, full)
				}
				if !reflect.DeepEqual(narrow.Config, full.Config) || !reflect.DeepEqual(narrow.Agents, full.Agents) {
					t.Fatalf("narrow config/agents differ from full resolution: narrow = %#v, full = %#v", narrow, full)
				}
				if got, want := skillNames(full.Skills), []string{"first"}; !slices.Equal(got, want) {
					t.Fatalf("full skills after narrow resolution = %#v, want %#v", got, want)
				}
				wantAgent, wantModel, wantProfileID := "coder", "workspace", ""
				if profileName != "" {
					wantAgent, wantModel, wantProfileID = "personal", "profile", availability.id
				}
				if narrow.Config.Defaults.Agent != wantAgent || agentModel(narrow.Agents, "coder") != wantModel ||
					narrow.ProfileID != wantProfileID || narrow.ProfileName != profileName {
					t.Fatalf(
						"narrow selection = %#v, want agent %q, model %q, profile ID %q",
						narrow,
						wantAgent,
						wantModel,
						wantProfileID,
					)
				}
			},
		)
	}
}

func TestWorkspaceContractAgentConfigDependencies(t *testing.T) {
	t.Parallel()

	for _, profileName := range []string{"", "marketing"} {
		t.Run("Should refresh changed config and agents before cache expiry for "+profileName, func(t *testing.T) {
			t.Parallel()

			resolver, ws, _ := newAgentConfigContractResolver(t)
			resourceDir := filepath.Join(ws.RootDir, compozyconfig.DirName)
			if profileName != "" {
				resourceDir = filepath.Join(resourceDir, compozyconfig.ProfilesDirName, profileName)
			}
			configFile := filepath.Join(resourceDir, compozyconfig.ConfigName)
			agentFile := filepath.Join(resourceDir, compozyconfig.AgentsDirName, "coder", agentDefinitionFile)
			writeFile(t, configFile, "[defaults]\nagent = \"coder\"\n")
			writeAgentDef(t, agentFile, "coder", "original")
			first, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, profileName)
			if err != nil {
				t.Fatalf("ResolveAgentConfig(first) error = %v", err)
			}
			if first.Config.Defaults.Agent != "coder" || agentModel(first.Agents, "coder") != "original" {
				t.Fatalf("initial config and agent selection = %#v", first)
			}

			writeFile(t, configFile, "[defaults]\nagent = \"reviewer\"\n")
			afterConfig, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, profileName)
			if err != nil {
				t.Fatalf("ResolveAgentConfig(after config edit) error = %v", err)
			}
			if afterConfig.Config.Defaults.Agent != "reviewer" {
				t.Fatalf("default agent after config edit = %q, want reviewer", afterConfig.Config.Defaults.Agent)
			}

			writeAgentDef(t, agentFile, "coder", "changed-model")
			afterAgent, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, profileName)
			if err != nil {
				t.Fatalf("ResolveAgentConfig(after agent edit) error = %v", err)
			}
			if got := agentModel(afterAgent.Agents, "coder"); got != "changed-model" {
				t.Fatalf("agent model after edit = %q, want changed-model", got)
			}
		})
	}
}

func TestWorkspaceContractAgentConfigInvalidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		invalidate func(*Resolver, string, *time.Time)
	}{
		{
			name: "Should refresh narrow config after workspace invalidation",
			invalidate: func(resolver *Resolver, workspaceID string, _ *time.Time) {
				resolver.Invalidate(workspaceID)
			},
		},
		{
			name: "Should refresh narrow config after global invalidation",
			invalidate: func(resolver *Resolver, _ string, _ *time.Time) {
				resolver.InvalidateAll()
			},
		},
		{
			name: "Should refresh narrow config after idle cache expiration",
			invalidate: func(_ *Resolver, _ string, now *time.Time) {
				*now = now.Add(10*time.Minute + time.Second)
			},
		},
	} {
		for _, profileName := range []string{"", "marketing"} {
			t.Run(test.name+" for profile "+profileName, func(t *testing.T) {
				t.Parallel()

				currentTime := time.Unix(1_700_040_000, 0).UTC()
				var loadedConfig compozyconfig.Config
				resolver, ws, homePaths := newAgentConfigContractResolver(t,
					WithConfigLoader(func(string) (compozyconfig.Config, error) {
						return loadedConfig, nil
					}),
					WithProfileConfigLoader(func(string, string) (compozyconfig.Config, error) {
						return loadedConfig, nil
					}),
					withNow(func() time.Time { return currentTime }),
				)
				loadedConfig = validConfig(homePaths)
				loadedConfig.Defaults.Agent = "coder"
				first, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, profileName)
				if err != nil {
					t.Fatalf("ResolveAgentConfig(first) error = %v", err)
				}
				if first.Config.Defaults.Agent != "coder" {
					t.Fatalf("initial default agent = %q, want coder", first.Config.Defaults.Agent)
				}
				loadedConfig.Defaults.Agent = "reviewer"
				cached, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, profileName)
				if err != nil {
					t.Fatalf("ResolveAgentConfig(before invalidation) error = %v", err)
				}
				if cached.Config.Defaults.Agent != "coder" {
					t.Fatalf("cached default agent before invalidation = %q, want coder", cached.Config.Defaults.Agent)
				}

				test.invalidate(resolver, ws.ID, &currentTime)
				refreshed, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, profileName)
				if err != nil {
					t.Fatalf("ResolveAgentConfig(after invalidation) error = %v", err)
				}
				if refreshed.Config.Defaults.Agent != "reviewer" {
					t.Fatalf("default agent after invalidation = %q, want reviewer", refreshed.Config.Defaults.Agent)
				}
			})
		}
	}

	for _, test := range []struct {
		name       string
		invalidate func(*Resolver, string)
	}{
		{
			name: "Should not republish narrow config after concurrent workspace invalidation",
			invalidate: func(resolver *Resolver, workspaceID string) {
				resolver.Invalidate(workspaceID)
			},
		},
		{
			name: "Should not republish narrow config after concurrent global invalidation",
			invalidate: func(resolver *Resolver, _ string) {
				resolver.InvalidateAll()
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			homePaths := newTestHomePaths(t)
			oldConfig := validConfig(homePaths)
			oldConfig.Defaults.Agent = "coder"
			newConfig := compozyconfig.CloneConfig(&oldConfig)
			newConfig.Defaults.Agent = "reviewer"

			loadStarted := make(chan struct{})
			releaseLoad := make(chan struct{})
			var loadMu sync.Mutex
			loadCalls := 0
			resolveCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			resolver, ws, _ := newAgentConfigContractResolver(t,
				WithConfigLoader(func(string) (compozyconfig.Config, error) {
					loadMu.Lock()
					loadCalls++
					call := loadCalls
					loadMu.Unlock()
					if call == 1 {
						close(loadStarted)
						select {
						case <-releaseLoad:
							return oldConfig, nil
						case <-resolveCtx.Done():
							return compozyconfig.Config{}, resolveCtx.Err()
						}
					}
					return newConfig, nil
				}),
			)

			type resolutionResult struct {
				resolved ResolvedAgentConfig
				err      error
			}
			result := make(chan resolutionResult, 1)
			go func() {
				resolved, err := resolver.ResolveAgentConfig(resolveCtx, ws.ID, "")
				result <- resolutionResult{resolved: resolved, err: err}
			}()

			select {
			case <-loadStarted:
			case first := <-result:
				t.Fatalf("ResolveAgentConfig finished before loader barrier: error = %v", first.err)
			case <-resolveCtx.Done():
				first := <-result
				t.Fatalf("ResolveAgentConfig did not reach loader barrier: error = %v", first.err)
			}
			test.invalidate(resolver, ws.ID)
			close(releaseLoad)
			first := <-result
			if first.err != nil {
				t.Fatalf("ResolveAgentConfig(concurrent invalidation) error = %v", first.err)
			}
			if first.resolved.Config.Defaults.Agent != "coder" {
				t.Fatalf("in-flight default agent = %q, want coder", first.resolved.Config.Defaults.Agent)
			}

			refreshed, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, "")
			if err != nil {
				t.Fatalf("ResolveAgentConfig(after concurrent invalidation) error = %v", err)
			}
			if refreshed.Config.Defaults.Agent != "reviewer" {
				t.Fatalf(
					"default agent after concurrent invalidation = %q, want reviewer",
					refreshed.Config.Defaults.Agent,
				)
			}
			loadMu.Lock()
			calls := loadCalls
			loadMu.Unlock()
			if calls != 2 {
				t.Fatalf("config loader calls after concurrent invalidation = %d, want 2", calls)
			}
		})
	}
}

func TestWorkspaceContractAgentConfigIdentity(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		change func(*testing.T, string)
		want   error
	}{
		{
			name: "Should reject a removed workspace root after narrow cache warmup",
			change: func(t *testing.T, root string) {
				t.Helper()
				if err := os.RemoveAll(root); err != nil {
					t.Fatalf("RemoveAll(root) error = %v", err)
				}
			},
			want: ErrWorkspaceRootMissing,
		},
		{
			name: "Should reject invalid current workspace identity after narrow cache warmup",
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, identityPath(root), `workspace_id = "invalid"`)
			},
			want: ErrWorkspaceIdentityInvalid,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resolver, ws, _ := newAgentConfigContractResolver(t)
			if _, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, ""); err != nil {
				t.Fatalf("ResolveAgentConfig(first) error = %v", err)
			}
			test.change(t, ws.RootDir)
			got, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, "")
			if !errors.Is(err, test.want) {
				t.Fatalf("ResolveAgentConfig(after identity change) error = %v, want %v", err, test.want)
			}
			if got.WorkspaceID != "" || got.ID != "" || len(got.Agents) != 0 {
				t.Fatalf("failed narrow resolution returned workspace/agent data: %#v", got)
			}
		})
	}

	t.Run("Should recheck current profile ownership and availability on narrow cache hits", func(t *testing.T) {
		t.Parallel()

		availability := &agentConfigContractProfileAvailability{id: "profile-first"}
		resolver, ws, _ := newAgentConfigContractResolver(t, WithProfileAvailabilityChecker(availability))
		first, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, " marketing ")
		if err != nil {
			t.Fatalf("ResolveAgentConfig(first profile) error = %v", err)
		}
		if first.ProfileName != "marketing" || first.ProfileID != "profile-first" {
			t.Fatalf("first profile identity = %q/%q, want marketing/profile-first", first.ProfileName, first.ProfileID)
		}
		availability.id = "profile-recreated"
		current, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, "marketing")
		if err != nil {
			t.Fatalf("ResolveAgentConfig(recreated profile) error = %v", err)
		}
		if current.ProfileID != "profile-recreated" {
			t.Fatalf("current profile ID = %q, want profile-recreated", current.ProfileID)
		}
		availability.err = errors.New("profile lifecycle operation in progress")
		if _, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, "marketing"); !errors.Is(err, availability.err) {
			t.Fatalf("ResolveAgentConfig(unavailable profile) error = %v, want %v", err, availability.err)
		}
		if _, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, ""); err != nil {
			t.Fatalf("ResolveAgentConfig(unprofiled workspace) error = %v", err)
		}
		availability.err = nil
		if _, err := resolver.ResolveAgentConfig(t.Context(), ws.ID, "../marketing"); err == nil ||
			!strings.Contains(
				err.Error(),
				`workspace: resolve profile resources: config: resource profile name "../marketing" must match`,
			) {
			t.Fatalf(
				"ResolveAgentConfig(invalid profile name) error = %v, want wrapped profile validation failure",
				err,
			)
		}
	})
}

func newAgentConfigContractResolver(t *testing.T, opts ...Option) (*Resolver, Workspace, compozyconfig.HomePaths) {
	t.Helper()

	homePaths := newTestHomePaths(t)
	ws := Workspace{ID: "ws_agent_config_contract", RootDir: t.TempDir(), Name: "repo"}
	baseOpts := []Option{
		WithHomePaths(homePaths),
		WithConfigLoader(func(root string) (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(root))
		}),
		WithProfileConfigLoader(func(root, profileName string) (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(
				homePaths,
				compozyconfig.WithWorkspaceRoot(root),
				compozyconfig.WithProfile(profileName),
			)
		}),
		withNow(func() time.Time { return time.Unix(1_700_030_000, 0).UTC() }),
		WithCacheTTL(10 * time.Minute),
	}
	return newTestResolver(t, newMockWorkspaceStore(ws), append(baseOpts, opts...)...), ws, homePaths
}

type agentConfigContractProfileAvailability struct {
	id  string
	err error
}

var _ ProfileAvailabilityChecker = (*agentConfigContractProfileAvailability)(nil)

func (a *agentConfigContractProfileAvailability) EnsureAvailableName(context.Context, string) error {
	return a.err
}

func (a *agentConfigContractProfileAvailability) AvailableProfileID(context.Context, string) (string, error) {
	return a.id, a.err
}
