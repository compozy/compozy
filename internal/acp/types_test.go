package acp

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAgentEventToolPayloadPreservesValueSemantics(t *testing.T) {
	t.Parallel()

	t.Run("Should return zero tool values when the payload is absent", func(t *testing.T) {
		t.Parallel()

		event := AgentEvent{}
		if event.HasToolPayload() {
			t.Fatal("HasToolPayload() = true, want false")
		}
		if event.ToolName() != "" || len(event.ToolInput()) != 0 ||
			event.ToolError() || event.ToolErrorDetail() != "" {
			t.Fatalf(
				"zero tool payload = (%q, %s, %t, %q), want empty values",
				event.ToolName(),
				event.ToolInput(),
				event.ToolError(),
				event.ToolErrorDetail(),
			)
		}
	})

	t.Run("Should isolate tool input and copied event payloads", func(t *testing.T) {
		t.Parallel()

		input := json.RawMessage(`{"command":"original"}`)
		event := (AgentEvent{Type: EventTypeToolCall}).WithToolDetail(
			"agh__terminal",
			input,
			true,
			"command failed",
		)
		if !event.HasToolPayload() {
			t.Fatal("HasToolPayload() = false, want true")
		}
		copied := event

		input[2] = 'X'
		returned := event.ToolInput()
		returned[2] = 'Y'
		copied = copied.WithTool("agh__read", json.RawMessage(`{"path":"README.md"}`), false)

		if got, want := event.ToolName(), "agh__terminal"; got != want {
			t.Fatalf("original tool name = %q, want %q", got, want)
		}
		if got, want := string(event.ToolInput()), `{"command":"original"}`; got != want {
			t.Fatalf("original tool input = %s, want %s", got, want)
		}
		if !event.ToolError() {
			t.Fatal("original tool error = false, want true")
		}
		if got, want := event.ToolErrorDetail(), "command failed"; got != want {
			t.Fatalf("original tool error detail = %q, want %q", got, want)
		}
		if got, want := copied.ToolName(), "agh__read"; got != want {
			t.Fatalf("copied tool name = %q, want %q", got, want)
		}
	})
}

func TestPromptMetaToMap(t *testing.T) {
	t.Parallel()

	t.Run("Should convert normalized prompt metadata", func(t *testing.T) {
		t.Parallel()

		metaMap, err := (PromptMeta{
			TurnSource: PromptTurnSourceSynthetic,
			Synthetic: &PromptSyntheticMeta{
				Reason: "wake",
			},
		}).ToMap()
		if err != nil {
			t.Fatalf("ToMap() error = %v", err)
		}
		if metaMap["turn_source"] != PromptTurnSourceSynthetic {
			t.Fatalf("ToMap() turn_source = %#v", metaMap["turn_source"])
		}
		synthetic, ok := metaMap["synthetic"].(map[string]any)
		if !ok || synthetic["reason"] != "wake" {
			t.Fatalf("ToMap() synthetic = %#v", metaMap["synthetic"])
		}
	})

	t.Run("Should return nil for empty metadata", func(t *testing.T) {
		t.Parallel()

		metaMap, err := (PromptMeta{}).ToMap()
		if err != nil {
			t.Fatalf("ToMap(empty) error = %v", err)
		}
		if metaMap != nil {
			t.Fatalf("ToMap(empty) = %#v, want nil", metaMap)
		}
	})
}

