package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteAgent_ConflictsWhenReferenced(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	body := map[string]any{
		"id":           "assistant",
		"instructions": "help the user",
		"model": map[string]any{
			"provider": "openai",
			"model":    "gpt-4o-mini",
		},
	}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "assistant", Body: body})
	require.NoError(t, err)
	wf := &workflow.Config{
		ID:     "wf1",
		Agents: []agent.Config{{ID: "assistant", Instructions: "todo", Model: agent.Model{Ref: "openai:gpt-4o-mini"}}},
	}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: wf.ID}, wf)
	require.NoError(t, err)
	childTask := &task.Config{
		BaseConfig: task.BaseConfig{ID: "call-agent", Type: task.TaskTypeBasic, Agent: &agent.Config{ID: "assistant"}},
	}
	_, err = store.Put(
		ctx,
		resources.ResourceKey{Project: project, Type: resources.ResourceTask, ID: childTask.ID},
		childTask,
	)
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "assistant"})
	require.Error(t, err)
	var conflict resourceutil.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Len(t, conflict.Details, 2)
	assert.Equal(t, "workflows", conflict.Details[0].Resource)
	assert.Equal(t, "tasks", conflict.Details[1].Resource)
}

func TestListAgents_FilterByWorkflow(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	fetch := func(id string) map[string]any {
		return map[string]any{
			"id":           id,
			"instructions": "assist",
			"model":        map[string]any{"provider": "openai", "model": "gpt-4o-mini"},
		}
	}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "a1", Body: fetch("a1")})
	require.NoError(t, err)
	_, err = NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "a2", Body: fetch("a2")})
	require.NoError(t, err)
	wf := &workflow.Config{ID: "wf1", Agents: []agent.Config{{ID: "a1", Instructions: "assist"}}}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: wf.ID}, wf)
	require.NoError(t, err)
	out, err := NewList(store).Execute(ctx, &ListInput{Project: project, WorkflowID: "wf1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, out.Items, 1)
	assert.Equal(t, "a1", out.Items[0]["id"])
}
