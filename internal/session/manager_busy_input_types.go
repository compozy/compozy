package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
)

// BusyInputMode selects how a user-facing prompt behaves while a session is busy.
type BusyInputMode string

const (
	BusyInputModeQueue     BusyInputMode = "queue"
	BusyInputModeInterrupt BusyInputMode = "interrupt"
	BusyInputModeSteer     BusyInputMode = "steer"
)

// SendPromptOpts carries one user-facing prompt plus optional busy-input mode.
type SendPromptOpts struct {
	Message           string
	MessageID         string
	IdempotencyKey    string
	Mode              BusyInputMode
	Runtime           *RuntimeSelection
	DeliveryContext   context.Context
	Caller            PromptCaller
	AllowGoalCommands bool
}

// SendPromptResult reports whether input streamed immediately or was staged.
type SendPromptResult struct {
	Status                     string
	Mode                       BusyInputMode
	MessageID                  string
	IdempotencyKey             string
	Replayed                   bool
	Events                     <-chan acp.AgentEvent
	QueueEntryID               string
	QueuePosition              int
	QueueGeneration            int64
	EstimatedSendAt            *time.Time
	PreviousTurnID             string
	NewTurnID                  string
	Interrupted                bool
	Staged                     bool
	Queued                     bool
	CanceledQueuedEntries      int
	FallbackModeIfNoToolResult string
	Goal                       *GoalCommandResult
}

// SteerPromptOpts carries one explicitly identified steering command.
type SteerPromptOpts struct {
	Message        string
	MessageID      string
	IdempotencyKey string
}
