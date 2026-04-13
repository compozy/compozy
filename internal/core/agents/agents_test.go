package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimeagent "github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

func TestDiscoverParsesValidAgentDefinition(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	agentDir := writeWorkspaceAgent(
		t,
		workspaceRoot,
		"council",
		strings.Join([]string{
			"---",
			"title: Council",
			"description: Multi-advisor decision agent",
			"ide: codex",
			"model: gpt-5.4",
			"reasoning_effort: high",
			"access_mode: full",
			"---",
			"",
			"You are the council agent.",
			"",
		}, "\n"),
		`{
  "mcpServers": {
    "filesystem": {
      "command": "./bin/server",
      "args": ["--token", "${API_TOKEN}"],
      "env": {
        "API_TOKEN": "${API_TOKEN}"
      }
    }
  }
}`,
	)

	registry := newTestRegistry(homeDir, map[string]string{
		"API_TOKEN": "secret-token",
	})

	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 0 {
		t.Fatalf("expected no discovery problems, got %#v", catalog.Problems)
	}
	if len(catalog.Agents) != 1 {
		t.Fatalf("expected one resolved agent, got %d", len(catalog.Agents))
	}

	resolved := catalog.Agents[0]
	if resolved.Name != "council" {
		t.Fatalf("unexpected name: %q", resolved.Name)
	}
	if resolved.Source.Scope != ScopeWorkspace {
		t.Fatalf("unexpected scope: %q", resolved.Source.Scope)
	}
	if resolved.Source.Dir != agentDir {
		t.Fatalf("unexpected source dir: %q", resolved.Source.Dir)
	}
	if resolved.Metadata.Title != "Council" || resolved.Metadata.Description != "Multi-advisor decision agent" {
		t.Fatalf("unexpected metadata: %#v", resolved.Metadata)
	}
	if resolved.Runtime.IDE != "codex" || resolved.Runtime.Model != "gpt-5.4" {
		t.Fatalf("unexpected runtime defaults: %#v", resolved.Runtime)
	}
	if resolved.Runtime.ReasoningEffort != "high" || resolved.Runtime.AccessMode != "full" {
		t.Fatalf("unexpected runtime overrides: %#v", resolved.Runtime)
	}
	if strings.TrimSpace(resolved.Prompt) != "You are the council agent." {
		t.Fatalf("unexpected prompt body: %q", resolved.Prompt)
	}
	if resolved.MCP == nil {
		t.Fatal("expected MCP config to be loaded")
	}
	if len(resolved.MCP.Servers) != 1 {
		t.Fatalf("expected one MCP server, got %d", len(resolved.MCP.Servers))
	}
	server := resolved.MCP.Servers[0]
	if server.Name != "filesystem" {
		t.Fatalf("unexpected MCP server name: %q", server.Name)
	}
	if want := filepath.Join(agentDir, "bin", "server"); server.Command != want {
		t.Fatalf("unexpected resolved command: got %q want %q", server.Command, want)
	}
	if len(server.Args) != 2 || server.Args[1] != "secret-token" {
		t.Fatalf("unexpected MCP args: %#v", server.Args)
	}
	if server.Env["API_TOKEN"] != "secret-token" {
		t.Fatalf("unexpected MCP env: %#v", server.Env)
	}

	lookedUp, err := catalog.Resolve("council")
	if err != nil {
		t.Fatalf("resolve from catalog: %v", err)
	}
	if lookedUp.Name != resolved.Name {
		t.Fatalf("unexpected resolved agent: %#v", lookedUp)
	}
	lookedUp, err = registry.Resolve(context.Background(), workspaceRoot, "council")
	if err != nil {
		t.Fatalf("resolve from registry: %v", err)
	}
	if lookedUp.Name != resolved.Name {
		t.Fatalf("unexpected registry resolve result: %#v", lookedUp)
	}

	if _, err := catalog.Resolve("missing"); !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestDiscoverRejectsReservedAndInvalidAgentNames(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(t, workspaceRoot, "compozy", validAgentMarkdown("Reserved", "Reserved prompt"), "")
	writeWorkspaceAgent(t, workspaceRoot, "Bad_Name", validAgentMarkdown("Bad", "Bad prompt"), "")

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Agents) != 0 {
		t.Fatalf("expected no resolved agents, got %d", len(catalog.Agents))
	}
	if len(catalog.Problems) != 2 {
		t.Fatalf("expected two discovery problems, got %#v", catalog.Problems)
	}

	problems := problemsByName(catalog.Problems)
	if !errors.Is(problems["compozy"], ErrReservedAgentName) {
		t.Fatalf("expected wrapped reserved-name problem to unwrap, got %v", problems["compozy"])
	}
	if !errors.Is(problems["compozy"].Err, ErrReservedAgentName) {
		t.Fatalf("expected reserved agent name error, got %v", problems["compozy"].Err)
	}
	if !errors.Is(problems["Bad_Name"].Err, ErrInvalidAgentName) {
		t.Fatalf("expected invalid slug error, got %v", problems["Bad_Name"].Err)
	}
}