func TestAgentProcessCapsSnapshotClonesConfigOptions(t *testing.T) {
	t.Parallel()

	t.Run("Should clone config options snapshots", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		proc.setCaps(Caps{
			ConfigOptions: []SessionConfigOption{
				{
					ID:      "model",
					Kind:    SessionConfigOptionKindSelect,
					Current: "model-a",
					Values:  []SessionConfigOptionValue{{Value: "model-a"}},
				},
			},
		})

		first := proc.CapsSnapshot()
		first.ConfigOptions[0].Current = "mutated"
		first.ConfigOptions[0].Values[0].Value = "mutated"

		second := proc.CapsSnapshot()
		if second.ConfigOptions[0].Current != "model-a" || second.ConfigOptions[0].Values[0].Value != "model-a" {
			t.Fatalf("CapsSnapshot() leaked mutable config options: %#v", second.ConfigOptions)
		}
	})

	t.Run("Should replace config options through setConfigOptions", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		proc.setCaps(Caps{
			ConfigOptions: []SessionConfigOption{
				{
					ID:      "model",
					Kind:    SessionConfigOptionKindSelect,
					Current: "model-a",
					Values:  []SessionConfigOptionValue{{Value: "model-a"}},
				},
			},
		})

		proc.setConfigOptions([]SessionConfigOption{{ID: "reasoning_effort", Kind: SessionConfigOptionKindSelect}})
		updated := proc.CapsSnapshot()
		if len(updated.ConfigOptions) != 1 || updated.ConfigOptions[0].ID != "reasoning_effort" {
			t.Fatalf("setConfigOptions() = %#v", updated.ConfigOptions)
		}
	})
}

func TestEndPromptClearsActivePromptWhileEmitterIsBackpressured(t *testing.T) {
	t.Parallel()

	proc := &AgentProcess{}
	active, err := proc.beginPrompt("turn-1", 1)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}

	active.events <- AgentEvent{Type: EventTypeAgentMessage, TurnID: "turn-1"}

	emitterStarted := make(chan struct{})
	emitterDone := make(chan struct{})
	go func() {
		active.sendMu.Lock()
		close(emitterStarted)
		active.events <- AgentEvent{Type: EventTypeDone, TurnID: "turn-1"}
		select {
		case active.activity <- struct{}{}:
		default:
		}
		active.sendMu.Unlock()
		close(emitterDone)
	}()
	<-emitterStarted

	endDone := make(chan struct{})
	go func() {
		proc.endPrompt(active)
		close(endDone)
	}()

	deadline := time.Now().Add(200 * time.Millisecond)
	for proc.currentPrompt() != nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if proc.currentPrompt() != nil {
		t.Fatal("currentPrompt() remained non-nil while endPrompt waited on a backpressured sender")
	}

	select {
	case <-endDone:
		t.Fatal("endPrompt() returned before the blocked emitPromptEvent() send was able to flush")
	default:
	}

	first := <-active.events
	if first.Type != EventTypeAgentMessage {
		t.Fatalf("first queued event = %q, want %q", first.Type, EventTypeAgentMessage)
	}

	select {
	case second, ok := <-active.events:
		if !ok {
			t.Fatal("active.events closed before backpressured event was delivered")
		}
		if second.Type != EventTypeDone {
			t.Fatalf("second queued event = %q, want %q", second.Type, EventTypeDone)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for backpressured emitPromptEvent() send to complete")
	}

	select {
	case <-emitterDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("emitPromptEvent() did not return after the queue drained")
	}

	select {
	case <-endDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("endPrompt() did not finish after the queue drained")
	}

	if _, ok := <-active.events; ok {
		t.Fatal("active.events remained open after endPrompt()")
	}
}

func TestEmitPromptEventDefersToolResultUntilToolCall(t *testing.T) {
	t.Parallel()

	t.Run("Should defer tool results until the tool call", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		active, err := proc.beginPrompt("turn-1", 4)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}

		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolResult, TurnID: "turn-1", ToolCallID: "tool-1"})

		select {
		case event := <-active.events:
			t.Fatalf("deferred tool result emitted early: %#v", event)
		default:
		}

		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolCall, TurnID: "turn-1", ToolCallID: "tool-1"})

		first := <-active.events
		if first.Type != EventTypeToolCall {
			t.Fatalf("first event = %q, want %q", first.Type, EventTypeToolCall)
		}
		second := <-active.events
		if second.Type != EventTypeToolResult {
			t.Fatalf("second event = %q, want %q", second.Type, EventTypeToolResult)
		}
	})
}

