package compozy

import (
	"testing"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetool "github.com/compozy/compozy/engine/tool"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneConfigEmptySlices(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"Should clone workflow configs when input is nil", func(t *testing.T) {
			clones, err := cloneWorkflowConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone agent configs when input is nil", func(t *testing.T) {
			clones, err := cloneAgentConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone tool configs when input is nil", func(t *testing.T) {
			clones, err := cloneToolConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone knowledge configs when input is nil", func(t *testing.T) {
			clones, err := cloneKnowledgeConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone memory configs when input is nil", func(t *testing.T) {
			clones, err := cloneMemoryConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone MCP configs when input is nil", func(t *testing.T) {
			clones, err := cloneMCPConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone schema configs when input is nil", func(t *testing.T) {
			clones, err := cloneSchemaConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone model configs when input is nil", func(t *testing.T) {
			clones, err := cloneModelConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone schedule configs when input is nil", func(t *testing.T) {
			clones, err := cloneScheduleConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone webhook configs when input is nil", func(t *testing.T) {
			clones, err := cloneWebhookConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestCloneConfigDeepCopy(t *testing.T) {
	t.Parallel()
	t.Run("Should deep copy workflow configs", func(t *testing.T) {
		t.Parallel()
		original := &engineworkflow.Config{ID: "deep-copy"}
		clones, err := cloneWorkflowConfigs([]*engineworkflow.Config{original})
		require.NoError(t, err)
		require.Len(t, clones, 1)
		assert.NotSame(t, original, clones[0])
		assert.Equal(t, original, clones[0])
		clones[0].ID = "mutated"
		assert.Equal(t, "deep-copy", original.ID)
	})
}

func TestBuildResourceClones(t *testing.T) {
	t.Parallel()
	t.Run("Should clone all resource configurations", func(t *testing.T) {
		cfg := &config{
			workflows:      []*engineworkflow.Config{{ID: "wf"}},
			agents:         []*engineagent.Config{{ID: "agent"}},
			tools:          []*enginetool.Config{{ID: "tool"}},
			knowledgeBases: []*engineknowledge.BaseConfig{{ID: "kb"}},
			memories:       []*enginememory.Config{{ID: "mem"}},
			mcps:           []*enginemcp.Config{{ID: "mcp"}},
			schemas:        []*engineschema.Schema{{"type": "object"}},
			models:         []*enginecore.ProviderConfig{{Provider: enginecore.ProviderName("openai"), Model: "gpt"}},
			schedules:      []*projectschedule.Config{{ID: "schedule", WorkflowID: "wf"}},
			webhooks:       []*enginewebhook.Config{{Slug: "hook"}},
		}
		clones, err := buildResourceClones(cfg)
		require.NoError(t, err)
		require.Len(t, clones.workflows, 1)
		require.Len(t, clones.agents, 1)
		require.Len(t, clones.tools, 1)
		require.Len(t, clones.knowledgeBases, 1)
		require.Len(t, clones.memories, 1)
		require.Len(t, clones.mcps, 1)
		require.Len(t, clones.schemas, 1)
		require.Len(t, clones.models, 1)
		require.Len(t, clones.schedules, 1)
		require.Len(t, clones.webhooks, 1)
		assert.NotSame(t, cfg.workflows[0], clones.workflows[0])
		assert.NotSame(t, cfg.agents[0], clones.agents[0])
		assert.NotSame(t, cfg.tools[0], clones.tools[0])
		assert.NotSame(t, cfg.knowledgeBases[0], clones.knowledgeBases[0])
		assert.NotSame(t, cfg.memories[0], clones.memories[0])
		assert.NotSame(t, cfg.mcps[0], clones.mcps[0])
		assert.NotSame(t, cfg.schemas[0], clones.schemas[0])
		assert.NotSame(t, cfg.models[0], clones.models[0])
		assert.NotSame(t, cfg.schedules[0], clones.schedules[0])
		assert.NotSame(t, cfg.webhooks[0], clones.webhooks[0])
	})
}
