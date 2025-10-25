package agentcatalog

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/require"
)

func TestDescribeHandlerReturnsAgentDetails(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	_, err := store.Put(
		t.Context(),
		resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: "agent.writer"},
		map[string]any{
			"actions": []any{
				map[string]any{"id": "default", "prompt": "write something"},
			},
		},
	)
	require.NoError(t, err)

	env := newTestEnvironment(store)
	ctx := core.WithProjectName(t.Context(), "demo")

	output, err := describeHandler(env)(ctx, map[string]any{"agent_id": "agent.writer"})
	require.NoError(t, err)

	require.Equal(t, "agent.writer", output["agent_id"])
	actions, ok := output["actions"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, actions, 1)
	require.Equal(t, "default", actions[0]["id"])
	require.Equal(t, "write something", actions[0]["prompt"])
}

func TestDescribeHandlerValidatesInput(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	env := newTestEnvironment(store)
	ctx := core.WithProjectName(t.Context(), "demo")

	_, err := describeHandler(env)(ctx, map[string]any{})
	require.Error(t, err)
}

func TestDescribeHandlerRequiresExistingAgent(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	env := newTestEnvironment(store)
	ctx := core.WithProjectName(t.Context(), "demo")

	_, err := describeHandler(env)(ctx, map[string]any{"agent_id": "missing"})
	require.Error(t, err)
}
