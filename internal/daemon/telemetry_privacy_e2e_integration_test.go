//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/ginutil"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/udsapi"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session/sessionid"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

const (
	privacyCanary = "SECRET-CANARY-7f3a"
	// privacySecret is resolved from an OTLP header ref, which registers it as a secret in the daemon.
	privacySecret = privacyCanary + "-otlp-key-0123456789"
)

// privacyModels are the session model values, present only in the agent definitions: an absolute, a relative, and a
// Windows forward-slash path, free text, and a registered secret inside an otherwise valid model value.
var privacyModels = []string{
	"/home/operator/" + privacyCanary + "/models/local.gguf",
	"models/" + privacyCanary + "/local.gguf",
	"C:/Users/" + privacyCanary + "/models/local.gguf",
	"my private " + privacyCanary + " model",
	"claude-sonnet-4-5-" + privacySecret,
}

// IT-014, E2E-003
func TestDaemonE2EOTelExportCarriesNoContent(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	cases := []struct {
		name      string
		protocol  string
		collector func(testing.TB) *otlptest.Collector
	}{
		{
			name:      "Should keep every canary out of the raw OTLP/HTTP trace and metric requests",
			protocol:  compozyconfig.OTelProtocolHTTPProtobuf,
			collector: otlptest.NewCollector,
		},
		{
			name:      "Should export healthy and keep every canary out of the raw OTLP/gRPC messages",
			protocol:  compozyconfig.OTelProtocolGRPC,
			collector: otlptest.NewGRPCCollector,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			collector := tc.collector(t)
			fixture := filepath.Join(t.TempDir(), "privacy_fixture.json")
			if err := os.WriteFile(fixture, privacyFixture(t), 0o600); err != nil {
				t.Fatalf("write fixture error = %v", err)
			}
			agents := make([]e2etest.MockAgentSpec, len(privacyModels))
			for i := range privacyModels {
				agents[i] = e2etest.MockAgentSpec{
					FixturePath:  fixture,
					FixtureAgent: fmt.Sprintf("canary-%d", i),
					AgentName:    fmt.Sprintf("mock-canary-%d", i),
				}
			}
			harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
				Env: map[string]string{
					"OTEL_RESOURCE_ATTRIBUTES": "deployment.environment=" + privacyCanary + "," + privacyCanary + "=1",
					"CANARY_OTLP_KEY":          privacySecret,
				},
				MockAgents: agents,
				ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
					cfg.Observability.OTel.Enabled = true
					cfg.Observability.OTel.Endpoint = collector.URL
					cfg.Observability.OTel.Protocol = tc.protocol
					cfg.Observability.OTel.MetricInterval = 5 * time.Second
					cfg.Observability.OTel.Headers = map[string]string{"x-canary-key": "env:CANARY_OTLP_KEY"}
				}},
			})
			ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
			defer cancel()

			sessionIDs := make([]string, len(agents))
			for i, agent := range agents {
				session := createFixtureBackedSession(t, ctx, harness, agent.AgentName, privacyCanary+" session")
				if model := sessionModel(&session); model != privacyModels[i] {
					t.Fatalf("session %s model = %q, want the configured %q", session.ID, model, privacyModels[i])
				}
				sessionIDs[i] = session.ID
			}
			getSessionWithTraceState(ctx, t, harness, sessionIDs[0])
			for _, sessionID := range sessionIDs {
				if stream, err := harness.PromptSessionHTTP(ctx, sessionID, "canary "+privacyCanary); err != nil ||
					len(stream) == 0 {
					t.Fatalf(
						"PromptSessionHTTP(%s) = %d updates, %v; want a completed turn",
						sessionID,
						len(stream),
						err,
					)
				}
			}
			if _, err := harness.PromptSessionHTTP(ctx, sessionIDs[0], "fail "+privacyCanary); err != nil {
				t.Logf("failing turn returned %v", err)
			}

			waitForTurnSpans(ctx, t, collector, len(sessionIDs)+1)
			otel := waitForOTelStatus(ctx, t, harness, func(otel *compozycontract.OTelStatusPayload) bool {
				return !otel.Traces.LastExportAt.IsZero() && !otel.Metrics.LastExportAt.IsZero()
			})
			if otel.State != "ok" || otel.Protocol != tc.protocol || otel.Traces.LastErrorClass != "" ||
				otel.Metrics.LastErrorClass != "" {
				t.Fatalf("status otel = %+v, want ok over %s", otel, tc.protocol)
			}

			// The canary placements ran: hosted tool calls both succeeded and failed, and the fail turn errored.
			outcomes := map[string]int{}
			for _, span := range collector.Spans() {
				if span.Name == telemetry.SpanToolExecute || span.Name == telemetry.SpanSessionTurn {
					outcomes[span.Name+"/"+fmt.Sprint(span.Attributes[string(telemetry.AttrOutcome)])]++
				}
			}
			for _, want := range []string{
				telemetry.SpanToolExecute + "/" + telemetry.OutcomeOK,
				telemetry.SpanToolExecute + "/" + telemetry.OutcomeError,
				telemetry.SpanSessionTurn + "/" + telemetry.OutcomeError,
			} {
				if outcomes[want] == 0 {
					t.Fatalf("exported span outcomes = %v, want %s", outcomes, want)
				}
			}

			traces := collector.Payloads(otlptest.TracesPath)
			metrics := collector.Payloads(otlptest.MetricsPath)
			if len(traces) == 0 || len(metrics) == 0 {
				t.Fatalf("collector got %d trace and %d metric requests, want both", len(traces), len(metrics))
			}
			for path, payloads := range map[string][][]byte{"traces": traces, "metrics": metrics} {
				for i, payload := range payloads {
					if bytes.Contains(payload, []byte(privacyCanary)) {
						t.Fatalf("raw %s request %d contains the canary: %q", path, i, payload)
					}
				}
			}
			policy := newExportPolicy(agents)
			for _, payload := range traces {
				policy.checkTraces(t, payload)
			}
			for _, payload := range metrics {
				policy.checkMetrics(t, payload)
			}
		})
	}
}

