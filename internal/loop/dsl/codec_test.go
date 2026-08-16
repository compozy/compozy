package dsl_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	"gopkg.in/yaml.v3"
)

// Invariant: strategy thresholds preserve their two authored forms exactly and reject every third form.
// The canonical DSL codec suite owns YAML/JSON round-trip behavior.
func TestCodecShouldRoundTripStrategyThresholds(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		authored string
		want     dsl.StrategyThreshold
	}{
		{name: "Should preserve a percent threshold", authored: "66%", want: dsl.StrategyThreshold{Kind: dsl.ThresholdPercent, Percent: 66}},
		{name: "Should preserve a count threshold", authored: "{count: 2}", want: dsl.StrategyThreshold{Kind: dsl.ThresholdCount, Count: 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var threshold dsl.StrategyThreshold
			if err := yaml.Unmarshal([]byte(tt.authored), &threshold); err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v", err)
			}
			if !reflect.DeepEqual(threshold, tt.want) {
				t.Fatalf("threshold = %#v, want %#v", threshold, tt.want)
			}
			encodedYAML, err := yaml.Marshal(threshold)
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}
			var yamlRoundTrip dsl.StrategyThreshold
			if err := yaml.Unmarshal(encodedYAML, &yamlRoundTrip); err != nil {
				t.Fatalf("yaml.Unmarshal(round trip) error = %v", err)
			}
			if !reflect.DeepEqual(yamlRoundTrip, tt.want) {
				t.Fatalf("YAML round trip = %#v, want %#v", yamlRoundTrip, tt.want)
			}
			encodedJSON, err := json.Marshal(threshold)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			var jsonRoundTrip dsl.StrategyThreshold
			if err := json.Unmarshal(encodedJSON, &jsonRoundTrip); err != nil {
				t.Fatalf("json.Unmarshal(round trip) error = %v", err)
			}
			if !reflect.DeepEqual(jsonRoundTrip, tt.want) {
				t.Fatalf("JSON round trip = %#v, want %#v", jsonRoundTrip, tt.want)
			}
		})
	}

	for _, authored := range []string{"0%", "101%", "{count: 0}", "{count: -1}", "{count: 2, percent: 50}", "{other: 2}", "sixty%"} {
		t.Run("Should reject "+authored, func(t *testing.T) {
			t.Parallel()
			var threshold dsl.StrategyThreshold
			if err := yaml.Unmarshal([]byte(authored), &threshold); err == nil {
				t.Fatalf("yaml.Unmarshal(%q) error = nil, want rejection", authored)
			}
		})
	}
}

func TestCodecShouldRoundTripNetworkParticipation(t *testing.T) {
	t.Parallel()

	body := minimalDefinition(`network_participation:
  mode: live
  channel_strategy: named
  channel_id: builders
  bounds:
    max_wakes: 3
    max_wake_wall_time: 30s
    max_total_wall_time: 2m
    max_input_tokens: 4096
    max_output_tokens: 2048
    max_wake_depth: 2
    coalesce_window: 250ms
contract:
  goal: Ship safely
  definition_of_done: Tests pass
  verification: []
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
`)

	definition, err := dsl.Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse(network participation) error = %v", err)
	}
	want := testNetworkParticipationRequest()
	if !reflect.DeepEqual(definition.NetworkParticipation, want) {
		t.Fatalf("NetworkParticipation = %#v, want %#v", definition.NetworkParticipation, want)
	}

	serialized, err := dsl.Serialize(definition)
	if err != nil {
		t.Fatalf("Serialize(network participation) error = %v", err)
	}
	serializedText := string(serialized)
	for _, field := range []string{
		"network_participation:",
		"channel_strategy: named",
		"channel_id: builders",
		"max_wakes: 3",
		"max_wake_wall_time: 30s",
		"max_total_wall_time: 2m",
		"max_input_tokens: 4096",
		"max_output_tokens: 2048",
		"max_wake_depth: 2",
		"coalesce_window: 250ms",
	} {
		if !strings.Contains(serializedText, field) {
			t.Fatalf("serialized Network participation missing %q:\n%s", field, serializedText)
		}
	}
	if strings.Contains(serializedText, "channelstrategy") || strings.Contains(serializedText, "maxwakes") {
		t.Fatalf("serialized Network participation contains non-contract field names:\n%s", serializedText)
	}

	reparsed, err := dsl.Parse(serialized)
	if err != nil {
		t.Fatalf("Parse(serialized network participation) error = %v", err)
	}
	if !reflect.DeepEqual(reparsed.NetworkParticipation, want) {
		t.Fatalf("round-trip NetworkParticipation = %#v, want %#v", reparsed.NetworkParticipation, want)
	}
}

