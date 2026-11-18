//go:build integration && !windows

package daemon

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/otlptest"
)

// metricTestInterval is the shortest metric_interval config accepts.
const metricTestInterval = 5 * time.Second

// E2E-001
func TestDaemonE2EOTelGoldenJourney(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should export the turn trace and its duration after enabling export through the CLI", func(t *testing.T) {
		t.Parallel()

		collector := otlptest.NewCollector(t)
		harness := startMetricHarness(t, nil)
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
		defer cancel()

		if otel := waitForOTelStatus(ctx, t, harness, func(*compozycontract.OTelStatusPayload) bool {
			return true
		}); otel.State != "disabled" {
			t.Fatalf("status otel before enabling = %+v, want disabled", otel)
		}
		for _, setting := range [][2]string{
			{"observability.otel.enabled", "true"},
			{"observability.otel.endpoint", collector.URL},
			{"observability.otel.metric_interval", metricTestInterval.String()},
		} {
			if _, stderr, err := harness.CLI.RunInDir(
				ctx, harness.WorkspaceRoot, "config", "set", setting[0], setting[1],
			); err != nil {
				t.Fatalf("compozy config set %s error = %v; stderr=%s", setting[0], err, stderr)
			}
		}
		harness.Restart(t)

		session := createFixtureBackedSession(t, ctx, harness, "mock-metrics", "otel-golden-session")
		if _, stderr, err := harness.CLI.RunInDir(
			ctx, harness.WorkspaceRoot, "session", "prompt", session.ID, "list sessions",
		); err != nil {
			t.Fatalf("compozy session prompt error = %v; stderr=%s", err, stderr)
		}

		spans := waitForHostedToolTrace(ctx, t, collector, session.ID)
		execute := spanNamed(t, spans, telemetry.SpanToolExecute)
		if !hasSpanNamed(spans, telemetry.SpanSessionTurn) ||
			execute.Attributes[string(telemetry.AttrTool)] != "compozy__session_list" {
			t.Fatalf("turn trace = %+v, want session.turn with a nested compozy__session_list tool.execute", spans)
		}
		waitForMetrics(ctx, t, collector, telemetry.MetricSessionTurnDuration)
		otel := waitForOTelStatus(ctx, t, harness, func(otel *compozycontract.OTelStatusPayload) bool {
			return !otel.Traces.LastExportAt.IsZero() && !otel.Metrics.LastExportAt.IsZero()
		})
		if otel.State != "ok" {
			t.Fatalf("status otel after a turn = %+v, want ok", otel)
		}
	})
}

// IT-011, IT-012
func TestDaemonE2EOTelExportsTurnMetrics(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should export latency, usage, and activity metrics with their exact attribute sets", func(t *testing.T) {
		t.Parallel()

		collector := otlptest.NewCollector(t)
		harness := startMetricHarness(t, func(cfg *compozyconfig.Config) {
			cfg.Observability.OTel.Enabled = true
			cfg.Observability.OTel.Endpoint = collector.URL
			cfg.Observability.OTel.MetricInterval = metricTestInterval
		})
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
		defer cancel()

		session := createFixtureBackedSession(t, ctx, harness, "mock-metrics", "otel-metrics-session")
		if stream, err := harness.PromptSession(ctx, session.ID, "list sessions"); err != nil || len(stream) == 0 {
			t.Fatalf("PromptSession() = %d updates, %v; want a completed turn", len(stream), err)
		}

		metrics := waitForMetrics(ctx, t, collector,
			telemetry.MetricSessionTurnDuration, telemetry.MetricSessionTTFT, telemetry.MetricACPRPCDuration,
			telemetry.MetricToolCallDuration, telemetry.MetricHTTPServerDuration, telemetry.MetricTokens,
			telemetry.MetricToolCalls, telemetry.MetricSessionActive, telemetry.MetricAgentActive,
			telemetry.MetricUptime,
		)
		turnAttrs := map[string]any{
			string(telemetry.AttrAgent):       "mock-metrics",
			string(telemetry.AttrGenAISystem): acpmock.ProviderName,
			string(telemetry.AttrGenAIModel):  telemetry.ValueOther,
		}
		requireHistogramPoint(t, metrics, telemetry.MetricSessionTurnDuration,
			withMetricAttr(turnAttrs, string(telemetry.AttrOutcome), telemetry.OutcomeOK))
		requireHistogramPoint(t, metrics, telemetry.MetricSessionTTFT, map[string]any{
			string(telemetry.AttrGenAISystem): acpmock.ProviderName,
			string(telemetry.AttrGenAIModel):  telemetry.ValueOther,
		})
		requireHistogramPoint(t, metrics, telemetry.MetricACPRPCDuration, map[string]any{
			string(telemetry.AttrGenAISystem): acpmock.ProviderName,
			string(telemetry.AttrRPCMethod):   "session/prompt",
			string(telemetry.AttrOutcome):     telemetry.OutcomeOK,
		})
		toolAttrs := map[string]any{
			string(telemetry.AttrTool):    "compozy__session_list",
			string(telemetry.AttrOutcome): telemetry.OutcomeOK,
		}
		requireHistogramPoint(t, metrics, telemetry.MetricToolCallDuration, toolAttrs)
		requireHistogramPoint(t, metrics, telemetry.MetricHTTPServerDuration, map[string]any{
			string(telemetry.AttrHTTPRoute):      "/api/workspaces/:workspace_id/sessions/:session_id/prompt",
			string(telemetry.AttrHTTPMethod):     http.MethodPost,
			string(telemetry.AttrHTTPStatusCode): int64(http.StatusOK),
		})
		requireSeries(t, metrics, telemetry.MetricToolCalls, toolAttrs, 1)

		var usage compozycontract.SessionUsageResponse
		path := fmt.Sprintf("/api/workspaces/%s/sessions/%s/usage", harness.WorkspaceID, session.ID)
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &usage); err != nil {
			t.Fatalf("GET %s error = %v", path, err)
		}
		for tokenType, stat := range map[string]*int64{
			telemetry.TokenInput:     usage.Usage.InputTokens,
			telemetry.TokenOutput:    usage.Usage.OutputTokens,
			telemetry.TokenCacheRead: usage.Usage.CacheReadTokens,
		} {
			if stat == nil {
				t.Fatalf("token_stats %+v lacks %s tokens", usage.Usage, tokenType)
			}
			requireSeries(t, metrics, telemetry.MetricTokens,
				withMetricAttr(turnAttrs, string(telemetry.AttrTokenType), tokenType), float64(*stat))
		}
		if _, ok := metrics[telemetry.MetricCost]; ok != countsCostStatus(usage.Usage.CostStatus) {
			t.Fatalf("compozy.cost exported = %v, token_stats cost status %q", ok, usage.Usage.CostStatus)
		}

		var status compozycontract.StatusPayload
		if err := harness.CLI.RunJSONInDir(ctx, harness.WorkspaceRoot, &status, "status", "-o", "json"); err != nil {
			t.Fatalf("compozy status -o json error = %v", err)
		}
		requireSeries(
			t,
			metrics,
			telemetry.MetricSessionActive,
			map[string]any{},
			float64(status.Daemon.ActiveSessions),
		)
		requireSeries(t, metrics, telemetry.MetricAgentActive, map[string]any{}, 1)
	})
}

