package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resources/exporter"
	"github.com/compozy/compozy/engine/tool"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestImporter_StrategiesAndRoundTrip(t *testing.T) {
	t.Run("Should honor seed_only and overwrite_conflicts and preserve round-trip", func(t *testing.T) {
		ctx := context.Background()
		project := "proj"
		store := resources.NewMemoryResourceStore()
		t.Setenv("MCP_PROXY_URL", "https://proxy.example.com")

		// Prepare repo-like directory with YAML files
		repo := t.TempDir()
		agentCfg := agent.Config{
			ID:           "writer",
			Instructions: "Write release notes",
			Model: agent.Model{
				Config: core.ProviderConfig{
					Provider: core.ProviderMock,
					Model:    "gpt-4o-mini",
				},
			},
		}
		agentBytes, err := yaml.Marshal(agentCfg)
		require.NoError(t, err)
		writeFile(t, repo, "agents/writer.yaml", string(agentBytes))

		toolDefaults := core.Input{
			"command": "fmt",
			"args":    []any{"a", "z"},
		}
		toolCfg := tool.Config{
			ID:          "fmt",
			Description: "Format slice values",
			Config:      &toolDefaults,
		}
		toolBytes, err := yaml.Marshal(toolCfg)
		require.NoError(t, err)
		writeFile(t, repo, "tools/fmt.yaml", string(toolBytes))

		mcpCfg := mcp.Config{
			ID:        "github",
			URL:       "https://mcp.example.com/github",
			Transport: mcpproxy.TransportSSE,
			Proto:     mcp.DefaultProtocolVersion,
		}
		mcpCfg.SetDefaults()
		mcpBytes, err := yaml.Marshal(mcpCfg)
		require.NoError(t, err)
		writeFile(t, repo, "mcps/github.yaml", string(mcpBytes))

		// First import (seed_only) writes both
		out, err := ImportFromDir(ctx, project, store, repo, SeedOnly, "tester")
		require.NoError(t, err)
		require.Equal(t, 1, out.Imported[resources.ResourceAgent])
		require.Equal(t, 1, out.Imported[resources.ResourceTool])
		require.Equal(t, 1, out.Imported[resources.ResourceMCP])

		// Second import (seed_only) skips both
		out, err = ImportFromDir(ctx, project, store, repo, SeedOnly, "tester")
		require.NoError(t, err)
		require.Equal(t, 1, out.Skipped[resources.ResourceAgent])
		require.Equal(t, 1, out.Skipped[resources.ResourceTool])
		require.Equal(t, 1, out.Skipped[resources.ResourceMCP])

		// Overwrite conflict for tool only by changing args order
		updatedDefaults := core.Input{
			"command": "fmt",
			"args":    []any{"z", "a"},
		}
		toolCfg.Config = &updatedDefaults
		toolBytes, err = yaml.Marshal(toolCfg)
		require.NoError(t, err)
		writeFile(t, repo, "tools/fmt.yaml", string(toolBytes))
		mcpCfg.URL = "https://mcp.example.com/github-v2"
		mcpBytes, err = yaml.Marshal(mcpCfg)
		require.NoError(t, err)
		writeFile(t, repo, "mcps/github.yaml", string(mcpBytes))
		out, err = ImportFromDir(ctx, project, store, repo, OverwriteConflicts, "tester")
		require.NoError(t, err)
		require.Equal(t, 1, out.Overwritten[resources.ResourceTool])
		require.Equal(t, 1, out.Overwritten[resources.ResourceMCP])

		// Round-trip: export -> import -> export should be equal
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		_, err = exporter.ExportToDir(ctx, project, store, dir1)
		require.NoError(t, err)
		// clear store and re-import
		store2 := resources.NewMemoryResourceStore()
		_, err = ImportFromDir(ctx, project, store2, repo, SeedOnly, "tester")
		require.NoError(t, err)
		_, err = exporter.ExportToDir(ctx, project, store2, dir2)
		require.NoError(t, err)
		b1, err := os.ReadFile(filepath.Join(dir1, "agents", "writer.yaml"))
		require.NoError(t, err)
		b2, err := os.ReadFile(filepath.Join(dir2, "agents", "writer.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(b1), string(b2))
		b3, err := os.ReadFile(filepath.Join(dir1, "mcps", "github.yaml"))
		require.NoError(t, err)
		b4, err := os.ReadFile(filepath.Join(dir2, "mcps", "github.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(b3), string(b4))
	})
}
