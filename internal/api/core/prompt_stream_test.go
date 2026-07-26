package core_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/core"
)

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
			RequestID: "req-1",
			Title:     "Bash",
			Action:    "session/request_permission",
			Resource:  "Bash",
		}
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
	t.Run("ShouldPreserveTextToolTextOrderWithDistinctTextBlocks", func(t *testing.T) {
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
			"data-agh-event",
			"tool-input-start:tool-3",
			"tool-input-available:tool-3",
			"data-agh-event",
			"text-start:turn-mixed-text-2",
			"text-delta:turn-mixed-text-2:text 4",
			"text-delta:turn-mixed-text-2:text 5",
			"text-end:turn-mixed-text-2",
			"finish",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("frame signatures = %#v, want %#v", got, want)
		}
	})

	t.Run("ShouldCloseTextBeforeReasoningAndResumeTextInANewBlock", func(t *testing.T) {
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
			Title:      "agh__skill_view",
			Raw:        json.RawMessage(`{"tool_input":{"tool_id":"agh__skill_view"}}`),
		})

		frames := promptToolFramesFromSSE(t, writer.String())
		if len(frames) == 0 {
			t.Fatal("expected at least one tool frame")
		}
		if got, want := frames[0].ToolName, "agh__skill_view"; got != want {
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
		if frame.Type == "data-agh-permission" {
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
