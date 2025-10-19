package embedder

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/embeddings"

	memoryembeddings "github.com/compozy/compozy/engine/memory/embeddings"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func newTestContext(t *testing.T) context.Context {
	ctx := t.Context()
	ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, manager.Close(t.Context()))
	})
	ctx = config.ContextWithManager(ctx, manager)
	return ctx
}

func TestAdapter_EmbedDocuments(t *testing.T) {
	t.Run("ShouldBatchInputsAccordingToConfig", func(t *testing.T) {
		ctx := newTestContext(t)
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
		ctx := newTestContext(t)
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
		ctx := newTestContext(t)
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

func TestAdapter_Cache(t *testing.T) {
	newCachedAdapter := func(t *testing.T) (context.Context, *Adapter, *fakeClient) {
		t.Helper()
		ctx := newTestContext(t)
		client := &fakeClient{}
		impl, err := embeddings.NewEmbedder(client, embeddings.WithBatchSize(4))
		require.NoError(t, err)
		cfg := &Config{
			ID:        "cache",
			Provider:  ProviderLocal,
			Model:     "stub",
			Dimension: 4,
			BatchSize: 4,
		}
		adapter, err := Wrap(cfg, impl)
		require.NoError(t, err)
		require.NoError(t, adapter.EnableCache(16))
		return ctx, adapter, client
	}

	t.Run("Should cache document embeddings and avoid redundant calls", func(t *testing.T) {
		ctx, adapter, client := newCachedAdapter(t)

		_, err := adapter.EmbedDocuments(ctx, []string{"alpha", "beta"})
		require.NoError(t, err)
		assert.Equal(t, 1, client.callCount)

		client.callCount = 0
		_, err = adapter.EmbedDocuments(ctx, []string{"beta", "alpha"})
		require.NoError(t, err)
		assert.Equal(t, 0, client.callCount)
	})

	t.Run("Should protect cached document vectors from mutation", func(t *testing.T) {
		ctx, adapter, client := newCachedAdapter(t)

		_, err := adapter.EmbedDocuments(ctx, []string{"alpha", "beta"})
		require.NoError(t, err)

		client.callCount = 0
		vectors, err := adapter.EmbedDocuments(ctx, []string{"beta", "alpha"})
		require.NoError(t, err)
		require.Len(t, vectors, 2)
		assert.Equal(t, 0, client.callCount)

		vectors[0][0] = 999 // mutate returned slice

		next, err := adapter.EmbedDocuments(ctx, []string{"beta"})
		require.NoError(t, err)
		require.Len(t, next, 1)
		assert.Equal(t, float32(len("beta")), next[0][0])
	})

	t.Run("Should cache query embeddings independently", func(t *testing.T) {
		ctx, adapter, client := newCachedAdapter(t)

		vec, err := adapter.EmbedQuery(ctx, "gamma")
		require.NoError(t, err)
		require.Len(t, vec, 1)
		assert.Equal(t, 1, client.callCount)

		client.callCount = 0
		cached, err := adapter.EmbedQuery(ctx, "gamma")
		require.NoError(t, err)
		require.Len(t, cached, 1)
		assert.Equal(t, 0, client.callCount)
		cached[0] = 77

		recomputed, err := adapter.EmbedQuery(ctx, "gamma")
		require.NoError(t, err)
		require.Len(t, recomputed, 1)
		assert.Equal(t, float32(len("gamma")), recomputed[0])
	})
}

func TestCategorizeError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected memoryembeddings.ErrorType
	}{
		{
			name:     "NilError",
			err:      nil,
			expected: memoryembeddings.ErrorTypeServerError,
		},
		{
			name:     "ContextDeadline",
			err:      context.DeadlineExceeded,
			expected: memoryembeddings.ErrorTypeRateLimit,
		},
		{
			name:     "RateLimitMessage",
			err:      errors.New("rate limit exceeded"),
			expected: memoryembeddings.ErrorTypeRateLimit,
		},
		{
			name:     "Status429",
			err:      errors.New("http 429 too many requests"),
			expected: memoryembeddings.ErrorTypeRateLimit,
		},
		{
			name:     "AuthFailure",
			err:      errors.New("unauthorized"),
			expected: memoryembeddings.ErrorTypeAuth,
		},
		{
			name:     "InvalidInput",
			err:      errors.New("bad request: invalid value"),
			expected: memoryembeddings.ErrorTypeInvalidInput,
		},
		{
			name:     "ServerErrorFallback",
			err:      errors.New("internal server error"),
			expected: memoryembeddings.ErrorTypeServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, categorizeError(tc.err))
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("ShouldReturnErrorForUnsupportedProvider", func(t *testing.T) {
		ctx := newTestContext(t)
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
		ctx := newTestContext(t)
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
