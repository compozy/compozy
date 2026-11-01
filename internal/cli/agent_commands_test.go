package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestAgentListAndInfoCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list and inspect agents from the inferred workspace", func(t *testing.T) {
		t.Parallel()

		agent := AgentRecord{
			Name: "coder",
			EffectiveRuntime: &contract.AgentEffectiveRuntimePayload{
				Provider: "fake",
				SessionEnvStatus: []contract.SettingsProviderCredentialStatusPayload{
					{
						Name:      "GH_TOKEN",
						TargetEnv: "GH_TOKEN",
						SecretRef: "env:GH_TOKEN",
						Source:    "env",
						Present:   true,
						Required:  true,
					},
				},
			},
			Provider:        "fake",
			Command:         "codex",
			Model:           "gpt-5.4",
			ReasoningEffort: "max",
			Speed:           contract.SpeedFast,
			ACPOptions: []contract.AgentACPOptionSelection{
				{ID: "context", ValueID: "1m"}, {ID: "thinking", BoolValue: new(true)},
			},
			Tools:        []string{"shell", "git"},
			Permissions:  "standard",
			CategoryPath: []string{"Marketing", "Sales"},
			Origin:       contract.AgentOriginWorkspace,
			WorkspaceID:  "ws-1",
			Layer:        "project_profile",
			Shadows: []contract.AgentDefinitionShadowPayload{
				{Layer: "profile", Path: "/profiles/marketing/agents/coder/AGENT.md"},
				{Layer: "user", Path: "/agents/coder/AGENT.md"},
			},
			Skills:           &contract.CreateAgentSkillsConfig{Disabled: []string{"legacy-review"}},
			DefinitionDigest: "digest-coder",
			Prompt:           "You are coder.",
			MCPServers: []AgentMCPServer{{
				Name:    "github",
				Command: "compozy-github",
				Args:    []string{"serve"},
			}},
		}

		deps := newWorkspaceTestDeps(t, &stubClient{
			listAgentsFn: func(_ context.Context, query AgentQuery) ([]AgentRecord, error) {
				if query.Workspace != "/workspace/project" {
					t.Fatalf("ListAgents() workspace = %q, want inferred workspace", query.Workspace)
				}
				return []AgentRecord{agent}, nil
			},
			getAgentFn: func(_ context.Context, name string, query AgentQuery) (AgentRecord, error) {
				if name != agent.Name {
					t.Fatalf("GetAgent() name = %q, want %q", name, agent.Name)
				}
				if query.Workspace != "/workspace/project" {
					t.Fatalf("GetAgent() workspace = %q, want inferred workspace", query.Workspace)
				}
				return agent, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "agent", "list", "-o", "json")
		if err != nil {
			t.Fatalf("agent list error = %v", err)
		}

		var listed []AgentRecord
		if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
			t.Fatalf("json.Unmarshal(agent list) error = %v", err)
		}
		if len(listed) != 1 || listed[0].Name != agent.Name {
			t.Fatalf("listed agents = %#v, want one %q record", listed, agent.Name)
		}
		if listed[0].Layer != agent.Layer || !slices.Equal(listed[0].Shadows, agent.Shadows) {
			t.Fatalf(
				"listed agent provenance = %#v/%#v, want %q/%#v",
				listed[0].Layer,
				listed[0].Shadows,
				agent.Layer,
				agent.Shadows,
			)
		}
		if got, want := strings.Join(listed[0].CategoryPath, ","), "Marketing,Sales"; got != want {
			t.Fatalf("listed agent category_path = %#v, want %q", listed[0].CategoryPath, want)
		}

		listHuman, _, err := executeRootCommand(t, deps, "agent", "list", "-o", "human")
		if err != nil {
			t.Fatalf("agent list human error = %v", err)
		}
		if !strings.Contains(listHuman, "Category") || !strings.Contains(listHuman, "Marketing / Sales") ||
			!strings.Contains(listHuman, "Origin") || !strings.Contains(listHuman, "workspace") ||
			!strings.Contains(listHuman, "Layer") || !strings.Contains(listHuman, "project_profile") ||
			!strings.Contains(listHuman, "Shadows") || !strings.Contains(listHuman, "profile, user") {
			t.Fatalf("agent list human output = %q, want category column", listHuman)
		}

		listToon, _, err := executeRootCommand(t, deps, "agent", "list", "-o", "toon")
		if err != nil {
			t.Fatalf("agent list toon error = %v", err)
		}
		const wantHeader = "agents[1]{name,provider,model,category,origin,workspace_id," +
			"disabled_skills,layer,shadows,definition_digest,tool_count,permissions}:"
		if !strings.Contains(listToon, wantHeader) ||
			!strings.Contains(listToon, "Marketing / Sales") {
			t.Fatalf("agent list toon output = %q, want category key", listToon)
		}

		human, _, err := executeRootCommand(t, deps, "agent", "info", agent.Name, "-o", "human")
		if err != nil {
			t.Fatalf("agent info human error = %v", err)
		}
		if !strings.Contains(human, "env:GH_TOKEN") || !strings.Contains(human, "Session Environment") {
			t.Fatalf("agent environment metadata missing: %s", human)
		}
		if !strings.Contains(human, "Agent") || !strings.Contains(human, agent.Name) ||
			!strings.Contains(human, "MCP Servers") ||
			!strings.Contains(human, "Marketing / Sales") ||
			!strings.Contains(human, "digest-coder") ||
			!strings.Contains(human, "legacy-review") ||
			!strings.Contains(human, "Reasoning Effort") ||
			!strings.Contains(human, "max") || !strings.Contains(human, "Speed") ||
			!strings.Contains(human, "fast") || !strings.Contains(human, "ACP Options") ||
			!strings.Contains(human, "context=1m, thinking=true") {
			t.Fatalf("agent info human output = %q, want agent details", human)
		}

		toon, _, err := executeRootCommand(t, deps, "agent", "info", agent.Name, "-o", "toon")
		if err != nil {
			t.Fatalf("agent info toon error = %v", err)
		}
		if !strings.Contains(toon, "session_env[1]{name,ref,source,present,required}") ||
			!strings.Contains(toon, "env:GH_TOKEN") {
			t.Fatalf("agent environment metadata missing: %s", toon)
		}
		if !strings.Contains(
			toon,
			"agent{name,provider,command,model,reasoning_effort,speed,acp_options,category,origin,workspace_id,disabled_skills,layer,shadows,definition_digest,tools,permissions,prompt}:",
		) ||
			!strings.Contains(toon, agent.Name) ||
			!strings.Contains(toon, "max") || !strings.Contains(toon, "fast") ||
			!strings.Contains(toon, "context=1m, thinking=true") || !strings.Contains(toon, "project_profile") ||
			!strings.Contains(toon, "profile, user") {
			t.Fatalf("agent info toon output = %q, want TOON agent object", toon)
		}
	})
}