func TestDiscoverRejectsUnsupportedDeferredMetadataFields(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"reviewer",
		strings.Join([]string{
			"---",
			"title: Reviewer",
			"skills: reviewer-pack",
			"---",
			"",
			"Review the code carefully.",
			"",
		}, "\n"),
		"",
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 1 {
		t.Fatalf("expected one discovery problem, got %#v", catalog.Problems)
	}
	if !errors.Is(catalog.Problems[0].Err, ErrUnsupportedMetadataField) {
		t.Fatalf("expected unsupported metadata field error, got %v", catalog.Problems[0].Err)
	}
	if !strings.Contains(catalog.Problems[0].Err.Error(), `"skills"`) {
		t.Fatalf("expected field name in error, got %v", catalog.Problems[0].Err)
	}
}

func TestDiscoverAcceptsOverlayIDERuntimeDefaults(t *testing.T) {
	restore, err := runtimeagent.ActivateOverlay([]runtimeagent.OverlayEntry{{
		Name:         "ext-adapter",
		Command:      "mock-acp --serve",
		DisplayName:  "Mock ACP",
		DefaultModel: "ext-model",
	}})
	if err != nil {
		t.Fatalf("activate ACP overlay: %v", err)
	}
	defer restore()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"reviewer",
		strings.Join([]string{
			"---",
			"title: Reviewer",
			"description: Reviews code",
			"ide: ext-adapter",
			"---",
			"",
			"Review the code carefully.",
			"",
		}, "\n"),
		"",
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 0 {
		t.Fatalf("expected no discovery problems, got %#v", catalog.Problems)
	}
	if len(catalog.Agents) != 1 {
		t.Fatalf("expected one resolved agent, got %d", len(catalog.Agents))
	}
	if got := catalog.Agents[0].Runtime.IDE; got != "ext-adapter" {
		t.Fatalf("resolved.Runtime.IDE = %q, want %q", got, "ext-adapter")
	}
	if got := catalog.Agents[0].Runtime.Model; got != "ext-model" {
		t.Fatalf("resolved.Runtime.Model = %q, want %q", got, "ext-model")
	}
}

func TestDiscoverFailsClosedWhenMCPPlaceholderIsMissing(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"planner",
		validAgentMarkdown("Planner", "Plan the work."),
		`{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}`,
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 1 {
		t.Fatalf("expected one discovery problem, got %#v", catalog.Problems)
	}
	if !errors.Is(catalog.Problems[0].Err, ErrMissingEnvironmentVariable) {
		t.Fatalf("expected missing environment variable error, got %v", catalog.Problems[0].Err)
	}
	if !strings.Contains(catalog.Problems[0].Err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("expected missing variable name in error, got %v", catalog.Problems[0].Err)
	}
}

