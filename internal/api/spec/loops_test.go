package spec

import (
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestLoopOpenAPIContract(t *testing.T) {
	t.Parallel()

	t.Run("Should register the complete typed input vocabularies", func(t *testing.T) {
		t.Parallel()

		assertStringSetEqual(
			t,
			schemaEnumValues[reflect.TypeFor[dsl.InputType]()],
			contract.LoopInputTypeValues(),
		)
		assertStringSetEqual(
			t,
			schemaEnumValues[reflect.TypeFor[dsl.InputRefKind]()],
			contract.LoopInputRefKindValues(),
		)
		assertStringSetEqual(
			t,
			schemaEnumValues[reflect.TypeFor[dsl.EntityKind]()],
			contract.LoopEntityKindValues(),
		)
	})

	t.Run("Should keep zero-omitted Loop fields optional", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		loopOperation := operationFor(t, doc, "/api/workspaces/{workspace_id}/loops/{name}", "GET")
		loopSchema := jsonResponseSchema(t, loopOperation, 200)
		definition := propertySchema(t, propertySchema(t, loopSchema, "loop"), "definition")
		loopContract := propertySchema(t, definition, "contract")
		assertNotRequired(t, loopContract, "stop_when")

		diffOperation := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/loop-runs/{run_id}/diff",
			"GET",
		)
		diffSchema := jsonResponseSchema(t, diffOperation, 200)
		nodes := propertySchema(t, diffSchema, "nodes")
		if nodes.Items == nil || nodes.Items.Value == nil {
			t.Fatal("Loop diff nodes have no item schema")
		}
		assertNotRequired(t, nodes.Items.Value, "base", "against")
	})

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
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "get input defaults",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/input-defaults",
				method:     "GET",
				statuses:   []int{200, 400, 404, 410, 503, 500},
				parameters: []string{"workspace_id", "name", "scope"},
			},
			{
				name:       "replace input defaults",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/input-defaults",
				method:     "PUT",
				statuses:   []int{200, 400, 404, 410, 422, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "get input default",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/input-defaults/{key}",
				method:     "GET",
				statuses:   []int{200, 400, 404, 410, 503, 500},
				parameters: []string{"workspace_id", "name", "key", "scope"},
			},
			{
				name:       "set input default",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/input-defaults/{key}",
				method:     "PUT",
				statuses:   []int{200, 400, 404, 410, 422, 503, 500},
				parameters: []string{"workspace_id", "name", "key"},
			},
			{
				name:       "delete input default",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/input-defaults/{key}",
				method:     "DELETE",
				statuses:   []int{200, 400, 404, 410, 503, 500},
				parameters: []string{"workspace_id", "name", "key", "scope"},
			},
			{
				name:       "get annotations",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/annotations",
				method:     "GET",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:       "put annotations",
				path:       "/api/workspaces/{workspace_id}/loops/{name}/annotations",
				method:     "PUT",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "name"},
			},
			{
				name:     "list runs",
				path:     "/api/workspaces/{workspace_id}/loop-runs",
				method:   "GET",
				statuses: []int{200, 400, 503, 500},
				parameters: []string{
					"workspace_id", "loop", "status", "origin", "origin_session", "live", "cursor", "limit",
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
				name:       "get run nodes",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes",
				method:     "GET",
				statuses:   []int{200, 400, 404, 503, 500},
				parameters: []string{"workspace_id", "run_id", "state", "generation", "cursor", "limit"},
			},
			{
				name:       "get run briefing",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/briefing",
				method:     "GET",
				statuses:   []int{200, 404, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "get run timeline",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/timeline",
				method:     "GET",
				statuses:   []int{200, 400, 404, 409, 503, 500},
				parameters: []string{"workspace_id", "run_id", "view", "cursor", "limit", "after_sequence"},
			},
			{
				name:       "cancel run",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/cancel",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "kill run",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/kill",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id"},
			},
			{
				name:       "pause node",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/pause",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id", "node_id"},
			},
			{
				name:       "resume node",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/resume",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id", "node_id"},
			},
			{
				name:       "cancel node",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/cancel",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id", "node_id"},
			},
			{
				name:       "kill node",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/kill",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id", "node_id"},
			},
			{
				name:       "requeue node",
				path:       "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/requeue",
				method:     "POST",
				statuses:   []int{200, 400, 404, 409, 422, 503, 500},
				parameters: []string{"workspace_id", "run_id", "node_id"},
			},
			{
				name:     "list nodes",
				path:     "/api/workspaces/{workspace_id}/loop-nodes",
				method:   "GET",
				statuses: []int{200, 400, 503, 500},
				parameters: []string{
					"workspace_id", "state", "loop", "run_id", "cursor", "limit",
				},
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
					case "workspace_id", "name", "node_id", "session_id", "key":
						assertParameter(t, operation, parameter, openapi3.ParameterInPath, true)
					case "run_id":
						location := openapi3.ParameterInPath
						required := true
						if tc.name == "list nodes" {
							location = openapi3.ParameterInQuery
							required = false
						}
						assertParameter(t, operation, parameter, location, required)
					case "Last-Event-ID":
						assertParameter(t, operation, parameter, openapi3.ParameterInHeader, false)
					case "scope":
						assertParameter(t, operation, parameter, openapi3.ParameterInQuery, true)
					case "state":
						assertParameter(
							t, operation, parameter, openapi3.ParameterInQuery, tc.name == "list nodes",
						)
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
		runUnprocessable := jsonResponseSchema(t, runLoop, 422)
		inputValidation := propertySchema(t, runUnprocessable, "input_validation")
		assertRequired(t, inputValidation, "loop", "field", "origin", "reason")
		propertySchema(t, inputValidation, "kind")
		propertySchema(t, inputValidation, "value")
		runConfig := propertySchema(t, jsonRequestSchema(t, runLoop), "config_overrides")
		assertRequired(t, propertySchema(t, runConfig, "environment"), "mode")

		createLoop := operationFor(t, doc, "/api/workspaces/{workspace_id}/loops", "POST")
		definition := propertySchema(t, jsonRequestSchema(t, createLoop), "definition")
		graph := propertySchema(t, definition, "graph")
		nodes := propertySchema(t, graph, "nodes")
		if nodes.Items == nil || nodes.Items.Value == nil {
			t.Fatal("POST /loops definition graph nodes items are unresolved")
		}
		params := propertySchema(t, nodes.Items.Value, "params")
		assertRequired(t, propertySchema(t, params, "environment"), "mode")
		if _, ok := params.Properties["cwd"]; ok {
			t.Fatal("POST /loops definition graph node params exposes retired cwd")
		}

		getConfig := operationFor(t, doc, "/api/workspaces/{workspace_id}/loops/{name}/config", "GET")
		configResponse := jsonResponseSchema(t, getConfig, 200)
		assertRequired(t, configResponse, "config", "effective_config")
		if config := propertySchema(t, configResponse, "config"); !config.Nullable {
			t.Fatal("GET /config response config must be nullable")
		}
		config := propertySchema(t, configResponse, "config")
		environment := propertySchema(t, config, "environment")
		assertRequired(t, environment, "mode")
		assertEnumValues(
			t,
			propertySchema(t, environment, "mode"),
			"directory",
			"per_run",
			"root",
			"worktree",
		)
		effective := propertySchema(t, configResponse, "effective_config")
		assertRequired(t, effective, "environment")

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

	t.Run("Should expose metric graph criteria through the custom schema", func(t *testing.T) {
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
		criteria := propertySchema(t, nodes.Items.Value, "criteria")
		if criteria.Items == nil || criteria.Items.Value == nil {
			t.Fatal("Loop graph criteria have no item schema")
		}
		criterion := criteria.Items.Value
		propertySchema(t, criterion, "contains")
		metric := propertySchema(t, criterion, "metric")
		assertRequired(t, metric, "direction")
		assertNotRequired(t, metric, "min_delta")
		assertEnumValues(
			t,
			propertySchema(t, metric, "direction"),
			contract.LoopMetricDirectionValues()...,
		)
		propertySchema(t, metric, "min_delta")
	})

	t.Run("Should expose typed generation and gate verdict SSE payloads", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		operation := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/loop-runs/{run_id}/events",
			"GET",
		)
		stream := responseSchema(t, operation, 200, "text/event-stream")
		if len(stream.OneOf) != 3 {
			t.Fatalf("Loop SSE response oneOf = %#v, want three disjoint event envelopes", stream.OneOf)
		}

		generation := loopEventVariantSchema(t, stream, string(contract.LoopRunEventGenerationStarted))
		assertEnumValues(
			t,
			propertySchema(t, generation, "kind"),
			string(contract.LoopRunEventGenerationStarted),
		)
		generationPayload := propertySchema(t, generation, "payload")
		assertRequired(t, generationPayload, "generation", "parent_generation", "origin")

		verdict := loopEventVariantSchema(t, stream, string(contract.LoopRunEventGateVerdict))
		assertEnumValues(
			t,
			propertySchema(t, verdict, "kind"),
			string(contract.LoopRunEventGateVerdict),
		)
		verdictPayload := propertySchema(t, verdict, "payload")
		assertRequired(t, verdictPayload, "generation", "gate_id", "verdict")
		propertySchema(t, verdictPayload, "score")
		propertySchema(t, verdictPayload, "best_generation")

		other := loopOtherEventVariantSchema(t, stream)
		otherKinds := propertySchema(t, other, "kind")
		for _, migrated := range []string{
			string(contract.LoopRunEventGenerationStarted),
			string(contract.LoopRunEventGateVerdict),
		} {
			if slices.ContainsFunc(otherKinds.Enum, func(value any) bool { return value == migrated }) {
				t.Fatalf("generic Loop SSE event kind includes typed variant %q", migrated)
			}
		}
	})

	t.Run("Should describe Goal prompt outcomes inside the durable prompt envelope", func(t *testing.T) {
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
			if status == 200 || status == 202 {
				assertRequired(t, schema, "prompt")
				assertPropertyAbsent(t, schema, "outcome")
			} else if len(schema.OneOf) != 2 {
				t.Fatalf("prompt response %d oneOf = %#v, want error or prompt envelope", status, schema.OneOf)
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

	t.Run("Should expose every authored graph field through OpenAPI", func(t *testing.T) {
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
		for _, field := range serializedStructFields(reflect.TypeFor[dsl.Graph]()) {
			propertySchema(t, graph, field)
		}
		nodes := propertySchema(t, graph, "nodes")
		if nodes.Items == nil || nodes.Items.Value == nil {
			t.Fatal("Loop graph nodes have no item schema")
		}
		for _, field := range serializedStructFields(reflect.TypeFor[dsl.Node]()) {
			propertySchema(t, nodes.Items.Value, field)
		}
		strategy := propertySchema(t, nodes.Items.Value, "strategy")
		if len(strategy.OneOf) != 2 || strategy.OneOf[0].Value == nil || strategy.OneOf[1].Value == nil {
			t.Fatalf("Loop strategy oneOf = %#v, want shorthand and object variants", strategy.OneOf)
		}
		assertEnumValues(t, strategy.OneOf[0].Value,
			string(dsl.StrategyWaitAll),
			string(dsl.StrategyFailFast),
			string(dsl.StrategyRace),
		)
		strategyObject := strategy.OneOf[1].Value
		assertEnumValues(t, propertySchema(t, strategyObject, "kind"),
			string(dsl.StrategyWaitAll),
			string(dsl.StrategyFailFast),
			string(dsl.StrategyBestEffort),
			string(dsl.StrategyRace),
		)
		threshold := propertySchema(t, strategyObject, "threshold")
		if len(threshold.OneOf) != 2 || threshold.OneOf[0].Value == nil || threshold.OneOf[1].Value == nil {
			t.Fatalf("Loop strategy threshold oneOf = %#v, want percentage and count variants", threshold.OneOf)
		}
		if got := threshold.OneOf[0].Value.Pattern; got != `^[0-9]+%$` {
			t.Fatalf("Loop strategy percentage pattern = %q, want %q", got, `^[0-9]+%$`)
		}
		count := propertySchema(t, threshold.OneOf[1].Value, "count")
		if count.Min == nil || *count.Min != 1 {
			t.Fatalf("Loop strategy count minimum = %v, want 1", count.Min)
		}
		edges := propertySchema(t, graph, "edges")
		if edges.Items == nil || edges.Items.Value == nil {
			t.Fatal("Loop graph edges have no item schema")
		}
		for _, field := range serializedStructFields(reflect.TypeFor[dsl.Edge]()) {
			propertySchema(t, edges.Items.Value, field)
		}
	})

	t.Run("Should publish closed run-read filters and an int64 timeline sequence", func(t *testing.T) {
		t.Parallel()
		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		nodes := operationFor(t, doc, "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes", "GET")
		assertEnumValues(t, parameterSchema(t, nodes, "state", openapi3.ParameterInQuery),
			"all", "running", "queued", "waiting", "retrying", "paused", "quarantined",
			"succeeded", "failed", "canceled", "not_taken",
		)
		timeline := operationFor(t, doc, "/api/workspaces/{workspace_id}/loop-runs/{run_id}/timeline", "GET")
		assertEnumValues(t, parameterSchema(t, timeline, "view", openapi3.ParameterInQuery), "notable", "all")
		if got := parameterSchema(t, timeline, "after_sequence", openapi3.ParameterInQuery).Format; got != "int64" {
			t.Fatalf("after_sequence format = %q, want int64", got)
		}
	})
}

func assertStringSetEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("enum values = %v, want %v", got, want)
	}
}

