// Package otlptest runs an in-process OTLP/HTTP collector for tests that export through the daemon's real telemetry
// runtime: the providers telemetry.Init installs, the sanitizing exporters, and the OTLP/HTTP exporter.
package otlptest

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

const flushTimeout = 10 * time.Second

// Span is one exported span as the collector decoded it.
type Span struct {
	Name         string
	Scope        string
	Kind         tracepb.Span_SpanKind
	TraceID      trace.TraceID
	SpanID       trace.SpanID
	ParentSpanID trace.SpanID
	Start        time.Time
	End          time.Time
	// Attributes holds string, int64, bool, and float64 values by key.
	Attributes    map[string]any
	Events        []Event
	Links         []trace.SpanContext
	StatusCode    tracepb.Status_StatusCode
	StatusMessage string
}

// Event is one exported span event.
type Event struct {
	Name string
	Time time.Time
}

// HasEvent reports whether the span carries an event named name.
func (s Span) HasEvent(name string) bool {
	return slices.ContainsFunc(s.Events, func(event Event) bool { return event.Name == name })
}

// Signal request paths of the OTLP/HTTP protocol; Payloads also files gRPC requests under them.
const (
	TracesPath  = "/v1/traces"
	MetricsPath = "/v1/metrics"
)

// Collector is an OTLP collector that keeps every request body and span it receives and the latest export of each
// metric.
type Collector struct {
	URL string

	mu       sync.Mutex
	payloads map[string][][]byte
	spans    []Span
	metrics  map[string]Metric
}

// NewCollector starts a collector that stops at test cleanup.
func NewCollector(t testing.TB) *Collector {
	t.Helper()
	collector := &Collector{}
	server := httptest.NewServer(http.HandlerFunc(collector.serve))
	t.Cleanup(server.Close)
	collector.URL = server.URL
	return collector
}

func (c *Collector) serve(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	c.record(req.URL.Path, body)
	if req.URL.Path == TracesPath {
		var export coltracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &export); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		c.keep(&export)
	}
	if req.URL.Path == MetricsPath {
		var export colmetricpb.ExportMetricsServiceRequest
		if err := proto.Unmarshal(body, &export); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		c.keepMetrics(&export)
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
}

func (c *Collector) keep(export *coltracepb.ExportTraceServiceRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, resourceSpans := range export.GetResourceSpans() {
		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			for _, span := range scopeSpans.GetSpans() {
				c.spans = append(c.spans, decodeSpan(scopeSpans.GetScope().GetName(), span))
			}
		}
	}
}

func (c *Collector) record(path string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.payloads == nil {
		c.payloads = make(map[string][][]byte)
	}
	c.payloads[path] = append(c.payloads[path], slices.Clone(body))
}

// Payloads returns the raw serialized export requests received so far for a signal path (TracesPath or MetricsPath):
// the HTTP request bodies, or the gRPC request message bytes.
func (c *Collector) Payloads(path string) [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.payloads[path])
}

// Spans returns every span received so far.
func (c *Collector) Spans() []Span {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.spans)
}

// Export starts the daemon telemetry runtime against the collector with the global OTel providers, the way the
// daemon boots it, and restores disabled export at cleanup. It installs process globals: callers must not run in
// parallel.
func (c *Collector) Export(t testing.TB, sampleRatio float64, reg telemetry.Registries) {
	t.Helper()
	c.export(t, sampleRatio, false, reg)
}

// ExportWithMetrics is Export with metric export on; FlushMetrics collects on demand.
func (c *Collector) ExportWithMetrics(t testing.TB, sampleRatio float64, reg telemetry.Registries) {
	t.Helper()
	c.export(t, sampleRatio, true, reg)
}

func (c *Collector) export(t testing.TB, sampleRatio float64, metrics bool, reg telemetry.Registries) {
	t.Helper()
	cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{}).Observability
	cfg.Enabled = true
	cfg.OTel.Enabled = true
	cfg.OTel.Endpoint = c.URL
	cfg.OTel.Protocol = "http/protobuf"
	cfg.OTel.SampleRatio = sampleRatio
	cfg.OTel.Metrics = metrics
	runtime := initTelemetry(t, cfg, reg)
	t.Cleanup(func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Errorf("telemetry Runtime.Shutdown() error = %v", err)
		}
		cfg.OTel.Enabled = false
		initTelemetry(t, cfg, reg)
	})
}

// Flush exports every span ended so far through the installed provider and returns everything received.
func (c *Collector) Flush(t testing.TB) []Span {
	t.Helper()
	provider, ok := otel.GetTracerProvider().(interface{ ForceFlush(context.Context) error })
	if !ok {
		t.Fatalf("global tracer provider %T cannot flush; call Export first", otel.GetTracerProvider())
	}
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatalf("TracerProvider.ForceFlush() error = %v", err)
	}
	return c.Spans()
}

func initTelemetry(t testing.TB, cfg compozyconfig.ObservabilityConfig, reg telemetry.Registries) telemetry.Runtime {
	t.Helper()
	runtime, err := telemetry.Init(
		context.Background(),
		cfg,
		nil,
		reg,
		version.Info{Version: "test"},
		slog.New(slog.DiscardHandler),
	)
	if err != nil {
		t.Fatalf("telemetry.Init() error = %v", err)
	}
	return runtime
}

func decodeSpan(scope string, span *tracepb.Span) Span {
	decoded := Span{
		Name:          span.GetName(),
		Scope:         scope,
		Kind:          span.GetKind(),
		TraceID:       trace.TraceID(span.GetTraceId()),
		SpanID:        trace.SpanID(span.GetSpanId()),
		Start:         unixNano(span.GetStartTimeUnixNano()),
		End:           unixNano(span.GetEndTimeUnixNano()),
		Attributes:    decodeAttributes(span.GetAttributes()),
		StatusCode:    span.GetStatus().GetCode(),
		StatusMessage: span.GetStatus().GetMessage(),
	}
	if parent := span.GetParentSpanId(); len(parent) == len(decoded.ParentSpanID) {
		decoded.ParentSpanID = trace.SpanID(parent)
	}
	for _, event := range span.GetEvents() {
		decoded.Events = append(decoded.Events, Event{Name: event.GetName(), Time: unixNano(event.GetTimeUnixNano())})
	}
	for _, link := range span.GetLinks() {
		decoded.Links = append(decoded.Links, trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: trace.TraceID(link.GetTraceId()),
			SpanID:  trace.SpanID(link.GetSpanId()),
		}))
	}
	return decoded
}

func decodeAttributes(attrs []*commonpb.KeyValue) map[string]any {
	decoded := make(map[string]any, len(attrs))
	for _, kv := range attrs {
		switch value := kv.GetValue().GetValue().(type) {
		case *commonpb.AnyValue_StringValue:
			decoded[kv.GetKey()] = value.StringValue
		case *commonpb.AnyValue_IntValue:
			decoded[kv.GetKey()] = value.IntValue
		case *commonpb.AnyValue_BoolValue:
			decoded[kv.GetKey()] = value.BoolValue
		case *commonpb.AnyValue_DoubleValue:
			decoded[kv.GetKey()] = value.DoubleValue
		default:
			decoded[kv.GetKey()] = kv.GetValue().String()
		}
	}
	return decoded
}

func unixNano(ns uint64) time.Time {
	return time.Unix(0, int64(ns)) //nolint:gosec // OTLP timestamps are nanoseconds since the Unix epoch.
}