func TestAgentInfoToonFallbackChain(t *testing.T) {
	t.Parallel()

	t.Run("Should emit every configured fallback route in order with all route fields", func(t *testing.T) {
		t.Parallel()

		agent := AgentRecord{
			Name:     "reviewer",
			Provider: "codex",
			FallbackChain: []contract.RoleFallbackStatus{
				{
					Provider: "claude", Model: "opus-4-8", ReasoningEffort: "high", Speed: contract.SpeedFast,
					ACPOptions:         []contract.AgentACPOptionSelection{{ID: "context", ValueID: "1m"}},
					Command:            "SEAT=2 claude --acp",
					CommandFingerprint: "sha256:seat-two",
				},
				{Provider: "codex", Model: "gpt-5.4"},
			},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
				return agent, nil
			},
		})

		toon, _, err := executeRootCommand(t, deps, "agent", "info", agent.Name, "-o", "toon")
		if err != nil {
			t.Fatalf("agent info toon error = %v", err)
		}
		header := "fallback_chain[2]{provider,model,reasoning_effort,speed,acp_options,command,command_fingerprint}:"
		first := strings.Index(toon, "claude,opus-4-8,high,fast,context=1m,SEAT=2 claude --acp,sha256:seat-two")
		second := strings.Index(toon, "codex,gpt-5.4,")
		if !strings.Contains(toon, header) || first < 0 || second < first {
			t.Fatalf("agent info toon output = %q, want ordered fallback_chain rows", toon)
		}
	})
}

func TestAgentCommandsPassWorkspaceQuery(t *testing.T) {
	t.Parallel()

	t.Run("Should pass workspace query to list and info calls", func(t *testing.T) {
		t.Parallel()

		const workspace = "ws-test"
		agent := AgentRecord{Name: "founder", Provider: "codex", Prompt: "lead"}
		deps := newWorkspaceTestDeps(t, &stubClient{
			listAgentsFn: func(_ context.Context, query AgentQuery) ([]AgentRecord, error) {
				if query.Workspace != workspace {
					t.Fatalf("ListAgents() workspace = %q, want %q", query.Workspace, workspace)
				}
				return []AgentRecord{agent}, nil
			},
			getAgentFn: func(_ context.Context, name string, query AgentQuery) (AgentRecord, error) {
				if name != agent.Name {
					t.Fatalf("GetAgent() name = %q, want %q", name, agent.Name)
				}
				if query.Workspace != workspace {
					t.Fatalf("GetAgent() workspace = %q, want %q", query.Workspace, workspace)
				}
				return agent, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "agent", "list", "--workspace", workspace, "-o", "json")
		if err != nil {
			t.Fatalf("agent list --workspace error = %v", err)
		}
		if !strings.Contains(stdout, agent.Name) {
			t.Fatalf("agent list --workspace output = %q, want %q", stdout, agent.Name)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"agent",
			"info",
			agent.Name,
			"--workspace",
			workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("agent info --workspace error = %v", err)
		}
		if !strings.Contains(stdout, agent.Name) {
			t.Fatalf("agent info --workspace output = %q, want %q", stdout, agent.Name)
		}
	})

	t.Run("Should carry the selected profile through agent reads", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listProfilesFn: func(context.Context) ([]contract.Profile, error) {
				return []contract.Profile{{ID: "profile-marketing", Name: "marketing", State: "active"}}, nil
			},
			listAgentsFn: func(ctx context.Context, _ AgentQuery) ([]AgentRecord, error) {
				if got, want := profileQueryValues(ctx, nil).Get(profileFlagName), "marketing"; got != want {
					t.Fatalf("ListAgents() profile query = %q, want %q", got, want)
				}
				return []AgentRecord{{Name: "marketer"}}, nil
			},
		}
		deps := newWorkspaceTestDeps(t, client)

		if _, _, err := executeRootCommand(
			t,
			deps,
			"agent",
			"list",
			"--profile",
			"marketing",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("agent list --profile error = %v", err)
		}
	})
}

