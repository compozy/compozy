package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resources/exporter"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func getTestMCPProxyURL() string {
	if url := os.Getenv("TEST_MCP_PROXY_URL"); url != "" {
		return url
	}
	return "https://proxy.example.com"
}

func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestImporter_StrategiesAndRoundTrip(t *testing.T) {
	t.Run("Should honor seed_only and overwrite_conflicts and preserve round-trip", func(t *testing.T) {
		ctx := t.Context()
		project := "proj"
		store := resources.NewMemoryResourceStore()
		proxyURL := getTestMCPProxyURL()
		t.Setenv("TEST_MCP_PROXY_URL", proxyURL)
		t.Setenv("MCP_PROXY_URL", proxyURL)

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
		b5, err := os.ReadFile(filepath.Join(dir1, "tools", "fmt.yaml"))
		require.NoError(t, err)
		b6, err := os.ReadFile(filepath.Join(dir2, "tools", "fmt.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(b5), string(b6))
	})
}

func TestImportTypeFromDir(t *testing.T) {
	t.Parallel()
	t.Run("Should import tasks directory", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		project := "proj"
		store := resources.NewMemoryResourceStore()
		repo := t.TempDir()
		taskCfg := task.Config{
			BaseConfig: task.BaseConfig{ID: "compile-report", Type: task.TaskTypeBasic},
			BasicTask:  task.BasicTask{Action: "mock"},
		}
		bytes, err := yaml.Marshal(taskCfg)
		require.NoError(t, err)
		writeFile(t, repo, "tasks/compile-report.yaml", string(bytes))
		out, err := ImportTypeFromDir(ctx, project, store, repo, SeedOnly, "tester", resources.ResourceTask)
		require.NoError(t, err)
		require.Equal(t, 1, out.Imported[resources.ResourceTask])
		_, _, err = store.Get(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceTask, ID: "compile-report"},
		)
		require.NoError(t, err)
	})
	t.Run("Should return zero counts when directory missing", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		project := "proj"
		store := resources.NewMemoryResourceStore()
		repo := t.TempDir()
		out, err := ImportTypeFromDir(ctx, project, store, repo, SeedOnly, "", resources.ResourceTask)
		require.NoError(t, err)
		require.Equal(t, 0, out.Imported[resources.ResourceTask])
	})
	t.Run("Should skip then overwrite existing tools based on strategy", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		project := "proj"
		store := resources.NewMemoryResourceStore()
		repo := t.TempDir()
		cfg := tool.Config{ID: "fmt", Description: "Format values"}
		bytes, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		writeFile(t, repo, "tools/fmt.yaml", string(bytes))
		res, err := ImportTypeFromDir(ctx, project, store, repo, SeedOnly, "tester", resources.ResourceTool)
		require.NoError(t, err)
		require.Equal(t, 1, res.Imported[resources.ResourceTool])
		res, err = ImportTypeFromDir(ctx, project, store, repo, SeedOnly, "tester", resources.ResourceTool)
		require.NoError(t, err)
		require.Equal(t, 1, res.Skipped[resources.ResourceTool])
		cfg.Description = "Updated description"
		bytes, err = yaml.Marshal(cfg)
		require.NoError(t, err)
		writeFile(t, repo, "tools/fmt.yaml", string(bytes))
		res, err = ImportTypeFromDir(ctx, project, store, repo, OverwriteConflicts, "tester", resources.ResourceTool)
		require.NoError(t, err)
		require.Equal(t, 1, res.Overwritten[resources.ResourceTool])
		value, _, err := store.Get(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceTool, ID: "fmt"},
		)
		require.NoError(t, err)
		stored, ok := value.(*tool.Config)
		require.True(t, ok)
		require.Equal(t, "Updated description", stored.Description)
	})
	t.Run("Should error when YAML file is missing id", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		project := "proj"
		store := resources.NewMemoryResourceStore()
		repo := t.TempDir()
		writeFile(t, repo, "tasks/invalid.yaml", "type: basic")
		_, err := ImportTypeFromDir(ctx, project, store, repo, SeedOnly, "tester", resources.ResourceTask)
		require.Error(t, err)
		require.ErrorContains(t, err, "missing id field")
	})
	t.Run("Should error when duplicate ids exist in directory", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		project := "proj"
		store := resources.NewMemoryResourceStore()
		repo := t.TempDir()
		cfg := tool.Config{ID: "fmt"}
		bytes, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		writeFile(t, repo, "tools/fmt.yaml", string(bytes))
		writeFile(t, repo, "tools/fmt_copy.yaml", string(bytes))
		_, err = ImportTypeFromDir(ctx, project, store, repo, SeedOnly, "tester", resources.ResourceTool)
		require.Error(t, err)
		require.ErrorContains(t, err, "duplicate id")
	})
}
