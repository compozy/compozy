package project

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/require"
)

func TestProject_IndexToResourceStore(t *testing.T) {
	ctx := context.Background()
	store := resources.NewMemoryResourceStore()
	p := &Config{
		Name:    "demo",
		Tools:   []tool.Config{{ID: "fmt", Description: "format"}},
		Schemas: []schema.Schema{{"id": "input_schema", "type": "object"}},
	}

	require.NoError(t, p.IndexToResourceStore(ctx, store))

	// Tool retrievable
	v, _, err := store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceTool, ID: "fmt"})
	require.NoError(t, err)
	require.NotNil(t, v)

	// Schema retrievable
	v2, _, err := store.Get(
		ctx,
		resources.ResourceKey{Project: "demo", Type: resources.ResourceSchema, ID: "input_schema"},
	)
	require.NoError(t, err)
	require.NotNil(t, v2)
}