func TestDiscoverRejectsReservedMCPServerName(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"planner",
		validAgentMarkdown("Planner", "Plan the work."),
		`{
  "mcpServers": {
    "compozy": {
      "command": "npx"
    }
  }
}`,
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 1 {
		t.Fatalf("expected one discovery problem, got %#v", catalog.Problems)
	}
	if !errors.Is(catalog.Problems[0].Err, ErrReservedMCPServerName) {
		t.Fatalf("expected reserved MCP server error, got %v", catalog.Problems[0].Err)
	}
}

func TestDiscoverReturnsBothScopesWithoutCollisions(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeGlobalAgent(t, homeDir, "global-reviewer", validAgentMarkdown("Global Reviewer", "Review globally."), "")
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"workspace-reviewer",
		validAgentMarkdown("Workspace Reviewer", "Review locally."),
		"",
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 0 {
		t.Fatalf("expected no discovery problems, got %#v", catalog.Problems)
	}
	if len(catalog.Agents) != 2 {
		t.Fatalf("expected two resolved agents, got %d", len(catalog.Agents))
	}
	if catalog.Agents[0].Name != "global-reviewer" || catalog.Agents[0].Source.Scope != ScopeGlobal {
		t.Fatalf("unexpected first agent: %#v", catalog.Agents[0])
	}
	if catalog.Agents[1].Name != "workspace-reviewer" || catalog.Agents[1].Source.Scope != ScopeWorkspace {
		t.Fatalf("unexpected second agent: %#v", catalog.Agents[1])
	}
}

func TestDiscoverKeepsWorkspaceAgentsWhenGlobalHomeLookupFails(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"workspace-reviewer",
		validAgentMarkdown("Workspace Reviewer", "Review locally."),
		"",
	)

	registry := New(
		WithHomeDir(func() (string, error) {
			return "", errors.New("home lookup failed")
		}),
	)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 0 {
		t.Fatalf("expected no discovery problems, got %#v", catalog.Problems)
	}
	if len(catalog.Agents) != 1 {
		t.Fatalf("expected one workspace agent, got %#v", catalog.Agents)
	}
	if catalog.Agents[0].Name != "workspace-reviewer" || catalog.Agents[0].Source.Scope != ScopeWorkspace {
		t.Fatalf("unexpected workspace agent result: %#v", catalog.Agents[0])
	}
}

func TestDiscoverUsesWorkspaceDirectoryAsWholeOverride(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeGlobalAgent(
		t,
		homeDir,
		"council",
		validAgentMarkdown("Global Council", "Global council prompt."),
		`{
  "mcpServers": {
    "github": {
      "command": "npx"
    }
  }
}`,
	)
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"council",
		validAgentMarkdown("Workspace Council", "Workspace council prompt."),
		"",
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Problems) != 0 {
		t.Fatalf("expected no discovery problems, got %#v", catalog.Problems)
	}
	if len(catalog.Agents) != 1 {
		t.Fatalf("expected one resolved agent, got %d", len(catalog.Agents))
	}
	resolved := catalog.Agents[0]
	if resolved.Source.Scope != ScopeWorkspace {
		t.Fatalf("expected workspace override, got %#v", resolved.Source)
	}
	if resolved.Metadata.Title != "Workspace Council" {
		t.Fatalf("expected workspace title, got %#v", resolved.Metadata)
	}
	if resolved.MCP != nil {
		t.Fatalf("expected workspace directory to win as a whole without merged mcp.json, got %#v", resolved.MCP)
	}
}

