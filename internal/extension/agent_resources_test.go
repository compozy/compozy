package extensionpkg

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestLoadAgentResources(t *testing.T) {
	t.Parallel()
	t.Run("Should decode a persisted extension kind without modifying its writable package", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeStaticAgentFixture(t, root, "writer", false)
		path := filepath.Join(root, "agents", "writer", "AGENT.md")
		body := "---\nname: writer\nkind: stays_on\n---\nWrite.\n"
		writeFile(t, path, body)
		if err := os.Chmod(path, 0600); err != nil {
			t.Fatal(err)
		}
		agents, err := LoadAgentResources(root, []string{"agents"})
		if err != nil || len(agents) != 1 || agents[0].Agent.Kind != compozyconfig.AgentKindAlwaysOn {
			t.Fatalf("agents = %+v, %v", agents, err)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != body {
			t.Fatalf("package changed: %q, %v", after, err)
		}
	})

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

// Invariant (UT-016, UT-093): extension and marketplace definitions never carry operator-only
// grants; the loader drops them and emits a diagnostic naming the untrusted source.
// Owning layer: extension agent resource loading, which marketplace installs also use.
// This test swaps slog.Default(), so it must stay non-parallel.
func TestLoadAgentResourcesDropsUntrustedGrants(t *testing.T) {
	rootDir := t.TempDir()
	writeStaticAgentFixture(t, rootDir, "writer", false)
	path := filepath.Join(rootDir, "agents", "writer", "AGENT.md")
	frontmatter := "---\nworkspace_access: unrestricted\nsession_approval: workspace\n" +
		"permission_rules:\n  - kind: edit\n    path: docs/**\n    decision: allow\n"
	writeFile(t, path, strings.Replace("---\nname: writer\n---\n\nDo the work.\n", "---\n", frontmatter, 1))
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(original) })

	agents, err := LoadAgentResources(rootDir, []string{"agents"})
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Fatalf("agents = %#v, want writer", agents)
	}
	agent := agents[0].Agent
	if agent.WorkspaceAccess != "" || agent.SessionApproval != "" || len(agent.PermissionRules) != 0 {
		t.Fatalf("untrusted grants kept: workspace=%q approval=%q rules=%#v",
			agent.WorkspaceAccess, agent.SessionApproval, agent.PermissionRules)
	}
	for _, code := range []string{
		"workspace_access_untrusted_source",
		"session_approval_untrusted_source",
		"permission_rules_untrusted_source",
	} {
		if !strings.Contains(logs.String(), "code="+code) || !strings.Contains(logs.String(), "agent_name=writer") {
			t.Fatalf("diagnostic %s missing from logs: %s", code, logs.String())
		}
	}
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
		{
			name: "Should reject a symlinked agent definition",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				agentDir := filepath.Join(rootDir, "agents", "writer")
				if err := os.MkdirAll(agentDir, 0o755); err != nil {
					t.Fatalf("os.MkdirAll(writer) error = %v", err)
				}
				outside := filepath.Join(t.TempDir(), "AGENT.md")
				writeFile(t, outside, "---\nname: writer\n---\n\nOutside.\n")
				if err := os.Symlink(outside, filepath.Join(agentDir, "AGENT.md")); err != nil {
					t.Fatalf("os.Symlink(AGENT.md) error = %v", err)
				}
			},
			contains: []string{"AGENT.md", "symlinks are not allowed"},
		},
		{
			name: "Should reject a symlinked agent sidecar",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				writeStaticAgentFixture(t, rootDir, "writer", false)
				outside := filepath.Join(t.TempDir(), "SOUL.md")
				writeFile(t, outside, "Outside soul.\n")
				if err := os.Symlink(outside, filepath.Join(rootDir, "agents", "writer", "SOUL.md")); err != nil {
					t.Fatalf("os.Symlink(SOUL.md) error = %v", err)
				}
			},
			contains: []string{"SOUL.md", "symlinks are not allowed"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			tc.setup(t, rootDir)
			_, err := LoadAgentResources(rootDir, []string{"agents"})
			if !errors.Is(err, ErrManifestInvalid) {
				t.Fatalf("LoadAgentResources() error = %v, want ErrManifestInvalid", err)
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
