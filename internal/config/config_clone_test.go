package config

import (
	"reflect"
	"testing"

	automationpkg "github.com/compozy/compozy/internal/automation/model"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/resources"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/windowmanager"
)

func TestCloneConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the complete config value", func(t *testing.T) {
		t.Parallel()

		source := configCloneFixture()
		cloned := CloneConfig(&source)
		if !reflect.DeepEqual(cloned, source) {
			t.Fatalf("CloneConfig() = %#v, want %#v", cloned, source)
		}
	})

	t.Run("Should isolate every mutable config family", func(t *testing.T) {
		t.Parallel()

		source := configCloneFixture()
		cloned := CloneConfig(&source)

		cloned.WindowManager.Snap.RepeatRatios[0] = 0.75
		cloned.WindowManager.Shortcuts["focus"][0] = "mutated"
		cloned.MCPServers[0].Args[0] = "mutated"
		cloned.MCPServers[0].Env["TOKEN"] = "mutated"
		cloned.MCPServers[0].Headers["X-Tenant"] = "mutated"
		cloned.MCPServers[0].SecretHeaders["Authorization"] = "mutated"
		cloned.MCPServers[0].Auth.Scopes[0] = "mutated"
		provider := cloned.Providers["codex"]
		provider.MCPServers[0].Env["TOKEN"] = "mutated"
		cloned.Providers["codex"] = provider
		*cloned.ModelCatalog.Sources.ModelsDev.Enabled = false
		sandbox := cloned.Sandboxes["secure"]
		sandbox.Env["MODE"] = "mutated"
		sandbox.Network.AllowList[0] = "mutated"
		cloned.Sandboxes["secure"] = sandbox
		cloned.Memory.Controller.Policy.AllowOrigins[0] = "mutated"
		cloned.Roles.Coordinator.FallbackChain[0].Model = "mutated"
		cloned.RoleSources[RoleCoordinator][RoleFieldModel] = RoleFieldSourceWorkspace
		cloned.Skills.DisabledSkills[0] = "mutated"
		cloned.Skills.AllowedMarketplaceMCP[0] = "mutated"
		cloned.Extensions.Resources.AllowedKinds[0] = resources.ResourceKind("task")
		cloned.Tools.Policy.TrustedSources[0] = "mutated"
		cloned.Session.Attachments.AllowedMIME[0] = "mutated"
		cloned.Automation.Jobs[0].Task.Owner.Ref = "mutated"
		*cloned.Automation.Jobs[0].Task.NetworkParticipation.ChannelID = "mutated"
		cloned.Automation.Jobs[0].LoopTarget.Inputs["nested"].(map[string]any)["value"] = "mutated"
		cloned.Automation.Jobs[0].LoopTarget.InputMapping["topic"] = "mutated"
		cloned.Automation.Triggers[0].Filter["branch"] = "mutated"
		cloned.Loops.Defaults.Delivery.RuntimeDefaults.Worker.Extra["token"] = "mutated"
		cloned.Loops.Defaults.Delivery.RuntimeRules[0].Match.Extra["scope"] = "mutated"
		cloned.Loops.Inputs["delivery"]["payload"].(map[string]any)["value"] = "mutated"
		cloned.Loops.inputSources["delivery"]["payload"] = RoleFieldSourceWorkspace
		cloned.Hooks.Declarations[0].Args[0] = "mutated"
		cloned.Hooks.Declarations[0].Env["TOKEN"] = "mutated"
		*cloned.Hooks.Declarations[0].Matcher.ToolReadOnly = false

		if want := configCloneFixture(); !reflect.DeepEqual(source, want) {
			t.Fatalf("source mutated through CloneConfig(): got %#v, want %#v", source, want)
		}
	})

	t.Run("Should return a zero config for nil", func(t *testing.T) {
		t.Parallel()

		if got := CloneConfig(nil); !reflect.DeepEqual(got, Config{}) {
			t.Fatalf("CloneConfig(nil) = %#v, want zero Config", got)
		}
	})
}

