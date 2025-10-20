package uc

import (
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteModel_ConflictsWhenAgentReferences(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := t.Context()
	project := "demo"
	modelBody := map[string]any{"provider": "openai", "model": "gpt-4o-mini"}
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "openai:gpt-4o-mini", Body: modelBody})
	require.NoError(t, err)
	ag := &agent.Config{ID: "reviewer"}
	ag.Model.Ref = "openai:gpt-4o-mini"
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceAgent, ID: ag.ID}, ag)
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "openai:gpt-4o-mini"})
	require.Error(t, err)
	var conflict resourceutil.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Equal(t, "agents", conflict.Details[0].Resource)
}

func TestDeleteModel_RemovesWhenUnused(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := t.Context()
	project := "demo"
	_, err := NewUpsert(
		store,
	).Execute(ctx, &UpsertInput{Project: project, ID: "openai:gpt-4o-mini", Body: map[string]any{"provider": "openai", "model": "gpt-4o-mini"}})
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "openai:gpt-4o-mini"})
	assert.NoError(t, err)
	_, getErr := NewGet(store).Execute(ctx, &GetInput{Project: project, ID: "openai:gpt-4o-mini"})
	assert.Error(t, getErr)
	assert.True(t, errors.Is(getErr, ErrNotFound))
}
