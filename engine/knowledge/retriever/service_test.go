package retriever_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/retriever"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
)

type stubEmbedder struct {
	fail bool
}

func (s *stubEmbedder) EmbedDocuments(context.Context, []string) ([][]float32, error) {
	return nil, errors.New("not implemented")
}

func (s *stubEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
	if s.fail {
		return nil, errors.New("embed query failed")
	}
	return []float32{1, 0, 0}, nil
}

type stubStore struct {
	matches []vectordb.Match
}

func (s *stubStore) Upsert(context.Context, []vectordb.Record) error {
	return nil
}

func (s *stubStore) Search(_ context.Context, _ []float32, opts vectordb.SearchOptions) ([]vectordb.Match, error) {
	filtered := make([]vectordb.Match, 0, len(s.matches))
	for i := range s.matches {
		if s.matches[i].Score < opts.MinScore {
			continue
		}
		filtered = append(filtered, s.matches[i])
	}
	if opts.TopK > 0 && len(filtered) > opts.TopK {
		filtered = filtered[:opts.TopK]
	}
	return append([]vectordb.Match(nil), filtered...), nil
}

func (s *stubStore) Delete(context.Context, vectordb.Filter) error {
	return nil
}

func (s *stubStore) Close(context.Context) error {
	return nil
}

type fixedEstimator struct {
	values []int
}

func (f *fixedEstimator) EstimateTokens(_ context.Context, _ string) int {
	if len(f.values) == 0 {
		return 0
	}
	val := f.values[0]
	f.values = f.values[1:]
	return val
}

func TestService_ShouldRespectTopKMinScoreAndOrdering(t *testing.T) {
	store := &stubStore{
		matches: []vectordb.Match{
			{ID: "c", Score: 0.45, Text: "third", Metadata: map[string]any{"source": "c"}},
			{ID: "a", Score: 0.72, Text: "first", Metadata: map[string]any{"source": "a"}},
			{ID: "b", Score: 0.72, Text: "second", Metadata: map[string]any{"source": "b"}},
			{ID: "d", Score: 0.30, Text: "low"},
		},
	}
	service, err := retriever.NewService(&stubEmbedder{}, store, nil)
	require.NoError(t, err)
	binding := &knowledge.ResolvedBinding{
		ID: "binding",
		KnowledgeBase: knowledge.BaseConfig{
			ID: "kb",
		},
		Retrieval: knowledge.RetrievalConfig{
			TopK:     3,
			MinScore: 0.4,
		},
	}
	ctx := context.Background()
	results, err := service.Retrieve(ctx, binding, "query")
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "a", results[0].Metadata["source"])
	assert.Equal(t, "b", results[1].Metadata["source"])
	assert.Equal(t, "c", results[2].Metadata["source"])
	assert.Equal(t, "binding", results[0].BindingID)
	assert.Equal(t, "binding", results[1].BindingID)
	assert.Equal(t, "binding", results[2].BindingID)
	assert.True(t, results[0].Score >= results[1].Score)
	assert.True(t, results[0].TokenEstimate >= 1)
}

func TestService_ShouldTrimByMaxTokens(t *testing.T) {
	store := &stubStore{
		matches: []vectordb.Match{
			{ID: "a", Score: 0.9, Text: "alpha"},
			{ID: "b", Score: 0.8, Text: "beta"},
			{ID: "c", Score: 0.7, Text: "gamma"},
		},
	}
	estimator := &fixedEstimator{values: []int{120, 80, 60}}
	service, err := retriever.NewService(&stubEmbedder{}, store, estimator)
	require.NoError(t, err)
	binding := &knowledge.ResolvedBinding{
		ID: "binding",
		KnowledgeBase: knowledge.BaseConfig{
			ID: "kb",
		},
		Retrieval: knowledge.RetrievalConfig{
			TopK:      3,
			MinScore:  0.1,
			MaxTokens: 220,
		},
	}
	ctx := context.Background()
	results, err := service.Retrieve(ctx, binding, "query")
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "binding", results[0].BindingID)
	assert.Equal(t, "binding", results[1].BindingID)
	total := results[0].TokenEstimate + results[1].TokenEstimate
	assert.LessOrEqual(t, total, binding.Retrieval.MaxTokens)
}
