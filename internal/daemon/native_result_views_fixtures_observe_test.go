package daemon

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesObserve(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload map[string]any
		if err := json.Unmarshal(
			[]byte(
				`{"health":{"status":"degraded","version":"test","uptime_seconds":2,"active_sessions":1,"active_agents":1,"failures":{"status":"failed","total":1,"by_kind":{"runtime":1},"recent":[{"session_id":"sess-a","failure_kind":"runtime","summary":"failure","updated_at":"2026-11-10T00:00:00Z"}]},"persistence":{"status":"ok","global_db_size_bytes":100},"retention":{"enabled":true,"last_sweep_status":"failed","last_sweep_error":"sweep failed"},"agent_probes":[{"agent_name":"agent-a","provider":"claude","status":"failed","error":"probe failed","command":"internal"}],"activities":[{"session_id":"sess-a","status":"running"}]},"provider_usage_collect_failed_total":2}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		var health contract.ObserveHealthPayload
		if err := json.Unmarshal(
			[]byte(
				`{"status":"degraded","version":"test","uptime_seconds":2,"active_sessions":1,"active_agents":1,"failures":{"status":"failed","total":1,"by_kind":{"runtime":1},"recent":[{"session_id":"sess-a","failure_kind":"runtime","summary":"failure","updated_at":"2026-11-10T00:00:00Z"}]},"persistence":{"status":"ok","global_db_size_bytes":100},"retention":{"enabled":true,"last_sweep_status":"failed","last_sweep_error":"sweep failed"},"agent_probes":[{"agent_name":"agent-a","provider":"claude","status":"failed","error":"probe failed","command":"internal"}],"activities":[{"session_id":"sess-a","status":"running"}]}`,
			),
			&health,
		); err != nil {
			t.Fatal(err)
		}
		payload["health"] = health
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__observe_metrics",
				Name:   "failure",
				MustKeep: []string{
					"health.failures.recent.0.summary",
					"health.retention.last_sweep_error", "health.agent_probes.0.error",
					"provider_usage_collect_failed_total",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[map[string]any]{Compact: compactObserveMetrics}).result(view, payload)
				},
			},
		)
	}
	return rows
}