func TestDiscoverInvalidWorkspaceOverrideDoesNotFallBackToGlobal(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeGlobalAgent(t, homeDir, "planner", validAgentMarkdown("Global Planner", "Global planner prompt."), "")
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"planner",
		strings.Join([]string{
			"---",
			"title: [broken",
			"---",
			"",
			"Broken planner prompt.",
			"",
		}, "\n"),
		"",
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Agents) != 0 {
		t.Fatalf("expected invalid workspace override to shadow the global agent, got %#v", catalog.Agents)
	}
	if len(catalog.Problems) != 1 {
		t.Fatalf("expected one discovery problem, got %#v", catalog.Problems)
	}
	if !errors.Is(catalog.Problems[0].Err, ErrMalformedFrontmatter) {
		t.Fatalf("expected malformed frontmatter error, got %v", catalog.Problems[0].Err)
	}
	if _, err := catalog.Resolve("planner"); !errors.Is(err, ErrMalformedFrontmatter) {
		t.Fatalf("expected resolve to expose malformed override, got %v", err)
	} else if !strings.Contains(err.Error(), "planner (workspace)") {
		t.Fatalf("expected resolve error to preserve problem context, got %v", err)
	}
}

func TestDiscoverSurfacesMalformedAgentWithoutCorruptingValidOnes(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	writeGlobalAgent(t, homeDir, "reviewer", validAgentMarkdown("Reviewer", "Review the code."), "")
	writeWorkspaceAgent(
		t,
		workspaceRoot,
		"planner",
		strings.Join([]string{
			"---",
			"title: [oops",
			"---",
			"",
			"Broken planner prompt.",
			"",
		}, "\n"),
		"",
	)

	registry := newTestRegistry(homeDir, nil)
	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(catalog.Agents) != 1 {
		t.Fatalf("expected one valid agent, got %#v", catalog.Agents)
	}
	if catalog.Agents[0].Name != "reviewer" || catalog.Agents[0].Source.Scope != ScopeGlobal {
		t.Fatalf("unexpected valid agent: %#v", catalog.Agents[0])
	}
	if len(catalog.Problems) != 1 {
		t.Fatalf("expected one malformed agent problem, got %#v", catalog.Problems)
	}
	if catalog.Problems[0].Name != "planner" {
		t.Fatalf("unexpected problem agent: %#v", catalog.Problems[0])
	}
	if !errors.Is(catalog.Problems[0].Err, ErrMalformedFrontmatter) {
		t.Fatalf("expected malformed frontmatter error, got %v", catalog.Problems[0].Err)
	}
}

func newTestRegistry(homeDir string, env map[string]string) *Registry {
	return New(
		WithHomeDir(func() (string, error) {
			return homeDir, nil
		}),
		WithLookupEnv(func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		}),
	)
}

func writeWorkspaceAgent(t *testing.T, workspaceRoot, name, agentContent, mcpContent string) string {
	t.Helper()
	return writeAgentFiles(
		t,
		filepath.Join(workspaceRoot, model.WorkflowRootDirName, agentDirName),
		name,
		agentContent,
		mcpContent,
	)
}

func writeGlobalAgent(t *testing.T, homeDir, name, agentContent, mcpContent string) string {
	t.Helper()
	return writeAgentFiles(
		t,
		filepath.Join(homeDir, model.WorkflowRootDirName, agentDirName),
		name,
		agentContent,
		mcpContent,
	)
}

func writeAgentFiles(t *testing.T, root, name, agentContent, mcpContent string) string {
	t.Helper()

	agentDir := filepath.Join(root, name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	if agentContent != "" {
		if err := os.WriteFile(filepath.Join(agentDir, agentFileName), []byte(agentContent), 0o644); err != nil {
			t.Fatalf("write AGENT.md: %v", err)
		}
	}
	if mcpContent != "" {
		if err := os.WriteFile(filepath.Join(agentDir, agentMCPConfig), []byte(mcpContent), 0o644); err != nil {
			t.Fatalf("write mcp.json: %v", err)
		}
	}
	return agentDir
}

func validAgentMarkdown(title, prompt string) string {
	return strings.Join([]string{
		"---",
		"title: " + title,
		"description: Test agent",
		"ide: codex",
		"reasoning_effort: medium",
		"access_mode: default",
		"---",
		"",
		prompt,
		"",
	}, "\n")
}

func problemsByName(problems []Problem) map[string]Problem {
	items := make(map[string]Problem, len(problems))
	for _, problem := range problems {
		items[problem.Name] = problem
	}
	return items
}
