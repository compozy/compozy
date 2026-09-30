package session

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

// DeriveSeed names how a derived session is seeded.
type DeriveSeed string

const (
	// DeriveSeedReplay seeds the child with the bounded carried context.
	DeriveSeedReplay DeriveSeed = store.SessionDerivationSeedReplay
	// DeriveSeedNativeFork seeds the child with the agent's own session clone.
	DeriveSeedNativeFork DeriveSeed = store.SessionDerivationSeedNativeFork
)

var (
	// ErrSessionNotDerivable rejects a continue or fork of a non-user session.
	ErrSessionNotDerivable = errors.New("only user sessions can be continued or forked")
	// ErrDeriveSourceArchived rejects a continue or fork of an archived source; it
	// matches ErrSessionArchived.
	ErrDeriveSourceArchived = fmt.Errorf("%w; unarchive it first", ErrSessionArchived)
	// ErrDeriveTurnInProgress reports a cut turn without a terminal event.
	ErrDeriveTurnInProgress = errors.New("the cut turn has not settled")
	// ErrDeriveFenceConflict reports client fences that differ from the snapshot.
	ErrDeriveFenceConflict = errors.New("transcript changed since the fences were read")
	// ErrDeriveIdempotencyConflict reports a derive key reused with a different request.
	ErrDeriveIdempotencyConflict = errors.New("idempotency key was used with a different request")
	// ErrDeriveRouteNotFound reports a declared route that does not exist (or changed).
	ErrDeriveRouteNotFound = errors.New("route_not_found")
	// ErrDeriveMessageNotFound reports a fork anchor that is not in the source session.
	ErrDeriveMessageNotFound = errors.New("message not found in session")
	// ErrDeriveAgentNotFound reports an unknown target agent.
	ErrDeriveAgentNotFound = errors.New("agent not found")
)

// DeriveRuntime is an explicit runtime for the derived session's first bind.
type DeriveRuntime struct {
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           speedpkg.Speed
	ACPOptions      []acp.SessionConfigOptionSelection
}

// DeriveFences are the optional transcript fences a client observed; all three or none.
type DeriveFences struct {
	ExpectedEpoch       *int64
	ExpectedGeneration  *int64
	ExpectedMaxSequence *int64
}

// ContinueSessionOpts continues a user session with another agent, runtime, or route.
type ContinueSessionOpts struct {
	SourceSessionID string
	WorkspaceID     string
	ProfileID       string
	AgentName       string
	// Runtime is exclusive with Route.
	Runtime *DeriveRuntime
	// Route is the 1-based declared route of AgentName's fallback_chain; 0 = none.
	Route          int
	Name           string
	Message        string
	IdempotencyKey string
	Fences         DeriveFences
}

// ForkSessionOpts forks a user session with the same agent, runtime, and account; the
// whole conversation, or through one durable user message and its turn.
type ForkSessionOpts struct {
	SourceSessionID string
	WorkspaceID     string
	ProfileID       string
	// MessageID is the durable user message to cut through; "" forks the whole session.
	MessageID      string
	Name           string
	IdempotencyKey string
	Fences         DeriveFences
}

// DeriveResult is the outcome of one continue or fork, fresh or replayed from its receipt.
type DeriveResult struct {
	// Child is the derived session's current read model; nil when the child was deleted.
	Child                *Info
	Kind                 store.LineageKind
	SourceSessionID      string
	OriginAgentName      string
	OriginMessageID      string
	Seed                 DeriveSeed
	NativeState          string
	ACPSessionID         string
	NativeForkError      string
	ReplayMessageCount   int
	ReplayBytes          int
	Truncated            bool
	OmittedCount         int
	SourceTurnInProgress bool
	ThroughTurnID        string
	FirstPrompt          string
	Replayed             bool
	ChildDeleted         bool
	ChildSessionID       string
}

// DerivePreview reports what a continue or fork of the source would carry right now.
type DerivePreview struct {
	MessageCount int
	// SourceMessageCount is the whole source transcript's message count (inherited
	// context plus every settled turn), before any fork cut or budget bounding.
	SourceMessageCount   int
	ReplayBytes          int
	OmittedCount         int
	Truncated            bool
	SourceTurnInProgress bool
	Cut                  *DeriveCut
	Epoch                int64
	Generation           int64
	MaxSequence          int64
	NativeForkPossible   bool
}

// DeriveCut is the turn-resolved cut of a fork anchor.
type DeriveCut struct {
	MessageID       string
	TurnID          string
	ThroughSequence int64
	TurnSettled     bool
}

// deriveError carries an operator-facing message while matching its sentinel.
type deriveError struct {
	sentinel error
	message  string
}

func (e *deriveError) Error() string {
	return e.message
}

func (e *deriveError) Unwrap() error {
	return e.sentinel
}

func deriveErr(sentinel error, format string, args ...any) error {
	return &deriveError{sentinel: sentinel, message: fmt.Sprintf(format, args...)}
}

func newDeriveRouteError(agentName string, declared int, index int) error {
	return deriveErr(
		ErrDeriveRouteNotFound,
		"route_not_found: agent %q declares %d route(s); route %d does not exist",
		agentName, declared, index,
	)
}
