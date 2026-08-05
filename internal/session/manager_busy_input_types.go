package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
)

// PendingInput is one daemon-owned operator input waiting for dispatch.
type PendingInput struct {
	ID               string
	SessionID        string
	MessageID        string
	IdempotencyKey   string
	TargetTurnID     string
	Status           string
	Mode             BusyInputMode
	Delivery         string
	Text             string
	QueueGeneration  int64
	EnqueuedAt       time.Time
	Runtime          *RuntimeSelection
	SkillInvocations []commandpkg.Invocation
}

// ReplacePendingInputOpts carries an atomic queued-input replacement.
type ReplacePendingInputOpts struct {
	Text           string
	MessageID      string
	IdempotencyKey string
	AllowCommands  bool
	Caller         PromptCaller
}

// PromotePendingInputOpts carries an atomic queue-to-steer replacement.
type PromotePendingInputOpts struct {
	Text           string
	MessageID      string
	IdempotencyKey string
	ExpectedTurnID string
	AllowCommands  bool
	Caller         PromptCaller
}

// BusyInputMode selects how a user-facing prompt behaves while a session is busy.
type BusyInputMode string

const (
	BusyInputModeQueue     BusyInputMode = "queue"
	BusyInputModeInterrupt BusyInputMode = "interrupt"
	BusyInputModeSteer     BusyInputMode = "steer"
)

// SendPromptOpts carries one user-facing prompt plus optional busy-input mode.
type SendPromptOpts struct {
	Message         string
	MessageID       string
	IdempotencyKey  string
	Mode            BusyInputMode
	Runtime         *RuntimeSelection
	ExpectedTurnID  string
	DeliveryContext context.Context
	Caller          PromptCaller
	AllowCommands   bool
}

// SendPromptResult reports how and when the daemon accepted input for delivery.
type SendPromptResult struct {
	Status                string
	Mode                  BusyInputMode
	Delivery              string
	MessageID             string
	IdempotencyKey        string
	Replayed              bool
	Events                <-chan acp.AgentEvent
	QueueEntryID          string
	QueuePosition         int
	QueueGeneration       int64
	EstimatedSendAt       *time.Time
	PreviousTurnID        string
	NewTurnID             string
	CanceledQueuedEntries int
	Goal                  *GoalCommandResult
}

// SteerPromptOpts carries one explicitly identified steering command.
type SteerPromptOpts struct {
	Message        string
	MessageID      string
	IdempotencyKey string
	ExpectedTurnID string
	AllowCommands  bool
	Caller         PromptCaller
}