func serializedStructFields(value reflect.Type) []string {
	fields := make([]string, 0, value.NumField())
	for field := range value.Fields() {
		tag := field.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" {
			embedded := field.Type
			if embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			fields = append(fields, serializedStructFields(embedded)...)
			continue
		}
		if name == "" {
			name = field.Name
		}
		fields = append(fields, name)
	}
	slices.Sort(fields)
	return fields
}

func loopEventVariantSchema(t *testing.T, response *openapi3.Schema, kind string) *openapi3.Schema {
	t.Helper()
	for _, candidate := range response.OneOf {
		if candidate == nil || candidate.Value == nil {
			continue
		}
		kindRef := candidate.Value.Properties["kind"]
		if kindRef != nil && kindRef.Value != nil &&
			slices.Equal(kindRef.Value.Enum, []any{kind}) {
			return candidate.Value
		}
	}
	t.Fatalf("Loop SSE response has no resolved %q variant: %#v", kind, response.OneOf)
	return nil
}

func loopOtherEventVariantSchema(t *testing.T, response *openapi3.Schema) *openapi3.Schema {
	t.Helper()
	for _, candidate := range response.OneOf {
		if candidate == nil || candidate.Value == nil {
			continue
		}
		kindRef := candidate.Value.Properties["kind"]
		if kindRef != nil && kindRef.Value != nil && len(kindRef.Value.Enum) > 1 {
			return candidate.Value
		}
	}
	t.Fatalf("Loop SSE response has no resolved generic event variant: %#v", response.OneOf)
	return nil
}

func promptGoalResultSchema(t *testing.T, response *openapi3.Schema) *openapi3.Schema {
	t.Helper()
	promptEnvelope := response
	for _, candidate := range response.OneOf {
		if candidate != nil && candidate.Value != nil && candidate.Value.Properties["prompt"] != nil {
			promptEnvelope = candidate.Value
			break
		}
	}
	prompt := propertySchema(t, promptEnvelope, "prompt")
	return propertySchema(t, prompt, "goal")
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
