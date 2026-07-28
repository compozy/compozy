package workspace

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
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
