package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/require"
)

func TestCreate_WritesMeta(t *testing.T) {
	ctx := context.Background()
	store := resources.NewMemoryResourceStore()
	uc := NewCreateResource(store)
	out, err := uc.Execute(
		ctx,
		&CreateInput{Project: "p", Type: resources.ResourceAgent, Body: map[string]any{"id": "a", "type": "agent"}},
	)
	require.NoError(t, err)
	require.Equal(t, "a", out.ID)
	v, _, err := store.Get(ctx, resources.ResourceKey{Project: "p", Type: resources.ResourceMeta, ID: "p:agent:a"})
	require.NoError(t, err)
	m := v.(map[string]any)
	require.Equal(t, "api", m["source"].(string))
}

func TestUpsert_WritesMeta(t *testing.T) {
	ctx := context.Background()
	store := resources.NewMemoryResourceStore()
	put := NewUpsertResource(store)
	out, err := put.Execute(
		ctx,
		&UpsertInput{
			Project: "p",
			Type:    resources.ResourceTool,
			ID:      "t",
			Body:    map[string]any{"id": "t", "type": "tool"},
		},
	)
	require.NoError(t, err)
	require.NotEmpty(t, out.ETag)
	v, _, err := store.Get(ctx, resources.ResourceKey{Project: "p", Type: resources.ResourceMeta, ID: "p:tool:t"})
	require.NoError(t, err)
	m := v.(map[string]any)
	require.Equal(t, "api", m["source"].(string))
}
