package telemetry_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// validAttributes carries a value every registered key accepts under testRegistries, plus one unregistered
// key, so an attribute missing from an export was removed by an allowlist and not by its value policy.
func validAttributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		telemetry.AttrProfileID.String(store.DefaultProfileID),
		telemetry.AttrWorkspaceID.String("ws_0123456789abcdef"),
		telemetry.AttrSessionID.String("sess-0123456789abcdef"),
		telemetry.AttrTurnID.String("turn-0123456789abcdef"),
		telemetry.AttrRunID.String("run-0123456789abcdef"),
		telemetry.AttrLoopID.String("looprun-0123456789abcdef"),
		telemetry.AttrAutomationID.String("job-0123456789abcdef"),
		telemetry.AttrAgent.String(testAgent),
		telemetry.AttrGenAISystem.String(testProvider),
		telemetry.AttrGenAIModel.String(testCatalogModel),
		telemetry.AttrTool.String(testTool),
		telemetry.AttrToolRisk.String("read"),
		telemetry.AttrOutcome.String(telemetry.OutcomeOK),
		telemetry.AttrTokenType.String(telemetry.TokenInput),
		telemetry.AttrCostStatus.String(telemetry.CostActual),
		telemetry.AttrDBOperation.String("sessiondb.flush"),
		telemetry.AttrDBCoalesced.Bool(true),
		telemetry.AttrDBFlushes.Int64(4),
		telemetry.AttrDBWriteMSTotal.Int64(120),
		telemetry.AttrHookEvent.String(testHookEvent),
		telemetry.AttrRPCSystem.String(telemetry.RPCSystemJSONRPC),
		telemetry.AttrRPCMethod.String(testACPMethod),
		telemetry.AttrHTTPRoute.String(testRoute),
		telemetry.AttrHTTPMethod.String("POST"),
		telemetry.AttrHTTPStatusCode.Int64(200),
		telemetry.AttrErrorType.String(telemetry.ErrorInternal),
		attribute.String("prompt.text", "SECRET-CANARY-7f3a"),
	}
}

func sortedKeys(attrs []attribute.KeyValue) []attribute.Key {
	keys := make([]attribute.Key, 0, len(attrs))
	for _, kv := range attrs {
		keys = append(keys, kv.Key)
	}
	slices.Sort(keys)
	return keys
}

// Attribute keys by name, so the _dx.md catalog tables below read like the catalog.
const (
	attrAgent       = telemetry.AttrAgent
	attrSystem      = telemetry.AttrGenAISystem
	attrModel       = telemetry.AttrGenAIModel
	attrOutcome     = telemetry.AttrOutcome
	attrErrorType   = telemetry.AttrErrorType
	attrTool        = telemetry.AttrTool
	attrRunID       = telemetry.AttrRunID
	attrDBOperation = telemetry.AttrDBOperation
	attrRPCMethod   = telemetry.AttrRPCMethod
)

// catalogSpanAttributes is the _dx.md span catalog; every registered span may also carry error.type.
var catalogSpanAttributes = []struct {
	name  string
	scope string
	span  string
	want  []attribute.Key
}{
	{
		name: "Should keep only the session.turn catalog attributes", scope: telemetry.ScopeSession,
		span: telemetry.SpanSessionTurn,
		want: []attribute.Key{
			telemetry.AttrSessionID, telemetry.AttrTurnID, attrRunID, telemetry.AttrWorkspaceID,
			telemetry.AttrProfileID, attrAgent, attrSystem, attrModel, attrOutcome, telemetry.AttrDBFlushes,
			telemetry.AttrDBWriteMSTotal, attrErrorType,
		},
	},
	{
		name: "Should keep only the acp.process.spawn catalog attributes", scope: telemetry.ScopeACP,
		span: telemetry.SpanACPProcessSpawn, want: []attribute.Key{attrSystem, attrOutcome, attrErrorType},
	},
	{
		name: "Should keep only the ACP RPC catalog attributes", scope: telemetry.ScopeACP,
		span: "acp " + testACPMethod,
		want: []attribute.Key{telemetry.AttrRPCSystem, attrRPCMethod, attrOutcome, attrErrorType},
	},
	{
		name: "Should keep only the acp.permission.wait catalog attributes", scope: telemetry.ScopeACP,
		span: telemetry.SpanACPPermissionWait, want: []attribute.Key{attrOutcome, attrErrorType},
	},
	{
		name: "Should keep only the mcp.hosted.call catalog attributes", scope: telemetry.ScopeMCP,
		span: telemetry.SpanMCPHostedCall, want: []attribute.Key{attrTool, attrRunID, attrOutcome, attrErrorType},
	},
	{
		name: "Should keep only the tool.execute catalog attributes", scope: telemetry.ScopeTools,
		span: telemetry.SpanToolExecute,
		want: []attribute.Key{attrTool, telemetry.AttrToolRisk, attrOutcome, attrErrorType},
	},
	{
		name: "Should keep only the db.write catalog attributes", scope: telemetry.ScopeStore,
		span: telemetry.SpanDBWrite,
		want: []attribute.Key{attrDBOperation, telemetry.AttrDBCoalesced, attrOutcome, attrErrorType},
	},
	{
		name: "Should keep only the hook.dispatch catalog attributes", scope: telemetry.ScopeHooks,
		span: telemetry.SpanHookDispatch, want: []attribute.Key{telemetry.AttrHookEvent, attrOutcome, attrErrorType},
	},
	{
		name: "Should keep only the loop.run catalog attributes", scope: telemetry.ScopeLoop,
		span: telemetry.SpanLoopRun, want: []attribute.Key{telemetry.AttrLoopID, attrErrorType},
	},
	{
		name: "Should keep only the automation.run catalog attributes", scope: telemetry.ScopeAutomation,
		span: telemetry.SpanAutomationRun, want: []attribute.Key{telemetry.AttrAutomationID, attrErrorType},
	},
	{
		name: "Should keep only the scheduler.tick catalog attributes", scope: telemetry.ScopeScheduler,
		span: telemetry.SpanSchedulerTick, want: []attribute.Key{attrErrorType},
	},
	{
		name: "Should keep only the HTTP route span catalog attributes", scope: telemetry.ScopeHTTP,
		span: "POST " + testRoute,
		want: []attribute.Key{
			telemetry.AttrHTTPRoute, telemetry.AttrHTTPMethod, telemetry.AttrHTTPStatusCode, attrErrorType,
		},
	},
	{
		name: "Should keep only the unmatched HTTP span catalog attributes", scope: telemetry.ScopeHTTP,
		span: "HTTP GET",
		want: []attribute.Key{
			telemetry.AttrHTTPRoute, telemetry.AttrHTTPMethod, telemetry.AttrHTTPStatusCode, attrErrorType,
		},
	},
}

