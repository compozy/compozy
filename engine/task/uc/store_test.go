package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resourceutil"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteTask_ConflictsWhenWorkflowReferences(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	body := map[string]any{
		"id":           "approve",
		"type":         "basic",
		"instructions": "do work",
		"model":        map[string]any{"provider": "openai", "model": "gpt-4o-mini"},
	}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "approve", Body: body})
	require.NoError(t, err)
	wf := &workflow.Config{ID: "wf1", Tasks: []task.Config{{BaseConfig: task.BaseConfig{ID: "approve"}}}}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: wf.ID}, wf)
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "approve"})
	require.Error(t, err)
	var conflict resourceutil.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Equal(t, "workflows", conflict.Details[0].Resource)
}

func TestListTasks_FilterByWorkflow(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	build := func(id string) map[string]any {
		return map[string]any{
			"id":           id,
			"type":         "basic",
			"instructions": "work",
			"model":        map[string]any{"provider": "openai", "model": "gpt-4o-mini"},
		}
	}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "t1", Body: build("t1")})
	require.NoError(t, err)
	_, err = NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "t2", Body: build("t2")})
	require.NoError(t, err)
	wf := &workflow.Config{ID: "wf1", Tasks: []task.Config{{BaseConfig: task.BaseConfig{ID: "t1"}}}}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: wf.ID}, wf)
	require.NoError(t, err)
	out, err := NewList(store).Execute(ctx, &ListInput{Project: project, WorkflowID: "wf1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, out.Items, 1)
	assert.Equal(t, "t1", out.Items[0]["id"])
}
