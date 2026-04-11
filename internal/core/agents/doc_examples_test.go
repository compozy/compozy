package agents

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestDocumentedExampleFixturesParseSuccessfully(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	copyDocumentedAgentFixture(t, workspaceRoot, "reviewer")
	copyDocumentedAgentFixture(t, workspaceRoot, "repo-copilot")

	registry := newTestRegistry(homeDir, map[string]string{
		"GITHUB_TOKEN": "ghu_example_token",
		"PROJECT_ROOT": workspaceRoot,
	})

	catalog, err := registry.Discover(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("discover documented fixtures: %v", err)
	}
	if len(catalog.Problems) != 0 {
		t.Fatalf("expected documented fixtures to validate cleanly, got %#v", catalog.Problems)
	}
	if got, want := len(catalog.Agents), 2; got != want {
		t.Fatalf("expected %d documented agents, got %d", want, got)
	}

	gotNames := []string{catalog.Agents[0].Name, catalog.Agents[1].Name}
	wantNames := []string{"repo-copilot", "reviewer"}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("unexpected documented agent names: got %v want %v", gotNames, wantNames)
	}
}

func TestDocumentedRepoCopilotMCPConfigValidatesWithRealRegistryRules(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	copyDocumentedAgentFixture(t, workspaceRoot, "repo-copilot")

	registry := newTestRegistry(homeDir, map[string]string{
		"GITHUB_TOKEN": "ghu_example_token",
		"PROJECT_ROOT": workspaceRoot,
	})

	resolved, err := registry.Resolve(context.Background(), workspaceRoot, "repo-copilot")
	if err != nil {
		t.Fatalf("resolve documented repo-copilot fixture: %v", err)
	}

	if resolved.Metadata.Title != "Repo Copilot" {
		t.Fatalf("unexpected documented title: %#v", resolved.Metadata)
	}
	if resolved.Runtime != (RuntimeDefaults{
		IDE:             model.IDECodex,
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		AccessMode:      model.AccessModeFull,
	}) {
		t.Fatalf("unexpected documented runtime defaults: %#v", resolved.Runtime)
	}
	if resolved.MCP == nil {
		t.Fatal("expected documented repo-copilot MCP config to be loaded")
	}
	if got, want := len(resolved.MCP.Servers), 2; got != want {
		t.Fatalf("expected %d documented MCP servers, got %d", want, got)
	}

	filesystem := resolved.MCP.Servers[0]
	if filesystem.Name != "filesystem" {
		t.Fatalf("unexpected first documented MCP server: %#v", filesystem)
	}
	if got, want := filesystem.Args[len(filesystem.Args)-1], workspaceRoot; got != want {
		t.Fatalf("unexpected PROJECT_ROOT placeholder expansion: got %q want %q", got, want)
	}

	github := resolved.MCP.Servers[1]
	if github.Name != "github" {
		t.Fatalf("unexpected second documented MCP server: %#v", github)
	}
	if got, want := github.Env["GITHUB_TOKEN"], "ghu_example_token"; got != want {
		t.Fatalf("unexpected GITHUB_TOKEN placeholder expansion: got %q want %q", got, want)
	}
}

func copyDocumentedAgentFixture(t *testing.T, workspaceRoot, name string) {
	t.Helper()

	sourceRoot := filepath.Join(mustAgentsRepoRootPath(t), "docs", "examples", "agents", name)
	targetRoot := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", name)

	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetRoot, relPath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, content, 0o600)
	}); err != nil {
		t.Fatalf("copy documented agent fixture %q: %v", name, err)
	}
}

func mustAgentsRepoRootPath(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}
	return filepath.Join(filepath.Dir(currentFile), "..", "..", "..")
}
