package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resourceutil"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteSchema_ConflictsWhenReferenced(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	_, err := NewUpsert(
		store,
	).Execute(ctx, &UpsertInput{Project: project, ID: "user", Body: map[string]any{"type": "object"}})
	require.NoError(t, err)
	ref := schema.Schema(map[string]any{"__schema_ref__": "user"})
	wf := &workflow.Config{ID: "wf1", Opts: workflow.Opts{InputSchema: &ref}}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: "wf1"}, wf)
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "user"})
	require.Error(t, err)
	var conflict resourceutil.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.NotEmpty(t, conflict.Details)
	assert.Equal(t, "workflows", conflict.Details[0].Resource)
	_, getErr := NewGet(store).Execute(ctx, &GetInput{Project: project, ID: "user"})
	assert.NoError(t, getErr)
}

func TestDeleteSchema_RemovesWhenUnreferenced(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	_, err := NewUpsert(
		store,
	).Execute(ctx, &UpsertInput{Project: project, ID: "user", Body: map[string]any{"type": "object"}})
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "user"})
	require.NoError(t, err)
	_, getErr := NewGet(store).Execute(ctx, &GetInput{Project: project, ID: "user"})
	assert.Error(t, getErr)
	assert.True(t, errors.Is(getErr, ErrNotFound))
}