func configCloneFixture() Config {
	enabled := true
	toolReadOnly := true
	channelID := "operations"
	return Config{
		MCP:         MCPConfig{OAuth: MCPOAuthConfig{ClientMetadataURL: "https://example.com/client.json"}},
		Marketplace: MarketplaceRuntimeConfig{Catalog: MarketplaceCatalogConfig{BaseURL: "file:///catalog"}},
		Redact:      RedactConfig{Enabled: true},
		WindowManager: WindowManagerConfig{
			Snap:      WindowManagerSnapConfig{RepeatRatios: []float64{0.5}},
			Shortcuts: map[string]windowmanager.ShortcutBinding{"focus": {"cmd+j"}},
		},
		MCPServers: []MCPServer{{
			Name: "github", Args: []string{"serve"}, Env: map[string]string{"TOKEN": "one"},
			Headers: map[string]string{"X-Tenant": "public"},
			SecretHeaders: map[string]string{
				"Authorization": "vault:extensions/github/env/GITHUB_TOKEN",
			},
			Auth: MCPAuthConfig{Scopes: []string{"repo:read"}},
		}},
		Providers: map[string]ProviderConfig{
			"codex": {MCPServers: []MCPServer{{Name: "provider", Env: map[string]string{"TOKEN": "one"}}}},
		},
		ModelCatalog: ModelCatalogConfig{Sources: ModelCatalogSourcesConfig{
			ModelsDev: ModelsDevSourceConfig{Enabled: &enabled},
		}},
		Sandboxes: map[string]SandboxProfile{
			"secure": {
				Env:     map[string]string{"MODE": "safe"},
				Network: NetworkProfile{AllowList: []string{"api.example.com"}},
			},
		},
		Memory: MemoryConfig{Controller: MemoryControllerConfig{
			Policy: MemoryControllerPolicyConfig{AllowOrigins: []string{"agent"}},
		}},
		Roles: RolesConfig{Coordinator: CoordinatorRoleConfig{RoleConfig: RoleConfig{
			FallbackChain: []RoleFallback{{Provider: "codex", Model: "gpt-5.6"}},
		}}},
		RoleSources: RoleFieldSources{
			RoleCoordinator: {RoleFieldModel: RoleFieldSourceGlobal},
		},
		Skills: SkillsConfig{
			DisabledSkills:          []string{"alpha"},
			AllowedMarketplaceMCP:   []string{"trusted-mcp"},
			AllowedMarketplaceHooks: []string{"trusted-hook"},
		},
		Extensions: ExtensionsConfig{Resources: ExtensionsResourcesConfig{
			AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
		}},
		Tools: ToolsConfig{Policy: ToolsPolicyConfig{TrustedSources: []string{"bundled"}}},
		Session: SessionConfig{Attachments: SessionAttachmentsConfig{
			AllowedMIME: []string{"image/png"},
		}},
		Automation: AutomationConfig{
			Jobs: []AutomationJob{{
				Task: &automationpkg.JobTaskConfig{
					Owner:                &taskpkg.Ownership{Kind: taskpkg.OwnerKindAgentSession, Ref: "agent-1"},
					NetworkParticipation: &participation.Request{ChannelID: &channelID},
				},
				LoopTarget: &automationpkg.LoopTarget{
					Inputs:       map[string]any{"nested": map[string]any{"value": "original"}},
					InputMapping: map[string]string{"topic": "input.topic"},
				},
			}},
			Triggers: []AutomationTrigger{{Filter: map[string]string{"branch": "main"}}},
		},
		Loops: LoopsConfig{
			Defaults: LoopsDefaultsConfig{Delivery: LoopDefaultConfig{
				RuntimeDefaults: dsl.RuntimeDefaults{Worker: dsl.RuntimeSpec{
					Extra: map[string]any{"token": "original"},
				}},
				RuntimeRules: []dsl.RuntimeRule{{
					Match: dsl.RuntimeMatch{Extra: map[string]any{"scope": "original"}},
				}},
			}},
			Inputs: LoopInputDefaults{
				"delivery": {"payload": map[string]any{"value": "original"}},
			},
			inputSources: LoopInputDefaultSources{
				"delivery": {"payload": RoleFieldSourceGlobal},
			},
		},
		Hooks: HooksConfig{Declarations: []hookspkg.HookDecl{{
			Name: "audit", Args: []string{"run"}, Env: map[string]string{"TOKEN": "one"},
			Matcher: hookspkg.HookMatcher{ToolReadOnly: &toolReadOnly},
		}}},
	}
}
