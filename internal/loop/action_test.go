package loop_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/tools"
)

func TestActionRegistryShouldResolveReservedKindsBeforeRuntimeAndRejectUnknownKinds(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve reserved kinds without consulting RuntimeRegistry", func(t *testing.T) {
		t.Parallel()

		registry := &fakeActionToolRegistry{getErr: errors.New("registry should not be called")}
		actions := newActionRegistryForTest(t, registry)

		for _, kind := range []string{
			string(dsl.ActionRunAgent),
			string(dsl.ActionRunLoop),
			string(dsl.ActionTransform),
		} {
			executor, err := actions.Resolve(context.Background(), tools.Scope{}, kind)
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", kind, err)
			}
			if executor == nil {
				t.Fatalf("Resolve(%q) executor = nil", kind)
			}
		}
		if registry.getCount() != 0 {
			t.Fatalf("RuntimeRegistry.Get calls = %d, want 0", registry.getCount())
		}
	})

	t.Run("Should resolve non reserved ToolID through RuntimeRegistry", func(t *testing.T) {
		t.Parallel()

		toolID := tools.ToolID("agh__task_read")
		registry := &fakeActionToolRegistry{views: map[tools.ToolID]tools.ToolView{
			toolID: {},
		}}
		actions := newActionRegistryForTest(t, registry)

		executor, err := actions.Resolve(context.Background(), tools.Scope{WorkspaceID: "ws-1"}, toolID.String())
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", toolID, err)
		}
		if _, ok := executor.(*loop.ToolCallActionExecutor); !ok {
			t.Fatalf("Resolve(%q) executor = %T, want ToolCallActionExecutor", toolID, executor)
		}
		if registry.getCount() != 1 {
			t.Fatalf("RuntimeRegistry.Get calls = %d, want 1", registry.getCount())
		}
	})

	t.Run("Should resolve an open action whose runtime schema matches the Run snapshot", func(t *testing.T) {
		t.Parallel()

		toolID := tools.ToolID("agh__task_read")
		registry := &fakeActionToolRegistry{views: map[tools.ToolID]tools.ToolView{
			toolID: {Descriptor: tools.Descriptor{
				ID:                 toolID,
				InputSchemaDigest:  "input-v1",
				OutputSchemaDigest: "output-v1",
			}},
		}}
		actions := newActionRegistryForTest(t, registry)
		expected := &loop.ToolSchemaSnapshot{
			ToolID:             toolID.String(),
			InputSchemaDigest:  "input-v1",
			OutputSchemaDigest: "output-v1",
		}

		executor, err := actions.ResolvePinned(
			context.Background(),
			tools.Scope{WorkspaceID: "ws-1"},
			toolID.String(),
			expected,
		)
		if err != nil {
			t.Fatalf("ResolvePinned() error = %v", err)
		}
		if _, ok := executor.(*loop.ToolCallActionExecutor); !ok {
			t.Fatalf("ResolvePinned() executor = %T, want ToolCallActionExecutor", executor)
		}
	})

	t.Run("Should reject an open action whose runtime schema drifted after Start", func(t *testing.T) {
		t.Parallel()

		toolID := tools.ToolID("agh__task_read")
		registry := &fakeActionToolRegistry{views: map[tools.ToolID]tools.ToolView{
			toolID: {Descriptor: tools.Descriptor{
				ID:                 toolID,
				InputSchemaDigest:  "input-v2",
				OutputSchemaDigest: "output-v1",
			}},
		}}
		actions := newActionRegistryForTest(t, registry)
		expected := &loop.ToolSchemaSnapshot{
			ToolID:             toolID.String(),
			InputSchemaDigest:  "input-v1",
			OutputSchemaDigest: "output-v1",
		}

		_, err := actions.ResolvePinned(
			context.Background(),
			tools.Scope{WorkspaceID: "ws-1"},
			toolID.String(),
			expected,
		)
		if !errors.Is(err, loop.ErrActionSchemaInvalid) {
			t.Fatalf("ResolvePinned() error = %v, want ErrActionSchemaInvalid", err)
		}
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeActionContractStale {
			t.Fatalf("ResolvePinned() reason = %#v, want %q", reason, loop.ReasonCodeActionContractStale)
		}
		if reason.Meta["expected_input_schema_digest"] != "input-v1" ||
			reason.Meta["current_input_schema_digest"] != "input-v2" {
			t.Fatalf("ResolvePinned() metadata = %#v, want pinned/current digests", reason.Meta)
		}
	})

	t.Run("Should reject misses deterministically", func(t *testing.T) {
		t.Parallel()

		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{})

		_, err := actions.Resolve(context.Background(), tools.Scope{}, "agh__missing")
		if !errors.Is(err, loop.ErrActionUnknownKind) {
			t.Fatalf("Resolve() error = %v, want ErrActionUnknownKind", err)
		}
		var reason *loop.ReasonError
		if !errors.As(err, &reason) {
			t.Fatalf("Resolve() error = %T, want ReasonError", err)
		}
		if reason.Code != loop.ReasonCodeUnknownActionKind {
			t.Fatalf("ReasonError.Code = %q, want %q", reason.Code, loop.ReasonCodeUnknownActionKind)
		}
	})

	t.Run("Should reject deleted channel post as unknown action kind", func(t *testing.T) {
		t.Parallel()

		registry := &fakeActionToolRegistry{views: map[tools.ToolID]tools.ToolView{
			tools.ToolID("channel-post"): {},
		}}
		actions := newActionRegistryForTest(t, registry)

		_, err := actions.Resolve(context.Background(), tools.Scope{}, "channel-post")
		if !errors.Is(err, loop.ErrActionUnknownKind) {
			t.Fatalf("Resolve(channel-post) error = %v, want ErrActionUnknownKind", err)
		}
		var reason *loop.ReasonError
		if !errors.As(err, &reason) || reason.Code != loop.ReasonCodeUnknownActionKind {
			t.Fatalf("Resolve(channel-post) reason = %v, want %q", err, loop.ReasonCodeUnknownActionKind)
		}
		if registry.getCount() != 0 {
			t.Fatalf("RuntimeRegistry.Get calls = %d, want 0", registry.getCount())
		}
	})
}

