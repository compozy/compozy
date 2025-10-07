package retriever

import (
	"context"
	"errors"
	"sort"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/pkg/logger"
)

type TokenEstimator interface {
	EstimateTokens(ctx context.Context, text string) int
}

type runeEstimator struct{}

func (r runeEstimator) EstimateTokens(_ context.Context, text string) int {
	count := len([]rune(text))
	if count == 0 {
		return 0
	}
	tokens := count / 4
	if tokens == 0 {
		return 1
	}
	return tokens
}

type Service struct {
	embedder  embedder.Embedder
	store     vectordb.Store
	estimator TokenEstimator
}

func NewService(emb embedder.Embedder, store vectordb.Store, estimator TokenEstimator) (*Service, error) {
	if emb == nil {
		return nil, errors.New("knowledge: retriever embedder is required")
	}
	if store == nil {
		return nil, errors.New("knowledge: retriever vector store is required")
	}
	if estimator == nil {
		estimator = runeEstimator{}
	}
	return &Service{embedder: emb, store: store, estimator: estimator}, nil
}

func (s *Service) Retrieve(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	query string,
) ([]knowledge.RetrievedContext, error) {
	if binding == nil {
		return nil, errors.New("knowledge: binding is required for retrieval")
	}
	if query == "" {
		return nil, errors.New("knowledge: query is required")
	}
	log := logger.FromContext(ctx)
	vector, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	opts := vectordb.SearchOptions{
		TopK:     binding.Retrieval.TopK,
		MinScore: binding.Retrieval.MinScore,
		Filters:  cloneStringMap(binding.Retrieval.Filters),
	}
	if opts.TopK <= 0 {
		opts.TopK = knowledge.DefaultDefaults().RetrievalTopK
	}
	matches, err := s.store.Search(ctx, vector, opts)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Score > matches[j].Score
	})
	contexts := make([]knowledge.RetrievedContext, len(matches))
	tokenCounts := make([]int, len(matches))
	totalTokens := 0
	for i := range matches {
		meta := cloneMetadata(matches[i].Metadata)
		tokens := s.estimator.EstimateTokens(ctx, matches[i].Text)
		totalTokens += tokens
		tokenCounts[i] = tokens
		contexts[i] = knowledge.RetrievedContext{
			BindingID:     binding.ID,
			Content:       matches[i].Text,
			Score:         matches[i].Score,
			TokenEstimate: tokens,
			Metadata:      meta,
		}
	}
	maxTokens := binding.Retrieval.MaxTokens
	if maxTokens > 0 {
		for totalTokens > maxTokens && len(contexts) > 0 {
			last := len(contexts) - 1
			totalTokens -= tokenCounts[last]
			contexts = contexts[:last]
			tokenCounts = tokenCounts[:last]
		}
	}
	log.Debug(
		"Knowledge retrieval executed",
		"binding_id",
		binding.ID,
		"kb_id",
		binding.KnowledgeBase.ID,
		"results",
		len(contexts),
	)
	return contexts, nil
}

func cloneMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
