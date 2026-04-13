package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestStartWorkflowEventStreamerLeanFiltersSessionUpdates(t *testing.T) {
	t.Parallel()

	bus := events.New[events.Event](8)
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	var stream bytes.Buffer
	streamer := startWorkflowEventStreamer(
		bus,
		&config{OutputFormat: model.OutputFormatJSON},
		&stream,
	)
	if streamer == nil {
		t.Fatal("expected workflow event streamer")
	}

	ctx := context.Background()
	started, err := newRuntimeEvent("run-123", events.EventKindRunStarted, kinds.RunStartedPayload{JobsTotal: 1})
	if err != nil {
		t.Fatalf("new runtime event: %v", err)
	}
	planUpdated, err := newRuntimeEvent(
		"run-123",
		events.EventKindSessionUpdate,
		kinds.SessionUpdatePayload{
			Index: 0,
			Update: kinds.SessionUpdate{
				Kind:   kinds.UpdateKindPlanUpdated,
				Status: kinds.StatusRunning,
			},
		},
	)
	if err != nil {
		t.Fatalf("new plan update event: %v", err)
	}
	agentChunk, err := newRuntimeEvent(
		"run-123",
		events.EventKindSessionUpdate,
		kinds.SessionUpdatePayload{
			Index: 0,
			Update: kinds.SessionUpdate{
				Kind: kinds.UpdateKindAgentMessageChunk,
				Blocks: []kinds.ContentBlock{
					mustMarshalWorkflowTextBlock(t, "task completed"),
				},
				Status: kinds.StatusRunning,
			},
		},
	)
	if err != nil {
		t.Fatalf("new agent chunk event: %v", err)
	}
	completed, err := newRuntimeEvent("run-123", events.EventKindRunCompleted, kinds.RunCompletedPayload{})
	if err != nil {
		t.Fatalf("new completed event: %v", err)
	}

	bus.Publish(ctx, started)
	bus.Publish(ctx, planUpdated)
	bus.Publish(ctx, agentChunk)
	bus.Publish(ctx, completed)

	if err := streamer.FinalizeAndStop(); err != nil {
		t.Fatalf("FinalizeAndStop: %v", err)
	}

	lines := decodeWorkflowJSONLMaps(t, stream.String())
	if len(lines) != 3 {
		t.Fatalf("expected 3 lean events, got %d\nstream:\n%s", len(lines), stream.String())
	}
	if got := lines[0]["type"]; got != string(events.EventKindRunStarted) {
		t.Fatalf("unexpected first event type: %#v", lines[0])
	}
	if got := lines[1]["type"]; got != string(events.EventKindSessionUpdate) {
		t.Fatalf("unexpected second event type: %#v", lines[1])
	}
	payload, ok := lines[1]["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected lean payload object, got %#v", lines[1]["payload"])
	}
	update, ok := payload["update"].(map[string]any)
	if !ok {
		t.Fatalf("expected lean session update payload, got %#v", payload)
	}
	if got := update["kind"]; got != string(kinds.UpdateKindAgentMessageChunk) {
		t.Fatalf("unexpected streamed session update kind: %#v", update)
	}
	if got := lines[2]["type"]; got != string(events.EventKindRunCompleted) {
		t.Fatalf("unexpected terminal event type: %#v", lines[2])
	}
}

func TestStartWorkflowEventStreamerRawEncodesCanonicalEvents(t *testing.T) {
	t.Parallel()

	bus := events.New[events.Event](8)
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	var stream bytes.Buffer
	streamer := startWorkflowEventStreamer(
		bus,
		&config{OutputFormat: model.OutputFormatRawJSON},
		&stream,
	)
	if streamer == nil {
		t.Fatal("expected workflow event streamer")
	}

	ctx := context.Background()
	started, err := newRuntimeEvent("run-raw", events.EventKindRunStarted, kinds.RunStartedPayload{JobsTotal: 1})
	if err != nil {
		t.Fatalf("new runtime event: %v", err)
	}
	completed, err := newRuntimeEvent("run-raw", events.EventKindRunCompleted, kinds.RunCompletedPayload{})
	if err != nil {
		t.Fatalf("new completed event: %v", err)
	}

	bus.Publish(ctx, started)
	bus.Publish(ctx, completed)

	if err := streamer.FinalizeAndStop(); err != nil {
		t.Fatalf("FinalizeAndStop: %v", err)
	}

	lines := decodeWorkflowJSONLMaps(t, stream.String())
	if len(lines) != 2 {
		t.Fatalf("expected 2 raw events, got %d\nstream:\n%s", len(lines), stream.String())
	}
	if got := lines[0]["kind"]; got != string(events.EventKindRunStarted) {
		t.Fatalf("unexpected first raw event: %#v", lines[0])
	}
	if got := lines[0]["run_id"]; got != "run-raw" {
		t.Fatalf("unexpected raw run id: %#v", lines[0])
	}
	if _, ok := lines[0]["type"]; ok {
		t.Fatalf("raw workflow event should not be projected into lean format: %#v", lines[0])
	}
	if got := lines[1]["kind"]; got != string(events.EventKindRunCompleted) {
		t.Fatalf("unexpected terminal raw event: %#v", lines[1])
	}
}

func decodeWorkflowJSONLMaps(t *testing.T, stream string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(stream), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode workflow jsonl line: %v\nline:\n%s", err, line)
		}
		events = append(events, payload)
	}
	return events
}

func mustMarshalWorkflowTextBlock(t *testing.T, text string) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.TextBlock{Text: text})
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}
	return block
}
