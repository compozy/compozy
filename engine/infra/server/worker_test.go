package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/helpers"
)

func TestBuildToolEnvironmentUsesExistingStore(t *testing.T) {
	t.Parallel()
	ctx := helpers.NewTestContext(t)
	store := resources.NewMemoryResourceStore()
	projectConfig := &project.Config{Name: "agentic"}
	require.NoError(t, projectConfig.SetCWD(t.TempDir()))
	require.NotNil(t, projectConfig.GetCWD())
	agentConfig := &agent.Config{
		Resource:     "agent",
		ID:           "research-web",
		Instructions: "Summarize topics.",
		Actions: []*agent.ActionConfig{
			{ID: "research_topic", Prompt: "Provide a summary."},
		},
	}
	require.NoError(t, agentConfig.SetCWD(projectConfig.GetCWD().PathStr()))
	key := resources.ResourceKey{Project: projectConfig.Name, Type: resources.ResourceAgent, ID: agentConfig.ID}
	_, err := store.Put(ctx, key, agentConfig)
	require.NoError(t, err)
	env, err := buildToolEnvironment(ctx, projectConfig, nil, &noopWorkflowRepo{}, &noopTaskRepo{}, store)
	require.NoError(t, err)
	require.Same(t, store, env.ResourceStore())
	getUC := agentuc.NewGet(env.ResourceStore())
	out, err := getUC.Execute(ctx, &agentuc.GetInput{Project: projectConfig.Name, ID: agentConfig.ID})
	require.NoError(t, err)
	require.Equal(t, agentConfig.ID, out.Agent["id"])
}

type noopWorkflowRepo struct{}

func (noopWorkflowRepo) ListStates(context.Context, *workflow.StateFilter) ([]*workflow.State, error) {
	panic("not implemented")
}
func (noopWorkflowRepo) UpsertState(context.Context, *workflow.State) error { panic("not implemented") }
func (noopWorkflowRepo) UpdateStatus(context.Context, core.ID, core.StatusType) error {
	panic("not implemented")
}
func (noopWorkflowRepo) GetState(context.Context, core.ID) (*workflow.State, error) {
	panic("not implemented")
}
func (noopWorkflowRepo) GetStateByID(context.Context, string) (*workflow.State, error) {
	panic("not implemented")
}
func (noopWorkflowRepo) GetStateByTaskID(context.Context, string, string) (*workflow.State, error) {
	panic("not implemented")
}
func (noopWorkflowRepo) GetStateByAgentID(context.Context, string, string) (*workflow.State, error) {
	panic("not implemented")
}
func (noopWorkflowRepo) GetStateByToolID(context.Context, string, string) (*workflow.State, error) {
	panic("not implemented")
}

func (noopWorkflowRepo) CompleteWorkflow(
	context.Context,
	core.ID,
	workflow.OutputTransformer,
) (*workflow.State, error) {
	panic("not implemented")
}
func (noopWorkflowRepo) MergeUsage(context.Context, core.ID, *usage.Summary) error {
	panic("not implemented")
}

type noopTaskRepo struct{}

func (noopTaskRepo) ListStates(context.Context, *task.StateFilter) ([]*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) UpsertState(context.Context, *task.State) error         { panic("not implemented") }
func (noopTaskRepo) GetState(context.Context, core.ID) (*task.State, error) { panic("not implemented") }
func (noopTaskRepo) GetUsageSummary(context.Context, core.ID) (*usage.Summary, error) {
	panic("not implemented")
}
func (noopTaskRepo) WithTransaction(context.Context, func(task.Repository) error) error {
	panic("not implemented")
}
func (noopTaskRepo) GetStateForUpdate(context.Context, core.ID) (*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) ListTasksInWorkflow(context.Context, core.ID) (map[string]*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) ListTasksByStatus(context.Context, core.ID, core.StatusType) ([]*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) ListTasksByAgent(context.Context, core.ID, string) ([]*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) ListTasksByTool(context.Context, core.ID, string) ([]*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) ListChildren(context.Context, core.ID) ([]*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) GetChildByTaskID(context.Context, core.ID, string) (*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) GetTaskTree(context.Context, core.ID) ([]*task.State, error) {
	panic("not implemented")
}
func (noopTaskRepo) ListChildrenOutputs(context.Context, core.ID) (map[string]*core.Output, error) {
	panic("not implemented")
}
func (noopTaskRepo) GetProgressInfo(context.Context, core.ID) (*task.ProgressInfo, error) {
	panic("not implemented")
}
func (noopTaskRepo) MergeUsage(context.Context, core.ID, *usage.Summary) error {
	panic("not implemented")
}