// countsCostStatus reports whether a token_stats cost status exports a compozy.cost point.
func countsCostStatus(status compozycontract.CostStatus) bool {
	return string(status) == telemetry.CostActual || string(status) == telemetry.CostEstimated
}

func startMetricHarness(t *testing.T, mutate func(*compozyconfig.Config)) *e2etest.RuntimeHarness {
	t.Helper()
	fixture := filepath.Join(t.TempDir(), "metrics_fixture.json")
	if err := os.WriteFile(fixture, []byte(metricsFixture), 0o600); err != nil {
		t.Fatalf("write fixture error = %v", err)
	}
	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  fixture,
			FixtureAgent: "metrics",
			AgentName:    "mock-metrics",
		}},
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: mutate},
	})
}

// metricsFixture calls a hosted native tool, answers, and reports terminal usage.
const metricsFixture = `{
	"version": 2,
	"agents": [{
		"name": "metrics",
		"provider": "acpmock",
		"permissions": "approve-all",
		"turns": [{
			"name": "list",
			"match": {"turn_source": "user", "user_text": "list sessions"},
			"steps": [
				{"kind": "native_tool_call", "tool_id": "compozy__session_list", "tool_call_id": "list-1", "raw_input": {}},
				{"kind": "assistant", "text": "listed"}
			],
			"usage": {"input_tokens": 1200, "output_tokens": 340, "cache_read_tokens": 800}
		}]
	}]
}`

// waitForMetrics polls the collector until every named metric has been exported, then returns the latest exports.
func waitForMetrics(
	ctx context.Context,
	t *testing.T,
	collector *otlptest.Collector,
	names ...string,
) map[string]otlptest.Metric {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		metrics := collector.Metrics()
		if !slices.ContainsFunc(names, func(name string) bool { _, ok := metrics[name]; return !ok }) {
			return metrics
		}
		select {
		case <-ctx.Done():
			t.Fatalf("collector metrics = %v; timed out waiting for %v", slices.Sorted(maps.Keys(metrics)), names)
		case <-ticker.C:
		}
	}
}

// requireHistogramPoint fails unless name is a second-scaled histogram whose points all carry exactly want's keys
// and one point carries want with at least one measurement.
func requireHistogramPoint(t *testing.T, metrics map[string]otlptest.Metric, name string, want map[string]any) {
	t.Helper()
	metric := metrics[name]
	if metric.Type != otlptest.MetricHistogram || metric.Unit != "s" {
		t.Fatalf("%s = %+v, want a histogram in seconds", name, metric)
	}
	found := false
	for _, point := range metric.Points {
		if !slices.Equal(slices.Sorted(maps.Keys(point.Attributes)), slices.Sorted(maps.Keys(want))) ||
			point.Exemplars != 0 || len(point.Bounds) == 0 || point.Bounds[0] >= 1 {
			t.Fatalf("%s point %+v, want keys %v, second-scaled bounds, and no exemplars", name, point, want)
		}
		found = found || (maps.Equal(point.Attributes, want) && point.Count > 0)
	}
	if !found {
		t.Fatalf("%s points = %+v, want one with %v", name, metric.Points, want)
	}
}

// requireSeries fails unless name holds a series with exactly attrs whose value is want.
func requireSeries(t *testing.T, metrics map[string]otlptest.Metric, name string, attrs map[string]any, want float64) {
	t.Helper()
	for _, point := range metrics[name].Points {
		if maps.Equal(point.Attributes, attrs) {
			if point.Value != want {
				t.Fatalf("%s %v = %v, want %v", name, attrs, point.Value, want)
			}
			return
		}
	}
	t.Fatalf("%s points = %+v, want a series with %v", name, metrics[name].Points, attrs)
}

func withMetricAttr(attrs map[string]any, key string, value any) map[string]any {
	extended := maps.Clone(attrs)
	extended[key] = value
	return extended
}