func TestAgentCreateCommand(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "Should require one prompt source",
			args: []string{"agent", "create", "coder", "--provider", "claude"},
			want: "--prompt or --prompt-file is required",
		},
		{
			name: "Should reject conflicting prompt sources",
			args: []string{
				"agent", "create", "coder", "--provider", "claude",
				"--prompt", "Code.", "--prompt-file", "prompt.md",
			},
			want: "use either --prompt or --prompt-file",
		},
		{
			name: "Should reject non-boolean ACP toggle values",
			args: []string{
				"agent", "create", "coder", "--provider", "claude",
				"--prompt", "Code.", "--acp-toggle", "thinking=1",
			},
			want: "expected id=true|false",
		},
		{
			name: "Should reject a fallback route command without an executable",
			args: []string{
				"agent", "create", "reviewer", "--provider", "claude", "--prompt", "Review.",
				"--fallback-route", "provider=claude,model=opus-4-8,command=FOO=bar",
			},
			want: `agent "reviewer" fallback_chain[0].command: command is missing an executable`,
		},
		{
			name: "Should reject a fallback route whose command is not the last key",
			args: []string{
				"agent", "create", "reviewer", "--provider", "claude", "--prompt", "Review.",
				"--fallback-route", "command=claude --acp,provider=claude,model=opus-4-8",
			},
			want: `agent "reviewer" fallback_chain[0].provider is required`,
		},
		{
			name: "Should reject an unknown fallback route key",
			args: []string{
				"agent", "create", "reviewer", "--provider", "claude", "--prompt", "Review.",
				"--fallback-route", "provider=claude,model=opus-4-8,comand=claude",
			},
			want: `unknown key "comand"`,
		},
		{
			name: "Should reject a duplicated fallback route key",
			args: []string{
				"agent", "create", "reviewer", "--provider", "claude", "--prompt", "Review.",
				"--fallback-route", "provider=claude,provider=codex,model=opus-4-8",
			},
			want: `duplicate key "provider"`,
		},
		{
			name: "Should reject combining fallback routes with clearing the chain",
			args: []string{
				"agent", "create", "reviewer", "--provider", "claude", "--prompt", "Review.",
				"--fallback-route", "provider=claude,model=opus-4-8", "--clear-fallback-chain",
			},
			want: "use either --fallback-route or --clear-fallback-chain",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := false
			deps := newWorkspaceTestDeps(t, &stubClient{
				createAgentFn: func(context.Context, contract.CreateAgentRequest) (AgentRecord, error) {
					called = true
					return AgentRecord{}, nil
				},
			})
			_, _, err := executeRootCommand(t, deps, tc.args...)
			assertErrorContains(t, err, tc.want)
			if called {
				t.Fatal("CreateAgent() called after invalid input")
			}
		})
	}

	t.Run("Should create a workspace-local agent through the daemon", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		deps := newWorkspaceTestDeps(t, &stubClient{
			createAgentFn: func(_ context.Context, request contract.CreateAgentRequest) (AgentRecord, error) {
				if request.Scope != contract.AgentCreateScopeWorkspace || request.Workspace != workspaceRoot {
					t.Fatalf("CreateAgent() scope/workspace = %q/%q", request.Scope, request.Workspace)
				}
				if request.Agent.Name != "pricing_strategist" || request.Agent.Provider != "claude" ||
					request.Agent.ReasoningEffort != "max" || request.Agent.Speed != contract.SpeedFast ||
					len(request.Agent.ACPOptions) != 2 || request.Agent.ACPOptions[0].ID != "context" ||
					request.Agent.ACPOptions[0].ValueID != "1m" || request.Agent.ACPOptions[1].ID != "thinking" ||
					request.Agent.ACPOptions[1].BoolValue == nil || !*request.Agent.ACPOptions[1].BoolValue ||
					len(request.Agent.Tools) != 1 ||
					request.Agent.Skills == nil ||
					!slices.Equal(request.Agent.Skills.Disabled, []string{"legacy-review"}) {
					t.Fatalf("CreateAgent() request = %#v", request)
				}
				return AgentRecord{
					Name:             request.Agent.Name,
					Provider:         request.Agent.Provider,
					Model:            request.Agent.Model,
					ReasoningEffort:  request.Agent.ReasoningEffort,
					Prompt:           request.Agent.Prompt,
					Origin:           contract.AgentOriginWorkspace,
					WorkspaceID:      "ws-alpha",
					DefinitionDigest: "digest-create",
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"agent",
			"create",
			"pricing_strategist",
			"--workspace",
			workspaceRoot,
			"--provider",
			"claude",
			"--model",
			"claude-sonnet-5",
			"--reasoning-effort",
			"max",
			"--speed",
			"fast",
			"--acp-option",
			"context=1m",
			"--acp-toggle",
			"thinking=true",
			"--prompt",
			"You own Ad8 pricing strategy.",
			"--tool",
			"builtin__shell",
			"--disable-skill",
			"legacy-review",
			"--category",
			"Strategy",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("agent create error = %v", err)
		}
		var created AgentRecord
		if err := json.Unmarshal([]byte(stdout), &created); err != nil {
			t.Fatalf("json.Unmarshal(agent create) error = %v", err)
		}
		if created.Name != "pricing_strategist" || created.Provider != "claude" ||
			created.Model != "claude-sonnet-5" || created.ReasoningEffort != "max" ||
			created.Prompt != "You own Ad8 pricing strategy." {
			t.Fatalf("created agent = %#v, want pricing strategist metadata", created)
		}

		if _, err := os.Stat(filepath.Join(workspaceRoot, compozyconfig.DirName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("CLI filesystem authoring path exists or stat failed: %v", err)
		}
	})

	t.Run("Should allow onboarding as an ordinary agent name", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{createAgentFn: func(
			_ context.Context,
			request contract.CreateAgentRequest,
		) (AgentRecord, error) {
			return AgentRecord{Name: request.Agent.Name, Provider: request.Agent.Provider}, nil
		}})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"agent",
			"create",
			"onboarding",
			"--provider",
			"claude",
			"--prompt",
			"Reserved.",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("agent create onboarding error = %v", err)
		}
		var created AgentRecord
		if err := json.Unmarshal([]byte(stdout), &created); err != nil {
			t.Fatalf("json.Unmarshal(agent create onboarding) error = %v", err)
		}
		if created.Name != "onboarding" {
			t.Fatalf("created.Name = %q, want onboarding", created.Name)
		}
	})
}

func TestAgentUpdateCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should require an expected digest before reading the agent", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
				called = true
				return AgentRecord{}, nil
			},
		})
		_, _, err := executeRootCommand(t, deps, "agent", "update", "coder", "--model", "new")
		if err == nil || !strings.Contains(err.Error(), "--expected-digest is required") || called {
			t.Fatalf("agent update digest error/called = %v/%t", err, called)
		}
	})

	t.Run("Should merge field flags and prompt file into a digest-guarded update", func(t *testing.T) {
		t.Parallel()

		promptPath := filepath.Join(t.TempDir(), "prompt.md")
		if err := os.WriteFile(promptPath, []byte("Review every edge.\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(prompt) error = %v", err)
		}
		current := AgentRecord{
			Name: "coder", Provider: "fake", Model: "old", Prompt: "Code.", Speed: contract.SpeedNormal,
			ACPOptions: []contract.AgentACPOptionSelection{
				{ID: "context", ValueID: "512k"}, {ID: "thinking", BoolValue: new(true)},
			},
			Origin: contract.AgentOriginWorkspace, WorkspaceID: "ws-1", DefinitionDigest: "digest-1",
			Skills: &contract.CreateAgentSkillsConfig{Disabled: []string{"legacy"}},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(_ context.Context, name string, query AgentQuery) (AgentRecord, error) {
				if name != "coder" || query.Workspace != "ws-1" {
					t.Fatalf("GetAgent() = %q/%#v", name, query)
				}
				return current, nil
			},
			updateAgentFn: func(_ context.Context, name string, request contract.UpdateAgentRequest) (AgentRecord, error) {
				if name != "coder" || request.Workspace != "ws-1" || request.ExpectedDigest != "digest-1" {
					t.Fatalf("UpdateAgent() route/request = %q/%#v", name, request)
				}
				if request.Agent.Model != "new" || request.Agent.Prompt != "Review every edge." ||
					request.Agent.Speed != contract.SpeedFast || len(request.Agent.ACPOptions) != 2 ||
					request.Agent.ACPOptions[0].ID != "context" || request.Agent.ACPOptions[0].ValueID != "1m" ||
					request.Agent.ACPOptions[1].ID != "thinking" || request.Agent.ACPOptions[1].BoolValue == nil ||
					!*request.Agent.ACPOptions[1].BoolValue ||
					request.Agent.Skills == nil ||
					!slices.Equal(request.Agent.Skills.Disabled, []string{"security-audit"}) {
					t.Fatalf("UpdateAgent() agent = %#v", request.Agent)
				}
				updated := current
				updated.Model = request.Agent.Model
				updated.Prompt = request.Agent.Prompt
				updated.DefinitionDigest = "digest-2"
				return updated, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t, deps, "agent", "update", "coder", "--workspace", "ws-1",
			"--expected-digest", "digest-1", "--model", "new", "--prompt-file", promptPath, "-o", "json",
			"--disable-skill", "security-audit", "--speed", "fast", "--acp-option", "context=1m",
			"--acp-toggle", "thinking=true",
		)
		if err != nil || !strings.Contains(stdout, `"definition_digest": "digest-2"`) {
			t.Fatalf("agent update = %q, %v", stdout, err)
		}
	})

	t.Run("Should clear disabled skills when the flag is explicitly empty", func(t *testing.T) {
		t.Parallel()

		current := AgentRecord{
			Name:             "coder",
			Provider:         "fake",
			Prompt:           "Code.",
			DefinitionDigest: "digest-1",
			Skills:           &contract.CreateAgentSkillsConfig{Disabled: []string{"legacy"}},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
				return current, nil
			},
			updateAgentFn: func(
				_ context.Context,
				_ string,
				request contract.UpdateAgentRequest,
			) (AgentRecord, error) {
				if request.Agent.Skills == nil || len(request.Agent.Skills.Disabled) != 0 {
					t.Fatalf("UpdateAgent() skills = %#v, want explicit empty disabled list", request.Agent.Skills)
				}
				return current, nil
			},
		})

		if _, _, err := executeRootCommand(
			t, deps, "agent", "update", "coder", "--expected-digest", "digest-1",
			"--disable-skill", "",
		); err != nil {
			t.Fatalf("agent update clear disabled skills error = %v", err)
		}
	})

	t.Run("Should replace, keep, and clear the fallback chain in declaration order", func(t *testing.T) {
		t.Parallel()

		seatTwo := "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
		current := AgentRecord{
			Name: "reviewer", Provider: "claude", Model: "opus-4-8", Prompt: "Review.",
			DefinitionDigest: "digest-1",
			FallbackChain: []contract.RoleFallbackStatus{{
				Provider: "claude", Model: "opus-4-8", Command: "claude --acp", CommandFingerprint: "sha256:old",
			}},
		}
		for _, tc := range []struct {
			name string
			args []string
			want []contract.AgentFallbackRoutePayload
		}{
			{
				name: "replace",
				args: []string{
					"--fallback-route", "provider=claude,model=opus-4-8,command=" + seatTwo + ",x='a, b'",
					"--fallback-route", "provider=cursor,model=grok-4.6,reasoning_effort=high",
				},
				want: []contract.AgentFallbackRoutePayload{
					{Provider: "claude", Model: "opus-4-8", Command: seatTwo + ",x='a, b'"},
					{Provider: "cursor", Model: "grok-4.6", ReasoningEffort: "high"},
				},
			},
			{
				name: "keep",
				args: []string{"--model", "opus-4-8"},
				want: []contract.AgentFallbackRoutePayload{
					{Provider: "claude", Model: "opus-4-8", Command: "claude --acp"},
				},
			},
			{name: "clear", args: []string{"--clear-fallback-chain"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var got []contract.AgentFallbackRoutePayload
				deps := newWorkspaceTestDeps(t, &stubClient{
					getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
						return current, nil
					},
					updateAgentFn: func(
						_ context.Context,
						_ string,
						request contract.UpdateAgentRequest,
					) (AgentRecord, error) {
						got = request.Agent.FallbackChain
						return current, nil
					},
				})
				args := append([]string{"agent", "update", "reviewer", "--expected-digest", "digest-1"}, tc.args...)
				if _, _, err := executeRootCommand(t, deps, args...); err != nil {
					t.Fatalf("agent update fallback chain error = %v", err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("UpdateAgent() fallback_chain = %#v, want %#v", got, tc.want)
				}
			})
		}
	})

	t.Run("Should print the fallback chain in agent human output", func(t *testing.T) {
		t.Parallel()

		seatTwo := "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
		record := AgentRecord{
			Name: "reviewer", Provider: "claude", Prompt: "Review.", DefinitionDigest: "digest-1",
			FallbackChain: []contract.RoleFallbackStatus{{Provider: "claude", Model: "opus-4-8", Command: seatTwo}},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
				return record, nil
			},
			updateAgentFn: func(context.Context, string, contract.UpdateAgentRequest) (AgentRecord, error) {
				return record, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t, deps, "agent", "update", "reviewer", "--expected-digest", "digest-1",
			"--fallback-route", "provider=claude,model=opus-4-8,command="+seatTwo,
		)
		if err != nil {
			t.Fatalf("agent update human output error = %v", err)
		}
		for _, want := range []string{"Fallback Chain", "opus-4-8", seatTwo} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("agent human output missing %q:\n%s", want, stdout)
			}
		}
	})

	t.Run(
		"Should clear provider-specific settings when provider changes without explicit override flags",
		func(t *testing.T) {
			t.Parallel()

			current := AgentRecord{
				Name:             "coder",
				Provider:         "claude",
				Command:          "claude-runtime",
				Model:            "claude-sonnet-5",
				ReasoningEffort:  "high",
				Prompt:           "Code.",
				DefinitionDigest: "digest-1",
				ACPOptions: []contract.AgentACPOptionSelection{
					{ID: "context", ValueID: "1m"},
				},
			}
			deps := newWorkspaceTestDeps(t, &stubClient{
				getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
					return current, nil
				},
				updateAgentFn: func(
					_ context.Context,
					_ string,
					request contract.UpdateAgentRequest,
				) (AgentRecord, error) {
					if request.Agent.Provider != "mock" {
						t.Fatalf("UpdateAgent() provider = %q, want mock", request.Agent.Provider)
					}
					if request.Agent.Command != "" {
						t.Fatalf("UpdateAgent() command = %q, want empty", request.Agent.Command)
					}
					if request.Agent.Model != "" {
						t.Fatalf("UpdateAgent() model = %q, want empty", request.Agent.Model)
					}
					if request.Agent.ReasoningEffort != "" {
						t.Fatalf("UpdateAgent() reasoning_effort = %q, want empty", request.Agent.ReasoningEffort)
					}
					if len(request.Agent.ACPOptions) != 0 {
						t.Fatalf("UpdateAgent() acp_options = %#v, want empty", request.Agent.ACPOptions)
					}
					return current, nil
				},
			})
			if _, _, err := executeRootCommand(
				t, deps, "agent", "update", "coder", "--expected-digest", "digest-1",
				"--provider", "mock",
			); err != nil {
				t.Fatalf("agent update error = %v", err)
			}
		},
	)

	t.Run("Should preserve provider-specific settings for equivalent provider names", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name              string
			currentProvider   string
			requestedProvider string
		}{
			{
				name:              "Should preserve settings across a provider alias",
				currentProvider:   "kimi",
				requestedProvider: "moonshot",
			},
			{
				name:              "Should preserve settings across provider casing",
				currentProvider:   "CLAUDE",
				requestedProvider: "claude",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				current := AgentRecord{
					Name:             "coder",
					Provider:         test.currentProvider,
					Command:          "provider-runtime",
					Model:            "provider-model",
					ReasoningEffort:  "high",
					Prompt:           "Code.",
					DefinitionDigest: "digest-1",
					ACPOptions: []contract.AgentACPOptionSelection{
						{ID: "context", ValueID: "1m"},
					},
				}
				deps := newWorkspaceTestDeps(t, &stubClient{
					getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
						return current, nil
					},
					updateAgentFn: func(
						_ context.Context,
						_ string,
						request contract.UpdateAgentRequest,
					) (AgentRecord, error) {
						if request.Agent.Provider != test.requestedProvider {
							t.Fatalf(
								"UpdateAgent() provider = %q, want %q",
								request.Agent.Provider,
								test.requestedProvider,
							)
						}
						if request.Agent.Command != current.Command || request.Agent.Model != current.Model ||
							request.Agent.ReasoningEffort != current.ReasoningEffort ||
							!slices.Equal(request.Agent.ACPOptions, current.ACPOptions) {
							t.Fatalf("UpdateAgent() runtime settings = %#v, want %#v", request.Agent, current)
						}
						return current, nil
					},
				})
				if _, _, err := executeRootCommand(
					t, deps, "agent", "update", "coder", "--expected-digest", "digest-1",
					"--provider", test.requestedProvider,
				); err != nil {
					t.Fatalf("agent update error = %v", err)
				}
			})
		}
	})

	t.Run("Should name a missing prompt file", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.md")
		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
				return AgentRecord{Name: "coder", Provider: "fake", Prompt: "Code."}, nil
			},
		})
		_, _, err := executeRootCommand(
			t, deps, "agent", "update", "coder", "--expected-digest", "digest-1", "--prompt-file", missing,
		)
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Fatalf("agent update missing prompt error = %v, want path %q", err, missing)
		}
	})

	t.Run("Should render a digest conflict as structured invalid input", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
				return AgentRecord{Name: "coder", Provider: "fake", Prompt: "Code."}, nil
			},
			updateAgentFn: func(context.Context, string, contract.UpdateAgentRequest) (AgentRecord, error) {
				return AgentRecord{}, &daemonAPIError{
					statusCode: http.StatusConflict,
					payload:    contract.ErrorPayload{Error: "api: agent definition conflict"},
				}
			},
		})
		exitCode, _, stderr := executeRootCommandWithExit(
			t, deps, "agent", "update", "coder", "--expected-digest", "stale", "-o", "json",
		)
		const wantStderr = "{\"error\":\"api: agent definition conflict\"}\n"
		if exitCode != agentidentity.ExitIdentityInvalid || stderr != wantStderr {
			t.Fatalf("agent update conflict exit/stderr = %d/%q", exitCode, stderr)
		}
	})
}

func TestAgentDeleteCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should decline a TTY prompt without calling the daemon", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newWorkspaceTestDeps(t, &stubClient{
			deleteAgentFn: func(context.Context, string, string) (contract.DeleteAgentResponse, error) {
				called = true
				return contract.DeleteAgentResponse{}, nil
			},
		})
		deps.inputIsTerminal = func(io.Reader) bool { return true }
		cmd := newRootCommand(deps)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetIn(strings.NewReader("n\n"))
		cmd.SetArgs([]string{"agent", "delete", "coder"})
		err := cmd.ExecuteContext(t.Context())
		if err == nil || !strings.Contains(err.Error(), "declined") || called {
			t.Fatalf("agent delete decline error/called = %v/%t", err, called)
		}
		if !strings.Contains(stderr.String(), `Delete agent "coder"? [y/N]`) {
			t.Fatalf("agent delete prompt = %q", stderr.String())
		}
	})

	t.Run("Should require yes for structured output before calling the daemon", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newWorkspaceTestDeps(t, &stubClient{
			deleteAgentFn: func(context.Context, string, string) (contract.DeleteAgentResponse, error) {
				called = true
				return contract.DeleteAgentResponse{}, nil
			},
		})
		_, _, err := executeRootCommand(t, deps, "agent", "delete", "coder", "-o", "toon")
		if err == nil || !strings.Contains(err.Error(), "requires --yes") || called {
			t.Fatalf("agent delete structured consent error/called = %v/%t", err, called)
		}
	})

	t.Run("Should emit the unshadowed origin in structured output", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			deleteAgentFn: func(_ context.Context, name string, workspace string) (contract.DeleteAgentResponse, error) {
				if name != "coder" || workspace != "ws-1" {
					t.Fatalf("DeleteAgent() = %q/%q", name, workspace)
				}
				return contract.DeleteAgentResponse{
					Name: name, Origin: contract.AgentOriginWorkspace, UnshadowedOrigin: contract.AgentOriginGlobal,
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t, deps, "agent", "delete", "coder", "--workspace", "ws-1", "--yes", "-o", "json",
		)
		if err != nil || !strings.Contains(stdout, `"unshadowed_origin": "global"`) {
			t.Fatalf("agent delete output/error = %q/%v", stdout, err)
		}
	})
}

func TestAgentDuplicateCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should infer a workspace for workspace scope", func(t *testing.T) {
		t.Parallel()

		var captured contract.DuplicateAgentRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				if ref != "/workspace/project" {
					t.Fatalf("GetWorkspace() ref = %q, want cwd", ref)
				}
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-cwd"}}, nil
			},
			duplicateAgentFn: func(
				_ context.Context,
				_ string,
				request contract.DuplicateAgentRequest,
			) (AgentRecord, error) {
				captured = request
				return AgentRecord{Name: "reviewer"}, nil
			},
		})
		_, _, err := executeRootCommand(
			t, deps, "agent", "duplicate", "coder", "reviewer", "--scope", "workspace",
		)
		if err != nil {
			t.Fatalf("agent duplicate error = %v", err)
		}
		if captured.Workspace != "ws-cwd" {
			t.Fatalf("request.Workspace = %q, want ws-cwd", captured.Workspace)
		}
	})

	t.Run("Should map override flags to one server-side duplicate request", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			duplicateAgentFn: func(_ context.Context, source string, request contract.DuplicateAgentRequest) (AgentRecord, error) {
				if source != "coder" {
					t.Fatalf("DuplicateAgent() source = %q", source)
				}
				if request.Name != "reviewer" ||
					request.Scope != contract.AgentCreateScopeWorkspace ||
					request.Workspace != "ws-1" ||
					request.Overrides == nil ||
					request.Overrides.Model != "new" || request.Overrides.Speed != contract.SpeedFast ||
					len(request.Overrides.ACPOptions) != 2 || request.Overrides.ACPOptions[0].ID != "context" ||
					request.Overrides.ACPOptions[0].ValueID != "1m" || request.Overrides.ACPOptions[1].ID != "thinking" ||
					request.Overrides.ACPOptions[1].BoolValue == nil || !*request.Overrides.ACPOptions[1].BoolValue ||
					request.Overrides.Prompt != "Review." ||
					request.Overrides.Skills == nil ||
					!slices.Equal(request.Overrides.Skills.Disabled, []string{"legacy-review"}) {
					t.Fatalf("DuplicateAgent() = %q/%#v", source, request)
				}
				return AgentRecord{
					Name: "reviewer", Provider: "fake", Model: "new", Prompt: "Review.",
					Origin: contract.AgentOriginWorkspace, WorkspaceID: "ws-1", DefinitionDigest: "copy-digest",
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t, deps, "agent", "duplicate", "coder", "reviewer", "--scope", "workspace",
			"--workspace", "ws-1", "--model", "new", "--prompt", "Review.",
			"--disable-skill", "legacy-review", "--speed", "fast", "--acp-option", "context=1m",
			"--acp-toggle", "thinking=true", "-o", "json",
		)
		if err != nil || !strings.Contains(stdout, `"name": "reviewer"`) {
			t.Fatalf("agent duplicate output/error = %q/%v", stdout, err)
		}
	})

	t.Run("Should render an existing target conflict deterministically", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			duplicateAgentFn: func(context.Context, string, contract.DuplicateAgentRequest) (AgentRecord, error) {
				return AgentRecord{}, &daemonAPIError{
					statusCode: http.StatusConflict,
					payload:    contract.ErrorPayload{Error: "api: agent definition already exists"},
				}
			},
		})
		exitCode, _, stderr := executeRootCommandWithExit(
			t, deps, "agent", "duplicate", "coder", "reviewer", "-o", "json",
		)
		if exitCode != agentidentity.ExitIdentityInvalid || !strings.Contains(stderr, "already exists") {
			t.Fatalf("agent duplicate conflict exit/stderr = %d/%q", exitCode, stderr)
		}
	})
}

