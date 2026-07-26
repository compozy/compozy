package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
)

func TestAgentListAndInfoCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list and inspect global agents", func(t *testing.T) {
		t.Parallel()

		agent := AgentRecord{
			Name:             "coder",
			Provider:         "fake",
			Command:          "codex",
			Model:            "gpt-5.4",
			ReasoningEffort:  "max",
			Tools:            []string{"shell", "git"},
			Permissions:      "standard",
			CategoryPath:     []string{"Marketing", "Sales"},
			Origin:           contract.AgentOriginWorkspace,
			WorkspaceID:      "ws-1",
			Skills:           &contract.CreateAgentSkillsConfig{Disabled: []string{"legacy-review"}},
			DefinitionDigest: "digest-coder",
			Prompt:           "You are coder.",
			MCPServers: []AgentMCPServer{{
				Name:    "github",
				Command: "agh-github",
				Args:    []string{"serve"},
			}},
		}

		deps := newTestDeps(t, &stubClient{
			listAgentsFn: func(_ context.Context, query AgentQuery) ([]AgentRecord, error) {
				if query.Workspace != "" {
					t.Fatalf("ListAgents() workspace = %q, want empty", query.Workspace)
				}
				return []AgentRecord{agent}, nil
			},
			getAgentFn: func(_ context.Context, name string, query AgentQuery) (AgentRecord, error) {
				if name != agent.Name {
					t.Fatalf("GetAgent() name = %q, want %q", name, agent.Name)
				}
				if query.Workspace != "" {
					t.Fatalf("GetAgent() workspace = %q, want empty", query.Workspace)
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
		if got, want := strings.Join(listed[0].CategoryPath, ","), "Marketing,Sales"; got != want {
			t.Fatalf("listed agent category_path = %#v, want %q", listed[0].CategoryPath, want)
		}

		listHuman, _, err := executeRootCommand(t, deps, "agent", "list", "-o", "human")
		if err != nil {
			t.Fatalf("agent list human error = %v", err)
		}
		if !strings.Contains(listHuman, "Category") || !strings.Contains(listHuman, "Marketing / Sales") ||
			!strings.Contains(listHuman, "Origin") || !strings.Contains(listHuman, "workspace") {
			t.Fatalf("agent list human output = %q, want category column", listHuman)
		}

		listToon, _, err := executeRootCommand(t, deps, "agent", "list", "-o", "toon")
		if err != nil {
			t.Fatalf("agent list toon error = %v", err)
		}
		const wantHeader = "agents[1]{name,provider,model,category,origin,workspace_id," +
			"disabled_skills,definition_digest,tool_count,permissions}:"
		if !strings.Contains(listToon, wantHeader) ||
			!strings.Contains(listToon, "Marketing / Sales") {
			t.Fatalf("agent list toon output = %q, want category key", listToon)
		}

		human, _, err := executeRootCommand(t, deps, "agent", "info", agent.Name, "-o", "human")
		if err != nil {
			t.Fatalf("agent info human error = %v", err)
		}
		if !strings.Contains(human, "Agent") || !strings.Contains(human, agent.Name) ||
			!strings.Contains(human, "MCP Servers") ||
			!strings.Contains(human, "Marketing / Sales") ||
			!strings.Contains(human, "digest-coder") ||
			!strings.Contains(human, "legacy-review") ||
			!strings.Contains(human, "Reasoning Effort") ||
			!strings.Contains(human, "max") {
			t.Fatalf("agent info human output = %q, want agent details", human)
		}

		toon, _, err := executeRootCommand(t, deps, "agent", "info", agent.Name, "-o", "toon")
		if err != nil {
			t.Fatalf("agent info toon error = %v", err)
		}
		if !strings.Contains(
			toon,
			"agent{name,provider,command,model,reasoning_effort,category,origin,workspace_id,disabled_skills,definition_digest,tools,permissions,prompt}:",
		) ||
			!strings.Contains(toon, agent.Name) ||
			!strings.Contains(toon, "max") {
			t.Fatalf("agent info toon output = %q, want TOON agent object", toon)
		}
	})
}

func TestAgentCommandsPassWorkspaceQuery(t *testing.T) {
	t.Parallel()

	t.Run("Should pass workspace query to list and info calls", func(t *testing.T) {
		t.Parallel()

		const workspace = "ws-test"
		agent := AgentRecord{Name: "founder", Provider: "codex", Prompt: "lead"}
		deps := newTestDeps(t, &stubClient{
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := false
			deps := newTestDeps(t, &stubClient{
				createAgentFn: func(context.Context, contract.CreateAgentRequest) (AgentRecord, error) {
					called = true
					return AgentRecord{}, nil
				},
			})
			_, _, err := executeRootCommand(t, deps, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) || called {
				t.Fatalf("agent create prompt error/called = %v/%t, want %q", err, called, tc.want)
			}
		})
	}

	t.Run("Should create a workspace-local agent through the daemon", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		deps := newTestDeps(t, &stubClient{
			createAgentFn: func(_ context.Context, request contract.CreateAgentRequest) (AgentRecord, error) {
				if request.Scope != contract.AgentCreateScopeWorkspace || request.Workspace != workspaceRoot {
					t.Fatalf("CreateAgent() scope/workspace = %q/%q", request.Scope, request.Workspace)
				}
				if request.Agent.Name != "pricing_strategist" || request.Agent.Provider != "claude" ||
					request.Agent.ReasoningEffort != "max" || len(request.Agent.Tools) != 1 ||
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

		if _, err := os.Stat(filepath.Join(workspaceRoot, aghconfig.DirName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("CLI filesystem authoring path exists or stat failed: %v", err)
		}
	})

	t.Run("Should allow onboarding as an ordinary agent name", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{createAgentFn: func(
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
		deps := newTestDeps(t, &stubClient{
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
			Name: "coder", Provider: "fake", Model: "old", Prompt: "Code.",
			Origin: contract.AgentOriginWorkspace, WorkspaceID: "ws-1", DefinitionDigest: "digest-1",
			Skills: &contract.CreateAgentSkillsConfig{Disabled: []string{"legacy"}},
		}
		deps := newTestDeps(t, &stubClient{
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
			"--disable-skill", "security-audit",
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
		deps := newTestDeps(t, &stubClient{
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

		_, _, err := executeRootCommand(
			t,
			deps,
			"agent",
			"update",
			"coder",
			"--expected-digest",
			"digest-1",
			"--disable-skill",
			"",
		)
		if err != nil {
			t.Fatalf("agent update clear disabled skills error = %v", err)
		}
	})

	t.Run("Should name a missing prompt file", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.md")
		deps := newTestDeps(t, &stubClient{
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

		deps := newTestDeps(t, &stubClient{
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
		deps := newTestDeps(t, &stubClient{
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
		deps := newTestDeps(t, &stubClient{
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

		deps := newTestDeps(t, &stubClient{
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

	t.Run("Should require a workspace for workspace scope before calling the daemon", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newTestDeps(t, &stubClient{
			duplicateAgentFn: func(context.Context, string, contract.DuplicateAgentRequest) (AgentRecord, error) {
				called = true
				return AgentRecord{}, nil
			},
		})
		_, _, err := executeRootCommand(
			t, deps, "agent", "duplicate", "coder", "reviewer", "--scope", "workspace",
		)
		if err == nil || !strings.Contains(err.Error(), "--workspace is required") || called {
			t.Fatalf("agent duplicate workspace error/called = %v/%t", err, called)
		}
	})

	t.Run("Should map override flags to one server-side duplicate request", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			duplicateAgentFn: func(_ context.Context, source string, request contract.DuplicateAgentRequest) (AgentRecord, error) {
				if source != "coder" {
					t.Fatalf("DuplicateAgent() source = %q", source)
				}
				if request.Name != "reviewer" ||
					request.Scope != contract.AgentCreateScopeWorkspace ||
					request.Workspace != "ws-1" ||
					request.Overrides == nil ||
					request.Overrides.Model != "new" ||
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
			"--disable-skill", "legacy-review", "-o", "json",
		)
		if err != nil || !strings.Contains(stdout, `"name": "reviewer"`) {
			t.Fatalf("agent duplicate output/error = %q/%v", stdout, err)
		}
	})

	t.Run("Should render an existing target conflict deterministically", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
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

		deps := newTestDeps(t, &stubClient{})
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

		deps := newTestDeps(t, &stubClient{
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
