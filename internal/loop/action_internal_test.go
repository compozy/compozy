package loop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/tools"
)

func TestActionRegistryInternalsShouldCoverOverrideAndValidationBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should reject missing runtime registry", func(t *testing.T) {
		t.Parallel()

		_, err := NewActionRegistry(nil)
		if !errors.Is(err, ErrActionDependencyMissing) {
			t.Fatalf("NewActionRegistry(nil) error = %v, want ErrActionDependencyMissing", err)
		}
	})

	t.Run("Should use explicit reserved executor overrides", func(t *testing.T) {
		t.Parallel()

		runAgent := stubActionExecutor{status: "agent-override"}
		runLoop := stubActionExecutor{status: "loop-override"}
		transform := stubActionExecutor{status: "transform-override"}
		actions, err := NewActionRegistry(
			&internalActionRegistryFake{},
			WithActionRunAgentExecutor(runAgent),
			WithActionRunLoopExecutor(runLoop),
			WithActionTransformExecutor(transform),
		)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		cases := []struct {
			kind string
			want string
		}{
			{kind: string(dsl.ActionRunAgent), want: "agent-override"},
			{kind: string(dsl.ActionRunLoop), want: "loop-override"},
			{kind: string(dsl.ActionTransform), want: "transform-override"},
		}
		for _, tc := range cases {
			executor, err := actions.Resolve(context.Background(), tools.Scope{}, tc.kind)
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", tc.kind, err)
			}
			raw, err := executor.Execute(context.Background(), dsl.Node{}, ActionExecutionInput{})
			if err != nil {
				t.Fatalf("Execute(%q) error = %v", tc.kind, err)
			}
			if raw.Status != tc.want {
				t.Fatalf("Execute(%q).Status = %q, want %q", tc.kind, raw.Status, tc.want)
			}
		}
	})
}

func TestActionHarvestInternalsShouldCoverEventRangeAndDependencyBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should reject tool executor without runtime", func(t *testing.T) {
		t.Parallel()

		executor := &ToolCallActionExecutor{}
		_, err := executor.Execute(context.Background(), dsl.Node{}, ActionExecutionInput{})
		if !errors.Is(err, ErrActionDependencyMissing) {
			t.Fatalf("Execute() error = %v, want ErrActionDependencyMissing", err)
		}
	})

	t.Run("Should reject event range harvest without reader", func(t *testing.T) {
		t.Parallel()

		executor := &ToolCallActionExecutor{}
		node := dsl.Node{Harvest: &dsl.HarvestSpec{Kind: harvestKindEventRange}}
		_, err := executor.Harvest(context.Background(), ActionRawResult{}, node)
		if !errors.Is(err, ErrActionDependencyMissing) {
			t.Fatalf("Harvest() error = %v, want ErrActionDependencyMissing", err)
		}
	})

	t.Run("Should marshal event range payload when reader returns events", func(t *testing.T) {
		t.Parallel()

		reader := &internalEventRangeReader{result: ActionEventRangeResult{Events: []ActionEvent{
			{Sequence: 4, Type: "agent_message", Text: "hello"},
		}}}
		executor := &ToolCallActionExecutor{eventReader: reader}
		node := dsl.Node{Harvest: &dsl.HarvestSpec{Kind: harvestKindAsync}}
		output, err := executor.Harvest(context.Background(), ActionRawResult{
			SessionID:     "sess-1",
			EventStartSeq: 4,
			EventEndSeq:   4,
		}, node)
		if err != nil {
			t.Fatalf("Harvest() error = %v", err)
		}
		if !strings.Contains(string(output.Structured), `"agent_message"`) {
			t.Fatalf("Structured = %s, want marshaled event", output.Structured)
		}
	})

	t.Run("Should reject invalid event ranges", func(t *testing.T) {
		t.Parallel()

		err := validateEventRange(ActionRawResult{SessionID: "sess-1", EventStartSeq: 5, EventEndSeq: 4})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("validateEventRange() error = %v, want ErrValidation", err)
		}
		err = validateEventRange(ActionRawResult{SessionID: "", EventStartSeq: 1, EventEndSeq: 2})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("validateEventRange(empty session) error = %v, want ErrValidation", err)
		}
	})
}