func TestCodecShouldRoundTripStrictPredicatePolicies(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve scalar and object stop-when forms", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			authored   string
			wantPolicy dsl.EvalErrorPolicy
		}{
			{name: "Should preserve the scalar default", authored: `"inputs.done == true"`},
			{
				name:       "Should preserve the object override",
				authored:   `{ expr: "inputs.done == true", on_eval_error: fail }`,
				wantPolicy: dsl.EvalErrorFail,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				definition, err := dsl.Parse([]byte(minimalDefinition(`
contract:
  goal: Ship safely
  definition_of_done: Tests pass
  stop_when: ` + tt.authored + `
  verification: []
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
`)))
				if err != nil {
					t.Fatalf("Parse() error = %v", err)
				}
				if definition.Contract.StopWhen.Expr != "inputs.done == true" ||
					definition.Contract.StopWhen.OnEvalError != tt.wantPolicy {
					t.Fatalf("StopWhen = %#v, want expression and policy %q", definition.Contract.StopWhen, tt.wantPolicy)
				}
				serialized, err := dsl.Serialize(definition)
				if err != nil {
					t.Fatalf("Serialize() error = %v", err)
				}
				reparsed, err := dsl.Parse(serialized)
				if err != nil {
					t.Fatalf("Parse(serialized) error = %v", err)
				}
				if !reflect.DeepEqual(reparsed.Contract.StopWhen, definition.Contract.StopWhen) {
					t.Fatalf("round-trip StopWhen = %#v, want %#v", reparsed.Contract.StopWhen, definition.Contract.StopWhen)
				}
			})
		}
	})

	t.Run("Should reject malformed stop-when shapes", func(t *testing.T) {
		t.Parallel()

		for _, authored := range []string{
			`{ expr: "true", unknown: value }`,
			`{ expr: "true", on_eval_error: continue }`,
			`{ on_eval_error: fail }`,
			`true`,
		} {
			body := minimalDefinition(`
contract:
  stop_when: ` + authored)
			if _, err := dsl.Parse([]byte(body)); err == nil {
				t.Fatalf("Parse(stop_when: %s) error = nil", authored)
			}
		}
	})

	t.Run("Should keep JSON and node policy codecs closed", func(t *testing.T) {
		t.Parallel()

		var stop dsl.StopWhenSpec
		if err := json.Unmarshal([]byte(`{"expr":"true","on_eval_error":"exit"}`), &stop); err != nil {
			t.Fatalf("json.Unmarshal(stop_when) error = %v", err)
		}
		if stop.OnEvalError != dsl.EvalErrorExit {
			t.Fatalf("StopWhen.OnEvalError = %q, want exit", stop.OnEvalError)
		}
		for _, raw := range []string{
			`{"expr":"true","unknown":1}`,
			`null`,
			`{"expr":"true"} {"expr":"false"}`,
		} {
			if err := json.Unmarshal([]byte(raw), &stop); err == nil {
				t.Fatalf("json.Unmarshal(%s) error = nil", raw)
			}
		}
		node := dsl.Node{OnEvalError: dsl.EvalErrorExit}
		encoded, err := json.Marshal(node)
		if err != nil {
			t.Fatalf("json.Marshal(node) error = %v", err)
		}
		var decoded dsl.Node
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(node) error = %v", err)
		}
		if decoded.OnEvalError != dsl.EvalErrorExit {
			t.Fatalf("Node.OnEvalError = %q, want exit", decoded.OnEvalError)
		}
	})
}