// Per-span-name allowlist (spec: Sanitation boundary, "Span attributes").
func TestSanitizingSpanExporterKeepsOnlyCatalogAttributesPerSpanName(t *testing.T) {
	t.Parallel()

	reg := testRegistries(nil)
	for _, tc := range catalogSpanAttributes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, reg)
			_, span := provider.Tracer(tc.scope).Start(context.Background(), tc.span,
				trace.WithAttributes(validAttributes()...))
			span.End()

			got := onlySpan(t, exported)
			want := slices.Sorted(slices.Values(tc.want))
			if keys := sortedKeys(got.Attributes); !slices.Equal(keys, want) {
				t.Fatalf("%q exported attribute keys = %v, want the catalog keys %v", tc.span, keys, want)
			}
		})
	}
}

// _dx.md metric types; an up-down counter (non-monotonic sum) is none of them.
const (
	typeHistogram = "histogram"
	typeCounter   = "counter"
	typeGauge     = "gauge"
	typeUpDown    = "up-down counter"
)

// catalogInstruments is the _dx.md metric catalog: type, unit, and attribute keys per instrument.
var catalogInstruments = []struct {
	name       string
	instrument string
	kind       string
	unit       string
	want       []attribute.Key
}{
	{
		name: "Should keep only the turn duration dimensions", instrument: telemetry.MetricSessionTurnDuration,
		kind: typeHistogram, unit: "s", want: []attribute.Key{attrSystem, attrModel, attrAgent, attrOutcome},
	},
	{
		name: "Should keep only the time-to-first-token dimensions", instrument: telemetry.MetricSessionTTFT,
		kind: typeHistogram, unit: "s", want: []attribute.Key{attrSystem, attrModel},
	},
	{
		name: "Should keep only the ACP RPC duration dimensions", instrument: telemetry.MetricACPRPCDuration,
		kind: typeHistogram, unit: "s", want: []attribute.Key{attrSystem, attrRPCMethod, attrOutcome},
	},
	{
		name: "Should keep only the tool call duration dimensions", instrument: telemetry.MetricToolCallDuration,
		kind: typeHistogram, unit: "s", want: []attribute.Key{attrTool, attrOutcome},
	},
	{
		name: "Should keep only the HTTP server duration dimensions", instrument: telemetry.MetricHTTPServerDuration,
		kind: typeHistogram, unit: "s",
		want: []attribute.Key{telemetry.AttrHTTPRoute, telemetry.AttrHTTPMethod, telemetry.AttrHTTPStatusCode},
	},
	{
		name: "Should keep only the write duration dimensions", instrument: telemetry.MetricDBWriteDuration,
		kind: typeHistogram, unit: "s", want: []attribute.Key{attrDBOperation, telemetry.AttrDBCoalesced, attrOutcome},
	},
	{
		name: "Should keep only the write admission wait dimensions", instrument: telemetry.MetricDBWriteAdmissionWait,
		kind: typeHistogram, unit: "s", want: []attribute.Key{attrDBOperation},
	},
	{
		name: "Should keep only the token counter dimensions", instrument: telemetry.MetricTokens,
		kind: typeCounter, unit: "{token}",
		want: []attribute.Key{attrSystem, attrModel, attrAgent, telemetry.AttrTokenType},
	},
	{
		name: "Should keep only the cost counter dimensions", instrument: telemetry.MetricCost,
		kind: typeCounter, unit: "{USD}",
		want: []attribute.Key{attrSystem, attrModel, attrAgent, telemetry.AttrCostStatus},
	},
	{
		name: "Should keep only the tool call counter dimensions", instrument: telemetry.MetricToolCalls,
		kind: typeCounter, unit: "{call}", want: []attribute.Key{attrTool, attrOutcome},
	},
	{
		name: "Should keep only the permission counter dimensions", instrument: telemetry.MetricPermissionRequests,
		kind: typeCounter, unit: "{request}", want: []attribute.Key{attrOutcome},
	},
	{
		name: "Should keep only the agent error counter dimensions", instrument: telemetry.MetricAgentErrors,
		kind: typeCounter, unit: "{error}", want: []attribute.Key{attrSystem, attrErrorType},
	},
	{
		name: "Should keep no session gauge dimensions", instrument: telemetry.MetricSessionActive,
		kind: typeGauge, unit: "{session}",
	},
	{
		name: "Should keep no agent gauge dimensions", instrument: telemetry.MetricAgentActive,
		kind: typeGauge, unit: "{agent}",
	},
	{name: "Should keep no uptime gauge dimensions", instrument: telemetry.MetricUptime, kind: typeGauge, unit: "s"},
}