func TestActionSchemaAndJSONInternalsShouldCoverStructuredExtraction(t *testing.T) {
	t.Parallel()

	t.Run("Should extract strict fenced and balanced JSON objects", func(t *testing.T) {
		t.Parallel()

		cases := []string{
			`{"summary":"strict"}`,
			"```json\n{\"summary\":\"fenced\"}\n```",
			"prefix {\"summary\":\"balanced\", \"brace\":\"}\"} suffix",
		}
		for _, source := range cases {
			raw, err := extractJSONObject(source)
			if err != nil {
				t.Fatalf("extractJSONObject(%q) error = %v", source, err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("unmarshal extracted JSON error = %v", err)
			}
			if decoded["summary"] == "" {
				t.Fatalf("decoded summary empty for %q", source)
			}
		}
	})

	t.Run("Should validate the final answer when earlier JSON objects fail the schema", func(t *testing.T) {
		t.Parallel()

		schema := dsl.Schema{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []any{"status", "summary"},
		}
		turn := "Implemented the app. Here is package.json:\n" +
			"```json\n{\"name\":\"todo-app\",\"private\":true}\n```\n" +
			"All checks passed.\n" +
			"{\"status\":\"completed\",\"summary\":\"todo app shipped\"}"
		raw, err := ValidateActionStructured(schema, ActionPromptResult{Text: turn})
		if err != nil {
			t.Fatalf("ValidateActionStructured(mixed turn) error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal validated output error = %v", err)
		}
		if decoded["status"] != "completed" || decoded["summary"] != "todo app shipped" {
			t.Fatalf("validated output = %v, want the final schema-passing object", decoded)
		}
	})

	t.Run("Should preserve source order across JSON extractors", func(t *testing.T) {
		t.Parallel()

		schema := dsl.Schema{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []any{"status"},
		}
		raw, err := ValidateActionStructured(schema, ActionPromptResult{
			Text: "Earlier {\"status\":\"embedded\"}.\n```json\n{\"status\":\"fenced\"}\n```",
		})
		if err != nil {
			t.Fatalf("ValidateActionStructured() error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal validated output error = %v", err)
		}
		if got := decoded["status"]; got != "fenced" {
			t.Fatalf("validated status = %v, want fenced", got)
		}
	})

	t.Run("Should surface the extraction detail when no JSON object exists", func(t *testing.T) {
		t.Parallel()

		schema := dsl.Schema{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []any{"status"},
		}
		_, err := ValidateActionStructured(schema, ActionPromptResult{Text: "Done — everything works."})
		if !errors.Is(err, ErrActionInvalidOutput) {
			t.Fatalf("ValidateActionStructured(prose) error = %v, want ErrActionInvalidOutput", err)
		}
		provider, ok := errors.AsType[SafeActionFailureProvider](err)
		if !ok {
			t.Fatalf("ValidateActionStructured(prose) error = %T, want SafeActionFailureProvider", err)
		}
		failure := provider.SafeActionFailure()
		if !strings.Contains(failure.Cause, "no JSON object found") {
			t.Fatalf("failure cause = %q, want the concrete extraction detail", failure.Cause)
		}
	})

	t.Run("Should append the authored schema to the run-agent prompt contract", func(t *testing.T) {
		t.Parallel()

		schema := dsl.Schema{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []any{"status"},
		}
		prompt, err := ActionPromptWithOutputContract("Do the task", schema)
		if err != nil {
			t.Fatalf("runAgentPromptWithOutputContract() error = %v", err)
		}
		if !strings.HasPrefix(prompt, "Do the task") || !strings.Contains(prompt, "Output contract:") ||
			!strings.Contains(prompt, `"required":["status"]`) {
			t.Fatalf("contract prompt = %q, want prompt plus authored schema", prompt)
		}
		unchanged, err := ActionPromptWithOutputContract("Do the task", nil)
		if err != nil {
			t.Fatalf("runAgentPromptWithOutputContract(no schema) error = %v", err)
		}
		if unchanged != "Do the task" {
			t.Fatalf("contract prompt without schema = %q, want untouched prompt", unchanged)
		}

		_, err = ActionPromptWithOutputContract("Do the task", dsl.Schema{
			"payload": map[any]any{12: "bad"},
		})
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "normalize output contract schema") {
			t.Fatalf(
				"runAgentPromptWithOutputContract(invalid schema) error = %v, want wrapped ErrValidation",
				err,
			)
		}
	})

	t.Run("Should validate full JSON schema and reject invalid output", func(t *testing.T) {
		t.Parallel()

		schema := dsl.Schema{
			"type":       "object",
			"properties": map[string]any{"summary": map[string]any{"type": "string"}},
			"required":   []any{"summary"},
		}
		valid := ActionPromptResult{Structured: json.RawMessage(`{"summary":"done"}`)}
		if _, err := validateRunAgentStructured(schema, valid); err != nil {
			t.Fatalf("validateRunAgentStructured(valid) error = %v", err)
		}
		invalid := ActionPromptResult{Structured: json.RawMessage(`{"summary":12}`)}
		_, err := validateRunAgentStructured(schema, invalid)
		if !errors.Is(err, ErrActionInvalidOutput) {
			t.Fatalf("validateRunAgentStructured(invalid) error = %v, want ErrActionInvalidOutput", err)
		}
		provider, ok := errors.AsType[SafeActionFailureProvider](err)
		if !ok {
			t.Fatalf("validateRunAgentStructured(invalid) error = %T, want SafeActionFailureProvider", err)
		}
		failure := provider.SafeActionFailure()
		if failure.Code != string(ReasonCodeInvalidOutput) || failure.Cause == "" || failure.Recovery == "" {
			t.Fatalf("schema failure = %#v, want structured invalid_output guidance", failure)
		}
	})

	t.Run("Should revalidate the exact lease result against the pinned output schema", func(t *testing.T) {
		t.Parallel()

		metadata := json.RawMessage(
			`{"generation":1,"node_id":"worker","item_index":0,"attempt":1,"epoch":0,` +
				`"output_schema":{"type":"object","properties":{"status":{"type":"string"},` +
				`"summary":{"type":"string"}},"required":["status","summary"]}}`,
		)
		run := task.Run{
			ID: "run-worker", RunKind: task.RunKindWorker, LoopRunID: "looprun-worker", Metadata: metadata,
		}
		valid := task.RunResult{Value: json.RawMessage(`{"status":"done","summary":"complete"}`)}
		if err := ValidateActionRunResult(run, valid); err != nil {
			t.Fatalf("ValidateActionRunResult(valid) error = %v", err)
		}
		invalid := task.RunResult{Value: json.RawMessage(`{"status":"done"}`)}
		err := ValidateActionRunResult(run, invalid)
		if !errors.Is(err, ErrActionInvalidOutput) {
			t.Fatalf("ValidateActionRunResult(invalid) error = %v, want ErrActionInvalidOutput", err)
		}
		provider, ok := errors.AsType[SafeActionFailureProvider](err)
		if !ok || provider.SafeActionFailure().Code != string(ReasonCodeInvalidOutput) {
			t.Fatalf("ValidateActionRunResult(invalid) failure = %#v, want invalid_output", provider)
		}
	})

	t.Run("Should normalize nested shorthand schema", func(t *testing.T) {
		t.Parallel()

		schema, err := normalizeLoopSchema(dsl.Schema{
			"user": map[string]any{
				"name": "string",
			},
		})
		if err != nil {
			t.Fatalf("normalizeLoopSchema() error = %v", err)
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("properties = %T, want map", schema["properties"])
		}
		userSchema, ok := properties["user"].(map[string]any)
		if !ok || userSchema["type"] != "object" {
			t.Fatalf("user schema = %#v, want object shorthand expansion", properties["user"])
		}
	})

	t.Run("Should expand shorthand fields named like JSON schema keywords", func(t *testing.T) {
		t.Parallel()

		schema, err := normalizeLoopSchema(dsl.Schema{
			"type":  "string",
			"count": "number",
		})
		if err != nil {
			t.Fatalf("normalizeLoopSchema() error = %v", err)
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("properties = %T, want map", schema["properties"])
		}
		typeSchema, ok := properties["type"].(map[string]any)
		if !ok || typeSchema["type"] != "string" {
			t.Fatalf("type property schema = %#v, want shorthand string property", properties["type"])
		}
		countSchema, ok := properties["count"].(map[string]any)
		if !ok || countSchema["type"] != "number" {
			t.Fatalf("count property schema = %#v, want shorthand number property", properties["count"])
		}
	})

	t.Run("Should return no structured output when schema is absent", func(t *testing.T) {
		t.Parallel()

		raw, err := validateRunAgentStructured(nil, ActionPromptResult{Text: "plain text"})
		if err != nil {
			t.Fatalf("validateRunAgentStructured(no schema) error = %v", err)
		}
		if raw != nil {
			t.Fatalf("structured = %s, want nil without schema", raw)
		}
	})
}

