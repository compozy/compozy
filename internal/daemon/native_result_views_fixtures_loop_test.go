package daemon

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesLoop(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload contract.LoopResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"loop":{"name":"repair","version":3,"source":"workspace","description":"Repair loop","catalog":{},"definition":{"meta":{"name":"repair"},"inputs":{"brief":{"type":"string"}}}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_create",
				Name:     "populated",
				MustKeep: []string{"loop.name", "loop.version", "loop.source"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.LoopResponse]{Compact: viewLoopCreate}).result(view, &payload)
				},
			},
		)
	}
	{
		var payload contract.LoopConfigResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(`{"config":{"iteration_cap":4},"effective_config":{"sources":{"iteration_cap":"workspace"}}}`),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_configure",
				Name:     "populated",
				MustKeep: []string{"config.iteration_cap"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopConfigResponse]{Compact: viewLoopConfigure}).result(view, payload)
				},
			},
		)
	}
	{
		var payload loopInspectResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"name":"repair","version":3,"inputs":{"brief":{"type":"string"}},"contract":{},"effective_config":{"sources":{}},"start":[]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_inspect",
				Name:     "populated",
				MustKeep: []string{"name", "version", "inputs", "contract", "start"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[loopInspectResult]{Compact: viewLoopInspect}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.LoopsResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"loops":[{"workspace_id":"ws-a","name":"repair","version":3,"description":"Repair loop","source":"workspace","catalog":{"use_when":"repair"},"inputs":{"brief":{"type":"string"}},"contract":{},"last_run":{"id":"lr-a","status":"failed","created_at":"2026-11-10T00:00:00Z"},"aggregate_30d":{"runs":1,"succeeded":0,"failed":1},"success_rate_30d":0}],"facets":{"kinds":{},"categories":{},"statuses":{"failed":1}},"page":{"next_cursor":"cursor-a","has_more":true,"total":2,"limit":1}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_list",
				Name:   "failure",
				MustKeep: []string{
					"loops.0.workspace_id",
					"loops.0.name",
					"loops.0.last_run.status",
					"page.next_cursor",
					"page.has_more",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopsResponse]{Compact: viewLoopList}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.LoopsResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"loops":[],"facets":{"kinds":{},"categories":{},"statuses":{}},"page":{"next_cursor":"cursor-a","has_more":true,"total":2,"limit":1}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_list",
				Name:     "empty",
				MustKeep: []string{"loops", "page.next_cursor"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopsResponse]{Compact: viewLoopList}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.RunLoopResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"run":{"id":"lr-a","profile_id":"prof-a","profile_name":"Default","workspace_id":"ws-a","loop_name":"repair","status":"blocked","historical":false,"completion_state":"partial","generation":2,"best_generation":1,"best_score":0.5,"forks":[],"created_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","last_progress_at":"2026-11-10T00:00:00Z","active_gate_id":"verify","definition_version":3,"definition_digest":"sha256:definition","budget_approval_seq":2,"iteration_cap":4,"budget_tokens":10000,"budget_wall_sec":300,"budget_on_exceeded":"halt","tokens_used":10000,"pause_requested":true,"inputs":{"brief":"authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text "},"start_metadata":{"source":"authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata "},"attention":{"kind":"request","count":1,"since":"2026-11-10T00:00:00Z"},"progress":{"round":2,"steps_done":1,"steps_total":3}},"web_url":"http://localhost/loop-runs/lr-a"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_run",
				Name:   "failure",
				MustKeep: []string{
					"run.id",
					"run.iteration_cap",
					"run.budget_tokens",
					"run.definition_digest",
					"run.budget_approval_seq",
					"run.attention.kind",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.RunLoopResponse]{Compact: viewLoopStart}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.RunLoopResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"dry_run":{"loop_name":"repair","resolved_inputs":{"brief":"requested content"},"input_origins":{"brief":"run"},"generation":1,"nodes":[],"contract":{},"materialized_contract":{},"effective_config":{"iteration_cap":3,"reattempt_strategy":"full_body"}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_run",
				Name:   "dry_run",
				MustKeep: []string{
					"dry_run",
					"dry_run.effective_config.iteration_cap",
					"dry_run.effective_config.reattempt_strategy",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.RunLoopResponse]{Compact: viewLoopStart}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.ForkLoopResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"run":{"id":"lr-a","profile_id":"prof-a","profile_name":"Default","workspace_id":"ws-a","loop_name":"repair","status":"blocked","historical":false,"completion_state":"partial","generation":2,"best_generation":1,"best_score":0.5,"forks":[],"created_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","last_progress_at":"2026-11-10T00:00:00Z","active_gate_id":"verify","definition_version":3,"definition_digest":"sha256:definition","budget_approval_seq":2,"iteration_cap":4,"budget_tokens":10000,"budget_wall_sec":300,"budget_on_exceeded":"halt","tokens_used":10000,"pause_requested":true,"inputs":{"brief":"authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text "},"start_metadata":{"source":"authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata "},"attention":{"kind":"request","count":1,"since":"2026-11-10T00:00:00Z"},"progress":{"round":2,"steps_done":1,"steps_total":3}},"replayed":true}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_fork",
				Name:     "failure",
				MustKeep: []string{"run.id", "run.status", "run.iteration_cap", "replayed"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.ForkLoopResponse]{Compact: viewLoopFork}).result(view, &payload)
				},
			},
		)
	}
	{
		var payload contract.LoopRunsResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"runs":[{"id":"lr-a","profile_id":"prof-a","profile_name":"Default","workspace_id":"ws-a","loop_name":"repair","status":"blocked","historical":false,"completion_state":"partial","generation":2,"best_generation":1,"best_score":0.5,"forks":[],"created_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","last_progress_at":"2026-11-10T00:00:00Z","active_gate_id":"verify","definition_version":3,"definition_digest":"sha256:definition","budget_approval_seq":2,"iteration_cap":4,"budget_tokens":10000,"budget_wall_sec":300,"budget_on_exceeded":"halt","tokens_used":10000,"pause_requested":true,"inputs":{"brief":"authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text "},"start_metadata":{"source":"authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata "},"attention":{"kind":"request","count":1,"since":"2026-11-10T00:00:00Z"},"progress":{"round":2,"steps_done":1,"steps_total":3}}],"aggregates":{"total":1,"live":1,"terminal":0,"succeeded":0,"failed":0,"historical":0},"next_cursor":"cursor-a"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_runs",
				Name:   "failure",
				MustKeep: []string{
					"runs.0.id",
					"runs.0.status",
					"runs.0.budget_tokens",
					"runs.0.definition_digest",
					"aggregates",
					"next_cursor",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopRunsResponse]{Compact: viewLoopRuns}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.LoopRunsResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"runs":[],"aggregates":{"total":0,"live":0,"terminal":0,"succeeded":0,"failed":0,"historical":0},"next_cursor":"cursor-a"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_runs",
				Name:     "empty",
				MustKeep: []string{"runs", "aggregates", "next_cursor"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopRunsResponse]{Compact: viewLoopRuns}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.LoopRunResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"run":{"id":"lr-a","profile_id":"prof-a","profile_name":"Default","workspace_id":"ws-a","loop_name":"repair","status":"blocked","historical":false,"completion_state":"partial","generation":2,"best_generation":1,"best_score":0.5,"forks":[],"created_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","last_progress_at":"2026-11-10T00:00:00Z","active_gate_id":"verify","definition_version":3,"definition_digest":"sha256:definition","budget_approval_seq":2,"iteration_cap":4,"budget_tokens":10000,"budget_wall_sec":300,"budget_on_exceeded":"halt","tokens_used":10000,"pause_requested":true,"inputs":{"brief":"authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text authored input text "},"start_metadata":{"source":"authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata authored metadata "},"attention":{"kind":"request","count":1,"since":"2026-11-10T00:00:00Z"},"progress":{"round":2,"steps_done":1,"steps_total":3}},"materialized_contract":{},"executed_definition":{"meta":{"name":"repair"}},"effective_config":{"sources":{"iteration_cap":"definition"}},"generations":[{"generation":2,"parent_generation":1,"origin":"reattempt","route_causes":[],"verdicts":[{"gate_id":"verify","item_index":1,"outcome":"blocked","blocking_issues":[{"id":"proof","note":"test failed"}],"criteria":[{"id":"test","type":"shell","outcome":"error","passed":false,"stderr":"test failed"}]}],"outputs":[{"generation":2,"node_id":"verify","item_index":1,"status":"failed","task_run_id":"tr-a","session_id":"sess-a","child_loop_run_id":"lr-child","attempt":3,"next_attempt_at":"2026-11-10T00:00:00Z","failure_class":"transient","disposition":"quarantine","resolved_runtime":{"provider":"codex"}}]}],"node_controls":[{"loop_run_id":"lr-a","node_id":"verify","paused":true,"quarantined":true,"quarantine_entry":{"failure":"dependency unavailable"},"attention_flag":"blocked","attention_reason":"repair dependency","attention_producer_node_id":"build","revision":3,"death_resume_streak":2,"updated_at":"2026-11-10T00:00:00Z","pause_provenance":{"actor_kind":"operator","actor_id":"u-a","reason":"blocked","requested_at":"2026-11-10T00:00:00Z"}}],"waits":[{"loop_run_id":"lr-a","generation":2,"node_id":"verify","item_index":1,"kind":"manual","resume_at":"2026-11-10T00:00:00Z","next_escalation_at":"2026-11-10T00:00:00Z","escalation_cursor":1,"claim_state":"pending","admission_failures":2,"expect":{"type":"object"},"issued_epoch":3,"created_at":"2026-11-10T00:00:00Z","age_seconds":30}],"requests":[{"profile_id":"prof-a","workspace_id":"ws-a","loop_run_id":"lr-a","loop_name":"repair","generation":2,"node_id":"verify","item_index":1,"kind":"review","state":"pending","prompt":"Approve the repair","context":{"review":"context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context "},"expect":{"type":"object"},"edit_schema":{"type":"object"},"respond_schema":{"type":"object"},"proposed_preview":{"patch":"preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview "},"decisions":["approve","reject"],"agents":"allow","opened_at":"2026-11-10T00:00:00Z","expires_at":"2026-11-10T00:00:00Z"}],"amendments":[{"loop_run_id":"lr-a","generation":2,"node_id":"verify","item_index":1,"amendment_seq":3,"original_summary":{"byte_size":20000,"content_hash":"sha256:old"},"amended_summary":{"byte_size":20000,"content_hash":"sha256:new"},"actor_kind":"operator","actor_id":"u-a","reason":"repair blocked output","created_at":"2026-11-10T00:00:00Z"}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		payload.Generations[0].Verdicts = append(payload.Generations[0].Verdicts, contract.LoopGateVerdictPayload{
			GateID:  "approve-release",
			Outcome: contract.LoopGateVerdictAwaitingApproval,
			BlockingIssues: []contract.LoopGateBlockingIssuePayload{{
				ID: "human_pending", Note: "human decision is pending",
			}},
			Criteria: []contract.LoopGateCriterionDetailPayload{{
				ID:      "operator",
				Type:    "human",
				Outcome: contract.LoopGateVerdictAwaitingApproval,
				Prompt:  "Approve release of build 42?",
				BlockingIssues: []contract.LoopGateBlockingIssuePayload{{
					ID: "human_pending", Note: "human decision is pending",
				}},
			}},
		})
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_status",
				Name:   "failure",
				MustKeep: []string{
					"run.id",
					"run.iteration_cap",
					"run.definition_digest",
					"generations.0.verdicts.0.blocking_issues.0.note",
					"generations.0.verdicts.0.criteria.0.stderr",
					"generations.0.verdicts.1.criteria.0.prompt",
					"generations.0.verdicts.1.criteria.0.blocking_issues.0.note",
					"generations.0.outputs.0.failure_class",
					"generations.0.outputs.0.session_id",
					"node_controls.0.revision",
					"node_controls.0.quarantine_entry",
					"waits.0.node_id",
					"waits.0.admission_failures",
					"requests.0.expires_at",
					"amendments.0.amendment_seq",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*contract.LoopRunResponse]{Compact: viewLoopStatus}).result(view, &payload)
				},
			},
		)
	}
	{
		var payload contract.LoopNodeInventoryResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"items":[{"workspace_id":"ws-a","state":"quarantined","loop_run_id":"lr-a","loop_name":"repair","generation":2,"node_id":"verify","item_index":1,"state_at":"2026-11-10T00:00:00Z","control":{"loop_run_id":"lr-a","node_id":"verify","paused":true,"quarantined":true,"quarantine_entry":{"failure":"dependency unavailable"},"attention_flag":"blocked","attention_reason":"repair dependency","attention_producer_node_id":"build","revision":3,"death_resume_streak":2,"updated_at":"2026-11-10T00:00:00Z","pause_provenance":{"actor_kind":"operator","actor_id":"u-a","reason":"blocked","requested_at":"2026-11-10T00:00:00Z"}},"wait":{"loop_run_id":"lr-a","generation":2,"node_id":"verify","item_index":1,"kind":"manual","resume_at":"2026-11-10T00:00:00Z","next_escalation_at":"2026-11-10T00:00:00Z","escalation_cursor":1,"claim_state":"pending","admission_failures":2,"expect":{"type":"object"},"issued_epoch":3,"created_at":"2026-11-10T00:00:00Z","age_seconds":30},"output":{"generation":2,"node_id":"verify","item_index":1,"status":"failed","task_run_id":"tr-a","session_id":"sess-a","child_loop_run_id":"lr-child","attempt":3,"next_attempt_at":"2026-11-10T00:00:00Z","failure_class":"transient","disposition":"quarantine","resolved_runtime":{"provider":"codex"}}}],"next_cursor":"cursor-a"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_nodes",
				Name:   "failure",
				MustKeep: []string{
					"items.0.node_id",
					"items.0.workspace_id",
					"items.0.output.session_id",
					"items.0.control.revision",
					"items.0.control.quarantine_entry",
					"items.0.wait.admission_failures",
					"items.0.output.failure_class",
					"next_cursor",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopNodeInventoryResponse]{Compact: viewLoopNodes}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload contract.LoopNodeInventoryResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(`{"items":[],"next_cursor":"cursor-a"}`),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_nodes",
				Name:     "empty",
				MustKeep: []string{"items", "next_cursor"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopNodeInventoryResponse]{Compact: viewLoopNodes}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload contract.LoopRequestsResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"items":[{"profile_id":"prof-a","workspace_id":"ws-a","loop_run_id":"lr-a","loop_name":"repair","generation":2,"node_id":"verify","item_index":1,"kind":"review","state":"pending","prompt":"Approve the repair","context":{"review":"context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context context "},"expect":{"type":"object"},"edit_schema":{"type":"object"},"respond_schema":{"type":"object"},"proposed_preview":{"patch":"preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview preview "},"decisions":["approve","reject"],"agents":"allow","opened_at":"2026-11-10T00:00:00Z","expires_at":"2026-11-10T00:00:00Z"}],"aggregates":{"pending":1},"next_cursor":"cursor-a"}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_requests",
				Name:   "failure",
				MustKeep: []string{
					"items.0.loop_run_id",
					"items.0.node_id",
					"items.0.item_index",
					"items.0.expires_at",
					"items.0.prompt",
					"aggregates",
					"next_cursor",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopRequestsResponse]{Compact: viewLoopRequests}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.LoopRequestsResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(`{"items":[],"aggregates":{"pending":0},"next_cursor":"cursor-a"}`),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__loop_requests",
				Name:     "empty",
				MustKeep: []string{"items", "aggregates", "next_cursor"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopRequestsResponse]{Compact: viewLoopRequests}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.LoopNodeAmendResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"ok":true,"amendment":{"loop_run_id":"lr-a","generation":2,"node_id":"verify","item_index":1,"amendment_seq":3,"original_summary":{"byte_size":20000,"content_hash":"sha256:old"},"amended_summary":{"byte_size":20000,"content_hash":"sha256:new"},"actor_kind":"operator","actor_id":"u-a","reason":"repair blocked output","created_at":"2026-11-10T00:00:00Z"}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__loop_node_amend",
				Name:   "populated",
				MustKeep: []string{
					"ok",
					"amendment.amendment_seq",
					"amendment.reason",
					"amendment.original_summary.content_hash",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopNodeAmendResponse]{Compact: viewLoopAmend}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.LoopNodeAmendResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"ok":true,"amendment":{"loop_run_id":"lr-a","generation":2,"node_id":"verify","item_index":1,"amendment_seq":3,"original":{"text":"Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output Original authored output "},"amended":{"text":"Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output Amended authored output "},"actor_kind":"human","actor_id":"u-a","reason":"repair blocked output","created_at":"2026-11-10T00:00:00Z"}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   toolspkg.ToolIDLoopNodeAmend,
				Name:     "inline amendment",
				MustKeep: []string{"amendment.amendment_seq", "amendment.reason"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.LoopNodeAmendResponse]{Compact: viewLoopAmend}).result(view, payload)
				},
			},
		)
	}
	return rows
}

func decodeLoopGoalTaskResultViewFixture(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}