func TestToolCallActionExecutorShouldExecuteAndHarvestToolResults(t *testing.T) {
	t.Parallel()

	t.Run("Should execute RuntimeRegistry call and sync harvest structured output", func(t *testing.T) {
		t.Parallel()

		toolID := tools.ToolID("agh__task_read")
		registry := &fakeActionToolRegistry{
			views: map[tools.ToolID]tools.ToolView{toolID: {}},
			callResult: tools.ToolResult{
				Structured: json.RawMessage(`{"ok":true}`),
			},
		}
		actions := newActionRegistryForTest(t, registry)
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, toolID.String())
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		node := dsl.Node{
			ID:    "read",
			Class: dsl.NodeClassAction,
			Kind:  toolID.String(),
			Params: dsl.NodeParams{
				"id": "{{ .inputs.task_id }}",
			},
		}
		raw, err := executor.Execute(context.Background(), node, loop.ActionExecutionInput{
			WorkspaceID: "ws-1",
			Namespace: map[string]any{
				"inputs": map[string]any{"task_id": "task-1"},
			},
			ToolScope: tools.Scope{SessionID: "sess-1", AgentName: "agent-a"},
			Actor:     mustDaemonActor(t),
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		call := registry.mustSingleCall(t)
		if call.WorkspaceID != "ws-1" || call.SessionID != "sess-1" || call.AgentName != "agent-a" {
			t.Fatalf("CallRequest scope = %#v, want workspace/session/agent propagated", call)
		}
		var input map[string]any
		if err := json.Unmarshal(call.Input, &input); err != nil {
			t.Fatalf("unmarshal call input error = %v", err)
		}
		if input["id"] != "task-1" {
			t.Fatalf("call input id = %#v, want task-1", input["id"])
		}

		output, err := executor.Harvest(context.Background(), raw, node)
		if err != nil {
			t.Fatalf("Harvest() error = %v", err)
		}
		got, ok := output.Value.(map[string]any)
		if !ok || got["ok"] != true {
			t.Fatalf("Harvest().Value = %#v, want ok=true", output.Value)
		}
	})

	t.Run("Should async harvest through stable event range", func(t *testing.T) {
		t.Parallel()

		toolID := tools.ToolID("agh__agent_prompt")
		reader := &fakeActionEventReader{
			result: loop.ActionEventRangeResult{
				Structured: json.RawMessage(`{"events":2}`),
			},
		}
		registry := &fakeActionToolRegistry{
			views: map[tools.ToolID]tools.ToolView{toolID: {}},
			callResult: tools.ToolResult{Metadata: map[string]json.RawMessage{
				"session_id":      json.RawMessage(`"sess-async"`),
				"event_start_seq": json.RawMessage(`3`),
				"event_end_seq":   json.RawMessage(`7`),
			}},
		}
		actions := newActionRegistryForTest(t, registry, loop.WithActionEventRangeReader(reader))
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, toolID.String())
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		node := dsl.Node{
			ID:      "prompt",
			Class:   dsl.NodeClassAction,
			Kind:    toolID.String(),
			Harvest: &dsl.HarvestSpec{Kind: "event_range"},
		}

		raw, err := executor.Execute(context.Background(), node, loop.ActionExecutionInput{})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		output, err := executor.Harvest(context.Background(), raw, node)
		if err != nil {
			t.Fatalf("Harvest() error = %v", err)
		}
		if reader.last.SessionID != "sess-async" || reader.last.EventStartSeq != 3 || reader.last.EventEndSeq != 7 {
			t.Fatalf("event range request = %#v, want sess-async 3..7", reader.last)
		}
		if !strings.Contains(string(output.Structured), `"events":2`) {
			t.Fatalf("Harvest().Structured = %s, want event count", output.Structured)
		}
	})

	t.Run("Should channel result harvest designated result or stall on silence", func(t *testing.T) {
		t.Parallel()

		toolID := tools.ToolID("agh__network_send")
		channel := &fakeChannelResultHarvester{
			result: loop.ChannelResultHarvestResult{
				Found:      true,
				MessageID:  "msg-1",
				Structured: json.RawMessage(`{"answer":"approved"}`),
			},
		}
		registry := &fakeActionToolRegistry{
			views: map[tools.ToolID]tools.ToolView{toolID: {}},
			callResult: tools.ToolResult{Metadata: map[string]json.RawMessage{
				"session_id":    json.RawMessage(`"sess-channel"`),
				"eventStartSeq": json.RawMessage(`10`),
				"eventEndSeq":   json.RawMessage(`11`),
			}},
		}
		actions := newActionRegistryForTest(t, registry, loop.WithActionChannelResultHarvester(channel))
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, toolID.String())
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		node := dsl.Node{
			ID:    "ask",
			Class: dsl.NodeClassAction,
			Kind:  toolID.String(),
			Harvest: &dsl.HarvestSpec{
				Kind:        "channel_result",
				Window:      "5m",
				Responder:   "{{ .inputs.reviewer }}",
				ContentRule: "contains:{{ .inputs.term }}",
			},
		}
		raw, err := executor.Execute(context.Background(), node, loop.ActionExecutionInput{
			Namespace: map[string]any{"inputs": map[string]any{"reviewer": "reviewer", "term": "approved"}},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		output, err := executor.Harvest(context.Background(), raw, node)
		if err != nil {
			t.Fatalf("Harvest() error = %v", err)
		}
		if channel.last.Window != "5m" ||
			channel.last.Responder != "reviewer" ||
			channel.last.ContentRule != "contains:approved" {
			t.Fatalf("channel harvest request = %#v, want authored harvest spec", channel.last)
		}
		got, ok := output.Value.(map[string]any)
		if !ok || got["answer"] != "approved" {
			t.Fatalf("Harvest().Value = %#v, want approved answer", output.Value)
		}

		channel.result = loop.ChannelResultHarvestResult{Found: false}
		_, err = executor.Harvest(context.Background(), raw, node)
		if !errors.Is(err, loop.ErrActionStalled) {
			t.Fatalf("Harvest() silence error = %v, want ErrActionStalled", err)
		}
	})
}

func TestReservedActionExecutorsShouldRunAgentLoopAndTransform(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsupported harvest kinds before sync output", func(t *testing.T) {
		t.Parallel()

		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{})
		for _, kind := range []string{
			string(dsl.ActionRunAgent),
			string(dsl.ActionRunLoop),
			string(dsl.ActionTransform),
		} {
			executor, err := actions.Resolve(context.Background(), tools.Scope{}, kind)
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", kind, err)
			}
			node := dsl.Node{
				ID:      dsl.NodeID(strings.ReplaceAll(kind, "-", "_")),
				Class:   dsl.NodeClassAction,
				Kind:    kind,
				Harvest: &dsl.HarvestSpec{Kind: "channel_result", Window: "5m"},
			}
			_, err = executor.Harvest(context.Background(), loop.ActionRawResult{}, node)
			if !errors.Is(err, loop.ErrValidation) {
				t.Fatalf("Harvest(%q) error = %v, want ErrValidation", kind, err)
			}
		}
	})

	t.Run("Should bind shared run agent session render contract and retry invalid schema once", func(t *testing.T) {
		t.Parallel()

		binder := &fakeActionSessionBinder{
			binding: loop.ActionSessionBinding{SessionID: "sess-shared", Handle: "main"},
			promptResults: []loop.ActionPromptResult{
				{Text: "not json", EventStartSeq: 1, EventEndSeq: 2, TokensUsed: 11},
				{Text: "```json\n{\"summary\":\"done\"}\n```", EventStartSeq: 3, EventEndSeq: 4, TokensUsed: 13},
			},
		}
		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{}, loop.WithActionSessionBinder(binder))
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, string(dsl.ActionRunAgent))
		if err != nil {
			t.Fatalf("Resolve(run-agent) error = %v", err)
		}
		reportedTokens := []int64{}
		ctx := loop.ContextWithActionUsageReporter(
			context.Background(),
			loop.ActionUsageReporterFunc(func(tokensUsed int64) {
				reportedTokens = append(reportedTokens, tokensUsed)
			}),
		)
		node := dsl.Node{
			ID:    "agent",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{
				"agent":         "planner",
				"prompt":        "Summarize {{ .inputs.topic }}",
				"model":         "gpt-5.4",
				"allowed_tools": []string{"agh__task_read"},
				"max_turns":     3,
				"output_schema": map[string]any{"summary": "string"},
			},
		}
		raw, err := executor.Execute(ctx, node, loop.ActionExecutionInput{
			WorkspaceID: "ws-1",
			ItemIndex:   2,
			Namespace: map[string]any{
				"inputs": map[string]any{"topic": "delivery"},
			},
			Contract: &dsl.Contract{
				Goal:             "Ship the loop",
				DefinitionOfDone: "The action registry works",
				Constraints:      []string{"Do not create a second registry"},
				Boundaries:       []string{"Keep source and control closed"},
				StopWhen:         "generation > 2",
			},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		output, err := executor.Harvest(context.Background(), raw, node)
		if err != nil {
			t.Fatalf("Harvest() error = %v", err)
		}
		if raw.TokensUsed != 24 || output.TokensUsed != 24 {
			t.Fatalf("tokens_used raw/output = %d/%d, want 24", raw.TokensUsed, output.TokensUsed)
		}
		if !slices.Equal(reportedTokens, []int64{11, 24}) {
			t.Fatalf("reported tokens = %#v, want [11 24]", reportedTokens)
		}
		got, ok := output.Value.(map[string]any)
		if !ok || got["summary"] != "done" {
			t.Fatalf("run-agent output = %#v, want summary done", output.Value)
		}
		bind := binder.mustSingleBind(t)
		if bind.Agent != "planner" || bind.Handle != "main" || bind.Isolated {
			t.Fatalf("bind request = %#v, want shared main planner", bind)
		}
		if bind.ItemIndex != 2 {
			t.Fatalf("bind item index = %d, want 2", bind.ItemIndex)
		}
		if bind.Model != "gpt-5.4" || bind.MaxTurns != 3 || len(bind.AllowedTools) != 1 {
			t.Fatalf("bind overrides = %#v, want model/tool/max_turns", bind)
		}
		for _, fragment := range []string{
			"Goal:\nShip the loop",
			"Definition of done:\nThe action registry works",
			"- Do not create a second registry",
			"- Keep source and control closed",
			"Stop when:\ngeneration > 2",
		} {
			if !strings.Contains(bind.ContractBlock, fragment) {
				t.Fatalf("contract block missing %q in:\n%s", fragment, bind.ContractBlock)
			}
		}
		prompts := binder.promptMessages()
		if len(prompts) != 2 {
			t.Fatalf("prompt calls = %d, want schema retry", len(prompts))
		}
		if prompts[0] != "Summarize delivery" {
			t.Fatalf("first prompt = %q, want rendered prompt", prompts[0])
		}
		if !strings.Contains(prompts[1], "did not satisfy output_schema") {
			t.Fatalf("retry prompt = %q, want schema validation feedback", prompts[1])
		}
	})

	t.Run("Should bind isolated run agent session and cancel in flight turn on timeout", func(t *testing.T) {
		t.Parallel()

		binder := &fakeActionSessionBinder{
			binding:     loop.ActionSessionBinding{SessionID: "sess-isolated", Handle: "branch", Isolated: true},
			blockPrompt: true,
		}
		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{}, loop.WithActionSessionBinder(binder))
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, string(dsl.ActionRunAgent))
		if err != nil {
			t.Fatalf("Resolve(run-agent) error = %v", err)
		}
		node := dsl.Node{
			ID:      "agent",
			Class:   dsl.NodeClassAction,
			Kind:    string(dsl.ActionRunAgent),
			Session: &dsl.SessionSpec{Handle: "branch", Isolated: true},
			Timeout: "1ms",
			Params: dsl.NodeParams{
				"agent":  "planner",
				"prompt": "wait",
			},
		}

		_, err = executor.Execute(context.Background(), node, loop.ActionExecutionInput{WorkspaceID: "ws-1"})
		if !errors.Is(err, loop.ErrActionTimeout) {
			t.Fatalf("Execute() error = %v, want ErrActionTimeout", err)
		}
		bind := binder.mustSingleBind(t)
		if bind.Handle != "branch" || !bind.Isolated {
			t.Fatalf("bind request = %#v, want isolated branch", bind)
		}
		if binder.cancelCount() != 1 {
			t.Fatalf("CancelActionSession calls = %d, want 1", binder.cancelCount())
		}
		if !binder.cancelHadDeadline() {
			t.Fatal("CancelActionSession context has no deadline")
		}
	})

	t.Run("Should use effective worker model when run agent params omit model", func(t *testing.T) {
		t.Parallel()

		binder := &fakeActionSessionBinder{
			binding:       loop.ActionSessionBinding{SessionID: "sess-default-model", Handle: "main"},
			promptResults: []loop.ActionPromptResult{{Text: "done"}},
		}
		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{}, loop.WithActionSessionBinder(binder))
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, string(dsl.ActionRunAgent))
		if err != nil {
			t.Fatalf("Resolve(run-agent) error = %v", err)
		}
		node := dsl.Node{
			ID:    "agent",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{
				"agent":  "planner",
				"prompt": "Summarize",
			},
		}

		_, err = executor.Execute(context.Background(), node, loop.ActionExecutionInput{
			WorkspaceID: "ws-1",
			WorkerModel: "default-worker-model",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		bind := binder.mustSingleBind(t)
		if bind.Model != "default-worker-model" {
			t.Fatalf("bind model = %q, want default-worker-model", bind.Model)
		}
	})

	t.Run("Should start child loop in await and detach modes", func(t *testing.T) {
		t.Parallel()

		starter := &fakeActionLoopStarter{returnRun: &loop.Run{ID: "child-1"}}
		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{}, loop.WithActionLoopStarter(starter))
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, string(dsl.ActionRunLoop))
		if err != nil {
			t.Fatalf("Resolve(run-loop) error = %v", err)
		}
		awaitNode := dsl.Node{
			ID:    "child",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionRunLoop),
			Params: dsl.NodeParams{
				"loop":   "qa-child",
				"inputs": map[string]any{"ticket": "{{ .inputs.ticket }}"},
			},
		}
		raw, err := executor.Execute(context.Background(), awaitNode, loop.ActionExecutionInput{
			WorkspaceID: "ws-1",
			LoopRunID:   "parent-1",
			Namespace:   map[string]any{"inputs": map[string]any{"ticket": "T-1"}},
		})
		if err != nil {
			t.Fatalf("Execute(await) error = %v", err)
		}
		if raw.ChildLoopRunID != "child-1" || raw.Status != "awaiting_child" {
			t.Fatalf("await raw = %#v, want child awaiting status", raw)
		}
		start := starter.mustLastStart(t)
		if start.inputs.ParentLoopRunID != "parent-1" || start.inputs.Values["ticket"] != "T-1" {
			t.Fatalf("start inputs = %#v, want parent + rendered inputs", start.inputs)
		}

		detachNode := awaitNode
		detachNode.Params["mode"] = string(dsl.RunLoopDetach)
		raw, err = executor.Execute(context.Background(), detachNode, loop.ActionExecutionInput{
			WorkspaceID: "ws-1",
			LoopRunID:   "parent-1",
			Namespace:   map[string]any{"inputs": map[string]any{"ticket": "T-2"}},
		})
		if err != nil {
			t.Fatalf("Execute(detach) error = %v", err)
		}
		if raw.Status != "" || raw.ChildLoopRunID != "child-1" {
			t.Fatalf("detach raw = %#v, want child id without awaiting status", raw)
		}
	})

	t.Run("Should evaluate transform map without side effects", func(t *testing.T) {
		t.Parallel()

		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{})
		executor, err := actions.Resolve(context.Background(), tools.Scope{}, string(dsl.ActionTransform))
		if err != nil {
			t.Fatalf("Resolve(transform) error = %v", err)
		}
		node := dsl.Node{
			ID:    "map",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{
					"summary": map[string]any{"from": "nodes.agent.output.summary"},
					"label":   map[string]any{"template": "Ticket {{ .inputs.ticket }}"},
					"count":   map[string]any{"value": 2},
					"literal": map[string]any{"value": "Keep {{ .inputs.ticket }} literal"},
				},
			},
		}
		raw, err := executor.Execute(context.Background(), node, loop.ActionExecutionInput{
			Namespace: map[string]any{
				"inputs": map[string]any{"ticket": "T-1"},
				"nodes": map[string]any{
					"agent": map[string]any{
						"output": map[string]any{"summary": "done"},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("Execute(transform) error = %v", err)
		}
		output, err := executor.Harvest(context.Background(), raw, node)
		if err != nil {
			t.Fatalf("Harvest(transform) error = %v", err)
		}
		got, ok := output.Value.(map[string]any)
		if !ok {
			t.Fatalf("transform value = %T, want map", output.Value)
		}
		if got["summary"] != "done" || got["label"] != "Ticket T-1" || got["count"] != 2 {
			t.Fatalf("transform value = %#v, want mapped fields", got)
		}
		if got["literal"] != "Keep {{ .inputs.ticket }} literal" {
			t.Fatalf("transform literal = %#v, want unrendered literal", got["literal"])
		}
	})
}

func newActionRegistryForTest(
	t *testing.T,
	registry *fakeActionToolRegistry,
	opts ...loop.ActionRegistryOption,
) *loop.ActionRegistry {
	t.Helper()

	actions, err := loop.NewActionRegistry(registry, opts...)
	if err != nil {
		t.Fatalf("NewActionRegistry() error = %v", err)
	}
	return actions
}

func mustDaemonActor(t *testing.T) task.ActorContext {
	t.Helper()

	actor, err := task.DeriveDaemonActorContext("loop", "")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	return actor
}

type fakeActionToolRegistry struct {
	mu         sync.Mutex
	views      map[tools.ToolID]tools.ToolView
	getErr     error
	callResult tools.ToolResult
	callErr    error
	gets       []tools.ToolID
	calls      []tools.CallRequest
}

func (r *fakeActionToolRegistry) List(context.Context, tools.Scope) ([]tools.ToolView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	views := make([]tools.ToolView, 0, len(r.views))
	for id := range r.views {
		views = append(views, r.views[id])
	}
	return views, nil
}

func (r *fakeActionToolRegistry) Search(context.Context, tools.Scope, tools.SearchQuery) ([]tools.ToolView, error) {
	return r.List(context.Background(), tools.Scope{})
}

func (r *fakeActionToolRegistry) Get(_ context.Context, _ tools.Scope, id tools.ToolID) (tools.ToolView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.gets = append(r.gets, id)
	if r.getErr != nil {
		return tools.ToolView{}, r.getErr
	}
	view, ok := r.views[id]
	if !ok {
		return tools.ToolView{}, tools.NewToolError(
			tools.ErrorCodeNotFound,
			id,
			"missing",
			tools.ErrToolNotFound,
			tools.ReasonToolUnknown,
		)
	}
	return view, nil
}

func (r *fakeActionToolRegistry) Call(
	_ context.Context,
	_ tools.Scope,
	req tools.CallRequest,
) (tools.ToolResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, req)
	if r.callErr != nil {
		return tools.ToolResult{}, r.callErr
	}
	return r.callResult, nil
}