func TestActionRenderingInternalsShouldNormalizeValuesAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize YAML maps and render nested templates", func(t *testing.T) {
		t.Parallel()

		rendered, err := renderAny("test", map[any]any{
			"items": []any{
				map[any]any{"label": "Ticket {{ .inputs.ticket }}"},
			},
		}, map[string]any{
			"inputs": map[string]any{"ticket": "T-1"},
		})
		if err != nil {
			t.Fatalf("renderAny() error = %v", err)
		}
		top, ok := rendered.(map[string]any)
		if !ok {
			t.Fatalf("rendered = %T, want map", rendered)
		}
		items, ok := top["items"].([]any)
		if !ok || len(items) != 1 {
			t.Fatalf("items = %#v, want one item", top["items"])
		}
		first, ok := items[0].(map[string]any)
		if !ok || first["label"] != "Ticket T-1" {
			t.Fatalf("first item = %#v, want rendered label", items[0])
		}
	})

	t.Run("Should preserve direct namespace reference value types", func(t *testing.T) {
		t.Parallel()

		issues := []any{map[string]any{"id": "issue-1"}}
		rendered, err := renderNodeParams(dsl.Node{
			ID: "write_artifacts",
			Params: dsl.NodeParams{
				"issues":  "{{ .inputs.issues }}",
				"enabled": "{{ .inputs.enabled }}",
				"label":   "Issues: {{ len .inputs.issues }}",
			},
		}, map[string]any{
			"inputs": map[string]any{"issues": issues, "enabled": false},
		})
		if err != nil {
			t.Fatalf("renderNodeParams() error = %v", err)
		}
		top := rendered
		gotIssues, ok := top["issues"].([]any)
		if !ok || len(gotIssues) != 1 {
			t.Fatalf("issues = %#v, want typed array", top["issues"])
		}
		if got, ok := top["enabled"].(bool); !ok || got {
			t.Fatalf("enabled = %#v, want typed false", top["enabled"])
		}
		if got, want := top["label"], "Issues: 1"; got != want {
			t.Fatalf("label = %#v, want %#v", got, want)
		}
	})

	t.Run("Should report invalid map keys and missing namespace path", func(t *testing.T) {
		t.Parallel()

		_, err := normalizeJSONValue(map[any]any{12: "bad"})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("normalizeJSONValue() error = %v, want ErrValidation", err)
		}
		_, err = namespacePathValue(map[string]any{"nodes": map[string]any{}}, "nodes.agent.output")
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("namespacePathValue() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should preserve raw params excluded from rendering", func(t *testing.T) {
		t.Parallel()

		rendered, err := renderNodeParamsExcept(dsl.Node{
			ID: "agent",
			Params: dsl.NodeParams{
				"prompt":             "Ticket {{ .inputs.ticket }}",
				outputSchemaParamKey: map[string]any{"summary": "{{ .inputs.schema_type }}"},
			},
		}, map[string]any{
			"inputs": map[string]any{"ticket": "T-1", "schema_type": "string"},
		}, map[string]struct{}{
			outputSchemaParamKey: {},
		})
		if err != nil {
			t.Fatalf("renderNodeParamsExcept() error = %v", err)
		}
		if rendered["prompt"] != "Ticket T-1" {
			t.Fatalf("prompt = %#v, want rendered prompt", rendered["prompt"])
		}
		schema, ok := rendered[outputSchemaParamKey].(map[string]any)
		if !ok || schema["summary"] != "{{ .inputs.schema_type }}" {
			t.Fatalf("output_schema = %#v, want raw schema", rendered[outputSchemaParamKey])
		}
	})

	t.Run("Should reject invalid timeouts and support item indexed tool call ids", func(t *testing.T) {
		t.Parallel()

		if _, err := parseActionTimeout("0s"); !errors.Is(err, ErrValidation) {
			t.Fatalf("parseActionTimeout(0s) error = %v, want ErrValidation", err)
		}
		if _, err := parseActionTimeout("soon"); !errors.Is(err, ErrValidation) {
			t.Fatalf("parseActionTimeout(soon) error = %v, want ErrValidation", err)
		}
		got := actionToolCallID(dsl.Node{ID: "fanout"}, ActionExecutionInput{ItemIndex: 2})
		if got != "fanout:2" {
			t.Fatalf("actionToolCallID() = %q, want fanout:2", got)
		}
	})

	t.Run("Should parse metadata variants and create timeout contexts", func(t *testing.T) {
		t.Parallel()

		metadata := map[string]json.RawMessage{
			"string_number": json.RawMessage(`"42"`),
			"bad_number":    json.RawMessage(`"nope"`),
			"bad_string":    json.RawMessage(`{}`),
		}
		if got := metadataInt64(metadata, "string_number"); got != 42 {
			t.Fatalf("metadataInt64(string) = %d, want 42", got)
		}
		if got := metadataInt64(metadata, "bad_number"); got != 0 {
			t.Fatalf("metadataInt64(bad) = %d, want 0", got)
		}
		if got := metadataString(metadata, "bad_string"); got != "" {
			t.Fatalf("metadataString(bad) = %q, want empty", got)
		}
		ctx, cancel, err := actionContextWithNodeTimeout(context.Background(), "1ms")
		if err != nil {
			t.Fatalf("actionContextWithNodeTimeout() error = %v", err)
		}
		defer cancel()
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("timeout context has no deadline")
		}
	})
}