func TestCodecShouldRoundTripRouteGrammar(t *testing.T) {
	t.Parallel()

	definition, err := dsl.Parse([]byte(`apiVersion: compozy.loop/v1
kind: Loop
meta: { name: route-loop }
graph:
  nodes:
    - id: router
      class: control
      kind: route
      routes:
        - { when: "inputs.risk == 'low'", to: quick }
        - { when: "inputs.risk == 'high'", to: review }
      default: fallback
      on_eval_error: fail
    - id: gate
      class: control
      kind: gate
      on_result:
        fail: { route: review }
  edges:
    - { from: router, to: quick }
    - { from: router, to: review }
    - { from: router, to: fallback }
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	serialized, err := dsl.Serialize(definition)
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}
	reparsed, err := dsl.Parse(serialized)
	if err != nil {
		t.Fatalf("Parse(serialized) error = %v", err)
	}
	if !reflect.DeepEqual(reparsed.Graph, definition.Graph) {
		t.Fatalf("round-trip graph = %#v, want %#v", reparsed.Graph, definition.Graph)
	}
	router := reparsed.Graph.Nodes[0]
	if router.Default != "fallback" || len(router.Routes) != 2 || router.Routes[1].To != "review" ||
		router.OnEvalError != dsl.EvalErrorFail {
		t.Fatalf("round-trip router = %#v", router)
	}
	gateRoute, ok := reparsed.Graph.Nodes[1].OnResult["fail"].(map[string]any)
	if !ok || gateRoute["route"] != "review" {
		t.Fatalf("round-trip gate route = %#v", reparsed.Graph.Nodes[1].OnResult["fail"])
	}
}

func testNetworkParticipationRequest() *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	channelID := "builders"
	maxWakes := 3
	maxWakeWallTime := "30s"
	maxTotalWallTime := "2m"
	maxInputTokens := int64(4096)
	maxOutputTokens := int64(2048)
	maxWakeDepth := 2
	coalesceWindow := "250ms"
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channelID,
		Bounds: &participation.BoundsRequest{
			MaxWakes:         &maxWakes,
			MaxWakeWallTime:  &maxWakeWallTime,
			MaxTotalWallTime: &maxTotalWallTime,
			MaxInputTokens:   &maxInputTokens,
			MaxOutputTokens:  &maxOutputTokens,
			MaxWakeDepth:     &maxWakeDepth,
			CoalesceWindow:   &coalesceWindow,
		},
	}
}

func TestCodecShouldRoundTripContractOptionalFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		body             string
		wantConstraints  []string
		wantBoundaries   []string
		wantStopWhen     string
		wantSerializedIn []string
	}{
		{
			name: "Should default absent optional contract fields to empty",
			body: minimalDefinition(`
contract:
  goal: Ship safely
  definition_of_done: Tests pass
  verification: []
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
`),
			wantConstraints: []string{},
			wantBoundaries:  []string{},
		},
		{
			name: "Should preserve authored optional contract fields",
			body: minimalDefinition(`
contract:
  goal: Ship safely
  definition_of_done: Tests pass
  constraints: [keep scope]
  boundaries: [do not deploy]
  stop_when: "inputs.done == true"
  verification: []
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
`),
			wantConstraints:  []string{"keep scope"},
			wantBoundaries:   []string{"do not deploy"},
			wantStopWhen:     "inputs.done == true",
			wantSerializedIn: []string{"constraints:", "boundaries:", "stop_when:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			def, err := dsl.Parse([]byte(tt.body))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(def.Contract.Constraints, tt.wantConstraints) {
				t.Fatalf(
					"Constraints = %#v, want %#v",
					def.Contract.Constraints,
					tt.wantConstraints,
				)
			}
			if !reflect.DeepEqual(def.Contract.Boundaries, tt.wantBoundaries) {
				t.Fatalf("Boundaries = %#v, want %#v", def.Contract.Boundaries, tt.wantBoundaries)
			}
			if def.Contract.StopWhen.Expr != tt.wantStopWhen {
				t.Fatalf("StopWhen.Expr = %q, want %q", def.Contract.StopWhen.Expr, tt.wantStopWhen)
			}

			serialized, err := dsl.Serialize(def)
			if err != nil {
				t.Fatalf("Serialize() error = %v", err)
			}
			reparsed, err := dsl.Parse(serialized)
			if err != nil {
				t.Fatalf("Parse(serialized) error = %v; yaml=%s", err, string(serialized))
			}
			if !reflect.DeepEqual(reparsed.Contract.Constraints, tt.wantConstraints) {
				t.Fatalf(
					"round-trip Constraints = %#v, want %#v",
					reparsed.Contract.Constraints,
					tt.wantConstraints,
				)
			}
			for _, fragment := range tt.wantSerializedIn {
				if !containsString(string(serialized), fragment) {
					t.Fatalf("serialized YAML missing %q:\n%s", fragment, string(serialized))
				}
			}
		})
	}
}

func TestCodecShouldStructurallyMergeGraphUnknownFields(t *testing.T) {
	t.Parallel()

	t.Run("Should structurally merge graph unknown fields", func(t *testing.T) {
		t.Parallel()

		def, err := dsl.Parse([]byte(`apiVersion: compozy.loop/v1
kind: Loop
meta: { name: test-loop }
concurrency: forbid
inputs:
  done: { type: boolean, required: true }
x_root: keep
contract:
  goal: Ship safely
  definition_of_done: Tests pass
  constraints: [keep scope]
  boundaries: [do not deploy]
  stop_when: "inputs.done == true"
  verification: []
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: source
      class: source
      kind: input
      input_ref: done
      x_agent_authored: preserve
  edges:
    - from: source
      to: source
      label: preserve-label
      when: preserve-when
      x_edge_authored: preserve
start: [{ kind: manual }]
`))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		graph := dsl.DefinitionToGraph(def)
		merged := dsl.GraphToDefinition(graph)
		if !reflect.DeepEqual(merged.Graph, def.Graph) {
			t.Fatalf(
				"GraphToDefinition(DefinitionToGraph(def)).Graph = %#v, want %#v",
				merged.Graph,
				def.Graph,
			)
		}
		if merged.Extra["x_root"] != "keep" {
			t.Fatalf("merged root extra = %#v, want keep", merged.Extra["x_root"])
		}
		if got := merged.Graph.Nodes[0].Extra["x_agent_authored"]; got != "preserve" {
			t.Fatalf("merged node extra = %#v, want preserve", got)
		}
		if got := merged.Graph.Edges[0].Extra["x_edge_authored"]; got != "preserve" {
			t.Fatalf("merged edge extra = %#v, want preserve", got)
		}
		if got := merged.Graph.Edges[0].Extra["label"]; got != "preserve-label" {
			t.Fatalf("merged edge label extra = %#v, want preserve-label", got)
		}
		if got := merged.Graph.Edges[0].Extra["when"]; got != "preserve-when" {
			t.Fatalf("merged edge when extra = %#v, want preserve-when", got)
		}
	})
}

func TestCodecShouldRoundTripWatchEventsBlock(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve multi subscription watch-events nodes", func(t *testing.T) {
		t.Parallel()

		def, err := dsl.Parse([]byte(`apiVersion: compozy.loop/v1
kind: Loop
meta: { name: watch-events-codec }
concurrency: forbid
inputs:
  parent_task_id: { type: string, required: true }
contract:
  goal: Watch task events
  definition_of_done: Events are summarized
  verification: []
  terminal_states: [done, failed]
  iteration_cap: 0
  no_progress: { window: 1 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: task_activity
      class: source
      kind: watch-events
      events:
        - kind: task.status_changed
          filter: "event.task_id == inputs.parent_task_id && event.payload.to_status == 'blocked'"
        - kind: task.run.completed
          filter: "event.task_id == inputs.parent_task_id"
  edges: []
start: [{ kind: manual }]
`))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		node := def.Graph.Nodes[0]
		want := []dsl.EventSubscription{
			{
				Kind:   "task.status_changed",
				Filter: "event.task_id == inputs.parent_task_id && event.payload.to_status == 'blocked'",
			},
			{
				Kind:   "task.run.completed",
				Filter: "event.task_id == inputs.parent_task_id",
			},
		}
		if !reflect.DeepEqual(node.Events, want) {
			t.Fatalf("Events = %#v, want %#v", node.Events, want)
		}

		serialized, err := dsl.Serialize(def)
		if err != nil {
			t.Fatalf("Serialize() error = %v", err)
		}
		if !containsString(string(serialized), "kind: watch-events") ||
			!containsString(string(serialized), "events:") {
			t.Fatalf("serialized YAML missing watch-events events block:\n%s", string(serialized))
		}
		reparsed, err := dsl.Parse(serialized)
		if err != nil {
			t.Fatalf("Parse(serialized) error = %v; yaml=%s", err, string(serialized))
		}
		if !reflect.DeepEqual(reparsed.Graph.Nodes[0].Events, want) {
			t.Fatalf("round-trip Events = %#v, want %#v", reparsed.Graph.Nodes[0].Events, want)
		}
	})
}

func TestCodecShouldRoundTripGoalEnvelope(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve goal session retry params and unknown fields", func(t *testing.T) {
		t.Parallel()

		def, err := dsl.Parse([]byte(`apiVersion: compozy.loop/v1
kind: Loop
meta: { name: goal-codec }
concurrency: forbid
inputs: {}
contract:
  goal: Converge safely
  definition_of_done: Goal judge approves
  verification: []
  terminal_states: [done, blocked, failed]
  iteration_cap: 3
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: converge
      class: action
      kind: goal
      session: { handle: primary, mode: continuous, x_session: keep }
      retry: { max_attempts: 2, on_failure: fresh_session, x_retry: keep }
      params:
        agent: codex
        objective: Make verification pass
        judge:
          - { id: verify, type: command, check: make verify }
        max_turns: 10
        on_exhausted: escalate
        output_schema:
          status: { type: string, enum: [complete, blocked] }
        x_goal: keep
  edges: []
start: [{ kind: manual }]
`))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		node := def.Graph.Nodes[0]
		if node.Session == nil || node.Session.Mode != "continuous" || node.Session.Extra["x_session"] != "keep" {
			t.Fatalf("Session = %#v, want continuous envelope with extra", node.Session)
		}
		if node.Retry == nil || node.Retry.MaxAttempts != 2 || node.Retry.OnFailure != "fresh_session" ||
			node.Retry.Extra["x_retry"] != "keep" {
			t.Fatalf("Retry = %#v, want max_attempts/on_failure with extra", node.Retry)
		}
		var params dsl.GoalParams
		if err := node.Params.Decode(&params); err != nil {
			t.Fatalf("Decode(GoalParams) error = %v", err)
		}
		if params.Extra["x_goal"] != "keep" || params.Judge[0].Check != "make verify" {
			t.Fatalf("GoalParams = %#v, want check and preserved extra", params)
		}

		serialized, err := dsl.Serialize(def)
		if err != nil {
			t.Fatalf("Serialize() error = %v", err)
		}
		reparsed, err := dsl.Parse(serialized)
		if err != nil {
			t.Fatalf("Parse(serialized) error = %v", err)
		}
		retry := reparsed.Graph.Nodes[0].Retry
		if retry == nil || retry.MaxAttempts != 2 || retry.OnFailure != "fresh_session" {
			t.Fatalf("round-trip Retry = %#v, want retained fields", retry)
		}
		for _, fragment := range []string{"kind: goal", "mode: continuous", "max_attempts: 2", "on_failure: fresh_session", "check: make verify"} {
			if !strings.Contains(string(serialized), fragment) {
				t.Fatalf("serialized goal missing %q:\n%s", fragment, serialized)
			}
		}
	})
}

func TestCodecShouldRoundTripLifecycleGrammar(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve every lifecycle field across serialization", func(t *testing.T) {
		t.Parallel()

		raw := []byte(`apiVersion: compozy.loop/v1
kind: Loop
meta: { name: lifecycle-loop }
concurrency: forbid
contract:
  goal: Exercise lifecycle grammar
  definition_of_done: All lifecycle fields survive
  terminal_states: [done, canceled]
  iteration_cap: 3
  no_progress: { window: 2 }
  budget: { tokens: 100, wall_clock_sec: 60, on_exceeded: halt }
  on_done:
    - emit: { kind: loop_done, payload: { source: contract } }
  on_canceled:
    - tool: known__notify
      with: { body: canceled }
graph:
  nodes:
    - id: worker
      class: action
      kind: known__work
      timeout: 30s
      deadline: 2m
      retry:
        max_attempts: 3
        backoff: { base: 1s, max: 10s }
        non_retryable: [payload_declared]
      result_contract: { failure_field: error, message_field: error.message }
      on_error:
        allow_fail: true
        effects:
          - emit: { kind: worker_failed }
      on_retry:
        - tool: known__notify
          with: { body: retrying }
      on_success:
        - emit: { kind: worker_succeeded }
    - id: child
      class: action
      kind: run-loop
      params: { loop: child-loop, mode: await }
      on_parent_close: cancel
    - id: wait_for_ack
      class: control
      kind: wait
      params:
        event: { kind: approval.received }
        expect: { type: object, required: [approved] }
        ahead_arrival: consume_on_entry
        expires:
          after: 72h
          escalate:
            - tool: known__notify
              with: { body: still-waiting }
          route: timed_out
    - id: timed_out
      class: action
      kind: transform
      params:
        map: { status: timed_out }
  edges:
    - { from: worker, to: child }
    - { from: child, to: wait_for_ack }
    - { from: wait_for_ack, to: timed_out }
start: [{ kind: manual }]
`)

		definition, err := dsl.Parse(raw)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		serialized, err := dsl.Serialize(definition)
		if err != nil {
			t.Fatalf("Serialize() error = %v", err)
		}
		reparsed, err := dsl.Parse(serialized)
		if err != nil {
			t.Fatalf("Parse(serialized) error = %v", err)
		}
		if !reflect.DeepEqual(reparsed, definition) {
			t.Fatalf("lifecycle round-trip changed definition\nfirst: %#v\nsecond: %#v", definition, reparsed)
		}
		for _, fragment := range []string{
			"deadline: 2m",
			"failure_field: error",
			"on_error:",
			"on_retry:",
			"on_canceled:",
			"on_parent_close: cancel",
			"ahead_arrival: consume_on_entry",
		} {
			if !strings.Contains(string(serialized), fragment) {
				t.Fatalf("serialized lifecycle grammar missing %q:\n%s", fragment, serialized)
			}
		}
	})
}

func minimalDefinition(contract string) string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: test-loop }
concurrency: forbid
inputs:
  done: { type: boolean, required: true }
` + contract + `
graph:
  nodes:
    - id: source
      class: source
      kind: input
      input_ref: done
      produces: { value: boolean }
  edges: []
start: [{ kind: manual }]
`
}

func containsString(value string, fragment string) bool {
	return strings.Contains(value, fragment)
}