func (r *fakeActionToolRegistry) getCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.gets)
}

func (r *fakeActionToolRegistry) mustSingleCall(t *testing.T) tools.CallRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.calls) != 1 {
		t.Fatalf("Call count = %d, want 1", len(r.calls))
	}
	return r.calls[0]
}

type fakeActionEventReader struct {
	mu     sync.Mutex
	last   loop.ActionEventRangeRequest
	result loop.ActionEventRangeResult
	err    error
}

func (r *fakeActionEventReader) ReadActionEventRange(
	_ context.Context,
	//nolint:gocritic // The fake implements the production interface under test.
	req loop.ActionEventRangeRequest,
) (loop.ActionEventRangeResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.last = req
	if r.err != nil {
		return loop.ActionEventRangeResult{}, r.err
	}
	return r.result, nil
}

type fakeChannelResultHarvester struct {
	mu     sync.Mutex
	last   loop.ChannelResultHarvestRequest
	result loop.ChannelResultHarvestResult
	err    error
}

func (h *fakeChannelResultHarvester) HarvestChannelResult(
	_ context.Context,
	req *loop.ChannelResultHarvestRequest,
) (loop.ChannelResultHarvestResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if req != nil {
		h.last = *req
	}
	if h.err != nil {
		return loop.ChannelResultHarvestResult{}, h.err
	}
	return h.result, nil
}