// privacyFixture scripts one agent per privacy model: the canary turn streams the canary, runs an ACP tool call
// that fails on a canary path, lists sessions (whose names hold the canary), and calls native tools with canary
// arguments and a canary file path that fail with errors echoing them; the fail turn ends with a provider error naming
// a canary file path.
func privacyFixture(t *testing.T) []byte {
	t.Helper()
	path := "/home/operator/" + privacyCanary + "/notes.md"
	turns := []map[string]any{
		{
			"name":  "canary",
			"match": map[string]any{"turn_source": "user", "user_text": "canary " + privacyCanary},
			"usage": map[string]any{"input_tokens": 11, "output_tokens": 22},
			"steps": []map[string]any{
				{"kind": "assistant", "chunks": []string{"echo ", privacyCanary}},
				{
					"kind": "tool_call", "tool_call_id": "read-1", "title": "Read " + path, "tool_kind": "read",
					"path": path, "status": "failed", "content_text": "open " + path + ": permission denied",
					"raw_input":  map[string]any{"path": path, "note": privacyCanary},
					"raw_output": map[string]any{"error": "open " + path + ": permission denied"},
				},
				{
					"kind": "native_tool_call", "tool_id": "compozy__session_list", "tool_call_id": "list-1",
					"raw_input": map[string]any{},
				},
				{
					"kind": "native_tool_call", "tool_id": "compozy__session_describe", "tool_call_id": "describe-1",
					"raw_input": map[string]any{"session_id": privacyCanary},
				},
				{
					"kind": "native_tool_call", "tool_id": "compozy__skill_view", "tool_call_id": "skill-1",
					"raw_input": map[string]any{"name": privacyCanary, "file": path},
				},
				{
					"kind": "native_tool_call", "tool_id": "compozy__config_get", "tool_call_id": "config-1",
					"raw_input": map[string]any{"key": privacyCanary + ".path"},
				},
				{"kind": "assistant", "text": "done " + privacyCanary},
			},
		},
		{
			"name":  "fail",
			"match": map[string]any{"turn_source": "user", "user_text": "fail " + privacyCanary},
			"steps": []map[string]any{{
				"kind": "driver_control",
				"driver_control": map[string]any{
					"action":        "fail_prompt",
					"error_message": "open /home/operator/" + privacyCanary + "/config.json: no such file or directory",
				},
			}},
		},
	}
	agents := make([]map[string]any, len(privacyModels))
	for i, model := range privacyModels {
		agents[i] = map[string]any{
			"name": fmt.Sprintf("canary-%d", i), "provider": "acpmock", "model": model,
			"permissions": "approve-all", "turns": turns,
			"config_options": []map[string]any{{
				"id": "model", "name": "Model", "category": "model", "current": model,
				"values": []map[string]any{{"value": model}},
			}},
		}
	}
	raw, err := json.Marshal(map[string]any{"version": acpmock.FixtureVersion, "agents": agents})
	if err != nil {
		t.Fatalf("encode privacy fixture: %v", err)
	}
	return raw
}

