package watch

import (
	"context"
	"encoding/json"
	"time"
)

// Poller calls one watch-source provider for a coordinator-owned poll cycle.
type Poller interface {
	Poll(ctx context.Context, req PollRequest) (PollResponse, error)
}

// PollerFunc adapts a function to Poller.
type PollerFunc func(context.Context, PollRequest) (PollResponse, error)

// Poll implements Poller.
func (f PollerFunc) Poll(ctx context.Context, req PollRequest) (PollResponse, error) {
	return f(ctx, req)
}

// PollRequest is the daemon -> extension watch/poll service payload.
type PollRequest struct {
	Spec                json.RawMessage `json:"spec"`
	ExpectedStateDigest string          `json:"expected_state_digest,omitempty"`
}

// PollResponse is the extension -> daemon watch/poll service result.
type PollResponse struct {
	Ready       bool            `json:"ready"`
	StateDigest string          `json:"state_digest,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	SettledAt   *time.Time      `json:"settled_at,omitempty"`
}

// Outcome classifies the coordinator-owned watch claim result.
type Outcome string

const (
	OutcomeWaiting Outcome = "waiting"
	OutcomeReady   Outcome = "ready"
	OutcomeStalled Outcome = "stalled"
)

// Transition records one ephemeral FSM transition from a coordinator-owned tick.
type Transition struct {
	Event string
	From  string
	To    string
}

// TickRequest carries durable re-entry state plus the current poll input.
type TickRequest struct {
	Spec                json.RawMessage
	ExpectedStateDigest string
	LastProgressAt      time.Time
	SilenceWindow       time.Duration
	Now                 time.Time
}

// TickResult is the adapter result consumed by the loop coordinator.
type TickResult struct {
	Outcome     Outcome
	Response    PollResponse
	Transitions []Transition
}
