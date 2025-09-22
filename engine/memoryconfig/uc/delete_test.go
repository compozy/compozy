package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMemoryBody(id string) map[string]any {
	return map[string]any{
		"resource":    "memory",
		"id":          id,
		"type":        "buffer",
		"persistence": map[string]any{"type": "in_memory"},
	}
}

func TestDeleteMemory_ConfigConflictsWhenAgentReferences(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	memBody := createMemoryBody("session")
	_, err := NewUpsert(store).Execute(ctx, &UpsertInput{Project: project, ID: "session", Body: memBody})
	require.NoError(t, err)
	ag := &agent.Config{ID: "assistant"}
	ag.Memory = []core.MemoryReference{{ID: "session"}}
	_, err = store.Put(ctx, resources.ResourceKey{Project: project, Type: resources.ResourceAgent, ID: ag.ID}, ag)
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "session"})
	require.Error(t, err)
	var conflict resourceutil.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Equal(t, "agents", conflict.Details[0].Resource)
}

func TestDeleteMemory_ConfigRemovesWhenUnused(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	ctx := context.Background()
	project := "demo"
	_, err := NewUpsert(
		store,
	).Execute(ctx, &UpsertInput{Project: project, ID: "session", Body: createMemoryBody("session")})
	require.NoError(t, err)
	err = NewDelete(store).Execute(ctx, &DeleteInput{Project: project, ID: "session"})
	require.NoError(t, err)
	_, getErr := NewGet(store).Execute(ctx, &GetInput{Project: project, ID: "session"})
	assert.Error(t, getErr)
	assert.True(t, errors.Is(getErr, ErrNotFound))
}