// Invariant: run-agent capture, durable settle, and wait admission share the contracts engine while
// preserving the established Loop verdict and invalid_output mapping.
// Owning layer: Loop contract adapters.
// Canonical suite: action_internal_test.go.
func TestActionContractAdaptersShouldPreserveParity(t *testing.T) {
	t.Parallel()

	schema := dsl.Schema{
		"type":       "object",
		"properties": map[string]any{"summary": map[string]any{"type": "string"}},
		"required":   []any{"summary"},
	}
	node := dsl.Node{
		ID: "worker", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
		Params: dsl.NodeParams{"agent": "codex", "prompt": "Return a summary", "output_schema": schema},
	}
	metadata := json.RawMessage(
		`{"generation":1,"node_id":"worker","item_index":0,"attempt":1,"epoch":0,` +
			`"output_schema":{"type":"object","properties":{"summary":{"type":"string"}},` +
			`"required":["summary"]}}`,
	)
	run := task.Run{ID: "run-worker", RunKind: task.RunKindWorker, LoopRunID: "looprun-worker", Metadata: metadata}

	t.Run("Should preserve invalid_output parity between capture and settle UT-017", func(t *testing.T) {
		t.Parallel()

		payload := json.RawMessage(`{"summary":12}`)
		captureErr := ValidateActionRunResult(run, task.RunResult{Value: payload})
		captureProvider, ok := errors.AsType[SafeActionFailureProvider](captureErr)
		if !ok {
			t.Fatalf("ValidateActionRunResult() error = %T, want SafeActionFailureProvider", captureErr)
		}
		settleFailure := completedRunAgentOutputFailure(node, payload)
		if settleFailure == nil || captureProvider.SafeActionFailure() != *settleFailure {
			t.Fatalf("capture failure = %#v, settle failure = %#v, want byte-identical mapping",
				captureProvider.SafeActionFailure(), settleFailure)
		}
	})

	t.Run("Should route capture and settle through one contracts validator IT-050", func(t *testing.T) {
		t.Parallel()

		calls := 0
		validator := func(schema dsl.Schema, payload json.RawMessage) error {
			calls++
			return validateJSONSchema(schema, payload)
		}
		payload := json.RawMessage(`{"summary":"done"}`)
		if err := validateActionRunResultWith(run, task.RunResult{Value: payload}, validator); err != nil {
			t.Fatalf("validateActionRunResultWith() error = %v", err)
		}
		if failure := completedRunAgentOutputFailureWith(node, payload, validator); failure != nil {
			t.Fatalf("completedRunAgentOutputFailureWith() = %#v, want nil", failure)
		}
		if calls != 2 {
			t.Fatalf("contracts validator calls = %d, want capture + settle", calls)
		}
	})

	t.Run("Should demote payload corruption discovered at settle IT-051", func(t *testing.T) {
		t.Parallel()

		if err := ValidateActionRunResult(run, task.RunResult{
			Value: json.RawMessage(`{"summary":"captured"}`),
		}); err != nil {
			t.Fatalf("ValidateActionRunResult(captured) error = %v", err)
		}
		failure := completedRunAgentOutputFailure(node, json.RawMessage(`{"summary":7}`))
		if failure == nil || failure.Code != string(ReasonCodeInvalidOutput) {
			t.Fatalf("settle corruption failure = %#v, want invalid_output", failure)
		}
	})

	t.Run("Should preserve ask and review golden acceptance UT-020", func(t *testing.T) {
		t.Parallel()

		fixtures := []struct {
			name    string
			expect  json.RawMessage
			payload json.RawMessage
			valid   bool
		}{
			{name: "empty contract", payload: json.RawMessage(`{"answer":true}`), valid: true},
			{name: "shorthand valid", expect: json.RawMessage(`{"answer":"boolean"}`),
				payload: json.RawMessage(`{"answer":true}`), valid: true},
			{name: "full schema valid", expect: json.RawMessage(
				`{"type":"object","required":["answer"],"properties":{"answer":{"type":"string"}}}`),
				payload: json.RawMessage(`{"answer":"yes"}`), valid: true},
			{name: "missing required", expect: json.RawMessage(`{"answer":"boolean"}`),
				payload: json.RawMessage(`{}`), valid: false},
			{name: "wrong type", expect: json.RawMessage(`{"answer":"boolean"}`),
				payload: json.RawMessage(`{"answer":"yes"}`), valid: false},
			{name: "invalid json", expect: json.RawMessage(`{"answer":"boolean"}`),
				payload: json.RawMessage(`{"answer":`), valid: false},
		}
		for _, fixture := range fixtures {
			fixture := fixture
			t.Run("Should preserve "+fixture.name, func(t *testing.T) {
				t.Parallel()
				before := append(json.RawMessage(nil), fixture.payload...)
				loopErr := ValidateWaitPayload(fixture.expect, fixture.payload)
				contractsErr := contracts.ValidateWaitPayload(fixture.expect, fixture.payload)
				if (loopErr == nil) != fixture.valid || (contractsErr == nil) != fixture.valid {
					t.Fatalf("acceptance loop=%v contracts=%v, want valid=%t", loopErr, contractsErr, fixture.valid)
				}
				if string(fixture.payload) != string(before) {
					t.Fatalf("payload changed from %q to %q", before, fixture.payload)
				}
			})
		}
	})
}