func TestAgentWorkspaceFlagRejectsEmptyExplicitValue(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an explicitly blank workspace flag", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(t, deps, "agent", "list", "--workspace", " ")
		if err == nil {
			t.Fatal("agent list --workspace blank error = nil, want error")
		}
		if !strings.Contains(err.Error(), "workspace flag cannot be empty") {
			t.Fatalf("agent list --workspace blank error = %v, want workspace flag message", err)
		}
	})
}

func TestAgentDefinitionDaemonErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a not-found API error and unavailable exit code", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
				return AgentRecord{}, &daemonAPIError{
					statusCode: http.StatusNotFound,
					payload:    contract.ErrorPayload{Error: `uds: agent "missing" is not available`},
				}
			},
		})
		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			deps,
			"agent",
			"info",
			"missing",
			"-o",
			"json",
		)
		if exitCode != agentidentity.ExitUnavailable {
			t.Fatalf("agent info missing exit code = %d, want %d", exitCode, agentidentity.ExitUnavailable)
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal([]byte(stderr), &payload); err != nil {
			t.Fatalf("json.Unmarshal(agent info error) = %v; stderr=%s", err, stderr)
		}
		if !strings.Contains(payload.Error, "missing") {
			t.Fatalf("agent info error payload = %#v, want missing agent", payload)
		}
	})
}

