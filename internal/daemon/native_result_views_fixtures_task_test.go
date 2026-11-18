package daemon

import (
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesTask(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload taskRecordResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"task":{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","description":"Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description ","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":{"reason":"blocked","at":"2026-11-10T00:00:00Z"},"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","metadata":{"brief":"Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata "}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_create",
				Name:   "failure",
				MustKeep: []string{
					"task.id",
					"task.max_attempts",
					"task.latest_event_seq",
					"task.paused_reason",
					"task.needs_attention",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskRecordResult]{Compact: viewTaskRecordResult}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskRecordResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"task":{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","description":"Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description ","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":{"reason":"blocked","at":"2026-11-10T00:00:00Z"},"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","metadata":{"brief":"Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata "}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_child_create",
				Name:   "failure",
				MustKeep: []string{
					"task.id",
					"task.max_attempts",
					"task.latest_event_seq",
					"task.paused_reason",
					"task.needs_attention",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskRecordResult]{Compact: viewTaskRecordResult}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskRecordResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"task":{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","description":"Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description ","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":{"reason":"blocked","at":"2026-11-10T00:00:00Z"},"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","metadata":{"brief":"Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata "}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_update",
				Name:   "failure",
				MustKeep: []string{
					"task.id",
					"task.max_attempts",
					"task.latest_event_seq",
					"task.paused_reason",
					"task.needs_attention",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskRecordResult]{Compact: viewTaskRecordResult}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskRecordResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"task":{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","description":"Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description ","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":{"reason":"blocked","at":"2026-11-10T00:00:00Z"},"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","metadata":{"brief":"Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata "}}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_cancel",
				Name:   "failure",
				MustKeep: []string{
					"task.id",
					"task.max_attempts",
					"task.latest_event_seq",
					"task.paused_reason",
					"task.needs_attention",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskRecordResult]{Compact: viewTaskRecordResult}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskRecoverResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"task":{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","description":"Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description ","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":true,"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","metadata":{"brief":"Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata "},"profile_name":"Default","needs_attention_reason":"dependency failed","effective_paused":true,"paused_by_task_id":"task-parent","blocked_reasons":[{"source":"dependency","reason":"dependency failed","depends_on_task_ids":["task-parent"]}]}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_recover",
				Name:   "failure",
				MustKeep: []string{
					"task.id",
					"task.max_attempts",
					"task.latest_event_seq",
					"task.blocked_reasons",
					"task.effective_paused",
					"task.needs_attention_reason",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*taskRecoverResult]{Compact: viewTaskRecover}).result(view, &payload)
				},
			},
		)
	}
	{
		var payload taskReadResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"task":{"summary":{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":true,"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","profile_name":"Default","needs_attention_reason":"dependency failed","effective_paused":true,"paused_by_task_id":"task-parent","blocked_reasons":[{"source":"dependency","reason":"dependency failed","depends_on_task_ids":["task-parent"]}]},"task":{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","description":"Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description Authored work description ","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":true,"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","metadata":{"brief":"Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata Authored metadata "},"profile_name":"Default","needs_attention_reason":"dependency failed","effective_paused":true,"paused_by_task_id":"task-parent","blocked_reasons":[{"source":"dependency","reason":"dependency failed","depends_on_task_ids":["task-parent"]}]},"children":[{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"paused":true,"paused_reason":"dependency failed","needs_attention":true,"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","profile_name":"Default","needs_attention_reason":"dependency failed","effective_paused":true,"paused_by_task_id":"task-parent","blocked_reasons":[{"source":"dependency","reason":"dependency failed","depends_on_task_ids":["task-parent"]}]}],"dependencies":[],"runs":[{"id":"tr-a","profile_id":"prof-a","task_id":"task-a","status":"failed","attempt":3,"recovery_count":1,"previous_run_id":"tr-old","failure_kind":"execution","error":"provider unavailable","session_id":"sess-a","worktree_id":"wt-a","resolved_worktree_mode":"dedicated","resolved_worktree_ref":"branch-a","designation_group_id":"tdg-a","lease_until":"2026-11-10T00:00:00Z","heartbeat_at":"2026-11-10T00:00:00Z","queued_at":"2026-11-10T00:00:00Z","claimed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","ended_at":"2026-11-10T00:00:00Z","result":{"disposition":"needs-approval","cause":"goal_reseed_confirmation_required"},"result_bytes":90,"metadata":{"brief":"Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation "},"origin":{"kind":"operator","ref":"u-a"}}],"events":[{"id":"event-a","event_type":"updated","timestamp":"2026-11-10T00:00:00Z","payload":{"detail":"Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit Event audit "}}]}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_read",
				Name:   "failure",
				MustKeep: []string{
					"task.summary.blocked_reasons",
					"task.task.description",
					"task.task.metadata",
					"task.runs.0.error",
					"task.runs.0.result",
					"task.children.0.paused_reason",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[*taskReadResult]{Compact: viewTaskRead}).result(view, &payload)
				},
			},
		)
	}
	{
		var payload contract.TasksResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"tasks":[{"id":"task-a","profile_id":"prof-a","identifier":"CY-1","scope":"workspace","workspace_id":"ws-a","parent_task_id":"task-parent","title":"Repair validation","priority":"high","max_attempts":3,"status":"blocked","approval_policy":"manual","approval_state":"pending","owner":{"kind":"agent","ref":"worker"},"current_run_id":"tr-a","latest_event_seq":8,"needs_attention":true,"created_by":{"kind":"operator","ref":"u-a"},"origin":{"kind":"operator","ref":"u-a"},"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","closed_at":"2026-11-10T00:00:00Z","profile_name":"Default","needs_attention_reason":"dependency failed","active_run":{"id":"tr-a","task_id":"task-a","status":"failed","attempt":3,"recovery_count":1,"previous_run_id":"tr-old","failure_kind":"execution","error":"provider unavailable","session_id":"sess-a","worktree_id":"wt-a","resolved_worktree_mode":"dedicated","resolved_worktree_ref":"branch-a","lease_until":"2026-11-10T00:00:00Z","heartbeat_at":"2026-11-10T00:00:00Z","queued_at":"2026-11-10T00:00:00Z","claimed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","ended_at":"2026-11-10T00:00:00Z"}}],"page":{"next_cursor":"cursor-a","has_more":true,"total":2,"limit":1},"facets":{"statuses":[],"owners":[]}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_list",
				Name:   "failure",
				MustKeep: []string{
					"tasks.0.id",
					"tasks.0.max_attempts",
					"tasks.0.latest_event_seq",
					"tasks.0.needs_attention_reason",
					"tasks.0.active_run.error",
					"page.next_cursor",
					"page.has_more",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.TasksResponse]{Compact: viewTaskList}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.TasksResponse
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"tasks":[],"page":{"next_cursor":"cursor-a","has_more":true,"total":2,"limit":1},"facets":{"statuses":[],"owners":[]}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__task_list",
				Name:     "empty",
				MustKeep: []string{"tasks", "page.next_cursor"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.TasksResponse]{Compact: viewTaskList}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskRunsResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"runs":[{"id":"tr-a","profile_id":"prof-a","workspace_id":"ws-a","task_id":"task-a","status":"failed","attempt":3,"recovery_count":1,"loop_run_id":"lr-a","previous_run_id":"tr-old","failure_kind":"execution","error":"provider unavailable","session_id":"sess-a","worktree_id":"wt-a","resolved_worktree_mode":"dedicated","resolved_worktree_ref":"branch-a","designation_group_id":"tdg-a","lease_until":"2026-11-10T00:00:00Z","heartbeat_at":"2026-11-10T00:00:00Z","queued_at":"2026-11-10T00:00:00Z","claimed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","ended_at":"2026-11-10T00:00:00Z","tokens_used":20,"result":{"disposition":"needs-approval","cause":"goal_reseed_confirmation_required"},"result_bytes":90,"metadata":{"brief":"Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation "},"origin":{"kind":"operator","ref":"u-a"}}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_run_list",
				Name:   "failure",
				MustKeep: []string{
					"runs.0.id",
					"runs.0.failure_kind",
					"runs.0.error",
					"runs.0.result.cause",
					"runs.0.lease_until",
					"runs.0.worktree_id",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskRunsResult]{Compact: viewTaskRuns}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskRunsResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(`{"runs":[]}`),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__task_run_list",
				Name:     "empty",
				MustKeep: []string{"runs"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskRunsResult]{Compact: viewTaskRuns}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskFanoutResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"designation_group_id":"tdg-a","runs":[{"id":"tr-a","profile_id":"prof-a","workspace_id":"ws-a","task_id":"task-a","status":"failed","attempt":3,"recovery_count":1,"loop_run_id":"lr-a","previous_run_id":"tr-old","failure_kind":"execution","error":"provider unavailable","session_id":"sess-a","worktree_id":"wt-a","resolved_worktree_mode":"dedicated","resolved_worktree_ref":"branch-a","designation_group_id":"tdg-a","lease_until":"2026-11-10T00:00:00Z","heartbeat_at":"2026-11-10T00:00:00Z","queued_at":"2026-11-10T00:00:00Z","claimed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","ended_at":"2026-11-10T00:00:00Z","tokens_used":20,"result":{"disposition":"needs-approval","cause":"goal_reseed_confirmation_required"},"result_bytes":90,"metadata":{"brief":"Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation "},"origin":{"kind":"operator","ref":"u-a"}}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__task_fanout_runs",
				Name:     "failure",
				MustKeep: []string{"designation_group_id", "runs.0.id", "runs.0.error", "runs.0.result.cause"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskFanoutResult]{Compact: viewTaskFanout}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskFanoutResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(`{"designation_group_id":"tdg-a","runs":[]}`),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__task_fanout_runs",
				Name:     "empty",
				MustKeep: []string{"designation_group_id", "runs"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskFanoutResult]{Compact: viewTaskFanout}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskReviewRequestResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"review":{"review_id":"review-a","task_id":"task-a","run_id":"tr-a","parent_review_id":"review-old","policy":"always","review_round":2,"attempt":3,"status":"recorded","outcome":"rejected","delivery_id":"delivery-a","confidence":0.9,"reason":"Missing integration proof","missing_work":{"files":["proof.txt"]},"next_round_guidance":"Run integration validation","review_text":"Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review ","reviewer_session_id":"sess-review","reviewer_agent_name":"reviewer","requested_at":"2026-11-10T00:00:00Z","deadline_at":"2026-11-10T00:00:00Z","reviewed_at":"2026-11-10T00:00:00Z","routed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"},"created":true}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_run_review_request",
				Name:   "failure",
				MustKeep: []string{
					"review.review_id",
					"review.deadline_at",
					"review.reason",
					"review.delivery_id",
					"review.missing_work",
					"review.next_round_guidance",
					"created",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskReviewRequestResult]{Compact: viewTaskReviewRequest}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskReviewsResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"reviews":[{"review_id":"review-a","task_id":"task-a","run_id":"tr-a","parent_review_id":"review-old","policy":"always","review_round":2,"attempt":3,"status":"recorded","outcome":"rejected","delivery_id":"delivery-a","confidence":0.9,"reason":"Missing integration proof","missing_work":{"files":["proof.txt"]},"next_round_guidance":"Run integration validation","review_text":"Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review ","reviewer_session_id":"sess-review","reviewer_agent_name":"reviewer","requested_at":"2026-11-10T00:00:00Z","deadline_at":"2026-11-10T00:00:00Z","reviewed_at":"2026-11-10T00:00:00Z","routed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_run_review_list",
				Name:   "failure",
				MustKeep: []string{
					"reviews.0.review_id",
					"reviews.0.deadline_at",
					"reviews.0.reason",
					"reviews.0.delivery_id",
					"reviews.0.missing_work",
					"reviews.0.next_round_guidance",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskReviewsResult]{Compact: viewTaskReviews}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskReviewsResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(`{"reviews":[]}`),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__task_run_review_list",
				Name:     "empty",
				MustKeep: []string{"reviews"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskReviewsResult]{Compact: viewTaskReviews}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskReviewSubmitResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"review":{"review_id":"review-a","task_id":"task-a","run_id":"tr-a","parent_review_id":"review-old","policy":"always","review_round":2,"attempt":3,"status":"recorded","outcome":"rejected","delivery_id":"delivery-a","confidence":0.9,"reason":"Missing integration proof","missing_work":{"files":["proof.txt"]},"next_round_guidance":"Run integration validation","review_text":"Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review ","reviewer_session_id":"sess-review","reviewer_agent_name":"reviewer","requested_at":"2026-11-10T00:00:00Z","deadline_at":"2026-11-10T00:00:00Z","reviewed_at":"2026-11-10T00:00:00Z","routed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"},"continuation_run":{"id":"tr-a","profile_id":"prof-a","workspace_id":"ws-a","task_id":"task-a","status":"failed","attempt":3,"recovery_count":1,"loop_run_id":"lr-a","previous_run_id":"tr-old","failure_kind":"execution","error":"provider unavailable","session_id":"sess-a","worktree_id":"wt-a","resolved_worktree_mode":"dedicated","resolved_worktree_ref":"branch-a","designation_group_id":"tdg-a","lease_until":"2026-11-10T00:00:00Z","heartbeat_at":"2026-11-10T00:00:00Z","queued_at":"2026-11-10T00:00:00Z","claimed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","ended_at":"2026-11-10T00:00:00Z","tokens_used":20,"result":{"disposition":"needs-approval","cause":"goal_reseed_confirmation_required"},"result_bytes":90,"metadata":{"brief":"Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation Authored designation "},"origin":{"kind":"operator","ref":"u-a"}},"circuit_opened":true}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__task_run_review_submit",
				Name:   "failure",
				MustKeep: []string{
					"review.review_id",
					"review.reason",
					"review.delivery_id",
					"review.missing_work",
					"review.next_round_guidance",
					"continuation_run.id",
					"continuation_run.result.cause",
					"circuit_opened",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskReviewSubmitResult]{Compact: viewTaskReviewSubmit}).result(view, payload)
				},
			},
		)
	}
	{
		var payload taskReviewSubmitResult
		if err := decodeLoopGoalTaskResultViewFixture(
			[]byte(
				`{"review":{"review_id":"review-a","task_id":"task-a","run_id":"tr-a","parent_review_id":"review-old","policy":"always","review_round":2,"attempt":3,"status":"recorded","outcome":"rejected","delivery_id":"delivery-a","confidence":0.9,"reason":"Missing integration proof","missing_work":{"files":["proof.txt"]},"next_round_guidance":"Run integration validation","review_text":"Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review Detailed review ","reviewer_session_id":"sess-review","reviewer_agent_name":"reviewer","requested_at":"2026-11-10T00:00:00Z","deadline_at":"2026-11-10T00:00:00Z","reviewed_at":"2026-11-10T00:00:00Z","routed_at":"2026-11-10T00:00:00Z","started_at":"2026-11-10T00:00:00Z","created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"},"continuation_run":null,"circuit_opened":false}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__task_run_review_submit",
				Name:     "no_continuation",
				MustKeep: []string{"review", "continuation_run", "circuit_opened"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[taskReviewSubmitResult]{Compact: viewTaskReviewSubmit}).result(view, payload)
				},
			},
		)
	}
	return rows
}