func sessionModel(session *compozycontract.SessionPayload) string {
	for _, selection := range []*compozycontract.RuntimeSelectionPayload{
		session.Runtime.Effective, session.Runtime.LastKnown,
	} {
		if selection != nil && selection.Model != "" {
			return selection.Model
		}
	}
	return ""
}

// getSessionWithTraceState reads a session over HTTP with a sampled parent whose tracestate holds the canary.
func getSessionWithTraceState(ctx context.Context, t *testing.T, harness *e2etest.RuntimeHarness, sessionID string) {
	t.Helper()
	target := harness.HTTPURL("/api/workspaces/" + harness.WorkspaceID + "/sessions/" + sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatalf("new session request: %v", err)
	}
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("tracestate", "canary="+privacyCanary)
	resp, err := harness.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s error = %v", target, err)
	}
	if err := resp.Body.Close(); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, close %v; want 200", target, resp.StatusCode, err)
	}
}

// waitForTurnSpans polls the collector until it holds count session.turn spans.
func waitForTurnSpans(ctx context.Context, t *testing.T, collector *otlptest.Collector, count int) {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		turns := 0
		for _, span := range collector.Spans() {
			if span.Name == telemetry.SpanSessionTurn {
				turns++
			}
		}
		if turns >= count {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("collector has %d session.turn spans, want %d", turns, count)
		case <-ticker.C:
		}
	}
}

// exportPolicy checks decoded OTLP requests against the exported catalog: registered names and keys, and each
// value its key's registry or owner validator admits.
type exportPolicy struct {
	values map[string]func(string) bool
	routes []string
}

func newExportPolicy(agents []e2etest.MockAgentSpec) exportPolicy {
	oneOf := func(values ...string) func(string) bool {
		return func(value string) bool { return slices.Contains(values, value) }
	}
	agentNames := []string{telemetry.ValueOther}
	for _, agent := range agents {
		agentNames = append(agentNames, agent.AgentName)
	}
	hookEvents := make([]string, 0)
	for _, event := range hookspkg.AllHookEvents() {
		hookEvents = append(hookEvents, event.String())
	}
	key := func(k attribute.Key) string { return string(k) }
	routes := mountedRouteTemplates()
	return exportPolicy{routes: routes, values: map[string]func(string) bool{
		key(telemetry.AttrProfileID):    profile.IsProfileID,
		key(telemetry.AttrWorkspaceID):  workspacepkg.IsWorkspaceIdentifier,
		key(telemetry.AttrSessionID):    sessionid.IsSessionID,
		key(telemetry.AttrTurnID):       sessionid.IsTurnID,
		key(telemetry.AttrRunID):        sessionid.IsRunID,
		key(telemetry.AttrLoopID):       looppkg.IsLoopID,
		key(telemetry.AttrAutomationID): automationpkg.IsAutomationID,
		key(telemetry.AttrAgent):        oneOf(agentNames...),
		key(telemetry.AttrGenAISystem):  oneOf(acpmock.ProviderName, telemetry.ValueOther),
		key(telemetry.AttrGenAIModel):   oneOf(telemetry.ValueOther, telemetry.ValueCustom),
		key(telemetry.AttrTool): func(value string) bool {
			return value == telemetry.ValueOther || strings.HasPrefix(value, "compozy__")
		},
		key(telemetry.AttrToolRisk): oneOf("read", "mutating", "open_world", "destructive"),
		key(telemetry.AttrOutcome): oneOf(
			telemetry.OutcomeOK, telemetry.OutcomeError, telemetry.OutcomeCancelled, telemetry.OutcomeApproved,
			telemetry.OutcomeDenied, telemetry.OutcomeUnbound, telemetry.OutcomeAfterRun,
			telemetry.OutcomeSetupFailed,
		),
		key(telemetry.AttrTokenType): oneOf(
			telemetry.TokenInput, telemetry.TokenOutput, telemetry.TokenCacheRead, telemetry.TokenCacheWrite,
		),
		key(telemetry.AttrCostStatus): oneOf(telemetry.CostActual, telemetry.CostEstimated),
		key(telemetry.AttrDBOperation): func(value string) bool {
			return value != "" && !strings.ContainsAny(value, " \t\n")
		},
		key(telemetry.AttrHookEvent):  oneOf(hookEvents...),
		key(telemetry.AttrRPCSystem):  oneOf(telemetry.RPCSystemJSONRPC),
		key(telemetry.AttrRPCMethod):  oneOf(acp.TracedMethods()...),
		key(telemetry.AttrHTTPRoute):  oneOf(routes...),
		key(telemetry.AttrHTTPMethod): oneOf(privacyHTTPMethods...),
		key(telemetry.AttrErrorType): oneOf(
			telemetry.ErrorCanceled, telemetry.ErrorDeadlineExceeded, telemetry.ErrorProvider,
			telemetry.ErrorPermissionDenied, telemetry.ErrorNotFound, telemetry.ErrorInvalidInput,
			telemetry.ErrorIO, telemetry.ErrorInternal,
		),
		// Typed values: the sanitizer keeps only their declared type.
		key(telemetry.AttrDBCoalesced):    nil,
		key(telemetry.AttrDBFlushes):      nil,
		key(telemetry.AttrDBWriteMSTotal): nil,
		key(telemetry.AttrHTTPStatusCode): nil,
	}}
}