// pointOfType builds a hand-made aggregation of one _dx.md metric type holding one point with attrs.
func pointOfType[N int64 | float64](kind string, attrs attribute.Set) metricdata.Aggregation {
	switch kind {
	case typeHistogram:
		return metricdata.Histogram[N]{
			Temporality: metricdata.CumulativeTemporality,
			DataPoints: []metricdata.HistogramDataPoint[N]{
				{Attributes: attrs, Count: 1, Sum: 1, Bounds: []float64{1}, BucketCounts: []uint64{1, 0}},
			},
		}
	case typeGauge:
		return metricdata.Gauge[N]{DataPoints: []metricdata.DataPoint[N]{{Attributes: attrs, Value: 1}}}
	default:
		return metricdata.Sum[N]{
			Temporality: metricdata.CumulativeTemporality,
			IsMonotonic: kind == typeCounter,
			DataPoints:  []metricdata.DataPoint[N]{{Attributes: attrs, Value: 1}},
		}
	}
}

// Per-instrument allowlist, unit, and description (spec: Sanitation boundary, "Metric attributes"). The input
// is built by hand so only the wrapper, not the SDK view, can be the reason a dimension is missing.
func TestSanitizingMetricExporterKeepsOnlyCatalogDimensionsPerInstrument(t *testing.T) {
	t.Parallel()

	for _, tc := range catalogInstruments {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			harness := newMetricHarness(t, testRegistries(nil))
			input := handBuiltMetrics(metricdata.Metrics{
				Name: tc.instrument, Description: "SECRET-CANARY-7f3a", Unit: "By",
				Data: pointOfType[int64](tc.kind, attribute.NewSet(validAttributes()...)),
			})
			if err := harness.exporter.Export(context.Background(), input); err != nil {
				t.Fatalf("Export() error = %v", err)
			}

			got := findMetric(t, harness.recorded.lastExport(), tc.instrument)
			if got.Description != "" || got.Unit != tc.unit {
				t.Fatalf("%s description %q unit %q, want an empty description and unit %q",
					tc.instrument, got.Description, got.Unit, tc.unit)
			}
			catalogDimensions := slices.DeleteFunc(validAttributes(), func(kv attribute.KeyValue) bool {
				return !slices.Contains(tc.want, kv.Key)
			})
			want := pointOfType[int64](tc.kind, attribute.NewSet(catalogDimensions...))
			if !metricdatatest.AssertAggregationsEqual(t, want, got.Data) {
				t.Fatalf("%s exported data differs from one %s point with the catalog dimensions %v",
					tc.instrument, tc.kind, tc.want)
			}
		})
	}
}

// Per-instrument type (_dx.md Metrics "Type"): a registered instrument reported as another type is dropped,
// whichever number type it carries.
func TestSanitizingMetricExporterKeepsOnlyTheCatalogTypePerInstrument(t *testing.T) {
	t.Parallel()

	for _, tc := range catalogInstruments {
		t.Run(fmt.Sprintf("Should export %s only as a %s", tc.instrument, tc.kind), func(t *testing.T) {
			t.Parallel()

			harness := newMetricHarness(t, testRegistries(nil))
			for _, kind := range []string{typeHistogram, typeCounter, typeUpDown, typeGauge} {
				for _, data := range []metricdata.Aggregation{
					pointOfType[int64](kind, attribute.NewSet()),
					pointOfType[float64](kind, attribute.NewSet()),
				} {
					input := handBuiltMetrics(metricdata.Metrics{Name: tc.instrument, Data: data})
					if err := harness.exporter.Export(context.Background(), input); err != nil {
						t.Fatalf("Export() error = %v", err)
					}
					exported := len(harness.recorded.lastExport().ScopeMetrics) == 1
					if want := kind == tc.kind; exported != want {
						t.Fatalf("%s reported as a %s (%T): exported %t, want %t",
							tc.instrument, kind, data, exported, want)
					}
				}
			}
		})
	}
}

type valuePolicyCase struct {
	name    string
	span    string
	key     attribute.Key
	value   attribute.Value
	want    attribute.Value
	omitted bool
}

func policyCases(span string, key attribute.Key, verb string, values []attribute.Value, build func(
	attribute.Value,
) valuePolicyCase,
) []valuePolicyCase {
	cases := make([]valuePolicyCase, 0, len(values))
	for _, value := range values {
		tc := build(value)
		tc.name = fmt.Sprintf("Should %s %s=%q", verb, key, value.String())
		tc.span, tc.key, tc.value = span, key, value
		cases = append(cases, tc)
	}
	return cases
}

