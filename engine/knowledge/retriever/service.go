package retriever

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	tracer    trace.Tracer
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
	return &Service{
		embedder:  emb,
		store:     store,
		estimator: estimator,
		tracer:    otel.Tracer("compozy.knowledge.retriever"),
	}, nil
}

func (s *Service) Retrieve(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	query string,
) (contexts []knowledge.RetrievedContext, err error) {
	if inputErr := validateRetrieveInput(binding, query); inputErr != nil {
		err = inputErr
		return nil, err
	}
	log := logger.FromContext(ctx).With(
		"kb_id", binding.KnowledgeBase.ID,
		"binding_id", binding.ID,
	)
	start := time.Now()
	ctx, span := s.tracer.Start(ctx, "compozy.knowledge.retriever.retrieve", trace.WithAttributes(
		attribute.String("kb_id", binding.KnowledgeBase.ID),
		attribute.String("binding_id", binding.ID),
	))
	defer s.finishRetrieve(ctx, binding, span, start, &contexts, &err)

	log.Info("Knowledge retrieval started", "query_length", len(query))
	vector, err := s.embedQueryWithSpan(ctx, binding, query)
	if err != nil {
		return nil, err
	}
	matches, err := s.searchMatches(ctx, binding, vector, s.buildSearchOptions(binding))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sortMatches(matches)
	contexts = s.buildContexts(ctx, binding, matches)
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

func validateRetrieveInput(binding *knowledge.ResolvedBinding, query string) error {
	if binding == nil {
		return errors.New("knowledge: binding is required for retrieval")
	}
	if strings.TrimSpace(query) == "" {
		return errors.New("knowledge: query is required")
	}
	return nil
}

func (s *Service) embedQueryWithSpan(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	query string,
) ([]float32, error) {
	spanCtx, span := s.tracer.Start(ctx, "compozy.knowledge.retriever.embed_query", trace.WithAttributes(
		attribute.String("kb_id", binding.KnowledgeBase.ID),
		attribute.String("binding_id", binding.ID),
		attribute.String("embedder_id", binding.Embedder.ID),
		attribute.String("embedder_provider", binding.Embedder.Provider),
		attribute.String("embedder_model", binding.Embedder.Model),
	))
	defer span.End()
	vector, err := s.embedder.EmbedQuery(spanCtx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return vector, nil
}

func (s *Service) searchMatches(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	vector []float32,
	opts vectordb.SearchOptions,
) ([]vectordb.Match, error) {
	spanCtx, span := s.tracer.Start(ctx, "compozy.knowledge.retriever.vector_search", trace.WithAttributes(
		attribute.String("kb_id", binding.KnowledgeBase.ID),
		attribute.String("binding_id", binding.ID),
		attribute.String("vector_id", binding.Vector.ID),
		attribute.String("vector_type", string(binding.Vector.Type)),
		attribute.Int("top_k", opts.TopK),
	))
	defer span.End()
	matches, err := s.store.Search(spanCtx, vector, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("matches", len(matches)))
	return matches, nil
}

func (s *Service) buildSearchOptions(binding *knowledge.ResolvedBinding) vectordb.SearchOptions {
	opts := vectordb.SearchOptions{
		TopK:     binding.Retrieval.TopK,
		MinScore: binding.Retrieval.MinScoreValue(),
		Filters:  core.CloneMap(binding.Retrieval.Filters),
	}
	if opts.TopK <= 0 {
		opts.TopK = knowledge.DefaultDefaults().RetrievalTopK
	}
	return opts
}

func (s *Service) buildContexts(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	matches []vectordb.Match,
) []knowledge.RetrievedContext {
	if len(matches) == 0 {
		return nil
	}
	contexts := make([]knowledge.RetrievedContext, len(matches))
	tokenCounts := make([]int, len(matches))
	totalTokens := 0
	for i := range matches {
		metadata := core.CloneMap(matches[i].Metadata)
		tokens := s.estimator.EstimateTokens(ctx, matches[i].Text)
		totalTokens += tokens
		tokenCounts[i] = tokens
		contexts[i] = knowledge.RetrievedContext{
			BindingID:     binding.ID,
			Content:       matches[i].Text,
			Score:         matches[i].Score,
			TokenEstimate: tokens,
			Metadata:      metadata,
		}
	}
	return trimContexts(binding, contexts, tokenCounts, totalTokens)
}

func trimContexts(
	binding *knowledge.ResolvedBinding,
	contexts []knowledge.RetrievedContext,
	tokenCounts []int,
	totalTokens int,
) []knowledge.RetrievedContext {
	maxTokens := binding.Retrieval.MaxTokens
	if maxTokens <= 0 {
		return contexts
	}
	for totalTokens > maxTokens && len(contexts) > 0 {
		last := len(contexts) - 1
		totalTokens -= tokenCounts[last]
		contexts = contexts[:last]
		tokenCounts = tokenCounts[:last]
	}
	return contexts
}

func (s *Service) finishRetrieve(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	span trace.Span,
	start time.Time,
	contexts *[]knowledge.RetrievedContext,
	runErr *error,
) {
	duration := time.Since(start)
	knowledge.RecordQueryLatency(ctx, binding.KnowledgeBase.ID, duration)
	log := logger.FromContext(ctx).With(
		"kb_id", binding.KnowledgeBase.ID,
		"binding_id", binding.ID,
	)
	seconds := duration.Seconds()
	if runErr != nil && *runErr != nil {
		err := *runErr
		log.Error("Knowledge retrieval failed", "error", err, "duration_seconds", seconds)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return
	}
	total := 0
	if contexts != nil && *contexts != nil {
		total = len(*contexts)
	}
	log.Info("Knowledge retrieval finished", "results", total, "duration_seconds", seconds)
	span.SetAttributes(attribute.Int("results", total))
	span.End()
}

func sortMatches(matches []vectordb.Match) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Score > matches[j].Score
	})
}