var (
	privacyResourceKeys = []string{"service.name", "service.version", "service.instance.id", "os.type", "host.arch"}
	privacyScopes       = []string{
		telemetry.ScopeHTTP, telemetry.ScopeSession, telemetry.ScopeACP, telemetry.ScopeMCP, telemetry.ScopeTools,
		telemetry.ScopeStore, telemetry.ScopeObserve, telemetry.ScopeHooks, telemetry.ScopeLoop,
		telemetry.ScopeAutomation, telemetry.ScopeScheduler, telemetry.UnknownName,
	}
	privacySpanNames = []string{
		telemetry.SpanSessionTurn, telemetry.SpanACPProcessSpawn, telemetry.SpanACPPermissionWait,
		telemetry.SpanMCPHostedCall, telemetry.SpanToolExecute, telemetry.SpanDBWrite, telemetry.SpanHookDispatch,
		telemetry.SpanLoopRun, telemetry.SpanAutomationRun, telemetry.SpanSchedulerTick, telemetry.UnknownName,
	}
	privacyHTTPMethods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
	privacyMetricNames = []string{
		telemetry.MetricSessionTurnDuration, telemetry.MetricSessionTTFT, telemetry.MetricACPRPCDuration,
		telemetry.MetricToolCallDuration, telemetry.MetricHTTPServerDuration, telemetry.MetricDBWriteDuration,
		telemetry.MetricDBWriteAdmissionWait, telemetry.MetricTokens, telemetry.MetricCost,
		telemetry.MetricToolCalls, telemetry.MetricPermissionRequests, telemetry.MetricAgentErrors,
		telemetry.MetricSessionActive, telemetry.MetricAgentActive, telemetry.MetricUptime,
	}
)

func (p exportPolicy) checkResource(t *testing.T, attrs []*commonpb.KeyValue) {
	t.Helper()
	for _, kv := range attrs {
		if !slices.Contains(privacyResourceKeys, kv.GetKey()) {
			t.Errorf("resource attribute %q is not registered", kv.GetKey())
		}
	}
}

func (p exportPolicy) checkScope(t *testing.T, scope *commonpb.InstrumentationScope) {
	t.Helper()
	if !slices.Contains(privacyScopes, scope.GetName()) || len(scope.GetAttributes()) != 0 {
		t.Errorf("scope %q (attributes %v) is not registered", scope.GetName(), scope.GetAttributes())
	}
}

func (p exportPolicy) checkAttributes(t *testing.T, owner string, attrs []*commonpb.KeyValue) {
	t.Helper()
	for _, kv := range attrs {
		valid, registered := p.values[kv.GetKey()]
		if !registered {
			t.Errorf("%s: attribute key %q is not registered", owner, kv.GetKey())
			continue
		}
		value, isString := kv.GetValue().GetValue().(*commonpb.AnyValue_StringValue)
		if (valid != nil) != isString {
			t.Errorf("%s: attribute %q has value %v of the wrong type", owner, kv.GetKey(), kv.GetValue())
			continue
		}
		if valid != nil && !valid(value.StringValue) {
			t.Errorf("%s: attribute %q = %q is outside its registry", owner, kv.GetKey(), value.StringValue)
		}
	}
}