type fakeActionSessionBinder struct {
	mu             sync.Mutex
	binding        loop.ActionSessionBinding
	binds          []loop.ActionSessionBindRequest
	prompts        []loop.ActionPromptRequest
	promptResults  []loop.ActionPromptResult
	promptErr      error
	blockPrompt    bool
	cancels        []loop.ActionSessionBinding
	cancelDeadline bool
}

func (b *fakeActionSessionBinder) BindActionSession(
	_ context.Context,
	req loop.ActionSessionBindRequest,
) (loop.ActionSessionBinding, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.binds = append(b.binds, req)
	return b.binding, nil
}

func (b *fakeActionSessionBinder) PromptActionSession(
	ctx context.Context,
	_ loop.ActionSessionBinding,
	req loop.ActionPromptRequest,
) (loop.ActionPromptResult, error) {
	b.mu.Lock()
	b.prompts = append(b.prompts, req)
	block := b.blockPrompt
	if !block {
		if b.promptErr != nil {
			err := b.promptErr
			b.mu.Unlock()
			return loop.ActionPromptResult{}, err
		}
		if len(b.promptResults) == 0 {
			b.mu.Unlock()
			return loop.ActionPromptResult{}, errors.New("missing prompt result")
		}
		result := b.promptResults[0]
		b.promptResults = b.promptResults[1:]
		if req.UsageReporter != nil && result.TokensUsed > 0 {
			req.UsageReporter.ReportActionTokensUsed(result.TokensUsed)
		}
		b.mu.Unlock()
		return result, nil
	}
	b.mu.Unlock()

	<-ctx.Done()
	return loop.ActionPromptResult{}, ctx.Err()
}

