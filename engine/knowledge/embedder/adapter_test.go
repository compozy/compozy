package embedder

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/embeddings"

	"github.com/compozy/compozy/test/helpers"
)

func TestAdapter_EmbedDocuments(t *testing.T) {
	t.Run("ShouldBatchInputsAccordingToConfig", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		client := &fakeClient{}
		impl, err := embeddings.NewEmbedder(client, embeddings.WithBatchSize(2), embeddings.WithStripNewLines(true))
		require.NoError(t, err)
		cfg := &Config{
			ID:            "embedder-test",
			Provider:      ProviderLocal,
			Model:         "stub",
			Dimension:     5,
			BatchSize:     2,
			StripNewLines: true,
		}
		adapter, err := Wrap(cfg, impl)
		require.NoError(t, err)

		documents := []string{"first\nchunk", "second", "third"}
		vectors, err := adapter.EmbedDocuments(ctx, documents)
		require.NoError(t, err)

		require.Len(t, client.batches, 2)
		assert.Equal(t, []string{"first chunk", "second"}, client.batches[0])
		assert.Equal(t, []string{"third"}, client.batches[1])
		require.Len(t, vectors, len(documents))
	})

	t.Run("ShouldWrapProviderErrors", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		client := &fakeClient{failAfter: 1}
		impl, err := embeddings.NewEmbedder(client, embeddings.WithBatchSize(1))
		require.NoError(t, err)
		cfg := &Config{
			ID:            "failing",
			Provider:      ProviderLocal,
			Model:         "stub",
			Dimension:     4,
			BatchSize:     1,
			StripNewLines: false,
		}
		adapter, err := Wrap(cfg, impl)
		require.NoError(t, err)

		_, err = adapter.EmbedDocuments(ctx, []string{"first", "second"})
		require.Error(t, err)
		assert.ErrorContains(t, err, `embedder "failing"`)
	})

	t.Run("ShouldEmbedQueryViaUnderlyingClient", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		client := &fakeClient{}
		impl, err := embeddings.NewEmbedder(client)
		require.NoError(t, err)
		cfg := &Config{
			ID:            "query",
			Provider:      ProviderLocal,
			Model:         "stub",
			Dimension:     4,
			BatchSize:     4,
			StripNewLines: false,
		}
		adapter, err := Wrap(cfg, impl)
		require.NoError(t, err)

		vector, err := adapter.EmbedQuery(ctx, "hello")
		require.NoError(t, err)
		assert.Equal(t, []float32{5}, vector)
	})
}

func TestNew(t *testing.T) {
	t.Run("ShouldReturnErrorForUnsupportedProvider", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		cfg := &Config{
			ID:            "unknown",
			Provider:      Provider("azure"),
			Model:         "model",
			Dimension:     10,
			BatchSize:     5,
			StripNewLines: true,
		}
		_, err := New(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "provider")
	})

	t.Run("ShouldValidateDimension", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		cfg := &Config{
			ID:            "bad",
			Provider:      ProviderLocal,
			Model:         "stub",
			Dimension:     0,
			BatchSize:     1,
			StripNewLines: true,
		}
		_, err := New(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "dimension")
	})
}

type fakeClient struct {
	batches   [][]string
	failAfter int
	callCount int
}

func (f *fakeClient) CreateEmbedding(_ context.Context, texts []string) ([][]float32, error) {
	f.batches = append(f.batches, append([]string(nil), texts...))
	f.callCount++
	if f.failAfter > 0 && f.callCount > f.failAfter {
		return nil, errors.New("provider failure")
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{float32(len(texts[i]))}
	}
	return result, nil
}
