package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestResolveExecutionContextAppliesRuntimePrecedence(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"council",
		strings.Join([]string{
			"---",
			"title: Council",
			"description: Multi-advisor decision agent",
			"ide: claude",
			"model: agent-model",
			"reasoning_effort: high",
			"access_mode: default",
			"---",
			"",
			"You are the council agent.",
			"",
		}, "\n"),
		"",
	)

	cfg := &model.RuntimeConfig{
		WorkspaceRoot: workspaceRoot,
		AgentName:     "council",
		Model:         "cli-model",
		ExplicitRuntime: model.ExplicitRuntimeFlags{
			Model: true,
		},
	}

	execution, err := resolveExecutionContext(context.Background(), newTestRegistry(homeDir, nil), cfg)
	if err != nil {
		t.Fatalf("resolve execution context: %v", err)
	}
	if execution == nil {
		t.Fatal("expected execution context")
	}
	if execution.Agent.Name != "council" {
		t.Fatalf("unexpected selected agent: %q", execution.Agent.Name)
	}
	if cfg.IDE != model.IDEClaude {
		t.Fatalf("expected agent ide to apply, got %q", cfg.IDE)
	}
	if cfg.Model != "cli-model" {
		t.Fatalf("expected explicit cli model to win, got %q", cfg.Model)
	}
	if cfg.ReasoningEffort != "high" {
		t.Fatalf("expected agent reasoning effort to apply, got %q", cfg.ReasoningEffort)
	}
	if cfg.AccessMode != model.AccessModeDefault {
		t.Fatalf("expected agent access mode to apply, got %q", cfg.AccessMode)
	}
}

func TestResolveExecutionContextAppliesAgentModelWhenModelFlagIsUnset(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"planner",
		strings.Join([]string{
			"---",
			"title: Planner",
			"description: Plans the work",
			"ide: codex",
			"model: agent-model",
			"---",
			"",
			"Plan the work.",
			"",
		}, "\n"),
		"",
	)

	cfg := &model.RuntimeConfig{
		WorkspaceRoot: workspaceRoot,
		AgentName:     "planner",
		Model:         "",
	}

	if _, err := resolveExecutionContext(context.Background(), newTestRegistry(homeDir, nil), cfg); err != nil {
		t.Fatalf("resolve execution context: %v", err)
	}
	if cfg.Model != "agent-model" {
		t.Fatalf("expected agent model to apply when flag is unset, got %q", cfg.Model)
	}
}

func TestExecutionContextSystemPromptUsesCanonicalOrder(t *testing.T) {
	t.Parallel()

	execution := &ExecutionContext{
		Agent: ResolvedAgent{
			Name: "council",
			Metadata: Metadata{
				Title:       "Council",
				Description: "Coordinates reviewers",
			},
			Prompt: "You are the council agent.",
			Source: Source{Scope: ScopeWorkspace},
		},
		Catalog: Catalog{
			Agents: []ResolvedAgent{
				{
					Name: "council",
					Metadata: Metadata{
						Title:       "Council",
						Description: "Coordinates reviewers",
					},
					Prompt: "You are the council agent.",
					Source: Source{Scope: ScopeWorkspace},
				},
				{
					Name: "reviewer",
					Metadata: Metadata{
						Title:       "Reviewer",
						Description: "Reviews code",
					},
					Runtime: RuntimeDefaults{
						IDE:             model.IDECodex,
						Model:           "ignored-model",
						ReasoningEffort: "ignored-reasoning",
						AccessMode:      model.AccessModeFull,
					},
					Prompt: "Review the code carefully.",
					Source: Source{Scope: ScopeGlobal},
				},
			},
		},
	}

	got := execution.SystemPrompt("built-in framing")

	framingIndex := strings.Index(got, "built-in framing")
	metadataIndex := strings.Index(got, "<agent_metadata>")
	discoveryIndex := strings.Index(got, "<available_agents>")
	bodyIndex := strings.Index(got, "You are the council agent.")
	if framingIndex < 0 || metadataIndex < 0 || discoveryIndex < 0 || bodyIndex < 0 {
		t.Fatalf("expected all prompt sections to be present, got:\n%s", got)
	}
	if framingIndex >= metadataIndex || metadataIndex >= discoveryIndex || discoveryIndex >= bodyIndex {
		t.Fatalf("expected canonical prompt order, got:\n%s", got)
	}
	if strings.Count(got, "<agent_metadata>") != 1 {
		t.Fatalf("expected one metadata block, got:\n%s", got)
	}
	if strings.Count(got, "You are the council agent.") != 1 {
		t.Fatalf("expected one agent body, got:\n%s", got)
	}
}

func TestExecutionContextSystemPromptKeepsDiscoveryCatalogCompact(t *testing.T) {
	t.Parallel()

	execution := &ExecutionContext{
		Agent: ResolvedAgent{
			Name: "council",
			Metadata: Metadata{
				Title:       "Council",
				Description: "Coordinates reviewers",
			},
			Prompt: "You are the council agent.",
			Source: Source{Scope: ScopeWorkspace},
		},
		Catalog: Catalog{
			Agents: []ResolvedAgent{
				{
					Name: "council",
					Metadata: Metadata{
						Title:       "Council",
						Description: "Coordinates reviewers",
					},
					Prompt: "You are the council agent.",
					Source: Source{Scope: ScopeWorkspace},
				},
				{
					Name: "planner",
					Metadata: Metadata{
						Title:       "Planner",
						Description: "Plans the work",
					},
					Runtime: RuntimeDefaults{
						IDE:             model.IDEClaude,
						Model:           "should-not-appear",
						ReasoningEffort: "high",
						AccessMode:      model.AccessModeDefault,
					},
					Prompt: "Plan the work.",
					Source: Source{Scope: ScopeGlobal},
				},
			},
		},
	}

	got := execution.SystemPrompt("")
	if !strings.Contains(got, "- planner: Plans the work (global)") {
		t.Fatalf("expected compact discovery entry, got:\n%s", got)
	}
	for _, forbidden := range []string{
		"title: Planner",
		"should-not-appear",
		"reasoning_effort",
		"access_mode",
		"- council:",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("expected compact discovery catalog to omit %q, got:\n%s", forbidden, got)
		}
	}
}

func TestExecutionContextSystemPromptFallsBackToBasePromptWhenNoAgentSelected(t *testing.T) {
	t.Parallel()

	var execution *ExecutionContext
	got := execution.SystemPrompt("existing non-agent behavior")
	if got != "existing non-agent behavior" {
		t.Fatalf("expected base system prompt to be preserved, got %q", got)
	}
}

func TestResolveExecutionContextPreservesExistingRuntimeWhenAgentOmitsField(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"planner",
		strings.Join([]string{
			"---",
			"title: Planner",
			"description: Plans the work",
			"model: agent-model",
			"---",
			"",
			"Plan the work.",
			"",
		}, "\n"),
		"",
	)

	cfg := &model.RuntimeConfig{
		WorkspaceRoot:   workspaceRoot,
		AgentName:       "planner",
		ReasoningEffort: "xhigh",
	}

	if _, err := resolveExecutionContext(context.Background(), newTestRegistry(homeDir, nil), cfg); err != nil {
		t.Fatalf("resolve execution context: %v", err)
	}
	if cfg.ReasoningEffort != "xhigh" {
		t.Fatalf("expected existing reasoning effort to remain when agent omits it, got %q", cfg.ReasoningEffort)
	}
}