func (b *fakeActionSessionBinder) CancelActionSession(
	ctx context.Context,
	binding loop.ActionSessionBinding,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, b.cancelDeadline = ctx.Deadline()
	b.cancels = append(b.cancels, binding)
	return nil
}

func (b *fakeActionSessionBinder) mustSingleBind(t *testing.T) loop.ActionSessionBindRequest {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.binds) != 1 {
		t.Fatalf("bind count = %d, want 1", len(b.binds))
	}
	return b.binds[0]
}

func (b *fakeActionSessionBinder) promptMessages() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	messages := make([]string, 0, len(b.prompts))
	for _, req := range b.prompts {
		messages = append(messages, req.Message)
	}
	return messages
}

func (b *fakeActionSessionBinder) cancelCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.cancels)
}

func (b *fakeActionSessionBinder) cancelHadDeadline() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.cancelDeadline
}

type fakeActionLoopStarter struct {
	mu                    sync.Mutex
	starts                []fakeLoopStart
	returnRun             *loop.Run
	err                   error
	blockUntilContextDone bool
}

type fakeLoopStart struct {
	ws          loop.WorkspaceID
	name        string
	inputs      loop.Inputs
	actor       task.ActorContext
	hadDeadline bool
}

func (s *fakeActionLoopStarter) Start(
	ctx context.Context,
	ws loop.WorkspaceID,
	name string,
	inputs loop.Inputs,
	actor task.ActorContext,
) (*loop.Run, error) {
	_, hadDeadline := ctx.Deadline()
	if s.blockUntilContextDone {
		<-ctx.Done()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.starts = append(s.starts, fakeLoopStart{
		ws:          ws,
		name:        name,
		inputs:      inputs,
		actor:       actor,
		hadDeadline: hadDeadline,
	})
	if s.err != nil {
		return nil, s.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.returnRun, nil
}

func (s *fakeActionLoopStarter) mustLastStart(t *testing.T) fakeLoopStart {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.starts) == 0 {
		t.Fatal("Start calls = 0, want at least 1")
	}
	return s.starts[len(s.starts)-1]
}

func TestActionTimeoutShouldCompleteQuickly(t *testing.T) {
	t.Parallel()

	t.Run("Should return timeout within parent deadline", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		binder := &fakeActionSessionBinder{
			binding:     loop.ActionSessionBinding{SessionID: "sess-timeout"},
			blockPrompt: true,
		}
		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{}, loop.WithActionSessionBinder(binder))
		executor, err := actions.Resolve(ctx, tools.Scope{}, string(dsl.ActionRunAgent))
		if err != nil {
			t.Fatalf("Resolve(run-agent) error = %v", err)
		}
		_, err = executor.Execute(ctx, dsl.Node{
			ID:      "agent",
			Class:   dsl.NodeClassAction,
			Kind:    string(dsl.ActionRunAgent),
			Timeout: "1ms",
			Params: dsl.NodeParams{
				"agent":  "planner",
				"prompt": "wait",
			},
		}, loop.ActionExecutionInput{})
		if !errors.Is(err, loop.ErrActionTimeout) {
			t.Fatalf("Execute() error = %v, want ErrActionTimeout", err)
		}
	})

	t.Run("Should cancel the bound session when the runtime supplies the deadline", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		binder := &fakeActionSessionBinder{
			binding:     loop.ActionSessionBinding{SessionID: "sess-runtime-timeout"},
			blockPrompt: true,
		}
		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{}, loop.WithActionSessionBinder(binder))
		executor, err := actions.Resolve(ctx, tools.Scope{}, string(dsl.ActionRunAgent))
		if err != nil {
			t.Fatalf("Resolve(run-agent) error = %v", err)
		}
		_, err = executor.Execute(ctx, dsl.Node{
			ID:    "agent",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{
				"agent":  "planner",
				"prompt": "wait",
			},
		}, loop.ActionExecutionInput{})
		if !errors.Is(err, loop.ErrActionTimeout) {
			t.Fatalf("Execute() error = %v, want ErrActionTimeout", err)
		}
		if got, want := binder.cancelCount(), 1; got != want {
			t.Fatalf("CancelActionSession() calls = %d, want %d", got, want)
		}
		if !binder.cancelHadDeadline() {
			t.Fatal("CancelActionSession() context had no bounded deadline")
		}
	})

	t.Run("Should bound run-loop start by node timeout", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		starter := &fakeActionLoopStarter{blockUntilContextDone: true}
		actions := newActionRegistryForTest(t, &fakeActionToolRegistry{}, loop.WithActionLoopStarter(starter))
		executor, err := actions.Resolve(ctx, tools.Scope{}, string(dsl.ActionRunLoop))
		if err != nil {
			t.Fatalf("Resolve(run-loop) error = %v", err)
		}

		_, err = executor.Execute(ctx, dsl.Node{
			ID:      "child",
			Class:   dsl.NodeClassAction,
			Kind:    string(dsl.ActionRunLoop),
			Timeout: "1ms",
			Params: dsl.NodeParams{
				"loop": "qa-child",
			},
		}, loop.ActionExecutionInput{WorkspaceID: "ws-1"})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Execute(run-loop timeout) error = %v, want context deadline exceeded", err)
		}
		if !starter.mustLastStart(t).hadDeadline {
			t.Fatal("Start context had no deadline, want node timeout deadline")
		}
	})
}
