package daemon

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesGoal(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload contract.GoalCommandResult
		if err := json.Unmarshal(
			[]byte(
				`{"outcome":"error","reason_code":"turn_limit","snapshot":{"run_id":"lr-a","node_id":"work","objective":"Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective ","origin_session_id":"sess-a","bound_session_id":"sess-a","status":"blocked","run_status":"blocked","cause":"turn_limit","turns_used":4,"turn_limit":4,"live":false,"pause_requested":true,"contract_summary":"Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria ","last_verdict":{"outcome":"blocked","blocking_issues":[{"id":"test","note":"verification failed"}],"evidence_ref":"artifact:a","evaluated_at":"2026-11-10T00:00:00Z"},"context":{"state":"unknown","diagnostic":"context unavailable","used":null,"size":null,"ratio":null,"nudge_ratio":0.8,"reported_at":null}},"replaced_run_id":"lr-old"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__goal_control",
				Name:   "failure",
				MustKeep: []string{
					"outcome",
					"reason_code",
					"snapshot.run_id",
					"snapshot.turn_limit",
					"snapshot.last_verdict.blocking_issues.0.note",
					"snapshot.context.diagnostic",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.GoalCommandResult]{Compact: viewGoalControl}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.GoalCommandResult
		if err := json.Unmarshal(
			[]byte(`{"outcome":"error","reason_code":"goal_budget_fenced","snapshot":null,"replaced_run_id":null}`),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__goal_control",
				Name:     "no_snapshot",
				MustKeep: []string{"outcome", "reason_code", "snapshot"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.GoalCommandResult]{Compact: viewGoalControl}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.SessionGoalResponse
		if err := json.Unmarshal(
			[]byte(
				`{"goal":{"run_id":"lr-a","node_id":"work","objective":"Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective Authored objective ","origin_session_id":"sess-a","bound_session_id":"sess-a","status":"blocked","run_status":"blocked","cause":"turn_limit","turns_used":4,"turn_limit":4,"live":false,"pause_requested":true,"contract_summary":"Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria Authored criteria ","last_verdict":{"outcome":"blocked","blocking_issues":[{"id":"test","note":"verification failed"}],"evidence_ref":"artifact:a","evaluated_at":"2026-11-10T00:00:00Z"},"context":{"state":"unknown","diagnostic":"context unavailable","used":null,"size":null,"ratio":null,"nudge_ratio":0.8,"reported_at":null}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__goal_get",
				Name:   "failure",
				MustKeep: []string{
					"goal.run_id",
					"goal.cause",
					"goal.last_verdict.blocking_issues.0.note",
					"goal.turn_limit",
					"goal.context.diagnostic",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.SessionGoalResponse]{Compact: viewGoalGet}).result(view, payload)
				},
			},
		)
	}
	return rows
}