func TestEmitPromptEventFlushesDeferredToolResultsBeforeDone(t *testing.T) {
	t.Parallel()

	t.Run("Should flush deferred tool results before done", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		active, err := proc.beginPrompt("turn-1", 4)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}

		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolResult, TurnID: "turn-1", ToolCallID: "tool-1"})
		proc.emitPromptEvent(AgentEvent{Type: EventTypeDone, TurnID: "turn-1"})

		first := <-active.events
		if first.Type != EventTypeToolResult {
			t.Fatalf("first event = %q, want %q", first.Type, EventTypeToolResult)
		}
		second := <-active.events
		if second.Type != EventTypeDone {
			t.Fatalf("second event = %q, want %q", second.Type, EventTypeDone)
		}
	})
}

func TestEmitPromptEventDeferredToolResultsStayBounded(t *testing.T) {
	t.Parallel()

	t.Run("Should dedupe deferred tool results by tool call id", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		active, err := proc.beginPrompt("turn-1", 8)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}

		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolResult, TurnID: "turn-1", ToolCallID: "tool-1"})
		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolResult, TurnID: "turn-1", ToolCallID: "tool-1"})

		if got, want := len(active.pendingToolResults), 1; got != want {
			t.Fatalf("len(active.pendingToolResults) = %d, want %d", got, want)
		}

		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolCall, TurnID: "turn-1", ToolCallID: "tool-1"})

		first := <-active.events
		if first.Type != EventTypeToolCall {
			t.Fatalf("first event = %q, want %q", first.Type, EventTypeToolCall)
		}
		second := <-active.events
		if second.Type != EventTypeToolResult {
			t.Fatalf("second event = %q, want %q", second.Type, EventTypeToolResult)
		}

		select {
		case event := <-active.events:
			t.Fatalf("unexpected extra deferred event = %#v", event)
		default:
		}
	})

	t.Run("Should drop the oldest deferred tool result when the cap is reached", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		active, err := proc.beginPrompt("turn-1", maxPendingToolResults*2)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}

		for i := 0; i <= maxPendingToolResults; i++ {
			toolCallID := "tool-" + strconv.Itoa(i)
			proc.emitPromptEvent(AgentEvent{Type: EventTypeToolResult, TurnID: "turn-1", ToolCallID: toolCallID})
		}

		if got, want := len(active.pendingToolResults), maxPendingToolResults; got != want {
			t.Fatalf("len(active.pendingToolResults) = %d, want %d", got, want)
		}
		if _, ok := active.pendingToolResultIDs["tool-0"]; ok {
			t.Fatal("oldest deferred tool result remained buffered after the cap was exceeded")
		}
		if _, ok := active.pendingToolResultIDs["tool-128"]; !ok {
			t.Fatal("newest deferred tool result was not retained after the cap was exceeded")
		}

		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolCall, TurnID: "turn-1", ToolCallID: "tool-0"})
		event := <-active.events
		if event.Type != EventTypeToolCall {
			t.Fatalf("oldest tool call event = %q, want %q", event.Type, EventTypeToolCall)
		}
		select {
		case event := <-active.events:
			t.Fatalf("dropped oldest tool result was still emitted: %#v", event)
		default:
		}

		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolCall, TurnID: "turn-1", ToolCallID: "tool-128"})
		first := <-active.events
		if first.Type != EventTypeToolCall {
			t.Fatalf("newest tool call event = %q, want %q", first.Type, EventTypeToolCall)
		}
		second := <-active.events
		if second.Type != EventTypeToolResult {
			t.Fatalf("newest tool result event = %q, want %q", second.Type, EventTypeToolResult)
		}
		if got, want := second.ToolCallID, "tool-128"; got != want {
			t.Fatalf("newest tool result ToolCallID = %q, want %q", got, want)
		}
	})
}

func TestLockedBufferRetainsOnlyTheLatestBytes(t *testing.T) {
	t.Parallel()

	buffer := &lockedBuffer{}
	payload := strings.Repeat("a", defaultTerminalOutputLimit) + "tail"
	if _, err := buffer.Write([]byte(payload)); err != nil {
		t.Fatalf("lockedBuffer.Write() error = %v", err)
	}

	got := buffer.String()
	if len(got) != defaultTerminalOutputLimit {
		t.Fatalf("len(buffer.String()) = %d, want %d", len(got), defaultTerminalOutputLimit)
	}
	if !strings.HasSuffix(got, "tail") {
		t.Fatalf("buffer.String() suffix = %q, want tail", got[len(got)-4:])
	}
	if strings.HasPrefix(got, "aa") {
		t.Logf("buffer head retained latest window as expected")
	}
}

