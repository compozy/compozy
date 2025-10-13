package uc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

// newContext returns a context carrying test logger and configuration manager.
func newContext(t *testing.T) context.Context {
	t.Helper()
	return testhelpers.NewTestContext(t)
}

// putResource stores a value in the given ResourceStore asserting the operation succeeds.
func putResource(t *testing.T, store resources.ResourceStore, key resources.ResourceKey, value any) {
	t.Helper()
	ctx := newContext(t)
	_, err := store.Put(ctx, key, value)
	require.NoError(t, err)
}

// stubKnowledgeTriple inserts a knowledge base, embedder, and vector configs pointing to each other.
func stubKnowledgeTriple(
	t *testing.T,
	store resources.ResourceStore,
	projectID string,
	base *knowledge.BaseConfig,
	emb *knowledge.EmbedderConfig,
	vec *knowledge.VectorDBConfig,
) {
	t.Helper()
	putResource(t, store, resources.ResourceKey{Project: projectID, Type: resources.ResourceEmbedder, ID: emb.ID}, emb)
	putResource(t, store, resources.ResourceKey{Project: projectID, Type: resources.ResourceVectorDB, ID: vec.ID}, vec)
	putResource(
		t,
		store,
		resources.ResourceKey{Project: projectID, Type: resources.ResourceKnowledgeBase, ID: base.ID},
		base,
	)
}
