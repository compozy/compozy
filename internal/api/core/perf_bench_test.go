package core

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

var (
	benchmarkLogsCursor        LogsCursor
	benchmarkSessionPayloads   []contract.SessionPayload
	benchmarkAgentEventPayload contract.AgentEventPayload
)

type benchmarkFlushWriter struct {
	bytes.Buffer
}

func (w *benchmarkFlushWriter) Flush() {}

func BenchmarkWriteSSE(b *testing.B) {
	b.ReportAllocs()

	writer := &benchmarkFlushWriter{}
	msg := SSEMessage{
		ID:   "evt-123",
		Name: "agent_message",
		Data: map[string]any{
			"id":    "m-001",
			"delta": "benchmark payload for SSE write path",
			"seq":   42,
		},
	}

	for b.Loop() {
		writer.Reset()
		if err := WriteSSE(writer, msg); err != nil {
			b.Fatalf("WriteSSE() error: %v", err)
		}
	}
}

func BenchmarkEmitLogs(b *testing.B) {
	b.ReportAllocs()

	events := make([]store.EventSummary, 0, 64)
	baseTime := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	for i := range 64 {
		events = append(events, store.EventSummary{
			ID:        fmt.Sprintf("evt-%03d", i),
			SessionID: "sess-1",
			Sequence:  int64(i + 1),
			Type:      "agent_message",
			AgentName: "codex",
			Summary:   "summary",
			Timestamp: baseTime.Add(time.Duration(i) * time.Millisecond),
		})
	}

	writer := &benchmarkFlushWriter{}
	for b.Loop() {
		writer.Reset()
		benchmarkLogsCursor = EmitLogs(writer, events, LogsCursor{})
	}
}

func BenchmarkSessionPayloadsFromInfos(b *testing.B) {
	b.ReportAllocs()

	infos := make([]*session.Info, 0, 256)
	baseTime := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	for i := range 256 {
		infos = append(infos, &session.Info{
			ID:                   fmt.Sprintf("sess-%03d", i),
			Name:                 fmt.Sprintf("Session %d", i),
			AgentName:            "codex",
			WorkspaceID:          "ws-alpha",
			Workspace:            "/tmp/workspace",
			NetworkParticipation: coreTestLiveParticipation("ws-alpha", "general"),
			State:                session.StateActive,
			CreatedAt:            baseTime,
			UpdatedAt:            baseTime.Add(time.Duration(i) * time.Second),
		})
	}

	for b.Loop() {
		benchmarkSessionPayloads = SessionPayloadsFromInfos(infos)
	}
}

func BenchmarkAgentEventPayloadFromEvent(b *testing.B) {
	b.ReportAllocs()

	inputTokens := int64(128)
	outputTokens := int64(64)
	totalTokens := int64(192)
	contextUsed := int64(4096)
	contextSize := int64(16384)
	costAmount := 0.0125
	currency := "USD"
	event := acp.AgentEvent{
		Type:       acp.EventTypeToolResult,
		SessionID:  "sess-1",
		TurnID:     "turn-1",
		RequestID:  "req-1",
		Timestamp:  time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		Text:       "tool completed",
		Title:      "read_file",
		ToolCallID: "tool-1",
		StopReason: "completed",
		Action:     "fs/read_text_file",
		Resource:   "/tmp/notes.md",
		Decision:   "approved",
		Error:      "",
		Usage: &acp.TokenUsage{
			TurnID:       "turn-1",
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
			TotalTokens:  &totalTokens,
			ContextUsed:  &contextUsed,
			ContextSize:  &contextSize,
			CostAmount:   &costAmount,
			CostCurrency: &currency,
			Timestamp:    time.Date(2026, 4, 17, 12, 0, 1, 0, time.UTC),
		},
		Raw: []byte(`{"result":{"path":"/tmp/notes.md","preview":"hello"},"ok":true}`),
	}

	for b.Loop() {
		benchmarkAgentEventPayload = AgentEventPayloadFromEvent(event)
	}
}

func BenchmarkPromptStreamEncoderEmit(b *testing.B) {
	b.ReportAllocs()

	events := []acp.AgentEvent{
		{
			Type:      acp.EventTypeAgentMessage,
			TurnID:    "turn-1",
			Text:      "hello from the agent",
			Timestamp: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		},
		{
			Type:      acp.EventTypeThought,
			TurnID:    "turn-1",
			Text:      "thinking through the request",
			Timestamp: time.Date(2026, 4, 17, 12, 0, 1, 0, time.UTC),
		},
		{
			Type:       acp.EventTypeToolCall,
			TurnID:     "turn-1",
			Title:      "read_file",
			ToolCallID: "tool-1",
			Raw:        []byte(`{"tool_name":"read_file","tool_input":{"path":"/tmp/notes.md"}}`),
			Timestamp:  time.Date(2026, 4, 17, 12, 0, 2, 0, time.UTC),
		},
		{
			Type:       acp.EventTypeToolResult,
			TurnID:     "turn-1",
			Title:      "read_file",
			ToolCallID: "tool-1",
			Text:       "read complete",
			Raw:        []byte(`{"result":{"path":"/tmp/notes.md","preview":"hello"},"ok":true}`),
			Timestamp:  time.Date(2026, 4, 17, 12, 0, 3, 0, time.UTC),
		},
		{
			Type:      acp.EventTypePermission,
			TurnID:    "turn-1",
			Action:    "fs/read_text_file",
			Decision:  "approved",
			Timestamp: time.Date(2026, 4, 17, 12, 0, 4, 0, time.UTC),
		},
		{
			Type:             acp.EventTypeDone,
			TurnID:           "turn-1",
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
			Timestamp:        time.Date(2026, 4, 17, 12, 0, 5, 0, time.UTC),
		},
	}
	writer := &benchmarkFlushWriter{}
	now := func() time.Time {
		return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	}

	for b.Loop() {
		b.StopTimer()
		writer.Reset()
		encoder := NewPromptStreamEncoder(now)
		b.StartTimer()
		for _, event := range events {
			if err := encoder.Emit(writer, event); err != nil {
				b.Fatalf("PromptStreamEncoder.Emit() error: %v", err)
			}
		}
	}
}
