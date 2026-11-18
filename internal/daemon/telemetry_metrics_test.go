package daemon

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/otlptest"
)

// Suite: daemon usage and activity metrics
// Invariant: accepted usage and activity export as the _dx.md counters and gauges with exactly their attribute sets.
// Boundary IN: observe.AcceptedUsage / AcceptedEvent values and an activity source.
// Boundary OUT: OTLP metric exports through the real telemetry runtime (the test owns the global providers, so it
// runs serially).
func TestEventMetricsExportUsageAndActivity(t *testing.T) {
	t.Run("Should export tokens per type and only actual or estimated USD cost", func(t *testing.T) {
		collector := otlptest.NewCollector(t)
		collector.ExportWithMetrics(t, 1, metricTestRegistries())
		ctx := t.Context()
		usage := observe.AcceptedUsage{
			Provider: "claude", Model: "claude-test", Agent: "coder",
			InputTokens: 1200, OutputTokens: 340, CacheReadTokens: 800,
		}
		recordUsage := func(status modelcatalog.CostStatus, amount float64, currency string) {
			usage.CostStatus, usage.CostAmount, usage.CostCurrency = status, amount, currency
			eventMetrics{}.RecordUsage(ctx, usage)
		}
		recordUsage(modelcatalog.CostStatusActual, 0.25, "USD")
		recordUsage(modelcatalog.CostStatusEstimated, 1.5, "usd")
		recordUsage(modelcatalog.CostStatusIncluded, 0, "")
		recordUsage(modelcatalog.CostStatusUnknown, 0, "")
		recordUsage(modelcatalog.CostStatusActual, 9, "EUR")

		metrics := collector.FlushMetrics(t)
		requireCounterPoints(t, metrics, telemetry.MetricTokens, "{token}", map[string]float64{
			telemetry.TokenInput:     5 * 1200,
			telemetry.TokenOutput:    5 * 340,
			telemetry.TokenCacheRead: 5 * 800,
		}, string(telemetry.AttrTokenType))
		requireCounterPoints(t, metrics, telemetry.MetricCost, "{USD}", map[string]float64{
			telemetry.CostActual:    0.25,
			telemetry.CostEstimated: 1.5,
		}, string(telemetry.AttrCostStatus))
	})

	t.Run("Should export one series per tool, permission outcome, and agent error class", func(t *testing.T) {
		collector := otlptest.NewCollector(t)
		collector.ExportWithMetrics(t, 1, metricTestRegistries())
		ctx := t.Context()
		for _, event := range []observe.AcceptedEvent{
			{Kind: observe.EventToolCallTerminal, ToolID: "compozy__session_list", Outcome: telemetry.OutcomeOK},
			{Kind: observe.EventToolCallTerminal, ToolID: "compozy__config_get", Outcome: telemetry.OutcomeOK},
			{Kind: observe.EventToolCallTerminal, ToolID: "compozy__session_list", Outcome: telemetry.OutcomeOK},
			{Kind: observe.EventPermission, Outcome: telemetry.OutcomeDenied},
			{Kind: observe.EventAgentError, Provider: "claude", ErrorType: telemetry.ErrorProvider},
		} {
			eventMetrics{}.RecordEvent(ctx, event)
		}

		metrics := collector.FlushMetrics(t)
		requireMetricSeries(t, metrics, telemetry.MetricToolCalls, "{call}", map[string]float64{
			"compozy.outcome=ok compozy.tool=compozy__config_get":   1,
			"compozy.outcome=ok compozy.tool=compozy__session_list": 2,
		})
		requireMetricSeries(t, metrics, telemetry.MetricPermissionRequests, "{request}", map[string]float64{
			"compozy.outcome=denied": 1,
		})
		requireMetricSeries(t, metrics, telemetry.MetricAgentErrors, "{error}", map[string]float64{
			"error.type=provider_error gen_ai.system=claude": 1,
		})
	})

	t.Run("Should report activity gauges from the cheap activity source", func(t *testing.T) {
		collector := otlptest.NewCollector(t)
		collector.ExportWithMetrics(t, 1, metricTestRegistries())
		if err := registerActivityGauges(fixedActivity{sessions: 3, agents: 2, uptime: 90 * time.Second}); err != nil {
			t.Fatalf("registerActivityGauges() error = %v", err)
		}

		metrics := collector.FlushMetrics(t)
		for name, want := range map[string]float64{
			telemetry.MetricSessionActive: 3,
			telemetry.MetricAgentActive:   2,
			telemetry.MetricUptime:        90,
		} {
			metric := metrics[name]
			if metric.Type != otlptest.MetricGauge || len(metric.Points) != 1 ||
				metric.Points[0].Value != want || len(metric.Points[0].Attributes) != 0 {
				t.Fatalf("%s = %+v, want one attribute-free gauge point %v", name, metric, want)
			}
		}
	})
}

// requireCounterPoints checks a counter whose series share the usage attributes and differ by kindKey.
func requireCounterPoints(
	t *testing.T,
	metrics map[string]otlptest.Metric,
	name, unit string,
	want map[string]float64,
	kindKey string,
) {
	t.Helper()
	series := make(map[string]float64, len(want))
	for kind, value := range want {
		series[seriesKey(map[string]any{
			string(telemetry.AttrGenAISystem): "claude",
			string(telemetry.AttrGenAIModel):  telemetry.ValueOther,
			string(telemetry.AttrAgent):       "coder",
			kindKey:                           kind,
		})] = value
	}
	requireMetricSeries(t, metrics, name, unit, series)
}

// requireMetricSeries checks that name is a monotonic counter in unit whose series, keyed by seriesKey, are want.
func requireMetricSeries(
	t *testing.T,
	metrics map[string]otlptest.Metric,
	name, unit string,
	want map[string]float64,
) {
	t.Helper()
	metric, ok := metrics[name]
	if !ok || metric.Type != otlptest.MetricSum || !metric.Monotonic || metric.Unit != unit {
		t.Fatalf("%s = %+v (present %v), want a monotonic counter in %s", name, metric, ok, unit)
	}
	got := make(map[string]float64, len(metric.Points))
	for _, point := range metric.Points {
		got[seriesKey(point.Attributes)] = point.Value
	}
	if !maps.Equal(got, want) {
		t.Fatalf("%s series = %v, want %v", name, got, want)
	}
}

// seriesKey renders an attribute set as sorted key=value pairs.
func seriesKey(attrs map[string]any) string {
	pairs := make([]string, 0, len(attrs))
	for key, value := range attrs {
		text, _ := value.(string)
		pairs = append(pairs, key+"="+text)
	}
	slices.Sort(pairs)
	return strings.Join(pairs, " ")
}

func metricTestRegistries() telemetry.Registries {
	member := func(names ...string) func(string) bool {
		return func(name string) bool { return slices.Contains(names, name) }
	}
	return telemetry.Registries{
		Providers: member("claude"),
		Agents:    member("coder"),
		ToolIDs:   member("compozy__session_list", "compozy__config_get"),
	}
}

type fixedActivity struct {
	sessions, agents int
	uptime           time.Duration
}

func (a fixedActivity) ActiveCounts(context.Context) (int, int, error) {
	return a.sessions, a.agents, nil
}

func (a fixedActivity) Uptime() time.Duration { return a.uptime }
