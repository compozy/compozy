package projectrouter

import (
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/require"
)

func TestToProjectDTO(t *testing.T) {
	t.Parallel()

	t.Run("Should map full config", func(t *testing.T) {
		t.Parallel()

		cfg := &project.Config{
			Name:        "demo",
			Version:     "1.0.0",
			Description: "Example project",
			Author: core.Author{
				Name:  "Alice",
				Email: "alice@example.com",
			},
			Workflows: []*project.WorkflowSourceConfig{{
				Source: "./flow.yaml",
			}},
			Models: []*core.ProviderConfig{{
				Provider: core.ProviderName("openai"),
				Model:    "gpt-4",
			}},
			Schemas: []schema.Schema{{
				"id":   "schema-1",
				"type": "object",
			}},
			Opts: project.Opts{
				SourceOfTruth: "repo",
			},
			Runtime: project.RuntimeConfig{
				Type: "bun",
			},
			AutoLoad: &autoload.Config{
				Enabled: true,
			},
			Tools: []tool.Config{{
				ID:          "tool-1",
				Description: "Test tool",
			}},
			Memories: []memory.Config{{
				ID: "memory-1",
			}},
			MonitoringConfig: &monitoring.Config{
				Enabled: true,
				Path:    "/metrics",
			},
		}

		dto, err := toProjectDTO(cfg)
		require.NoError(t, err)

		require.Equal(t, "demo", dto.Name)
		require.Equal(t, "1.0.0", dto.Version)
		require.Equal(t, "Example project", dto.Description)

		require.NotNil(t, dto.Author)
		require.Equal(t, "Alice", dto.Author.Name)
		require.Equal(t, "alice@example.com", dto.Author.Email)

		require.Len(t, dto.Workflows, 1)
		require.Equal(t, "./flow.yaml", dto.Workflows[0]["source"])

		require.Len(t, dto.Models, 1)
		require.Equal(t, "openai", dto.Models[0]["provider"])
		require.Equal(t, "gpt-4", dto.Models[0]["model"])

		require.Equal(t, "repo", dto.Config["source_of_truth"])
		require.Equal(t, "bun", dto.Runtime["type"])

		enabled, ok := dto.AutoLoad["enabled"].(bool)
		require.True(t, ok)
		require.True(t, enabled)

		require.Len(t, dto.Tools, 1)
		require.Equal(t, "tool-1", dto.Tools[0]["id"])

		require.Len(t, dto.Memories, 1)
		require.Equal(t, "memory-1", dto.Memories[0]["id"])

		require.NotNil(t, dto.Monitoring)
		monEnabled, ok := dto.Monitoring["enabled"].(bool)
		require.True(t, ok)
		require.True(t, monEnabled)
	})

	t.Run("Should return zero DTO for nil input", func(t *testing.T) {
		t.Parallel()
		dto, err := toProjectDTO(nil)
		require.NoError(t, err)
		require.Equal(t, ProjectDTO{}, dto)
	})
}