func TestAgentIdentityFlags(t *testing.T) {
	t.Parallel()
	t.Run("Should send explicit identity and emoji appearance flags", func(t *testing.T) {
		t.Parallel()
		cmd := newAgentCreateCommand(commandDeps{})
		if err := cmd.ParseFlags(
			[]string{"--display-name", "Triage Lead", "--kind", "stays_on", "--appearance-emoji", "🦊"},
		); err != nil {
			t.Fatal(err)
		}
		f := agentDefinitionFlags{displayName: "Triage Lead", kind: "stays_on", emoji: "🦊"}
		p := contract.CreateAgentPayload{Name: "triage"}
		applyAgentIdentityFlags(cmd, &p, f)
		if p.DisplayName == nil || *p.DisplayName != "Triage Lead" || p.Kind != "stays_on" ||
			p.Appearance["avatar"] != "emoji" {
			t.Fatalf("identity flags: %#v", p)
		}
	})
}

func TestAgentInstanceCommands(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"check", "occurrence"} {
		t.Run("Should read the completed occurrence through "+command, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				body := `{"state":"finished","result":"quiet","reason":"quiet"}`
				switch r.URL.Path {
				case "/api/agent-instances":
					body = `{"instances":[{"id":"ainst-test"}]}`
				case "/api/agent-instances/ainst-test/check":
					w.WriteHeader(202)
					body = `{"occurrence_id":"occ-test"}`
				case "/api/agent-instances/ainst-test/occurrences/occ-test":
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
					w.WriteHeader(404)
				}
				if _, err := io.WriteString(w, body); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			target, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"), httpClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
				return server.Client().Transport.RoundTrip(r)
			})}}
			args := []string{"agent", "instance", "occurrence", "occ-test", "-o", "json"}
			if command == "check" {
				args = []string{"agent", "instance", "check", "ainst-test", "--global", "-o", "json"}
			}
			stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client), args...)
			if err != nil {
				t.Fatal(err)
			}
			var result contract.AgentInstanceOccurrenceResponse
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if result.State != "finished" || result.Result != "quiet" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
	t.Run("Should create a global instance through the HTTP client", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/agent-instances" {
				t.Errorf("request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(404)
				return
			}
			var request contract.AgentInstanceCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if request.Agent != "triage" || request.Workspace != "@user" {
				t.Errorf("request: %#v", request)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			if _, err := io.WriteString(w, `{"instance":{"id":"ainst-test","agent_name":"triage","state":"active","main_session_id":"sess-test"}}`); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(server.Close)
		target, err := url.Parse(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"), httpClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
			return server.Client().Transport.RoundTrip(r)
		})}}
		deps := newWorkspaceTestDeps(t, client)
		stdout, _, err := executeRootCommand(t, deps, "agent", "instance", "create", "triage", "--global", "-o", "json")
		if err != nil {
			t.Fatal(err)
		}
		var response contract.AgentInstanceResponse
		if err := json.Unmarshal([]byte(stdout), &response); err != nil {
			t.Fatal(err)
		}
		if response.Instance.ID != "ainst-test" || response.Instance.MainSessionID != "sess-test" {
			t.Fatalf("response: %#v", response)
		}
	})
}

func TestAgentReadCommands(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		args                     []string
	}{
		{"Should read away reports", "GET", "/api/agent-instances/ainst-test/away", `{"last_seen_at":null,"quiet_check_ins":2,"reports":[]}`, []string{"agent", "instance", "away", "ainst-test", "--global", "-o", "json"}},
		{"Should mark an instance seen", "POST", "/api/agent-instances/ainst-test/away", `{"last_seen_at":"2026-11-01T12:00:00Z"}`, []string{"agent", "instance", "mark-seen", "ainst-test", "--global", "-o", "json"}},
		{"Should page check-ins", "GET", "/api/agent-instances/ainst-test/check-ins", `{"check_ins":[],"next_cursor":""}`, []string{"agent", "instance", "check-ins", "ainst-test", "--global", "--limit", "10", "-o", "json"}},
		{"Should read agent activity", "GET", "/api/agents/coder/activity", `{"reports_by_day":[],"cost_by_project":[],"busy_by_hour":[]}`, []string{"agent", "activity", "coder", "--days", "7", "-o", "json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
					return
				}
				if strings.HasSuffix(tc.path, "check-ins") && r.URL.Query().Get("limit") != "10" {
					t.Error("missing page limit")
				}
				if strings.HasSuffix(tc.path, "activity") && r.URL.Query().Get("days") != "7" {
					t.Error("missing time window")
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := io.WriteString(w, tc.body); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			target, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"), httpClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
				return server.Client().Transport.RoundTrip(r)
			})}}
			stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client), tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("output = %s", stdout)
			}
		})
	}
}