func keeps(span string, key attribute.Key, values ...attribute.Value) []valuePolicyCase {
	return policyCases(span, key, "keep", values, func(value attribute.Value) valuePolicyCase {
		return valuePolicyCase{want: value}
	})
}

func omits(span string, key attribute.Key, values ...attribute.Value) []valuePolicyCase {
	return policyCases(span, key, "omit", values, func(attribute.Value) valuePolicyCase {
		return valuePolicyCase{omitted: true}
	})
}

func replaces(span string, key attribute.Key, want string, values ...attribute.Value) []valuePolicyCase {
	return policyCases(span, key, "export as "+want, values, func(attribute.Value) valuePolicyCase {
		return valuePolicyCase{want: attribute.StringValue(want)}
	})
}

func stringValues(values ...string) []attribute.Value {
	out := make([]attribute.Value, 0, len(values))
	for _, value := range values {
		out = append(out, attribute.StringValue(value))
	}
	return out
}

func intValues(values ...int64) []attribute.Value {
	out := make([]attribute.Value, 0, len(values))
	for _, value := range values {
		out = append(out, attribute.Int64Value(value))
	}
	return out
}

func concatCases(groups ...[]valuePolicyCase) []valuePolicyCase {
	return slices.Concat(groups...)
}

// identityPolicyCases covers every Registries.IDs key: an owner-format value is kept, anything else omitted.
func identityPolicyCases() []valuePolicyCase {
	turn := telemetry.SpanSessionTurn
	wrongTypes := []attribute.Value{attribute.Int64Value(7), attribute.BoolValue(true)}
	return concatCases(
		keeps(turn, telemetry.AttrProfileID, attribute.StringValue(store.DefaultProfileID)),
		omits(turn, telemetry.AttrProfileID,
			append(stringValues("", "work", "/home/ana/profile.toml"), wrongTypes...)...),
		keeps(turn, telemetry.AttrWorkspaceID, attribute.StringValue("ws_0123456789abcdef")),
		omits(turn, telemetry.AttrWorkspaceID, stringValues("/home/ana/project", "ws_0123456789ABCDEF")...),
		keeps(turn, telemetry.AttrSessionID, attribute.StringValue("sess-0123456789abcdef")),
		omits(turn, telemetry.AttrSessionID, append(stringValues("my session", "sess-short"), wrongTypes...)...),
		keeps(turn, telemetry.AttrTurnID, attribute.StringValue("turn-0123456789abcdef")),
		omits(turn, telemetry.AttrTurnID, stringValues("turn_story", "sess-0123456789abcdef")...),
		keeps(turn, telemetry.AttrRunID, attribute.StringValue("run-0123456789abcdef")),
		omits(turn, telemetry.AttrRunID, stringValues("run_01ARZ3NDEKTSV4RRFFQ69G5FAV", "run-0123")...),
		keeps(
			telemetry.SpanLoopRun,
			telemetry.AttrLoopID,
			stringValues("looprun-0123456789abcdef", "nightly_review")...),
		omits(telemetry.SpanLoopRun, telemetry.AttrLoopID, stringValues("Nightly Review", "/home/ana/loops/x")...),
		keeps(telemetry.SpanAutomationRun, telemetry.AttrAutomationID,
			stringValues("job-0123456789abcdef", "trg-0123456789abcdef")...),
		omits(telemetry.SpanAutomationRun, telemetry.AttrAutomationID,
			stringValues("nightly report for ana", "my-job")...),
	)
}

// membershipPolicyCases covers the registry-membership keys: a member is kept, anything else becomes "other".
func membershipPolicyCases() []valuePolicyCase {
	turn := telemetry.SpanSessionTurn
	notStrings := []attribute.Value{attribute.BoolValue(true), attribute.Int64Value(7)}
	return concatCases(
		keeps(turn, attrAgent, attribute.StringValue(testAgent)),
		replaces(
			turn,
			attrAgent,
			telemetry.ValueOther,
			append(stringValues("unregistered-agent", "../../etc"), notStrings...)...),
		keeps(turn, attrSystem, attribute.StringValue(testProvider)),
		replaces(
			turn,
			attrSystem,
			telemetry.ValueOther,
			append(stringValues("unregistered-provider"), notStrings...)...),
		keeps(telemetry.SpanToolExecute, attrTool, attribute.StringValue(testTool)),
		replaces(telemetry.SpanToolExecute, attrTool, telemetry.ValueOther,
			append(stringValues("compozy__unregistered", "/home/ana/tool"), notStrings...)...),
	)
}

