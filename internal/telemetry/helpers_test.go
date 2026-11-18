package telemetry_test

import (
	"context"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session/sessionid"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/workspace"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const (
	testServiceVersion = "1.2.3"
	testInstanceID     = "boot-1"
	testRoute          = "/api/workspaces/:workspace_id/sessions/:session_id/prompt"
	testACPMethod      = "session/prompt"
	testHookEvent      = "prompt.pre_submit"
	testTool           = "compozy__session_list"
	testProvider       = "claude"
	testAgent          = "general"
	testCatalogModel   = "org/model"
)

// testConfiguredModels exist only in user configuration: paths and free text.
var testConfiguredModels = []string{
	"/home/ana/private/model.gguf",
	"./private/model.gguf",
	"../private/model.gguf",
	"C:/Users/ana/model.gguf",
	"my model v2",
}

// registeredScopes is every instrumentation scope constant; the sanitizing exporters must keep each name.
var registeredScopes = []string{
	telemetry.ScopeHTTP, telemetry.ScopeSession, telemetry.ScopeACP, telemetry.ScopeMCP, telemetry.ScopeTools,
	telemetry.ScopeStore, telemetry.ScopeObserve, telemetry.ScopeHooks, telemetry.ScopeLoop,
	telemetry.ScopeAutomation, telemetry.ScopeScheduler,
}

func members(values ...string) func(string) bool {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return func(value string) bool {
		_, ok := set[value]
		return ok
	}
}

// ownerIdentityValidators wires each identity key to its owning package's validator.
func ownerIdentityValidators() map[attribute.Key]func(string) bool {
	return map[attribute.Key]func(string) bool{
		telemetry.AttrProfileID:    profile.IsProfileID,
		telemetry.AttrWorkspaceID:  workspace.IsWorkspaceIdentifier,
		telemetry.AttrSessionID:    sessionid.IsSessionID,
		telemetry.AttrTurnID:       sessionid.IsTurnID,
		telemetry.AttrRunID:        sessionid.IsRunID,
		telemetry.AttrLoopID:       loop.IsLoopID,
		telemetry.AttrAutomationID: automation.IsAutomationID,
	}
}

func testRegistries(agents []string, catalogModels ...string) telemetry.Registries {
	catalog := members(append([]string{testCatalogModel}, catalogModels...)...)
	configured := members(testConfiguredModels...)
	return telemetry.Registries{
		RouteTemplates: []string{testRoute},
		ACPMethods:     []string{testACPMethod},
		HookEvents:     []string{testHookEvent},
		ToolIDs:        members(testTool),
		Providers:      members(testProvider),
		Agents:         members(append([]string{testAgent}, agents...)...),
		Models: func(_, model string) telemetry.ModelClass {
			switch {
			case catalog(model):
				return telemetry.ModelCatalog
			case configured(model):
				return telemetry.ModelCustom
			default:
				return telemetry.ModelUnknown
			}
		},
		IDs: ownerIdentityValidators(),
	}
}

func newTestResource() *resource.Resource {
	return telemetry.NewResource("compozy-daemon", testServiceVersion, testInstanceID)
}

// newTracerProvider exports synchronously through the real sanitizing wrapper into memory.
func newTracerProvider(
	t *testing.T,
	reg telemetry.Registries,
	options ...sdktrace.TracerProviderOption,
) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter) {
	t.Helper()
	exported := tracetest.NewInMemoryExporter()
	var exporter sdktrace.SpanExporter = telemetry.NewSanitizingSpanExporter(
		exported,
		telemetry.NewSanitizer(reg, newTestResource(), testServiceVersion),
	)
	provider := sdktrace.NewTracerProvider(append(options, sdktrace.WithSyncer(exporter))...)
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("TracerProvider.Shutdown() error = %v", err)
		}
	})
	return provider, exported
}

func onlySpan(t *testing.T, exported *tracetest.InMemoryExporter) tracetest.SpanStub {
	t.Helper()
	spans := exported.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	return spans[0]
}

func attributeMap(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	values := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, kv := range attrs {
		values[kv.Key] = kv.Value
	}
	return values
}

// recordingMetricExporter keeps the last collection the sanitizing wrapper forwarded.
type recordingMetricExporter struct {
	mu   sync.Mutex
	last *metricdata.ResourceMetrics
}

func (*recordingMetricExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return sdkmetric.DefaultTemporalitySelector(kind)
}

func (*recordingMetricExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(kind)
}

func (e *recordingMetricExporter) Export(_ context.Context, metrics *metricdata.ResourceMetrics) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.last = metrics
	return nil
}

func (*recordingMetricExporter) ForceFlush(context.Context) error { return nil }

func (*recordingMetricExporter) Shutdown(context.Context) error { return nil }

func (e *recordingMetricExporter) lastExport() *metricdata.ResourceMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.last
}

// metricHarness places a ManualReader collection behind the real sanitizing metric wrapper.
type metricHarness struct {
	provider *sdkmetric.MeterProvider
	reader   *sdkmetric.ManualReader
	exporter sdkmetric.Exporter
	recorded *recordingMetricExporter
}

func newMetricHarness(t *testing.T, reg telemetry.Registries) *metricHarness {
	t.Helper()
	res := newTestResource()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(append(telemetry.MeterProviderOptions(res), sdkmetric.WithReader(reader))...)
	recorded := &recordingMetricExporter{}
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("MeterProvider.Shutdown() error = %v", err)
		}
	})
	return &metricHarness{
		provider: provider,
		reader:   reader,
		exporter: telemetry.NewSanitizingMetricExporter(recorded, telemetry.NewSanitizer(reg, res, testServiceVersion)),
		recorded: recorded,
	}
}

// collect returns the raw SDK collection and what the sanitizing wrapper forwarded for it.
func (h *metricHarness) collect(t *testing.T) (metricdata.ResourceMetrics, *metricdata.ResourceMetrics) {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := h.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("ManualReader.Collect() error = %v", err)
	}
	if err := h.exporter.Export(context.Background(), &collected); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	return collected, h.recorded.lastExport()
}

func findMetric(t *testing.T, metrics *metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()
	for _, scopeMetrics := range metrics.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			if metric.Name == name {
				return metric
			}
		}
	}
	t.Fatalf("metric %q not exported", name)
	return metricdata.Metrics{}
}
