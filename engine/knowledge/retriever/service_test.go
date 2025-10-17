package retriever_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/retriever"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/pkg/logger"
)

func floatPtr(v float64) *float64 {
	return &v
}

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

type logEntry struct {
	level  string
	msg    string
	fields map[string]any
}

type capturingLogger struct {
	entries *[]logEntry
	fields  map[string]any
}

func newCapturingLogger() *capturingLogger {
	return &capturingLogger{
		entries: &[]logEntry{},
		fields:  map[string]any{},
	}
}

func (l *capturingLogger) Info(msg string, keyvals ...any) {
	l.record("info", msg, keyvals...)
}

func (l *capturingLogger) Debug(msg string, keyvals ...any) {
	l.record("debug", msg, keyvals...)
}

func (l *capturingLogger) Warn(msg string, keyvals ...any) {
	l.record("warn", msg, keyvals...)
}

func (l *capturingLogger) Error(msg string, keyvals ...any) {
	l.record("error", msg, keyvals...)
}

func (l *capturingLogger) With(args ...any) logger.Logger {
	nextFields := core.CloneMap(l.fields)
	if nextFields == nil {
		nextFields = make(map[string]any, len(args)/2)
	}
	for i := 0; i < len(args); i += 2 {
		key := fmt.Sprint(args[i])
		var val any
		if i+1 < len(args) {
			val = args[i+1]
		}
		nextFields[key] = val
	}
	return &capturingLogger{
		entries: l.entries,
		fields:  nextFields,
	}
}

func (l *capturingLogger) record(level, msg string, keyvals ...any) {
	if l.entries == nil {
		return
	}
	fields := core.CloneMap(l.fields)
	if fields == nil {
		fields = make(map[string]any, len(keyvals)/2)
	}
	for i := 0; i < len(keyvals); i += 2 {
		key := fmt.Sprint(keyvals[i])
		var val any
		if i+1 < len(keyvals) {
			val = keyvals[i+1]
		}
		fields[key] = val
	}
	*l.entries = append(*l.entries, logEntry{level: level, msg: msg, fields: fields})
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
	t.Run("ShouldRespectTopKMinScoreAndOrdering", func(t *testing.T) {
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
		minScore := 0.4
		binding := &knowledge.ResolvedBinding{
			ID: "binding",
			KnowledgeBase: knowledge.BaseConfig{
				ID: "kb",
			},
			Retrieval: knowledge.RetrievalConfig{
				TopK:     3,
				MinScore: &minScore,
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
	})
}

func TestService_ShouldTrimByMaxTokens(t *testing.T) {
	t.Run("ShouldTrimMatchesByTokenBudget", func(t *testing.T) {
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
		minScore := 0.1
		binding := &knowledge.ResolvedBinding{
			ID: "binding",
			KnowledgeBase: knowledge.BaseConfig{
				ID: "kb",
			},
			Retrieval: knowledge.RetrievalConfig{
				TopK:      3,
				MinScore:  &minScore,
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
	})
}

func TestService_ShouldEmitObservabilitySignals(t *testing.T) {
	knowledge.ResetMetricsForTesting()
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prevMeter := otel.GetMeterProvider()
	otel.SetMeterProvider(meterProvider)
	t.Cleanup(func() {
		require.NoError(t, meterProvider.Shutdown(context.Background()))
		otel.SetMeterProvider(prevMeter)
	})
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	t.Cleanup(func() {
		require.NoError(t, tracerProvider.Shutdown(context.Background()))
		otel.SetTracerProvider(prevTracer)
	})
	store := &stubStore{
		matches: []vectordb.Match{
			{ID: "m1", Score: 0.9, Text: "first"},
			{ID: "m2", Score: 0.8, Text: "second"},
		},
	}
	embed := &stubEmbedder{}
	service, err := retriever.NewService(embed, store, nil)
	require.NoError(t, err)
	binding := &knowledge.ResolvedBinding{
		ID: "binding",
		KnowledgeBase: knowledge.BaseConfig{
			ID: "kb-support",
		},
		Embedder: knowledge.EmbedderConfig{
			ID:       "embedder",
			Provider: "openai",
			Model:    "text-embedding-3-small",
		},
		Vector: knowledge.VectorDBConfig{
			ID:   "vector",
			Type: knowledge.VectorDBTypeFilesystem,
		},
		Retrieval: knowledge.RetrievalConfig{
			TopK:     3,
			MinScore: floatPtr(0.1),
		},
	}
	log := newCapturingLogger()
	ctx := logger.ContextWithLogger(context.Background(), log)
	contexts, err := service.Retrieve(ctx, binding, "observability query text")
	require.NoError(t, err)
	require.Len(t, contexts, 2)
	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)
	foundLatency := false
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "knowledge_query_latency_seconds" {
				continue
			}
			hist, ok := metric.Data.(metricdata.Histogram[float64])
			require.True(t, ok)
			require.Len(t, hist.DataPoints, 1)
			dp := hist.DataPoints[0]
			attrs := attributesToMap(dp.Attributes)
			assert.Equal(t, binding.KnowledgeBase.ID, attrs["kb_id"])
			assert.Greater(t, dp.Sum, 0.0)
			foundLatency = true
		}
	}
	assert.True(t, foundLatency, "expected knowledge_query_latency_seconds metric")
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)
	retrieveSpan := findSpan(t, spans, "compozy.knowledge.retriever.retrieve")
	require.NotNil(t, retrieveSpan)
	embedSpan := findSpan(t, spans, "compozy.knowledge.retriever.embed_query")
	require.NotNil(t, embedSpan)
	embedAttrs := spanAttributesToMap(embedSpan.Attributes())
	assert.Equal(t, binding.Embedder.Provider, embedAttrs["embedder_provider"])
	assert.Equal(t, binding.Embedder.Model, embedAttrs["embedder_model"])
	searchSpan := findSpan(t, spans, "compozy.knowledge.retriever.vector_search")
	require.NotNil(t, searchSpan)
	searchAttrs := spanAttributesToMap(searchSpan.Attributes())
	assert.Equal(t, binding.Vector.ID, searchAttrs["vector_id"])
	assert.Equal(t, string(binding.Vector.Type), searchAttrs["vector_type"])
	require.NotEmpty(t, *log.entries)
	startLog := findLogEntry(*log.entries, "Knowledge retrieval started")
	require.NotNil(t, startLog)
	assert.Equal(t, binding.KnowledgeBase.ID, startLog.fields["kb_id"])
	finishLog := findLogEntry(*log.entries, "Knowledge retrieval finished")
	require.NotNil(t, finishLog)
	assert.Equal(t, binding.KnowledgeBase.ID, finishLog.fields["kb_id"])
}

func attributesToMap(set attribute.Set) map[string]string {
	items := set.ToSlice()
	out := make(map[string]string, len(items))
	for _, item := range items {
		out[string(item.Key)] = item.Value.AsString()
	}
	return out
}

func spanAttributesToMap(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value.AsString()
	}
	return out
}

func findSpan(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range spans {
		if span.Name() == name {
			return span
		}
	}
	return nil
}

func findLogEntry(entries []logEntry, msg string) *logEntry {
	for i := range entries {
		if entries[i].msg == msg {
			return &entries[i]
		}
	}
	return nil
}
