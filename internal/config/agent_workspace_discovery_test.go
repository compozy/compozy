package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/fileutil"
)

func TestLoadWorkspaceAgentDefsRejectsMismatchedDirectoryNameContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject agent files whose declared name differs from the containing directory", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}

		root := t.TempDir()
		writeAgentDefinition(
			t,
			filepath.Join(root, DirName, AgentsDirName, "not-reviewer", agentDefName),
			"reviewer",
			"claude",
			"workspace-mismatch",
		)
		writeAgentDefinition(
			t,
			filepath.Join(homePaths.AgentsDir, "reviewer", agentDefName),
			"reviewer",
			"claude",
			"global-review",
		)

		_, err = LoadWorkspaceAgentDefs(root, nil, homePaths, "")
		if err == nil {
			t.Fatal("LoadWorkspaceAgentDefs() error = nil, want mismatched name failure")
		}
		if !strings.Contains(err.Error(), "not-reviewer") || !strings.Contains(err.Error(), "reviewer") {
			t.Fatalf("LoadWorkspaceAgentDefs() error = %v, want directory and declared name context", err)
		}
	})
}

func TestLoadWorkspaceAgentDefsSkipsReservedNames(t *testing.T) {
	t.Parallel()

	t.Run("Should exclude reserved workspace and global agent directories", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		root := t.TempDir()
		writeAgentDefinition(
			t,
			filepath.Join(root, DirName, AgentsDirName, BuiltinCoordinatorAgentName, agentDefName),
			BuiltinCoordinatorAgentName,
			"claude",
			"shadowed-coordinator",
		)
		writeAgentDefinition(
			t,
			filepath.Join(homePaths.AgentsDir, BuiltinDreamingCuratorAgentName, agentDefName),
			BuiltinDreamingCuratorAgentName,
			"claude",
			"shadowed-curator",
		)
		writeAgentDefinition(
			t,
			filepath.Join(root, DirName, AgentsDirName, "worker", agentDefName),
			"worker",
			"claude",
			"worker-model",
		)

		agents, err := LoadWorkspaceAgentDefs(root, nil, homePaths, "")
		if err != nil {
			t.Fatalf("LoadWorkspaceAgentDefs() error = %v", err)
		}
		if len(agents) != 1 || agents[0].Name != "worker" {
			t.Fatalf("LoadWorkspaceAgentDefs() = %#v, want only worker", agents)
		}
	})
}

func TestLoadWorkspaceAgentDefsReturnsLexicalDirectoryOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should return valid same-root agents in lexical directory order", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}

		root := t.TempDir()
		writeAgentDefinition(
			t,
			filepath.Join(root, DirName, AgentsDirName, "worker-z", agentDefName),
			"worker-z",
			"claude",
			"created-first",
		)
		writeAgentDefinition(
			t,
			filepath.Join(root, DirName, AgentsDirName, "worker-a", agentDefName),
			"worker-a",
			"claude",
			"lexical-first",
		)

		agents, err := LoadWorkspaceAgentDefs(root, nil, homePaths, "")
		if err != nil {
			t.Fatalf("LoadWorkspaceAgentDefs() error = %v", err)
		}
		if got, want := len(agents), 2; got != want {
			t.Fatalf("len(LoadWorkspaceAgentDefs()) = %d, want %d", got, want)
		}
		if got, want := agents[0].Name, "worker-a"; got != want {
			t.Fatalf("LoadWorkspaceAgentDefs()[0].Name = %q, want %q", got, want)
		}
		if got, want := agents[0].Model, "lexical-first"; got != want {
			t.Fatalf("LoadWorkspaceAgentDefs()[0].Model = %q, want %q", got, want)
		}
		if got, want := agents[1].Name, "worker-z"; got != want {
			t.Fatalf("LoadWorkspaceAgentDefs()[1].Name = %q, want %q", got, want)
		}
	})
}

func TestLoadAgentDefFromDirectoryRetainsAgentDirectoryIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should read agent definition and sidecars from the held directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		agentsDir := filepath.Join(root, DirName, AgentsDirName)
		originalDir := filepath.Join(agentsDir, "coder")
		writeAgentDefinition(t, filepath.Join(originalDir, agentDefName), "coder", "claude", "original-model")
		writeFile(t, filepath.Join(originalDir, MCPJSONName), `{
  "mcpServers": {
    "original": {"command": "original-command"}
  }
}`)
		writeFile(t, filepath.Join(originalDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "original-capability"
summary = "Original capability."
outcome = "Original result."
`)

		rootDirectory, err := fileutil.OpenDirectory(agentsDir)
		if err != nil {
			t.Fatalf("fileutil.OpenDirectory() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := rootDirectory.Close(); closeErr != nil {
				t.Errorf("root Directory.Close() error = %v", closeErr)
			}
		})
		agentDirectory, err := rootDirectory.OpenDirectory("coder")
		if err != nil {
			t.Fatalf("Directory.OpenDirectory() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := agentDirectory.Close(); closeErr != nil {
				t.Errorf("agent Directory.Close() error = %v", closeErr)
			}
		})

		archivedDir := filepath.Join(agentsDir, "coder-original")
		if err := os.Rename(originalDir, archivedDir); err != nil {
			t.Fatalf("os.Rename() error = %v", err)
		}
		replacementDir := filepath.Join(agentsDir, "coder")
		writeAgentDefinition(t, filepath.Join(replacementDir, agentDefName), "coder", "claude", "replacement-model")
		writeFile(t, filepath.Join(replacementDir, MCPJSONName), `{
  "mcpServers": {
    "replacement": {"command": "replacement-command"}
  }
}`)
		writeFile(t, filepath.Join(replacementDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "replacement-capability"
summary = "Replacement capability."
outcome = "Replacement result."
`)

		agent, err := loadAgentDefFromDirectory(agentDirectory, agentDefName, filepath.Join(originalDir, agentDefName))
		if err != nil {
			t.Fatalf("loadAgentDefFromDirectory() error = %v", err)
		}
		if got, want := agent.Model, "original-model"; got != want {
			t.Fatalf("AgentDef.Model = %q, want %q", got, want)
		}
		if got, want := len(agent.MCPServers), 1; got != want {
			t.Fatalf("len(AgentDef.MCPServers) = %d, want %d", got, want)
		}
		if got, want := agent.MCPServers[0].Name, "original"; got != want {
			t.Fatalf("AgentDef.MCPServers[0].Name = %q, want %q", got, want)
		}
		if agent.Capabilities == nil {
			t.Fatal("AgentDef.Capabilities = nil, want original catalog")
		}
		if got, want := len(agent.Capabilities.Capabilities), 1; got != want {
			t.Fatalf("len(AgentDef.Capabilities.Capabilities) = %d, want %d", got, want)
		}
		if got, want := agent.Capabilities.Capabilities[0].ID, "original-capability"; got != want {
			t.Fatalf("AgentDef.Capabilities.Capabilities[0].ID = %q, want %q", got, want)
		}
	})
}
