package compozy

import (
	"testing"

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
		{"Workflow", func(t *testing.T) {
			clones, err := cloneWorkflowConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Agent", func(t *testing.T) {
			clones, err := cloneAgentConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Tool", func(t *testing.T) {
			clones, err := cloneToolConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Knowledge", func(t *testing.T) {
			clones, err := cloneKnowledgeConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Memory", func(t *testing.T) {
			clones, err := cloneMemoryConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"MCP", func(t *testing.T) {
			clones, err := cloneMCPConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Schema", func(t *testing.T) {
			clones, err := cloneSchemaConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Model", func(t *testing.T) {
			clones, err := cloneModelConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Schedule", func(t *testing.T) {
			clones, err := cloneScheduleConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Webhook", func(t *testing.T) {
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
