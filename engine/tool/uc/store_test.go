package uc

import (
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteTool_ConflictsWhenReferenced(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := t.Context()
	project := "demo"
	body := map[string]any{
		"id":     "http",
		"type":   "http",
		"config": map[string]any{"method": "GET", "url": "https://example.com"},
	}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "http", Body: body})
	require.NoError(t, err)
	wf := &workflow.Config{ID: "wf1", Tools: []tool.Config{{ID: "http"}}}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: wf.ID}, wf)
	require.NoError(t, err)
	tk := &task.Config{
		BaseConfig: task.BaseConfig{ID: "call", Type: task.TaskTypeBasic, Tool: &tool.Config{ID: "http"}},
	}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceTask, ID: tk.ID}, tk)
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "http"})
	require.Error(t, err)
	var conflict resourceutil.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Len(t, conflict.Details, 2)
}

func TestListTools_FilterByWorkflow(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := t.Context()
	project := "demo"
	doc := func(id string) map[string]any {
		return map[string]any{
			"id":     id,
			"type":   "http",
			"config": map[string]any{"method": "GET", "url": "https://example.com"},
		}
	}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "t1", Body: doc("t1")})
	require.NoError(t, err)
	_, err = NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "t2", Body: doc("t2")})
	require.NoError(t, err)
	wf := &workflow.Config{ID: "wf1", Tools: []tool.Config{{ID: "t1"}}}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: wf.ID}, wf)
	require.NoError(t, err)
	out, err := NewList(store).Execute(ctx, &ListInput{Project: project, WorkflowID: "wf1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, out.Items, 1)
	assert.Equal(t, "t1", out.Items[0]["id"])
}

func TestUpsert_RejectsNativeImplementation(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := t.Context()
	body := map[string]any{
		"implementation": "native",
		"name":           "Native Tool",
		"description":    "Blocked",
	}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: "demo", ID: "native", Body: body})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNativeImplementation)
}