// enumPolicyCases covers the enum and numeric keys: a value in its set is kept, anything else omitted.
func enumPolicyCases() []valuePolicyCase {
	tool, turn := telemetry.SpanToolExecute, telemetry.SpanSessionTurn
	wrongTypes := []attribute.Value{attribute.BoolValue(true), attribute.Float64Value(1.5)}
	return concatCases(
		keeps(tool, telemetry.AttrToolRisk, stringValues("read", "mutating", "open_world", "destructive")...),
		omits(tool, telemetry.AttrToolRisk, stringValues("", "admin", "Read")...),
		keeps(tool, attrOutcome, stringValues(telemetry.OutcomeOK, telemetry.OutcomeError, telemetry.OutcomeCancelled,
			telemetry.OutcomeApproved, telemetry.OutcomeDenied, telemetry.OutcomeUnbound, telemetry.OutcomeAfterRun,
			telemetry.OutcomeSetupFailed)...),
		omits(tool, attrOutcome, append(stringValues("", "exploded", "canceled", "OK"), wrongTypes...)...),
		keeps(telemetry.SpanDBWrite, telemetry.AttrDBCoalesced, attribute.BoolValue(true), attribute.BoolValue(false)),
		omits(telemetry.SpanDBWrite, telemetry.AttrDBCoalesced, attribute.StringValue("true"), attribute.Int64Value(1)),
		keeps(turn, telemetry.AttrDBFlushes, intValues(0, 4)...),
		omits(turn, telemetry.AttrDBFlushes,
			append(intValues(-1), attribute.StringValue("4"), attribute.Float64Value(1.5))...),
		keeps(turn, telemetry.AttrDBWriteMSTotal, intValues(0, 120)...),
		omits(turn, telemetry.AttrDBWriteMSTotal, append(intValues(-5), attribute.StringValue("120"))...),
		keeps(telemetry.SpanHookDispatch, telemetry.AttrHookEvent, attribute.StringValue(testHookEvent)),
		omits(telemetry.SpanHookDispatch, telemetry.AttrHookEvent, stringValues("", "prompt.secret_text")...),
		keeps("acp "+testACPMethod, telemetry.AttrRPCSystem, attribute.StringValue("jsonrpc")),
		omits("acp "+testACPMethod, telemetry.AttrRPCSystem, stringValues("grpc", "")...),
		keeps("acp "+testACPMethod, attrRPCMethod, attribute.StringValue(testACPMethod)),
		omits("acp "+testACPMethod, attrRPCMethod, stringValues("fs/read_text_file", "/home/ana/x", "")...),
		keeps(tool, attrErrorType, stringValues(telemetry.ErrorCanceled, telemetry.ErrorDeadlineExceeded,
			telemetry.ErrorProvider, telemetry.ErrorPermissionDenied, telemetry.ErrorNotFound,
			telemetry.ErrorInvalidInput, telemetry.ErrorIO, telemetry.ErrorInternal)...),
		// The outcome spelling is not an error class.
		omits(tool, attrErrorType,
			stringValues("", telemetry.OutcomeCancelled, "open /home/ana/secret.txt: permission denied")...),
	)
}

// httpPolicyCases covers the HTTP keys carried by a route span.
func httpPolicyCases() []valuePolicyCase {
	span := "POST " + testRoute
	rawPath := "/api/workspaces/ws_0123456789abcdef/sessions/sess-0123456789abcdef/prompt"
	return concatCases(
		keeps(span, telemetry.AttrHTTPRoute, attribute.StringValue(testRoute)),
		omits(span, telemetry.AttrHTTPRoute, stringValues("/api/x/:unregistered", rawPath, "")...),
		keeps(span, telemetry.AttrHTTPMethod, stringValues("GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT",
			"OPTIONS", "TRACE", "PATCH")...),
		omits(span, telemetry.AttrHTTPMethod, stringValues("BREW", "get", "")...),
		keeps(span, telemetry.AttrHTTPStatusCode, intValues(100, 200, 404, 599)...),
		omits(span, telemetry.AttrHTTPStatusCode,
			append(intValues(-1, 0, 99, 600), attribute.StringValue("200"), attribute.Float64Value(200))...),
		// Operations are code-owned names and caller function names, so every character of the registered pattern
		// (digits, underscore, slash, hyphen, and a one-character name) must survive.
		keeps(telemetry.SpanDBWrite, attrDBOperation, stringValues("sessiondb.flush", "observe.retention",
			"(*SessionDB).flush", "x", "flush2", "snake_case_op", "store/sessiondb.(*SessionDB).flush-v2",
			strings.Repeat("a", 160))...),
		omits(telemetry.SpanDBWrite, attrDBOperation,
			stringValues("", "has space", "semi;colon", "op:name", strings.Repeat("a", 161))...),
	)
}

// Per-key fallback policy (spec: value shapes and "Per-key fallback policy"), one case per key and value.
func TestSanitizingSpanExporterAppliesValuePolicyPerKey(t *testing.T) {
	t.Parallel()

	reg := testRegistries(nil)
	cases := concatCases(identityPolicyCases(), membershipPolicyCases(), enumPolicyCases(), httpPolicyCases())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, reg)
			_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), tc.span,
				trace.WithAttributes(attribute.KeyValue{Key: tc.key, Value: tc.value}))
			span.End()

			got, present := attributeMap(onlySpan(t, exported).Attributes)[tc.key]
			switch {
			case tc.omitted && present:
				t.Fatalf("%s = %q exported, want it omitted", tc.key, got.String())
			case !tc.omitted && !present:
				t.Fatalf("%s omitted, want %q", tc.key, tc.want.String())
			case !tc.omitted && (got.Type() != tc.want.Type() || got.String() != tc.want.String()):
				t.Fatalf("%s = %s %q, want %s %q", tc.key, got.Type(), got.String(), tc.want.Type(), tc.want.String())
			}
		})
	}
}

