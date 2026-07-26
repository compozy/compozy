package transcript

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

type canonicalEventPayload struct {
	Schema          string `json:"schema,omitempty"`
	Type            string `json:"type,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	TurnID          string `json:"turn_id,omitempty"`
	ClientMessageID string `json:"client_message_id,omitempty"`
	RequestID       string `json:"request_id,omitempty"`
	store.EventCorrelation
	Timestamp         time.Time                        `json:"timestamp"`
	Text              string                           `json:"text,omitempty"`
	AuthoredText      string                           `json:"authored_text,omitempty"`
	Title             string                           `json:"title,omitempty"`
	ToolName          string                           `json:"tool_name,omitempty"`
	ToolKind          string                           `json:"tool_kind,omitempty"`
	ToolCallID        string                           `json:"tool_call_id,omitempty"`
	ToolInput         json.RawMessage                  `json:"tool_input,omitempty"`
	ToolResult        *ToolResult                      `json:"tool_result,omitempty"`
	ToolError         bool                             `json:"tool_error,omitempty"`
	StopReason        string                           `json:"stop_reason,omitempty"`
	PromptStopReason  acp.PromptStopReason             `json:"prompt_stop_reason,omitempty"`
	Action            string                           `json:"action,omitempty"`
	Resource          string                           `json:"resource,omitempty"`
	Decision          string                           `json:"decision,omitempty"`
	Error             string                           `json:"error,omitempty"`
	Failure           *store.SessionFailure            `json:"failure,omitempty"`
	Synthetic         *acp.PromptSyntheticMeta         `json:"synthetic,omitempty"`
	Goal              *acp.GoalPromptMeta              `json:"goal,omitempty"`
	AvailableCommands []store.SessionAdvertisedCommand `json:"available_commands,omitempty"`
	Usage             *acp.TokenUsage                  `json:"usage,omitempty"`
	Runtime           *acp.RuntimeActivity             `json:"runtime,omitempty"`
	Raw               json.RawMessage                  `json:"raw,omitempty"`
}
