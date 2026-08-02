package extensionpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAgentResources(t *testing.T) {
	t.Parallel()

	t.Run("Should load strict agent directories with optional sidecars", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		writeStaticAgentFixture(t, rootDir, "writer", true)
		agents, err := LoadAgentResources(rootDir, []string{"agents"})
		if err != nil {
			t.Fatalf("LoadAgentResources() error = %v", err)
		}
		if len(agents) != 1 || agents[0].Agent.Name != "writer" {
			t.Fatalf("LoadAgentResources() = %#v, want writer", agents)
		}
		if agents[0].Soul == nil || agents[0].Soul.SourcePath != "agents/writer/SOUL.md" ||
			agents[0].Soul.Body != "Write with care.\n" {
			t.Fatalf("Soul = %#v, want parsed source and body", agents[0].Soul)
		}
		if agents[0].Heartbeat == nil || agents[0].Heartbeat.SourcePath != "agents/writer/HEARTBEAT.md" ||
			agents[0].Heartbeat.Body != "Check the queue.\n" {
			t.Fatalf("Heartbeat = %#v, want parsed source and body", agents[0].Heartbeat)
		}
	})

	t.Run("Should load an agent without sidecars", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		writeStaticAgentFixture(t, rootDir, "writer", false)
		agents, err := LoadAgentResources(rootDir, []string{"agents"})
		if err != nil {
			t.Fatalf("LoadAgentResources() error = %v", err)
		}
		if len(agents) != 1 || agents[0].Soul != nil || agents[0].Heartbeat != nil {
			t.Fatalf("LoadAgentResources() = %#v, want one agent without sidecars", agents)
		}
	})

	t.Run("Should merge MCP servers and capabilities from agent sidecars", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		writeStaticAgentFixture(t, rootDir, "writer", false)
		agentDir := filepath.Join(rootDir, "agents", "writer")
		writeFile(t, filepath.Join(agentDir, "mcp.json"), `{"mcpServers":{"linear":{"command":"linear-mcp"}}}`)
		writeFile(t, filepath.Join(agentDir, "capabilities.toml"), `
[[capabilities]]
id = "write"
summary = "Write copy."
outcome = "Finished copy."
version = "1.0.0"
execution_outline = ["draft"]
requirements = ["workspace-read"]
`)
		agents, err := LoadAgentResources(rootDir, []string{"agents"})
		if err != nil {
			t.Fatalf("LoadAgentResources() error = %v", err)
		}
		if got := len(agents[0].Agent.MCPServers); got != 1 {
			t.Fatalf("len(MCPServers) = %d, want 1", got)
		}
		if agents[0].Agent.Capabilities == nil || len(agents[0].Agent.Capabilities.Capabilities) != 1 {
			t.Fatalf("Capabilities = %#v, want one capability", agents[0].Agent.Capabilities)
		}
	})
}

func TestLoadAgentResourcesRejectsInvalidLayoutsAndSidecars(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		setup    func(*testing.T, string)
		contains []string
	}{
		{
			name: "Should reject loose markdown files",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(rootDir, "agents"), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(agents) error = %v", err)
				}
				writeFile(t, filepath.Join(rootDir, "agents", "stray.md"), "stray")
			},
			contains: []string{"stray.md", "<entry>/<agent>/AGENT.md"},
		},
		{
			name: "Should reject an agent directory without AGENT.md",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(rootDir, "agents", "writer"), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(writer) error = %v", err)
				}
			},
			contains: []string{"agents/writer", "AGENT.md"},
		},
		{
			name: "Should reject malformed SOUL frontmatter",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				writeStaticAgentFixture(t, rootDir, "writer", false)
				writeFile(t, filepath.Join(rootDir, "agents", "writer", "SOUL.md"), "---\nrole: [\n---\nBody\n")
			},
			contains: []string{"SOUL.md", "frontmatter"},
		},
		{
			name: "Should reject unsupported HEARTBEAT frontmatter",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				writeStaticAgentFixture(t, rootDir, "writer", false)
				writeFile(
					t,
					filepath.Join(rootDir, "agents", "writer", "HEARTBEAT.md"),
					"---\nunknown: true\n---\nBody\n",
				)
			},
			contains: []string{"HEARTBEAT.md", "unknown", "unsupported_field"},
		},
		{
			name: "Should reject an agent directory symlink escaping the extension root",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				outside := t.TempDir()
				writeStaticAgentDirectory(t, outside, "writer")
				if err := os.MkdirAll(filepath.Join(rootDir, "agents"), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(agents) error = %v", err)
				}
				if err := os.Symlink(
					filepath.Join(outside, "writer"),
					filepath.Join(rootDir, "agents", "writer"),
				); err != nil {
					t.Fatalf("os.Symlink() error = %v", err)
				}
			},
			contains: []string{"escapes extension root"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			tc.setup(t, rootDir)
			_, err := LoadAgentResources(rootDir, []string{"agents"})
			if err == nil {
				t.Fatal("LoadAgentResources() error = nil, want validation failure")
			}
			for _, want := range tc.contains {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("LoadAgentResources() error = %v, want error containing %q", err, want)
				}
			}
		})
	}
}

func writeStaticAgentFixture(t *testing.T, rootDir, name string, sidecars bool) {
	t.Helper()

	writeStaticAgentDirectory(t, filepath.Join(rootDir, "agents"), name)
	if !sidecars {
		return
	}
	agentDir := filepath.Join(rootDir, "agents", name)
	writeFile(t, filepath.Join(agentDir, "SOUL.md"), "Write with care.\n")
	writeFile(t, filepath.Join(agentDir, "HEARTBEAT.md"), "Check the queue.\n")
}

func writeStaticAgentDirectory(t *testing.T, parentDir, name string) {
	t.Helper()

	agentDir := filepath.Join(parentDir, name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v", agentDir, err)
	}
	writeFile(t, filepath.Join(agentDir, "AGENT.md"), "---\nname: "+name+"\n---\n\nDo the work.\n")
}
