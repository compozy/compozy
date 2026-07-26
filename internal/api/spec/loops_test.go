package spec

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestLoopOpenAPIContract(t *testing.T) {
	t.Parallel()

	t.Run("Should expose every Loop route with expected status bodies", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}

		tests := []struct {
			name       string
			path       string
			method     string
			statuses   []int
			parameters []string
		}{
			{
				name:       "list catalog",
				path:       "/api/workspaces/{workspace_id}/loops",
				method:     "GET",
				statuses:   []int{200, 400, 404, 410, 500, 503},
				parameters: []string{"workspace_id"},
			},
			{
				name:       "create catalog entry",
				path:       "/api/workspaces/{workspace_id}/loops",
				method:     "POST",
				statuses:   []int{201, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id"},
			},
			{
				name:       "inspect loop",
				path:       "/api/workspaces/{workspace_id}/loops/{name}",
				method:     "GET",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "publish loop",
				path:       "/api/workspaces/{workspace_id}/loops/{name}",
				method:     "PATCH",
				statuses:   []int{200, 400, 403, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "delete loop",
				path:       "/api/workspaces/{workspace_id}/loops/{name}",
				method:     "DELETE",
				statuses:   []int{204, 400, 403, 404, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "validate loop",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/validate",
				method:     "POST",
				statuses:   []int{200, 400, 422, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "run loop",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/run",
				method:     "POST",
				statuses:   []int{200, 201, 401, 400, 403, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "name", "dry"},
			},
			{
				name:       "get config",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/config",
				method:     "GET",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "put config",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/config",
				method:     "PUT",
				statuses:   []int{200, 400, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "get annotations",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/annotations",
				method:     "GET",
				statuses:   []int{200, 400, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "put annotations",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/annotations",
				method:     "PUT",
				statuses:   []int{200, 400, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:     "list runs",
				path:     "/api/workspaces/{workspace_id}/loop-runs",
				method:   "GET",
				statuses: []int{200, 400, 503, 500},
				parameters: []string{
					"workspace_id", "loop", "status", "origin", "origin_session", "live", "limit",
				},
			},
			{
				name:       "list goal turns",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/turns",
				method:     "GET",
				statuses:   []int{200, 400, 404, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id", "node", "item", "after_seq", "limit"},
			},
			{
				name:       "get session goal",
				path:       "/api/workspaces/{workspace_id}/sessions/{session_id}/goal",
				method:     "GET",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "session_id"},
			},
			{
				name:       "get run",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}",
				method:     "GET",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "stop run",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/stop",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "pause run",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/pause",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "resume run",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/resume",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "approve run",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/approve",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "stream events",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/events",
				method:     "GET",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "run_id", "after_sequence", "Last-Event-ID"},
			},
		}

		for _, tc := range tests {
			t.Run("Should describe "+tc.name, func(t *testing.T) {
				t.Parallel()

				operation := operationFor(t, doc, tc.path, tc.method)
				assertTagsContain(t, operation, specLoopsKey)
				assertLoopResponseStatusesExactly(t, operation, tc.statuses)
				for _, parameter := range tc.parameters {
					switch parameter {
					case "workspace_id", "name", "run_id", "session_id":
						assertParameter(t, operation, parameter, openapi3.ParameterInPath, true)
					case "Last-Event-ID":
						assertParameter(t, operation, parameter, openapi3.ParameterInHeader, false)
					default:
						assertParameter(t, operation, parameter, openapi3.ParameterInQuery, false)
					}
				}
			})
		}

		stream := operationFor(t, doc, "/api/workspaces/{workspace_id}/loop-runs/{run_id}/events", "GET")
		responseSchema(t, stream, 200, "text/event-stream")

		patchLoop := operationFor(t, doc, "/api/workspaces/{workspace_id}/loops/{name}", "PATCH")
		patchLintSchema := jsonResponseSchema(t, patchLoop, 422)
		assertRequired(t, patchLintSchema, "valid")
		propertySchema(t, patchLintSchema, "errors")

		runLoop := operationFor(t, doc, "/api/workspaces/{workspace_id}/loops/{name}/run", "POST")
		assertRequired(t, jsonResponseSchema(t, runLoop, 422), "error")

		pauseRun := operationFor(t, doc, "/api/workspaces/{workspace_id}/loop-runs/{run_id}/pause", "POST")
		assertRequired(t, jsonResponseSchema(t, pauseRun, 422), "error")
	})

	t.Run("Should co-ship automation Loop target additions", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}

		for _, op := range []*openapi3.Operation{
			operationFor(t, doc, "/api/automation/jobs", "GET"),
			operationFor(t, doc, "/api/automation/triggers", "GET"),
		} {
			assertParameter(t, op, "loop", openapi3.ParameterInQuery, false)
		}

		for _, op := range []*openapi3.Operation{
			operationFor(t, doc, "/api/automation/jobs", "POST"),
			operationFor(t, doc, "/api/automation/jobs/{id}", "PATCH"),
			operationFor(t, doc, "/api/automation/triggers", "POST"),
			operationFor(t, doc, "/api/automation/triggers/{id}", "PATCH"),
		} {
			assertResponseStatus(t, op, 422)
		}

		createJobSchema := jsonRequestSchema(t, operationFor(t, doc, "/api/automation/jobs", "POST"))
		assertNotRequired(t, createJobSchema, "target_kind", "loop_target")
		propertySchema(t, createJobSchema, "target_kind")
		loopTargetSchema := propertySchema(t, createJobSchema, "loop_target")
		assertRequired(t, loopTargetSchema, "workspace_id", "loop_name")

		updateTriggerSchema := jsonRequestSchema(t, operationFor(t, doc, "/api/automation/triggers/{id}", "PATCH"))
		assertNotRequired(t, updateTriggerSchema, "target_kind", "loop_target")
	})

	t.Run("Should expose watch-events subscriptions in Loop authoring requests", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		validate := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/loops/{name}/validate",
			"POST",
		)
		definition := propertySchema(t, jsonRequestSchema(t, validate), "definition")
		graph := propertySchema(t, definition, "graph")
		nodes := propertySchema(t, graph, "nodes")
		if nodes.Items == nil || nodes.Items.Value == nil {
			t.Fatal("Loop graph nodes have no item schema")
		}
		events := propertySchema(t, nodes.Items.Value, "events")
		if events.Items == nil || events.Items.Value == nil {
			t.Fatal("Loop watch-events subscriptions have no item schema")
		}
		subscription := events.Items.Value
		assertRequired(t, subscription, "kind")
		assertNotRequired(t, subscription, "filter")
		assertEnumValues(
			t,
			propertySchema(t, subscription, "kind"),
			contract.LoopWatchEventKindValues()...,
		)
		propertySchema(t, subscription, "filter")
	})

	t.Run("Should describe structured Goal prompt outcomes beside ordinary prompt results", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		operation := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/sessions/{session_id}/prompt",
			"POST",
		)
		assertLoopResponseStatusesExactly(t, operation, []int{
			200, 202, 400, 404, 409, 413, 422, 500, 503,
		})
		for _, status := range []int{200, 202, 404, 409, 422} {
			schema := jsonResponseSchema(t, operation, status)
			if len(schema.OneOf) != 2 {
				t.Fatalf("prompt response %d oneOf = %#v, want two contracts", status, schema.OneOf)
			}
			if !slices.ContainsFunc(schema.OneOf, func(ref *openapi3.SchemaRef) bool {
				return strings.Contains(ref.Ref, "GoalCommandResult") ||
					(ref.Value != nil && ref.Value.Properties["outcome"] != nil)
			}) {
				t.Fatalf("prompt response %d does not include GoalCommandResult: %#v", status, schema.OneOf)
			}
			goalResult := promptGoalResultSchema(t, schema)
			if reasonCode := propertySchema(t, goalResult, "reason_code"); !reasonCode.Nullable {
				t.Fatalf("prompt response %d reason_code is not nullable", status)
			}
			snapshot := propertySchema(t, goalResult, "snapshot")
			if !snapshot.Nullable {
				t.Fatalf("prompt response %d snapshot is not nullable", status)
			}
			if cause := propertySchema(t, snapshot, "cause"); !cause.Nullable {
				t.Fatalf("prompt response %d snapshot cause is not nullable", status)
			}
			assertEnumValues(
				t,
				propertySchema(t, goalResult, "outcome"),
				"started", "replaced", "status", "paused", "resumed", "cleared", "error",
			)
			assertEnumValues(
				t,
				propertySchema(t, snapshot, "status"),
				"active", "paused", "blocked", "usage-limited", "budget-limited", "complete",
			)
		}
	})

	t.Run("Should close Goal turn result, verdict, and ACP stop vocabularies", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		operation := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/loop-runs/{run_id}/turns",
			"GET",
		)
		turns := propertySchema(t, jsonResponseSchema(t, operation, 200), "turns")
		if turns.Items == nil || turns.Items.Value == nil {
			t.Fatal("Goal turns response has no item schema")
		}
		turn := turns.Items.Value
		assertEnumValues(t, propertySchema(t, turn, "result_status"),
			"completed", "invalid-result", "failed", "ambiguous")
		assertEnumValues(t, propertySchema(t, turn, "verdict_outcome"),
			"approved", "rejected", "awaiting_approval", "blocked", "error", "timeout", "invalid_output")
		assertEnumValues(t, propertySchema(t, turn, "stop_reason"),
			"end_turn", "max_tokens", "max_turn_requests", "refusal",
			string(contract.ACPStopReasonCancelled))
	})
}

func promptGoalResultSchema(t *testing.T, response *openapi3.Schema) *openapi3.Schema {
	t.Helper()
	for _, candidate := range response.OneOf {
		if candidate != nil && candidate.Value != nil && candidate.Value.Properties["outcome"] != nil {
			return candidate.Value
		}
	}
	t.Fatal("prompt response has no Goal command result schema")
	return nil
}

func assertLoopResponseStatusesExactly(t *testing.T, operation *openapi3.Operation, statuses []int) {
	t.Helper()

	want := make([]string, 0, len(statuses))
	for _, status := range statuses {
		want = append(want, strconv.Itoa(status))
	}
	sort.Strings(want)
	got := operation.Responses.Keys()
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("response statuses = %v, want %v", got, want)
	}
}
