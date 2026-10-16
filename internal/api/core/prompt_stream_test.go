package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subagentid"
	"github.com/compozy/compozy/internal/transcript"
)

func TestDeliverPromptEventStream(t *testing.T) {
	t.Run("Should stream provider diagnostics before the terminal error and finish", func(t *testing.T) {
		t.Parallel()
		at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time { return at })
		if err := encoder.Start(writer, "turn-provider"); err != nil {
			t.Fatalf("start stream: %v", err)
		}
		stream := make(chan acp.AgentEvent, 1)
		stream <- acp.AgentEvent{
			Type: acp.EventTypeError, TurnID: "turn-provider", Error: "authentication required",
			ProviderError: &acp.ProviderErrorDiagnostic{
				Code: acp.ProviderErrorAuthRequired, Provider: "provider-a", NextAction: acp.ProviderFailureActionBindSecret,
				Guidance: "update credential token=provider-stream-secret", OccurrenceCount: 2, FirstSeenAt: at, LastSeenAt: at,
			},
		}
		close(stream)
		core.DeliverPromptEventStream(t.Context(), make(chan struct{}), stream, func() {}, encoder, writer)
		body := writer.String()
		want := []string{"start", "data-compozy-event", "error", "finish"}
		if got := promptFrameSignatures(t, body); !reflect.DeepEqual(got, want) {
			t.Fatalf("frame order = %#v, want %#v", got, want)
		}
		if strings.Contains(body, "provider-stream-secret") {
			t.Fatal("live provider diagnostic leaked a credential")
		}
		var diagnostic *acp.ProviderErrorDiagnostic
		for record := range strings.SplitSeq(body, "\n\n") {
			data := promptSSEData(record)
			if data == "" || data == "[DONE]" {
				continue
			}
			var frame struct {
				Type string `json:"type"`
				Data struct {
					ProviderError *acp.ProviderErrorDiagnostic `json:"provider_error"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(data), &frame); err != nil {
				t.Fatalf("decode frame: %v", err)
			}
			if frame.Type == "data-compozy-event" {
				diagnostic = frame.Data.ProviderError
			}
		}
		if diagnostic == nil || diagnostic.Code != acp.ProviderErrorAuthRequired ||
			diagnostic.NextAction != acp.ProviderFailureActionBindSecret ||
			diagnostic.OccurrenceCount != 2 {
			t.Fatalf("live provider diagnostic = %#v", diagnostic)
		}
	})

	t.Run("Should emit an explicit terminal failure when the upstream stream closes without a terminal event", func(
		t *testing.T,
	) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
		})
		if err := encoder.Start(writer, "turn-incomplete"); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		events := make(chan acp.AgentEvent, 1)
		events <- acp.AgentEvent{
			Type:   acp.EventTypeAgentMessage,
			TurnID: "turn-incomplete",
			Text:   "partial response",
		}
		close(events)

		core.DeliverPromptEventStream(
			t.Context(),
			make(chan struct{}),
			events,
			func() {},
			encoder,
			writer,
		)

		stream := writer.String()
		if !strings.Contains(stream, `"type":"error"`) ||
			!strings.Contains(stream, "ended before a terminal event") {
			t.Fatalf("stream = %q, want explicit incomplete-stream failure", stream)
		}
		if !strings.Contains(stream, `"reason_code":"prompt_stream_incomplete"`) {
			t.Fatalf("stream = %q, want machine-readable incomplete-stream reason", stream)
		}
		if !strings.Contains(stream, "data: [DONE]") {
			t.Fatalf("stream = %q, want a terminal sentinel after the failure", stream)
		}
	})
}

func TestPromptStreamEncoderStart(t *testing.T) {
	t.Run("ShouldEmitAcceptedTurnIDImmediately", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
		})

		if err := encoder.Start(writer, " turn-accepted "); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		wantStart := "data: {\"type\":\"start\",\"messageId\":\"turn-accepted\"}\n\n"
		if body := writer.String(); !strings.HasPrefix(body, wantStart) {
			t.Fatalf("start frame = %q, want accepted turn id", body)
		}
	})

	t.Run("ShouldRejectEmptyAcceptedTurnID", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(nil)
		if err := encoder.Start(writer, "   "); err == nil {
			t.Fatal("Start(empty) error = nil, want non-nil")
		}
		if body := writer.String(); body != "" {
			t.Fatalf("writer body = %q, want empty", body)
		}
	})
}

func TestPromptStreamEncoderPermissionDataPartIdentity(t *testing.T) {
	t.Run("ShouldReuseRequestIDForPendingAndFinalPermissionParts", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		})

		pending := acp.AgentEvent{
			Type:      acp.EventTypePermission,
			SessionID: "sess-1",
			TurnID:    "turn-1",
			Title:     "Bash",
			Action:    "session/request_permission",
			Resource:  "Bash",
		}.WithRequestID("req-1")
		final := pending
		final.Decision = "allow-once"

		if err := encoder.Emit(writer, pending); err != nil {
			t.Fatalf("Emit(pending) error = %v", err)
		}
		if err := encoder.Emit(writer, final); err != nil {
			t.Fatalf("Emit(final) error = %v", err)
		}

		frames := promptPermissionFramesFromSSE(t, writer.String())
		if got, want := len(frames), 2; got != want {
			t.Fatalf("len(permission frames) = %d, want %d; frames=%#v", got, want, frames)
		}
		if got, want := frames[0].ID, "req-1"; got != want {
			t.Fatalf("pending frame ID = %q, want %q", got, want)
		}
		if got, want := frames[1].ID, "req-1"; got != want {
			t.Fatalf("final frame ID = %q, want %q", got, want)
		}
		if frames[0].Data.Decision != "" {
			t.Fatalf("pending decision = %q, want empty", frames[0].Data.Decision)
		}
		if got, want := frames[1].Data.Decision, "allow-once"; got != want {
			t.Fatalf("final decision = %q, want %q", got, want)
		}
	})
}

func TestPromptStreamEncoderPartBoundaries(t *testing.T) {
	t.Run("Should preserve text-tool-text order with distinct text blocks", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
		})

		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:   acp.EventTypeAgentMessage,
			TurnID: "turn-mixed",
			Text:   "text 1",
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			TurnID:     "turn-mixed",
			ToolCallID: "tool-2",
			Title:      "Bash",
			Raw:        json.RawMessage("{\"tool_input\":{\"command\":\"pwd\"}}"),
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			TurnID:     "turn-mixed",
			ToolCallID: "tool-3",
			Title:      "Read",
			Raw:        json.RawMessage("{\"tool_input\":{\"file_path\":\"README.md\"}}"),
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:   acp.EventTypeAgentMessage,
			TurnID: "turn-mixed",
			Text:   "text 4",
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:   acp.EventTypeAgentMessage,
			TurnID: "turn-mixed",
			Text:   "text 5",
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:             acp.EventTypeDone,
			TurnID:           "turn-mixed",
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
		})

		got := promptFrameSignatures(t, writer.String())
		want := []string{
			"start",
			"text-start:turn-mixed-text-1",
			"text-delta:turn-mixed-text-1:text 1",
			"text-end:turn-mixed-text-1",
			"tool-input-start:tool-2",
			"tool-input-available:tool-2",
			"data-compozy-event",
			"tool-input-start:tool-3",
			"tool-input-available:tool-3",
			"data-compozy-event",
			"text-start:turn-mixed-text-2",
			"text-delta:turn-mixed-text-2:text 4",
			"text-delta:turn-mixed-text-2:text 5",
			"text-end:turn-mixed-text-2",
			"tool-output-available:tool-2",
			"tool-output-available:tool-3",
			"finish",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("frame signatures = %#v, want %#v", got, want)
		}
	})

	t.Run("Should close text before reasoning and resume text in a new block", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 5, 13, 9, 1, 0, 0, time.UTC)
		})

		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:   acp.EventTypeAgentMessage,
			TurnID: "turn-reasoning",
			Text:   "visible",
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:   acp.EventTypeThought,
			TurnID: "turn-reasoning",
			Text:   "checking",
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:   acp.EventTypeAgentMessage,
			TurnID: "turn-reasoning",
			Text:   "answer",
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:             acp.EventTypeDone,
			TurnID:           "turn-reasoning",
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
		})

		got := promptFrameSignatures(t, writer.String())
		want := []string{
			"start",
			"text-start:turn-reasoning-text-1",
			"text-delta:turn-reasoning-text-1:visible",
			"text-end:turn-reasoning-text-1",
			"reasoning-start:turn-reasoning-reasoning-1",
			"reasoning-delta:turn-reasoning-reasoning-1:checking",
			"reasoning-end:turn-reasoning-reasoning-1",
			"text-start:turn-reasoning-text-2",
			"text-delta:turn-reasoning-text-2:answer",
			"text-end:turn-reasoning-text-2",
			"finish",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("frame signatures = %#v, want %#v", got, want)
		}
	})

	t.Run("Should terminalize unresolved tool calls before finishing", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			TurnID:     "turn-unresolved",
			ToolCallID: "tool-open",
			Title:      "exec",
			Raw:        json.RawMessage(`{"tool_input":{"command":"make test"}}`),
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:             acp.EventTypeDone,
			TurnID:           "turn-unresolved",
			PromptStopReason: acp.PromptStopReasonEndTurn,
		})

		got := promptFrameSignatures(t, writer.String())
		want := []string{
			"start",
			"tool-input-start:tool-open",
			"tool-input-available:tool-open",
			"data-compozy-event",
			"tool-output-available:tool-open",
			"finish",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("frame signatures = %#v, want %#v", got, want)
		}
		if !strings.Contains(writer.String(), "Tool call ended before returning a result.") {
			t.Fatalf("stream = %q, want truthful unresolved tool error", writer.String())
		}
	})

	t.Run("Should emit exactly one terminal output for a completed tool call", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 7, 30, 9, 1, 0, 0, time.UTC)
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			TurnID:     "turn-completed",
			ToolCallID: "tool-completed",
			Title:      "exec",
			Raw:        json.RawMessage(`{"tool_input":{"command":"make test"}}`),
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			TurnID:     "turn-completed",
			ToolCallID: "tool-completed",
			Title:      "exec",
			Text:       "tests passed",
			Raw:        json.RawMessage(`{"result":{"exit_code":0,"output":"ok"}}`),
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:             acp.EventTypeDone,
			TurnID:           "turn-completed",
			PromptStopReason: acp.PromptStopReasonEndTurn,
		})

		terminal := make([]string, 0, 2)
		for _, signature := range promptFrameSignatures(t, writer.String()) {
			if signature == "finish" || strings.HasPrefix(signature, "tool-output-available:") {
				terminal = append(terminal, signature)
			}
		}
		want := []string{"tool-output-available:tool-completed", "finish"}
		if !reflect.DeepEqual(terminal, want) {
			t.Fatalf("terminal frame signatures = %#v, want %#v", terminal, want)
		}
	})
}

func TestPromptStreamEncoderSubagentParts(t *testing.T) {
	// Live parity with the transcript projection (UT-038 / S1, S8): the prompt stream emits the
	// same `data-compozy-subagent` cards and keeps provider-native subagent work attributed, so
	// a running turn renders cards and nesting before its transcript entry reconciles.
	t.Run("Should stream subagent cards and attribute native subagent work", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)
		})
		encoder.SetSessionID("sess-parent")
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type: acp.EventTypeToolCall, TurnID: "turn-1", ToolCallID: "toolu_agent", Title: "Review the diff",
		}.WithProviderToolMetadata("", "Agent", "pending"))
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, TurnID: "turn-1", Text: "Inside",
		}.WithProviderToolMetadata("toolu_agent", "", ""))
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, TurnID: "turn-1", Text: "Outside",
		})
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type: acp.EventTypeToolResult, TurnID: "turn-1", ToolCallID: "delegate",
			Raw: json.RawMessage(`{"rawOutput":{"subagent_id":"sub-delegated"}}`),
		}.WithTool("compozy__subagent_delegate", nil, false))

		type frame struct {
			Type             string `json:"type"`
			ID               string `json:"id"`
			ToolCallID       string `json:"toolCallId"`
			Delta            string `json:"delta"`
			ProviderMetadata *struct {
				Compozy struct {
					ParentToolCallID string `json:"parentToolCallId"`
				} `json:"compozy"`
			} `json:"providerMetadata"`
			Data struct {
				SubagentID string `json:"subagent_id"`
				ToolCallID string `json:"tool_call_id"`
				TurnID     string `json:"turn_id"`
				Origin     string `json:"origin"`
			} `json:"data"`
		}
		var frames []frame
		for record := range strings.SplitSeq(writer.String(), "\n\n") {
			data := promptSSEData(strings.TrimSpace(record))
			if data == "" || data == "[DONE]" {
				continue
			}
			var decoded frame
			if err := json.Unmarshal([]byte(data), &decoded); err != nil {
				t.Fatalf("decode frame: %v", err)
			}
			frames = append(frames, decoded)
		}
		var cards []frame
		textParents := map[string]string{}
		blockText := map[string]string{}
		for _, decoded := range frames {
			switch decoded.Type {
			case "data-compozy-subagent":
				cards = append(cards, decoded)
			case "text-start":
				parent := ""
				if decoded.ProviderMetadata != nil {
					parent = decoded.ProviderMetadata.Compozy.ParentToolCallID
				}
				textParents[decoded.ID] = parent
			case "text-delta":
				blockText[decoded.Delta] = decoded.ID
			}
		}
		if len(cards) != 2 {
			t.Fatalf("cards = %#v, want native and delegated", cards)
		}
		native, delegated := cards[0], cards[1]
		if native.ID != subagentid.Derive("sess-parent", "toolu_agent") || native.ID != native.Data.SubagentID ||
			native.Data.Origin != "provider_native" || native.Data.TurnID != "turn-1" ||
			native.Data.ToolCallID != "toolu_agent" {
			t.Fatalf("native card = %#v", native)
		}
		if delegated.ID != "sub-delegated" || delegated.Data.Origin != "delegated" ||
			delegated.Data.ToolCallID != "delegate" {
			t.Fatalf("delegated card = %#v", delegated)
		}
		inside, outside := blockText["Inside"], blockText["Outside"]
		if inside == outside || textParents[inside] != "toolu_agent" || textParents[outside] != "" {
			t.Fatalf(
				"text blocks inside=%q(%q) outside=%q(%q)",
				inside, textParents[inside], outside, textParents[outside],
			)
		}
	})
}

func TestPromptStreamEncoderToolNameResolution(t *testing.T) {
	t.Run("Should prefer typed tool metadata over conflicting legacy raw fields", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)
		})
		event := (acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			ToolCallID: "tool-typed-1",
			Title:      "descriptive title",
			Raw: json.RawMessage(
				`{"_meta":{"legacy":{"toolName":"legacy__search"}},"tool_input":{"query":"legacy"}}`,
			),
		}).WithTool("typed__search", json.RawMessage(`{"query":"typed"}`), false)

		mustEmitPromptEvent(t, encoder, writer, event)

		frames := promptToolFramesFromSSE(t, writer.String())
		if len(frames) != 2 {
			t.Fatalf("tool frames = %d, want start and input-available", len(frames))
		}
		for _, frame := range frames {
			if got, want := frame.ToolName, "typed__search"; got != want {
				t.Fatalf("tool frame toolName = %q, want %q; frame=%#v", got, want, frame)
			}
		}
		input, ok := frames[1].Input.(map[string]any)
		if !ok || input["query"] != "typed" {
			t.Fatalf("tool input = %#v, want typed query", frames[1].Input)
		}
	})

	t.Run("ShouldPreferMetaToolNameOverDescriptiveTitle", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
		})

		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			ToolCallID: "tool-read-1",
			Title:      "Read routes.go",
			Raw: json.RawMessage(
				`{"_meta":{"claudeCode":{"toolName":"Read"}},"tool_input":{"file_path":"routes.go"}}`,
			),
		})

		frames := promptToolFramesFromSSE(t, writer.String())
		if len(frames) == 0 {
			t.Fatal("expected at least one tool frame")
		}
		for _, frame := range frames {
			if got, want := frame.ToolName, "Read"; got != want {
				t.Fatalf("tool frame toolName = %q, want %q; frame=%#v", got, want, frame)
			}
		}
	})

	t.Run("ShouldFallBackToTitleWhenNoCanonicalToolName", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(func() time.Time {
			return time.Date(2026, 5, 25, 12, 1, 0, 0, time.UTC)
		})

		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			ToolCallID: "tool-custom-1",
			Title:      "compozy__skill_view",
			Raw:        json.RawMessage(`{"tool_input":{"tool_id":"compozy__skill_view"}}`),
		})

		frames := promptToolFramesFromSSE(t, writer.String())
		if len(frames) == 0 {
			t.Fatal("expected at least one tool frame")
		}
		if got, want := frames[0].ToolName, "compozy__skill_view"; got != want {
			t.Fatalf("tool frame toolName = %q, want %q", got, want)
		}
	})
}

func mustEmitPromptEvent(
	t *testing.T,
	encoder *core.PromptStreamEncoder,
	writer *bufferFlusher,
	event acp.AgentEvent,
) {
	t.Helper()
	if err := encoder.Emit(writer, event); err != nil {
		t.Fatalf("Emit(%s) error = %v", event.Type, err)
	}
}

type promptPermissionFrame struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Data struct {
		RequestID string `json:"request_id"`
		Decision  string `json:"decision"`
	} `json:"data"`
}

type promptToolFrame struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Input      any    `json:"input"`
}

func promptToolFramesFromSSE(t *testing.T, body string) []promptToolFrame {
	t.Helper()

	frames := make([]promptToolFrame, 0)
	for record := range strings.SplitSeq(body, "\n\n") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		data := promptSSEData(record)
		if data == "" || data == "[DONE]" {
			continue
		}

		var frame promptToolFrame
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", data, err)
		}
		switch frame.Type {
		case "tool-input-start", "tool-input-available":
			frames = append(frames, frame)
		}
	}
	return frames
}

func promptFrameSignatures(t *testing.T, body string) []string {
	t.Helper()

	signatures := make([]string, 0)
	for record := range strings.SplitSeq(body, "\n\n") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		data := promptSSEData(record)
		if data == "" || data == "[DONE]" {
			continue
		}

		var frame map[string]any
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", data, err)
		}
		signatures = append(signatures, promptFrameSignature(t, frame))
	}
	return signatures
}

func promptFrameSignature(t *testing.T, frame map[string]any) string {
	t.Helper()

	frameType := promptRequiredString(t, frame, "type")
	switch frameType {
	case "text-start", "text-end", "reasoning-start", "reasoning-end":
		return frameType + ":" + promptRequiredString(t, frame, "id")
	case "text-delta", "reasoning-delta":
		return frameType + ":" + promptRequiredString(t, frame, "id") + ":" +
			promptRequiredString(t, frame, "delta")
	case "tool-input-start", "tool-input-available", "tool-output-available":
		return frameType + ":" + promptRequiredString(t, frame, "toolCallId")
	default:
		return frameType
	}
}

func promptRequiredString(t *testing.T, frame map[string]any, key string) string {
	t.Helper()

	value, ok := frame[key]
	if !ok {
		t.Fatalf("frame missing %q: %#v", key, frame)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("frame[%q] = %#v, want string", key, value)
	}
	return text
}

func promptPermissionFramesFromSSE(t *testing.T, body string) []promptPermissionFrame {
	t.Helper()

	frames := make([]promptPermissionFrame, 0, 2)
	for record := range strings.SplitSeq(body, "\n\n") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		data := promptSSEData(record)
		if data == "" || data == "[DONE]" {
			continue
		}

		var frame promptPermissionFrame
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", data, err)
		}
		if frame.Type == "data-compozy-permission" {
			frames = append(frames, frame)
		}
	}
	return frames
}

func promptSSEData(record string) string {
	var data strings.Builder
	for line := range strings.SplitSeq(record, "\n") {
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			data.WriteString(after)
		}
	}
	return data.String()
}

// UT-014: live session-message parts preserve tool-call correlation.
func TestPromptStreamSessionMessage(t *testing.T) {
	t.Run("Should emit running and completed reply cards with one stable id UT-014", func(t *testing.T) {
		t.Parallel()
		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(
			func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) },
		)
		mustEmitPromptEvent(
			t,
			encoder,
			writer,
			acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				TurnID:     "turn",
				ToolCallID: "send",
			}.WithTool(
				"compozy__session_prompt",
				json.RawMessage(`{"session_id":"target","message_id":"message","mode":"queue"}`),
				false,
			),
		)
		mustEmitPromptEvent(
			t,
			encoder,
			writer,
			acp.AgentEvent{
				Type:       acp.EventTypeToolResult,
				TurnID:     "turn",
				ToolCallID: "send",
			}.WithSessionMessageCard(
				json.RawMessage(
					`{"tool_call_id":"send","message_id":"message","reply_watch_id":"rw-1","state":"done"}`,
				),
			),
		)
		var states []string
		for line := range strings.SplitSeq(writer.String(), "\n") {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var frame struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Data struct {
					Target string `json:"target_session_id"`
					State  string `json:"state"`
					Watch  string `json:"reply_watch_id"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame); err != nil {
				continue
			}
			if frame.Type != "data-compozy-session-message" {
				continue
			}
			if frame.ID != "send" || frame.Data.Target != "target" {
				t.Fatalf("frame = %+v", frame)
			}
			if frame.Data.State == "done" && frame.Data.Watch != "rw-1" {
				t.Fatalf("reply = %+v", frame)
			}
			states = append(states, frame.Data.State)
		}
		if strings.Join(states, ",") != "running,done" {
			t.Fatalf("states = %v", states)
		}
	})
	// Recorded Claude and Codex sends (#742), delivered as the session manager does: each stored
	// event decoded and annotated with the turn's SessionMessageCards fold.
	for _, tc := range []struct {
		fixture string
		want    map[string]transcript.UISessionMessagePayload
	}{
		{fixture: "claude.jsonl", want: map[string]transcript.UISessionMessagePayload{
			"toolu_01L3DE1yXY8CcKF8hkd3k8Z5": {
				ToolCallID: "toolu_01L3DE1yXY8CcKF8hkd3k8Z5", TargetSessionID: "sess-844b851f052bd406",
				TargetWorkspaceID: "ws_a5a306022a1c98a1", MessageID: "issue742-goal-unavailable-v1", Mode: "queue",
				Delivery: "direct", ReplyWatchID: "rw-f9a45066bce98c57", State: "done",
			},
		}},
		{fixture: "codex.jsonl", want: map[string]transcript.UISessionMessagePayload{
			"exec-2536d1eb-7fab-4bd7-b971-e62f6b970e22": {
				ToolCallID: "exec-2536d1eb-7fab-4bd7-b971-e62f6b970e22", TargetSessionID: "sess-844b851f052bd406",
				TargetWorkspaceID: "ws_a5a306022a1c98a1", MessageID: "736-review-remediation-1", Mode: "queue",
				Delivery: "after_turn", State: "done",
			},
			"exec-5f2789fa-43ce-4158-a1b6-190745d05f03": {
				ToolCallID: "exec-5f2789fa-43ce-4158-a1b6-190745d05f03", TargetSessionID: "sess-844b851f052bd406",
				TargetWorkspaceID: "ws_a5a306022a1c98a1", MessageID: "736-fixture-remediation-1518", Mode: "steer",
				Delivery: "interrupt_then_prompt", SteerDelivery: "injected", State: "done",
			},
		}},
	} {
		t.Run("Should stream the settled card of every recorded send in "+tc.fixture, func(t *testing.T) {
			t.Parallel()
			writer := &bufferFlusher{}
			encoder := core.NewPromptStreamEncoder(
				func() time.Time { return time.Date(2026, 10, 16, 14, 45, 0, 0, time.UTC) },
			)
			var cards transcript.SessionMessageCards
			for _, stored := range recordedPromptStream(t, tc.fixture) {
				event, err := transcript.UnmarshalAgentEvent(stored.Content)
				if err != nil {
					t.Fatal(err)
				}
				if card, ok := cards.Card(stored); ok {
					data, err := json.Marshal(card)
					if err != nil {
						t.Fatal(err)
					}
					event = event.WithSessionMessageCard(data)
				}
				mustEmitPromptEvent(t, encoder, writer, event)
			}
			got := make(map[string]transcript.UISessionMessagePayload)
			for line := range strings.SplitSeq(writer.String(), "\n") {
				var frame struct {
					Type string                             `json:"type"`
					Data transcript.UISessionMessagePayload `json:"data"`
				}
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame) != nil ||
					frame.Type != "data-compozy-session-message" {
					continue
				}
				// Every frame names its target: no client ever receives a card it cannot draw.
				if frame.Data.TargetSessionID == "" {
					t.Fatalf("frame without a target: %+v", frame.Data)
				}
				got[frame.Data.ToolCallID] = frame.Data
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("settled cards = %+v, want %+v", got, tc.want)
			}
		})
	}
	t.Run("Should attribute a send made inside a provider-native subagent to its parent call", func(t *testing.T) {
		t.Parallel()
		writer := &bufferFlusher{}
		encoder := core.NewPromptStreamEncoder(
			func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) },
		)
		mustEmitPromptEvent(t, encoder, writer, acp.AgentEvent{
			Type: acp.EventTypeToolCall, TurnID: "turn", ToolCallID: "child-send",
		}.WithTool(
			"compozy__session_prompt",
			json.RawMessage(`{"session_id":"target","message_id":"message"}`),
			false,
		).WithProviderToolMetadata("native-task", "compozy__session_prompt", ""))
		found := false
		for line := range strings.SplitSeq(writer.String(), "\n") {
			var frame struct {
				Type             string `json:"type"`
				ProviderMetadata *struct {
					Compozy struct {
						ParentToolCallID string `json:"parentToolCallId"`
					} `json:"compozy"`
				} `json:"providerMetadata"`
			}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame) != nil ||
				frame.Type != "data-compozy-session-message" {
				continue
			}
			found = true
			if frame.ProviderMetadata == nil || frame.ProviderMetadata.Compozy.ParentToolCallID != "native-task" {
				t.Fatalf("session message frame = %+v, want parent attribution", frame)
			}
		}
		if !found {
			t.Fatal("no session message frame")
		}
	})
}

// recordedPromptStream loads a recorded provider stream shared with the transcript suite.
func recordedPromptStream(t *testing.T, name string) []store.SessionEvent {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "transcript", "testdata", "session-prompt", name))
	if err != nil {
		t.Fatal(err)
	}
	var rows []store.SessionEvent
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var recorded struct {
			Type    string          `json:"type"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal([]byte(line), &recorded); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, store.SessionEvent{
			Sequence: int64(len(rows) + 1), Type: recorded.Type, Content: string(recorded.Content),
		})
	}
	return rows
}
