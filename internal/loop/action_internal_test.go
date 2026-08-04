package loop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
		if !errors.Is(err, ErrActionSchemaInvalid) {
			t.Fatalf("validateRunAgentStructured(invalid) error = %v, want ErrActionSchemaInvalid", err)
		}
		provider, ok := errors.AsType[SafeActionFailureProvider](err)
		if !ok {
			t.Fatalf("validateRunAgentStructured(invalid) error = %T, want SafeActionFailureProvider", err)
		}
		failure := provider.SafeActionFailure()
		if failure.Code != string(ReasonCodeActionSchemaInvalid) || failure.Cause == "" || failure.Recovery == "" {
			t.Fatalf("schema failure = %#v, want structured action_schema_invalid guidance", failure)
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
		if !errors.Is(err, ErrActionSchemaInvalid) {
			t.Fatalf("Execute(run-agent invalid retry) error = %v, want ErrActionSchemaInvalid", err)
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
