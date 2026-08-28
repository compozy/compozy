package acp

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// AvailableCommandSet is the optional replacement payload carried only by
// available-commands events.
type AvailableCommandSet []store.SessionAdvertisedCommand

// NewAvailableCommandSet returns an isolated, explicitly present replacement
// set. An empty input remains distinguishable from an absent event payload.
func NewAvailableCommandSet(commands []store.SessionAdvertisedCommand) *AvailableCommandSet {
	cloned := AvailableCommandSet(store.CloneSessionAdvertisedCommands(commands))
	if cloned == nil {
		cloned = AvailableCommandSet{}
	}
	return &cloned
}

// Values returns an isolated slice representation of the replacement set.
func (s *AvailableCommandSet) Values() []store.SessionAdvertisedCommand {
	if s == nil {
		return nil
	}
	return store.CloneSessionAdvertisedCommands([]store.SessionAdvertisedCommand(*s))
}

// AgentEvent is the stream item exposed to session/.
type AgentEvent struct {
	Type      string
	SessionID string
	TurnID    string
	store.EventCorrelation
	Timestamp        time.Time
	Text             string
	Title            string
	ToolCallID       string
	payload          *agentEventPayload
	StopReason       string
	PromptStopReason PromptStopReason
	Action           string
	Resource         string
	Decision         string
	Error            string
	Failure          *store.SessionFailure
	Synthetic        *PromptSyntheticMeta
	Goal             *GoalPromptMeta
	Usage            *TokenUsage
	Runtime          *RuntimeActivity
	ReportedTerminal *AgentReportedTerminal
	Raw              json.RawMessage
}

// Origin returns the source classification derived from the event type.
func (e AgentEvent) Origin() string {
	if e.Type == EventTypeAgentReportedTerminal {
		return AgentEventOriginAgentReported
	}
	return ""
}
