package bridges

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	redactpkg "github.com/compozy/agh/internal/redact"
)

const maxToolProgressErrorRunes = bridgecontract.MaxToolProgressErrorRunes

// ToolProgressPhase is one closed tool lifecycle phase sent to bridge adapters.
type ToolProgressPhase string

const (
	ToolProgressPhaseStarted   ToolProgressPhase = ToolProgressPhase(bridgecontract.ToolProgressPhaseStarted)
	ToolProgressPhaseCompleted ToolProgressPhase = ToolProgressPhase(bridgecontract.ToolProgressPhaseCompleted)
	ToolProgressPhaseFailed    ToolProgressPhase = ToolProgressPhase(bridgecontract.ToolProgressPhaseFailed)
)

const (
	projectionEventAgentMessage = "agent_message"
	projectionEventDone         = "done"
	projectionEventError        = "error"
	projectionEventToolCall     = "tool_call"
	projectionEventToolResult   = "tool_result"
)

// Normalize returns the canonical lifecycle phase.
func (p ToolProgressPhase) Normalize() ToolProgressPhase {
	return ToolProgressPhase(bridgecontract.ToolProgressPhase(p).Normalize())
}

// Validate reports whether the phase belongs to the bridge progress contract.
func (p ToolProgressPhase) Validate() error {
	return bridgecontract.ToolProgressPhase(p).Validate()
}

// ToolProgress is the presentation-only tool lifecycle payload delivered to adapters.
type ToolProgress struct {
	ToolCallID string            `json:"tool_call_id"`
	ToolID     string            `json:"tool_id"`
	Phase      ToolProgressPhase `json:"phase"`
	Label      string            `json:"label"`
	Preview    string            `json:"preview,omitempty"`
	Emoji      string            `json:"emoji,omitempty"`
	DurationMS int64             `json:"duration_ms,omitempty"`
	Error      string            `json:"error,omitempty"`
	Index      int               `json:"index"`
}

// Validate enforces the typed, redacted progress wire contract.
func (p ToolProgress) Validate() error {
	return toolProgressToContract(p).Validate()
}

func (p ToolProgress) normalize() ToolProgress {
	return toolProgressFromContract(bridgecontract.NormalizeToolProgress(toolProgressToContract(p)))
}

func sanitizeToolProgressError(value string) string {
	value = redactpkg.String(strings.Join(strings.Fields(value), " "))
	if utf8.RuneCountInString(value) <= maxToolProgressErrorRunes {
		return value
	}
	return strings.TrimSpace(string([]rune(value)[:maxToolProgressErrorRunes]))
}

func cloneToolProgress(progress *ToolProgress) *ToolProgress {
	if progress == nil {
		return nil
	}
	cloned := progress.normalize()
	return &cloned
}

// ProjectAgentEvent is the single presentation projection gate used by live
// and replay delivery paths. The returned event is a template: the broker adds
// delivery identity, accumulated text, sequence, index, and rendered chrome.
func ProjectAgentEvent(
	event DeliveryProjectionEvent,
	mode ProgressMode,
) (DeliveryEvent, bool, error) {
	normalized := event.normalize()
	switch normalized.Type {
	case projectionEventAgentMessage:
		if normalized.Text == "" {
			return DeliveryEvent{}, false, nil
		}
		return DeliveryEvent{
			EventType: DeliveryEventTypeDelta,
			Content:   MessageContent{Text: normalized.Text},
		}, true, nil
	case projectionEventDone:
		return DeliveryEvent{EventType: DeliveryEventTypeFinal, Final: true}, true, nil
	case projectionEventError:
		return DeliveryEvent{
			EventType: DeliveryEventTypeError,
			Final:     true,
			Error:     &DeliveryErrorDetail{Message: normalized.Error},
		}, true, nil
	case projectionEventToolCall, projectionEventToolResult:
		return projectToolAgentEvent(normalized, mode)
	default:
		return DeliveryEvent{}, false, nil
	}
}

func projectToolAgentEvent(
	event DeliveryProjectionEvent,
	mode ProgressMode,
) (DeliveryEvent, bool, error) {
	mode = mode.Normalize()
	if err := mode.Validate(); err != nil {
		return DeliveryEvent{}, false, err
	}
	if mode == ProgressModeOff {
		return DeliveryEvent{}, false, nil
	}
	if event.Tool == nil {
		return DeliveryEvent{}, false, fmt.Errorf("bridges: %s projection requires tool progress", event.Type)
	}

	progress := event.Tool.normalize()
	if progress.ToolCallID == "" {
		return DeliveryEvent{}, false, errors.New("bridges: projected tool call id is required")
	}
	if event.Type == projectionEventToolCall {
		if progress.ToolID == "" {
			return DeliveryEvent{}, false, errors.New("bridges: projected tool id is required")
		}
		progress.Phase = ToolProgressPhaseStarted
	} else if progress.Phase != ToolProgressPhaseFailed {
		progress.Phase = ToolProgressPhaseCompleted
	}
	if err := progress.Phase.Validate(); err != nil {
		return DeliveryEvent{}, false, err
	}
	if progress.DurationMS < 0 {
		return DeliveryEvent{}, false, errors.New("bridges: projected tool duration cannot be negative")
	}

	// ACP input is not a trusted presentation surface. Chrome is always rebuilt
	// daemon-side after redaction and the index is allocated only after enqueue.
	progress.Label = ""
	progress.Preview = ""
	progress.Emoji = ""
	progress.Error = sanitizeToolProgressError(progress.Error)
	progress.Index = 0
	return DeliveryEvent{
		EventType: DeliveryEventTypeProgress,
		Progress:  &progress,
	}, true, nil
}