// The metric-only keys never ride on a span, so they are exercised through their counters.
func TestSanitizingMetricExporterAppliesValuePolicyToMetricOnlyKeys(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		instrument string
		key        attribute.Key
		value      string
		omitted    bool
	}{
		{
			name:       "Should keep input tokens",
			instrument: telemetry.MetricTokens,
			key:        telemetry.AttrTokenType,
			value:      "input",
		},
		{
			name:       "Should keep output tokens",
			instrument: telemetry.MetricTokens,
			key:        telemetry.AttrTokenType,
			value:      "output",
		},
		{
			name: "Should keep cache read tokens", instrument: telemetry.MetricTokens,
			key: telemetry.AttrTokenType, value: "cache_read",
		},
		{
			name: "Should keep cache write tokens", instrument: telemetry.MetricTokens,
			key: telemetry.AttrTokenType, value: "cache_write",
		},
		{
			name: "Should omit a free-text token type", instrument: telemetry.MetricTokens,
			key: telemetry.AttrTokenType, value: "prompt_text", omitted: true,
		},
		{
			name: "Should omit a differently cased token type", instrument: telemetry.MetricTokens,
			key: telemetry.AttrTokenType, value: "Input", omitted: true,
		},
		{
			name:       "Should keep an actual cost",
			instrument: telemetry.MetricCost,
			key:        telemetry.AttrCostStatus,
			value:      "actual",
		},
		{
			name: "Should keep an estimated cost", instrument: telemetry.MetricCost,
			key: telemetry.AttrCostStatus, value: "estimated",
		},
		{
			name: "Should omit an included cost status", instrument: telemetry.MetricCost,
			key: telemetry.AttrCostStatus, value: "included", omitted: true,
		},
		{
			name: "Should omit an unknown cost status", instrument: telemetry.MetricCost,
			key: telemetry.AttrCostStatus, value: "unknown", omitted: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			harness := newMetricHarness(t, testRegistries(nil))
			counter, err := harness.provider.Meter(telemetry.ScopeObserve).Int64Counter(tc.instrument)
			if err != nil {
				t.Fatalf("Int64Counter(%q) error = %v", tc.instrument, err)
			}
			counter.Add(context.Background(), 1, metric.WithAttributes(tc.key.String(tc.value)))

			_, exported := harness.collect(t)
			sum, ok := findMetric(t, exported, tc.instrument).Data.(metricdata.Sum[int64])
			if !ok || len(sum.DataPoints) != 1 {
				t.Fatalf("%s data = %#v, want one int64 sum point", tc.instrument, sum)
			}
			got, present := sum.DataPoints[0].Attributes.Value(tc.key)
			switch {
			case tc.omitted && present:
				t.Fatalf("%s = %q exported, want it omitted", tc.key, got.AsString())
			case !tc.omitted && (!present || got.AsString() != tc.value):
				t.Fatalf("%s = %q (present %t), want %q", tc.key, got.AsString(), present, tc.value)
			}
		})
	}
}

// The registered-secret check is the last condition for every exported string, not only for models.
func TestSanitizingSpanExporterMarksRegisteredSecretsInvalidForEveryStringKey(t *testing.T) {
	t.Parallel()

	const idSecret = "5e4d3c2b1a099887"
	secret := "policy-matrix-secret-6b1f"
	t.Cleanup(redact.RegisterRequiredSecret(secret))
	t.Cleanup(redact.RegisterRequiredSecret(idSecret))
	reg := testRegistries([]string{"agent-" + secret})
	reg.Providers = members(testProvider, "provider-"+secret)
	reg.ToolIDs = members(testTool, "compozy__tool_"+secret)

	cases := []struct {
		name  string
		span  string
		key   attribute.Key
		value string
	}{
		{
			name: "Should export a registered agent holding a registered secret as invalid",
			span: telemetry.SpanSessionTurn, key: attrAgent, value: "agent-" + secret,
		},
		{
			name: "Should export a registered provider holding a registered secret as invalid",
			span: telemetry.SpanSessionTurn, key: attrSystem, value: "provider-" + secret,
		},
		{
			name: "Should export a registered tool holding a registered secret as invalid",
			span: telemetry.SpanToolExecute, key: attrTool, value: "compozy__tool_" + secret,
		},
		{
			name: "Should export a write operation holding a registered secret as invalid",
			span: telemetry.SpanDBWrite, key: attrDBOperation, value: "observe.retention-" + secret,
		},
		{
			name: "Should export a well-formed session id holding a registered secret as invalid",
			span: telemetry.SpanSessionTurn, key: telemetry.AttrSessionID, value: "sess-" + idSecret,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, reg)
			_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), tc.span,
				trace.WithAttributes(tc.key.String(tc.value)))
			span.End()

			got, present := attributeMap(onlySpan(t, exported).Attributes)[tc.key]
			if !present || got.AsString() != telemetry.ValueInvalid {
				t.Fatalf("%s = %q (present %t), want %q", tc.key, got.AsString(), present, telemetry.ValueInvalid)
			}
		})
	}
}