func TestDeclaredOutputResolverShouldServeLintReviewAndAmendment(t *testing.T) {
	t.Parallel()
	t.Run("Should serve the same declared output to lint review and amendment", func(t *testing.T) {
		t.Parallel()

		// Invariant: lint, review, and amendment observe exactly one declared
		// output schema for the same node.
		// Owning layer: Loop declared-output resolution.
		// Canonical suite: action_internal_test.go.
		node := dsl.Node{
			ID:    "worker",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{
				"agent":         "codex",
				"prompt":        "Return a result",
				"output_schema": map[string]any{"summary": "string"},
			},
		}
		definition := dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{node}}}
		resolved := &ResolvedDefinition{Definition: definition, ToolSchemas: map[string]ToolSchemaSnapshot{}}

		lintSchema, ok := newLintContext(definition, &DefinitionLinter{}).declaredSchema(node)
		if !ok {
			t.Fatal("lint declared schema = missing")
		}
		amendmentSchema, err := resolvedDefinitionOutputSchema(resolved, node)
		if err != nil {
			t.Fatalf("resolvedDefinitionOutputSchema() error = %v", err)
		}
		_, reviewSchema, err := reviewSchemas(resolved, node, nil, []dsl.ReviewDecision{dsl.ReviewDecisionRespond})
		if err != nil {
			t.Fatalf("reviewSchemas() error = %v", err)
		}
		lintRaw, err := json.Marshal(lintSchema)
		if err != nil {
			t.Fatalf("json.Marshal(lint schema) error = %v", err)
		}
		amendmentRaw, err := json.Marshal(amendmentSchema)
		if err != nil {
			t.Fatalf("json.Marshal(amendment schema) error = %v", err)
		}
		if string(lintRaw) != string(amendmentRaw) || string(lintRaw) != string(reviewSchema) {
			t.Fatalf("schema mismatch: lint=%s amendment=%s review=%s", lintRaw, amendmentRaw, reviewSchema)
		}
	})
}

func TestReservedActionInternalsShouldCoverErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should reject run-loop nil child run", func(t *testing.T) {
		t.Parallel()

		executor := &RunLoopActionExecutor{starter: &internalLoopStarter{}}
		_, err := executor.Execute(context.Background(), dsl.Node{
			Params: dsl.NodeParams{"loop": "child", "mode": string(dsl.RunLoopDetach)},
		}, ActionExecutionInput{})
		if !errors.Is(err, ErrActionDependencyMissing) {
			t.Fatalf("Execute(run-loop nil child) error = %v, want ErrActionDependencyMissing", err)
		}
	})

	t.Run("Should harvest run-loop raw output", func(t *testing.T) {
		t.Parallel()

		executor := &RunLoopActionExecutor{}
		output, err := executor.Harvest(context.Background(), ActionRawResult{
			Structured:     json.RawMessage(`{"loop_run_id":"child"}`),
			ChildLoopRunID: "child",
		}, dsl.Node{})
		if err != nil {
			t.Fatalf("Harvest(run-loop) error = %v", err)
		}
		if output.ChildLoopRunID != "child" {
			t.Fatalf("ChildLoopRunID = %q, want child", output.ChildLoopRunID)
		}
	})

	t.Run("Should return schema error after free retry also fails", func(t *testing.T) {
		t.Parallel()

		binder := &internalSessionBinder{
			binding: ActionSessionBinding{SessionID: "sess-retry"},
			results: []ActionPromptResult{
				{Text: `{"summary":12}`},
				{Text: `{"summary":13}`},
			},
		}
		executor := &RunAgentActionExecutor{binder: binder}
		_, err := executor.Execute(context.Background(), dsl.Node{
			Params: dsl.NodeParams{
				"agent":         "planner",
				"prompt":        "summarize",
				"output_schema": map[string]any{"summary": "string"},
			},
		}, ActionExecutionInput{})
		if !errors.Is(err, ErrActionInvalidOutput) {
			t.Fatalf("Execute(run-agent invalid retry) error = %v, want ErrActionInvalidOutput", err)
		}
		if len(binder.prompts) != 2 {
			t.Fatalf("prompt calls = %d, want 2", len(binder.prompts))
		}
	})

	t.Run("Should persist the exact managed session identity for cancellation", func(t *testing.T) {
		t.Parallel()

		run := Run{ID: "looprun-session-metadata", WorkspaceID: "ws-session-metadata", LoopName: "metadata"}
		actionNode := dsl.Node{
			ID: "worker", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Session: &dsl.SessionSpec{Handle: "main"},
			Params: dsl.NodeParams{"output_schema": map[string]any{
				"type": "object", "required": []string{"summary"},
			}},
		}
		metadata, err := coordinatorNodeMetadataWithFanOutItem(run, 2, actionNode, 3, 1, 4, nil, false)
		if err != nil {
			t.Fatalf("coordinatorNodeMetadataWithFanOutItem(run-agent) error = %v", err)
		}
		var actionPayload map[string]any
		if err := json.Unmarshal(metadata, &actionPayload); err != nil {
			t.Fatalf("json.Unmarshal(run-agent metadata) error = %v", err)
		}
		wantActionKey := actionSessionSharedKey(2, actionNode.ID, 3, "main")
		if got := actionPayload["session_handle"]; got != wantActionKey {
			t.Fatalf("run-agent session_handle = %#v, want %q", got, wantActionKey)
		}
		if _, ok := actionPayload[outputSchemaParamKey].(map[string]any); !ok {
			t.Fatalf("run-agent output_schema = %#v, want pinned schema", actionPayload[outputSchemaParamKey])
		}

		goalNode := dsl.Node{
			ID: "goal", Class: dsl.NodeClassAction, Kind: string(dsl.ActionGoal),
			Session: &dsl.SessionSpec{Handle: "review"},
		}
		metadata, err = coordinatorNodeMetadataWithFanOutItem(run, 2, goalNode, 3, 1, 4, nil, false)
		if err != nil {
			t.Fatalf("coordinatorNodeMetadataWithFanOutItem(Goal) error = %v", err)
		}
		var goalPayload map[string]any
		if err := json.Unmarshal(metadata, &goalPayload); err != nil {
			t.Fatalf("json.Unmarshal(Goal metadata) error = %v", err)
		}
		wantGoalKey, err := dsl.DeriveGoalHandle(goalNode.ID, "review", 3)
		if err != nil {
			t.Fatalf("DeriveGoalHandle() error = %v", err)
		}
		if got := goalPayload["session_handle"]; got != wantGoalKey {
			t.Fatalf("Goal session_handle = %#v, want %q", got, wantGoalKey)
		}
	})
}