func TestWaitForPromptQuiescenceHasMaximumDuration(t *testing.T) {
	t.Parallel()

	driver := New(WithPromptDrainWait(20 * time.Millisecond))
	active := &activePromptState{
		activity: make(chan struct{}, 1),
	}

	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				select {
				case active.activity <- struct{}{}:
				default:
				}
			}
		}
	}()
	defer close(stop)

	started := time.Now()
	driver.waitForPromptQuiescence(active)
	elapsed := time.Since(started)

	if elapsed > 120*time.Millisecond {
		t.Fatalf("waitForPromptQuiescence() took %v, want bounded wait", elapsed)
	}
}

func TestPromptMetaValidateSyntheticRequiresWakeupReason(t *testing.T) {
	t.Parallel()

	err := (PromptMeta{
		TurnSource: PromptTurnSourceSynthetic,
		Synthetic: &PromptSyntheticMeta{
			TaskRunID: "run-1",
		},
	}).Validate()
	if err == nil {
		t.Fatal("PromptMeta.Validate() error = nil, want synthetic validation failure")
	}
	if !strings.Contains(err.Error(), "requires a reason") {
		t.Fatalf("PromptMeta.Validate() error = %v, want missing-reason detail", err)
	}
}

func TestPromptMetaValidateRejectsSyntheticFieldsOnUserAndNetworkTurns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		meta PromptMeta
		want string
	}{
		{
			name: "user",
			meta: PromptMeta{
				TurnSource: PromptTurnSourceUser,
				Synthetic:  &PromptSyntheticMeta{Reason: "wake"},
			},
			want: "cannot include network or synthetic fields",
		},
		{
			name: "network",
			meta: PromptMeta{
				TurnSource: PromptTurnSourceNetwork,
				Synthetic:  &PromptSyntheticMeta{Reason: "wake"},
			},
			want: "cannot include synthetic fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.meta.Validate()
			if err == nil {
				t.Fatal("PromptMeta.Validate() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PromptMeta.Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestPromptSyntheticMetaNormalizeAndValidate(t *testing.T) {
	t.Parallel()

	meta := PromptSyntheticMeta{
		TaskID:    " task-1 ",
		TaskRunID: " run-1 ",
		Reason:    " task_run_completed ",
		Summary:   " ready ",
	}
	normalized := meta.Normalize()

	if got, want := normalized.TaskID, "task-1"; got != want {
		t.Fatalf("Normalize().TaskID = %q, want %q", got, want)
	}
	if got, want := normalized.TaskRunID, "run-1"; got != want {
		t.Fatalf("Normalize().TaskRunID = %q, want %q", got, want)
	}
	if got, want := normalized.Reason, "task_run_completed"; got != want {
		t.Fatalf("Normalize().Reason = %q, want %q", got, want)
	}
	if got, want := normalized.Summary, "ready"; got != want {
		t.Fatalf("Normalize().Summary = %q, want %q", got, want)
	}
	if normalized.IsZero() {
		t.Fatal("Normalize().IsZero() = true, want false")
	}
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := (PromptSyntheticMeta{}).Validate(); err == nil {
		t.Fatal("Validate(empty) error = nil, want validation failure")
	}
}

func TestPromptStopReasonShouldRequireExactWireValue(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a canonical stop reason", func(t *testing.T) {
		t.Parallel()

		if !PromptStopReasonEndTurn.Valid() {
			t.Fatal("PromptStopReasonEndTurn.Valid() = false, want true")
		}
	})

	t.Run("Should reject whitespace around a stop reason", func(t *testing.T) {
		t.Parallel()

		if PromptStopReason(" end_turn ").Valid() {
			t.Fatal("PromptStopReason(whitespace).Valid() = true, want false")
		}
	})
}