// Resource values and the scope version pass the registered-secret check on every export: service.name is
// operator input, the version is build input, and a secret can be registered after the exporters are built.
func TestSanitizingExportersMarkResourceAndScopeSecretsInvalid(t *testing.T) {
	t.Parallel()

	builtIn := func(serviceName, version string) *resource.Resource {
		return telemetry.NewResource(serviceName, version, testInstanceID)
	}
	// exportOnce exports one batch and returns the resource and instrumentation scope it was exported with.
	type exportOnce func(t *testing.T) (*resource.Resource, instrumentation.Scope)
	cases := []struct {
		name   string
		secret string
		// build wraps a recording exporter in a sanitizer built for serviceName and version.
		build func(serviceName, version string) exportOnce
	}{
		{
			name:   "Should export span resource and scope versions holding a secret registered after the build as invalid",
			secret: "span-build-secret-7c4e",
			build: func(serviceName, version string) exportOnce {
				exported := tracetest.NewInMemoryExporter()
				exporter := telemetry.NewSanitizingSpanExporter(exported,
					telemetry.NewSanitizer(testRegistries(nil), builtIn(serviceName, version), version))
				return func(t *testing.T) (*resource.Resource, instrumentation.Scope) {
					t.Helper()
					exported.Reset()
					spans := tracetest.SpanStubs{{Name: telemetry.SpanToolExecute}}.Snapshots()
					if err := exporter.ExportSpans(context.Background(), spans); err != nil {
						t.Fatalf("ExportSpans() error = %v", err)
					}
					span := onlySpan(t, exported)
					return span.Resource, span.InstrumentationScope
				}
			},
		},
		{
			name:   "Should export metric resource and scope versions holding a secret registered after the build as invalid",
			secret: "metric-build-secret-2a9b",
			build: func(serviceName, version string) exportOnce {
				recorded := &recordingMetricExporter{}
				exporter := telemetry.NewSanitizingMetricExporter(recorded,
					telemetry.NewSanitizer(testRegistries(nil), builtIn(serviceName, version), version))
				return func(t *testing.T) (*resource.Resource, instrumentation.Scope) {
					t.Helper()
					if err := exporter.Export(context.Background(), handBuiltMetrics(permissionCounter())); err != nil {
						t.Fatalf("Export() error = %v", err)
					}
					exported := recorded.lastExport()
					if len(exported.ScopeMetrics) != 1 {
						t.Fatalf("scope metrics = %+v, want one", exported.ScopeMetrics)
					}
					return exported.Resource, exported.ScopeMetrics[0].Scope
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serviceName, version := "svc-"+tc.secret, testServiceVersion+"+"+tc.secret
			export := tc.build(serviceName, version)
			if res, scope := export(t); !res.Equal(builtIn(serviceName, version)) || scope.Version != version {
				t.Fatalf("before registration: resource = %v, scope version = %q; want both unchanged",
					res.Attributes(), scope.Version)
			}
			t.Cleanup(redact.RegisterRequiredSecret(tc.secret))
			res, scope := export(t)
			if !res.Equal(builtIn(telemetry.ValueInvalid, telemetry.ValueInvalid)) {
				t.Fatalf("resource = %v, want service.name and service.version %q and the other values unchanged",
					res.Attributes(), telemetry.ValueInvalid)
			}
			if scope.Version != telemetry.ValueInvalid {
				t.Fatalf("scope version = %q, want %q", scope.Version, telemetry.ValueInvalid)
			}
		})
	}
}

// Every scope constant is a registered scope. A name missing from the registry would silently export that
// component's spans and metrics under compozy.unknown, so each constant is checked, not only the few the
// name-and-scope and rebuild cases happen to use.
func TestSanitizingExportersKeepEveryRegisteredScope(t *testing.T) {
	t.Parallel()

	for _, scope := range registeredScopes {
		t.Run("Should keep the registered span scope "+scope, func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, testRegistries(nil))
			_, span := provider.Tracer(scope).Start(context.Background(), telemetry.SpanToolExecute)
			span.End()

			if got := onlySpan(t, exported).InstrumentationScope.Name; got != scope {
				t.Fatalf("span scope = %q, want the registered scope %q", got, scope)
			}
		})

		t.Run("Should keep the registered metric scope "+scope, func(t *testing.T) {
			t.Parallel()

			harness := newMetricHarness(t, testRegistries(nil))
			input := handBuiltMetrics(permissionCounter())
			input.ScopeMetrics[0].Scope = instrumentation.Scope{Name: scope}
			if err := harness.exporter.Export(context.Background(), input); err != nil {
				t.Fatalf("Export() error = %v", err)
			}

			exported := harness.recorded.lastExport()
			if len(exported.ScopeMetrics) != 1 {
				t.Fatalf("scope metrics = %+v, want one", exported.ScopeMetrics)
			}
			if got := exported.ScopeMetrics[0].Scope.Name; got != scope {
				t.Fatalf("metric scope = %q, want the registered scope %q", got, scope)
			}
		})
	}
}

