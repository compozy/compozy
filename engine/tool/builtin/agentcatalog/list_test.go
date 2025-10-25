package agentcatalog

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/stretchr/testify/require"
)

func TestListHandlerReturnsAgents(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	seedAgentResource(t, store, "demo", "agent.writer", "default", "edit")
	seedAgentResource(t, store, "demo", "agent.researcher", "default")

	env := toolenv.New(nil, nil, nil, nil, store)
	ctx := core.WithProjectName(t.Context(), "demo")

	output, err := listHandler(env)(ctx, map[string]any{})
	require.NoError(t, err)

	agents, ok := output["agents"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, agents, 2)
	collected := make(map[string]struct{})
	for _, agent := range agents {
		collected[agent["agent_id"].(string)] = struct{}{}
	}
	require.Contains(t, collected, "agent.writer")
	require.Contains(t, collected, "agent.researcher")
}

func TestListHandlerRequiresProject(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	env := toolenv.New(nil, nil, nil, nil, store)

	_, err := listHandler(env)(t.Context(), map[string]any{})
	require.Error(t, err)
}

func seedAgentResource(t *testing.T, store resources.ResourceStore, project, id string, actions ...string) {
	t.Helper()
	payload := map[string]any{
		"actions": make([]any, 0, len(actions)),
	}
	for _, action := range actions {
		payload["actions"] = append(payload["actions"].([]any), map[string]any{"id": action})
	}
	_, err := store.Put(
		t.Context(),
		resources.ResourceKey{Project: project, Type: resources.ResourceAgent, ID: id},
		payload,
	)
	require.NoError(t, err)
}
