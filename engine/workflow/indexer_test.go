package workflow

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/require"
)

func TestWorkflow_IndexToResourceStore_AndCompile(t *testing.T) {
	ctx := context.Background()
	store := resources.NewMemoryResourceStore()
	proj := &project.Config{Name: "demo"}
	wf := &Config{
		ID:     "wf1",
		Agents: []agent.Config{{ID: "writer"}},
		Tools:  []tool.Config{{ID: "fmt", Description: "format"}},
		Tasks: []task.Config{{
			BaseConfig: task.BaseConfig{ID: "t1", Type: task.TaskTypeBasic, Agent: &agent.Config{ID: "writer"}},
		}},
	}
	require.NoError(t, wf.IndexToResourceStore(ctx, proj.Name, store))

	// Verify agent indexed
	_, _, err := store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: "writer"})
	require.NoError(t, err)

	// Compile should resolve selector from store
	compiled, err := wf.Compile(ctx, proj, store)
	require.NoError(t, err)
	require.NotNil(t, compiled.Tasks[0].Agent)
	require.Equal(t, "writer", compiled.Tasks[0].Agent.ID)
}