// Names are exported strings too: a Secrets predicate that reports a secret in every string leaves no exported
// span or metric string other than "invalid" (registered constants are injected-registry or build content).
func TestSanitizingExportersCheckEveryExportedStringForSecrets(t *testing.T) {
	t.Parallel()

	reg := testRegistries(nil)
	reg.Secrets = func(string) bool { return true }

	t.Run("Should export every span name, event, scope, resource, and attribute string as invalid", func(t *testing.T) {
		t.Parallel()

		provider, exported := newTracerProvider(t, reg)
		names := []string{telemetry.SpanSessionTurn, "POST " + testRoute, "acp " + testACPMethod, "prompt: hello"}
		for _, name := range names {
			_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), name,
				trace.WithAttributes(validAttributes()...))
			span.AddEvent(telemetry.EventFirstToken)
			span.End()
		}

		spans := exported.GetSpans()
		if len(spans) != len(names) {
			t.Fatalf("exported %d spans, want %d", len(spans), len(names))
		}
		for _, span := range spans {
			if len(span.Events) != 1 {
				t.Fatalf("span %q events = %+v, want the registered event", span.Name, span.Events)
			}
			got := []string{
				span.Name,
				span.Events[0].Name,
				span.InstrumentationScope.Name,
				span.InstrumentationScope.Version,
			}
			got = append(got, stringAttributeValues(span.Resource.Attributes())...)
			assertAllInvalid(t, append(got, stringAttributeValues(span.Attributes)...))
		}
	})

	t.Run(
		"Should export every metric name, unit, scope, resource, and attribute string as invalid",
		func(t *testing.T) {
			t.Parallel()

			harness := newMetricHarness(t, reg)
			tokens, err := harness.provider.Meter(telemetry.ScopeSession).Int64Counter(telemetry.MetricTokens)
			if err != nil {
				t.Fatalf("Int64Counter() error = %v", err)
			}
			tokens.Add(context.Background(), 1, metric.WithAttributes(validAttributes()...))

			_, exported := harness.collect(t)
			if len(exported.ScopeMetrics) != 1 || len(exported.ScopeMetrics[0].Metrics) != 1 {
				t.Fatalf("scope metrics = %+v, want one scope with one metric", exported.ScopeMetrics)
			}
			scopeMetrics := exported.ScopeMetrics[0]
			sum, ok := scopeMetrics.Metrics[0].Data.(metricdata.Sum[int64])
			if !ok || len(sum.DataPoints) != 1 {
				t.Fatalf("metric data = %#v, want one int64 sum point", scopeMetrics.Metrics[0].Data)
			}
			got := []string{
				scopeMetrics.Metrics[0].Name,
				scopeMetrics.Metrics[0].Unit,
				scopeMetrics.Scope.Name,
				scopeMetrics.Scope.Version,
			}
			got = append(got, stringAttributeValues(exported.Resource.Attributes())...)
			assertAllInvalid(t, append(got, stringAttributeValues(sum.DataPoints[0].Attributes.ToSlice())...))
		},
	)
}

func stringAttributeValues(attrs []attribute.KeyValue) []string {
	var values []string
	for _, kv := range attrs {
		if kv.Value.Type() == attribute.STRING {
			values = append(values, kv.Value.AsString())
		}
	}
	return values
}

func assertAllInvalid(t *testing.T, exported []string) {
	t.Helper()
	for _, value := range exported {
		if value != telemetry.ValueInvalid {
			t.Fatalf("exported strings = %q, want every one %q", exported, telemetry.ValueInvalid)
		}
	}
}

// A zero-value Registries (nothing wired yet) fails closed: no identity, route, method, or hook event is
// exported, and every membership value is "other".
func TestSanitizingSpanExporterFailsClosedWithZeroRegistries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		span string
		want map[attribute.Key]string
	}{
		{
			name: "Should replace every membership value and omit every identity on a turn span",
			span: telemetry.SpanSessionTurn,
			want: map[attribute.Key]string{
				attrAgent: telemetry.ValueOther, attrSystem: telemetry.ValueOther, attrModel: telemetry.ValueOther,
				attrOutcome: telemetry.OutcomeOK, telemetry.AttrDBFlushes: "4", telemetry.AttrDBWriteMSTotal: "120",
				attrErrorType: telemetry.ErrorInternal,
			},
		},
		{
			name: "Should replace the tool with other on a tool span",
			span: telemetry.SpanToolExecute,
			want: map[attribute.Key]string{
				attrTool: telemetry.ValueOther, telemetry.AttrToolRisk: "read", attrOutcome: telemetry.OutcomeOK,
				attrErrorType: telemetry.ErrorInternal,
			},
		},
		{
			name: "Should omit the hook event on a hook span",
			span: telemetry.SpanHookDispatch,
			want: map[attribute.Key]string{attrOutcome: telemetry.OutcomeOK, attrErrorType: telemetry.ErrorInternal},
		},
		{
			name: "Should omit the route on an unmatched HTTP span",
			span: "HTTP GET",
			want: map[attribute.Key]string{
				telemetry.AttrHTTPMethod:     "POST",
				telemetry.AttrHTTPStatusCode: "200",
				attrErrorType:                telemetry.ErrorInternal,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, telemetry.Registries{})
			_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), tc.span,
				trace.WithAttributes(validAttributes()...))
			span.End()

			got := make(map[attribute.Key]string)
			for key, value := range attributeMap(onlySpan(t, exported).Attributes) {
				got[key] = value.String()
			}
			if !maps.Equal(got, tc.want) {
				t.Fatalf("%q attributes = %v, want %v", tc.span, got, tc.want)
			}
		})
	}

	t.Run("Should replace a route span name with compozy.unknown when no route is registered", func(t *testing.T) {
		t.Parallel()

		provider, exported := newTracerProvider(t, telemetry.Registries{})
		_, span := provider.Tracer(telemetry.ScopeHTTP).Start(context.Background(), "POST "+testRoute)
		span.End()

		if got := onlySpan(t, exported); got.Name != telemetry.UnknownName {
			t.Fatalf("span name = %q, want %q without registered routes", got.Name, telemetry.UnknownName)
		}
	})
}