type stubActionExecutor struct {
	status string
}

func (e stubActionExecutor) Execute(context.Context, dsl.Node, ActionExecutionInput) (ActionRawResult, error) {
	return ActionRawResult{Status: e.status}, nil
}

func (e stubActionExecutor) Harvest(context.Context, ActionRawResult, dsl.Node) (ActionOutput, error) {
	return ActionOutput{Status: e.status}, nil
}

type internalActionRegistryFake struct {
	view tools.ToolView
}

func (r *internalActionRegistryFake) List(context.Context, tools.Scope) ([]tools.ToolView, error) {
	return []tools.ToolView{r.view}, nil
}

func (r *internalActionRegistryFake) Search(context.Context, tools.Scope, tools.SearchQuery) ([]tools.ToolView, error) {
	return []tools.ToolView{r.view}, nil
}

func (r *internalActionRegistryFake) Get(context.Context, tools.Scope, tools.ToolID) (tools.ToolView, error) {
	return r.view, nil
}

func (r *internalActionRegistryFake) Call(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
	return tools.ToolResult{}, nil
}

type internalEventRangeReader struct {
	result ActionEventRangeResult
}

func (r *internalEventRangeReader) ReadActionEventRange(
	context.Context,
	ActionEventRangeRequest,
) (ActionEventRangeResult, error) {
	return r.result, nil
}

type internalLoopStarter struct{}

func (s *internalLoopStarter) Start(context.Context, WorkspaceID, string, Inputs, task.ActorContext) (*Run, error) {
	return nil, nil
}

type internalSessionBinder struct {
	binding ActionSessionBinding
	results []ActionPromptResult
	prompts []ActionPromptRequest
}

func (b *internalSessionBinder) BindActionSession(
	context.Context,
	ActionSessionBindRequest,
) (ActionSessionBinding, error) {
	return b.binding, nil
}

func (b *internalSessionBinder) PromptActionSession(
	context.Context,
	ActionSessionBinding,
	ActionPromptRequest,
) (ActionPromptResult, error) {
	req := ActionPromptRequest{}
	if len(b.prompts) == 0 {
		req.Message = "summarize"
	}
	b.prompts = append(b.prompts, req)
	if len(b.results) == 0 {
		return ActionPromptResult{}, errors.New("missing prompt result")
	}
	result := b.results[0]
	b.results = b.results[1:]
	return result, nil
}

func (b *internalSessionBinder) CancelActionSession(context.Context, ActionSessionBinding) error {
	return nil
}