func (p exportPolicy) checkTraces(t *testing.T, payload []byte) {
	t.Helper()
	var export coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(payload, &export); err != nil {
		t.Fatalf("decode trace request: %v", err)
	}
	for _, resourceSpans := range export.GetResourceSpans() {
		p.checkResource(t, resourceSpans.GetResource().GetAttributes())
		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			p.checkScope(t, scopeSpans.GetScope())
			for _, span := range scopeSpans.GetSpans() {
				if !p.registeredSpanName(span.GetName()) {
					t.Errorf("span name %q is not registered", span.GetName())
				}
				if span.GetTraceState() != "" || span.GetStatus().GetMessage() != "" || len(span.GetLinks()) != 0 {
					t.Errorf("span %q carries trace state %q, status message %q, or links",
						span.GetName(), span.GetTraceState(), span.GetStatus().GetMessage())
				}
				p.checkAttributes(t, span.GetName(), span.GetAttributes())
				for _, event := range span.GetEvents() {
					if !slices.Contains(
						[]string{telemetry.EventFirstToken, telemetry.EventRecovery},
						event.GetName(),
					) ||
						len(event.GetAttributes()) != 0 {
						t.Errorf("span %q event %q is not registered", span.GetName(), event.GetName())
					}
				}
			}
		}
	}
}

// registeredSpanName admits catalog span names, traced ACP methods, "HTTP <METHOD>" for unmatched requests, and an
// HTTP method plus a mounted route template: the names the production sanitizer keeps.
func (p exportPolicy) registeredSpanName(name string) bool {
	if slices.Contains(privacySpanNames, name) {
		return true
	}
	if method, ok := strings.CutPrefix(name, "acp "); ok {
		return slices.Contains(acp.TracedMethods(), method)
	}
	if method, ok := strings.CutPrefix(name, "HTTP "); ok {
		return slices.Contains(privacyHTTPMethods, method)
	}
	method, route, ok := strings.Cut(name, " ")
	return ok && slices.Contains(privacyHTTPMethods, method) && slices.Contains(p.routes, route)
}

// mountedRouteTemplates returns the route templates the daemon mounts on its local HTTP and UDS servers, the set the
// telemetry sanitizer admits in span names and http.route values.
func mountedRouteTemplates() []string {
	httpEngine, udsEngine := gin.New(), gin.New()
	ginutil.QuietDebug(func() {
		httpapi.RegisterSurfaceRoutes(httpEngine, &httpapi.Handlers{}, httpapi.SurfaceSetLocal)
		udsapi.RegisterRoutes(udsEngine, &udsapi.Handlers{})
	})
	return slices.Concat(ginutil.RouteTemplates(httpEngine), ginutil.RouteTemplates(udsEngine))
}

func (p exportPolicy) checkMetrics(t *testing.T, payload []byte) {
	t.Helper()
	var export colmetricpb.ExportMetricsServiceRequest
	if err := proto.Unmarshal(payload, &export); err != nil {
		t.Fatalf("decode metric request: %v", err)
	}
	for _, resourceMetrics := range export.GetResourceMetrics() {
		p.checkResource(t, resourceMetrics.GetResource().GetAttributes())
		for _, scopeMetrics := range resourceMetrics.GetScopeMetrics() {
			p.checkScope(t, scopeMetrics.GetScope())
			for _, metric := range scopeMetrics.GetMetrics() {
				if !slices.Contains(privacyMetricNames, metric.GetName()) {
					t.Errorf("metric %q is not registered", metric.GetName())
				}
				p.checkPoints(t, metric)
			}
		}
	}
}

func (p exportPolicy) checkPoints(t *testing.T, metric *metricpb.Metric) {
	t.Helper()
	check := func(attrs []*commonpb.KeyValue, exemplars int) {
		p.checkAttributes(t, metric.GetName(), attrs)
		if exemplars != 0 {
			t.Errorf("metric %q exports %d exemplars, want none", metric.GetName(), exemplars)
		}
	}
	switch data := metric.GetData().(type) {
	case *metricpb.Metric_Sum:
		for _, point := range data.Sum.GetDataPoints() {
			check(point.GetAttributes(), len(point.GetExemplars()))
		}
	case *metricpb.Metric_Gauge:
		for _, point := range data.Gauge.GetDataPoints() {
			check(point.GetAttributes(), len(point.GetExemplars()))
		}
	case *metricpb.Metric_Histogram:
		for _, point := range data.Histogram.GetDataPoints() {
			check(point.GetAttributes(), len(point.GetExemplars()))
		}
	default:
		t.Errorf("metric %q has unexpected data type %T", metric.GetName(), data)
	}
}
