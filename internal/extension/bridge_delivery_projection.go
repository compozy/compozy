package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/transcript"
)

func projectionEventFromAgentEvent(
	event *acp.AgentEvent,
) (bridgepkg.DeliveryProjectionEvent, error) {
	canonical, err := transcript.MarshalAgentEvent(*event)
	if err != nil {
		return bridgepkg.DeliveryProjectionEvent{}, fmt.Errorf(
			"extension: canonicalize bridge projection event: %w",
			err,
		)
	}
	fingerprint, err := canonicalProjectionFingerprint(canonical)
	if err != nil {
		return bridgepkg.DeliveryProjectionEvent{}, fmt.Errorf(
			"extension: fingerprint bridge projection event: %w",
			err,
		)
	}
	decoded, err := transcript.UnmarshalAgentEvent(canonical)
	if err != nil {
		return bridgepkg.DeliveryProjectionEvent{}, fmt.Errorf(
			"extension: decode canonical bridge projection event: %w",
			err,
		)
	}
	return projectionEventFromCanonicalAgentEvent(&decoded, fingerprint), nil
}

func canonicalProjectionFingerprint(raw string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("decode canonical event: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", errors.New("canonical event contains trailing JSON data")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode canonical event: %w", err)
	}
	return string(encoded), nil
}

func projectionEventFromCanonicalAgentEvent(
	event *acp.AgentEvent,
	fingerprint string,
) bridgepkg.DeliveryProjectionEvent {
	projected := bridgepkg.DeliveryProjectionEvent{
		Type:        event.Type,
		TurnID:      event.TurnID,
		Timestamp:   event.Timestamp,
		Text:        event.Text,
		Error:       event.Error,
		Fingerprint: fingerprint,
		ToolInput:   event.ToolInput(),
	}
	switch event.Type {
	case acp.EventTypeToolCall:
		projected.Tool = &bridgepkg.ToolProgress{
			ToolCallID: event.ToolCallID,
			ToolID:     event.ToolName(),
			Phase:      bridgepkg.ToolProgressPhaseStarted,
		}
	case acp.EventTypeToolResult:
		phase := bridgepkg.ToolProgressPhaseCompleted
		if event.ToolError() {
			phase = bridgepkg.ToolProgressPhaseFailed
		}
		projected.Tool = &bridgepkg.ToolProgress{
			ToolCallID: event.ToolCallID,
			ToolID:     event.ToolName(),
			Phase:      phase,
			Error:      event.ToolErrorDetail(),
		}
	}
	return projected
}
