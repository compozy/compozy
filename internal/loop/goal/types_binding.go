package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
)

// BindingOwnership determines whether terminal cleanup may stop the bound session.
type BindingOwnership string

const (
	BindingOwnershipOriginBorrowed BindingOwnership = "origin-borrowed"
	BindingOwnershipRunOwned       BindingOwnership = "run-owned"
)

// BindingState is the append-only binding-attempt lifecycle.
type BindingState string

const (
	BindingStateCreating BindingState = "creating"
	BindingStateActive   BindingState = "active"
	BindingStateFailed   BindingState = "failed"
	BindingStateClosed   BindingState = "closed"
	BindingStateReseeded BindingState = "reseeded"
)

// BindingKey identifies one run-local continuous handle.
type BindingKey struct {
	WorkspaceID loop.WorkspaceID
	LoopRunID   loop.RunID
	Handle      string
}

// Validate enforces workspace-safe binding identity.
func (k BindingKey) Validate() error {
	if strings.TrimSpace(string(k.WorkspaceID)) == "" ||
		strings.TrimSpace(string(k.LoopRunID)) == "" || strings.TrimSpace(k.Handle) == "" {
		return fmt.Errorf("%w: binding workspace_id, loop_run_id, and handle are required", loop.ErrValidation)
	}
	return nil
}

// SessionBinding is one immutable binding attempt plus its lifecycle timestamps.
type SessionBinding struct {
	Key                BindingKey
	BindingEpoch       int64
	BindingAttemptID   string
	SessionID          string
	CreationProfileRef string
	PolicySpecDigest   string
	CreationDigest     string
	Ownership          BindingOwnership
	State              BindingState
	AdoptedGeneration  int
	AdoptionAttemptID  string
	FailureCode        string
	CreatedAt          time.Time
	ActivatedAt        *time.Time
	FailedAt           *time.Time
	ClosedAt           *time.Time
}

// GetOrCreateBindingRequest atomically adopts a validated origin or returns its active epoch.
type GetOrCreateBindingRequest struct {
	Key                            BindingKey
	CheckpointKey                  TurnKey
	ExpectedControlEpoch           int64
	ExpectedCheckpointPhase        string
	ExpectedTaskRunID              string
	ExpectedQueueEntryID           string
	ExpectedPromptID               string
	ExpectedCheckpointBindingEpoch int64
	ExpectedCheckpointSessionID    string
	ExpectedCheckpointHandle       string
	BindingAttemptID               string
	SessionID                      string
	CreationProfileRef             string
	PolicySpecDigest               string
	CreationDigest                 string
	Ownership                      BindingOwnership
	CreatedAt                      time.Time
}

// PrepareBindingAttemptRequest persists a run-owned creating attempt before session creation.
type PrepareBindingAttemptRequest struct {
	Key                BindingKey
	BindingEpoch       int64
	BindingAttemptID   string
	SessionID          string
	CreationProfileRef string
	PolicySpecDigest   string
	CreationDigest     string
	CreatedAt          time.Time
}

// AdvanceBindingCreationFailureRequest atomically consumes one Goal-owned creation attempt.
type AdvanceBindingCreationFailureRequest struct {
	Key                            BindingKey
	CheckpointKey                  TurnKey
	ExpectedControlEpoch           int64
	ExpectedCheckpointPhase        string
	ExpectedTaskRunID              string
	ExpectedQueueEntryID           string
	ExpectedPromptID               string
	ExpectedCheckpointBindingEpoch int64
	ExpectedCheckpointSessionID    string
	ExpectedCheckpointHandle       string
	ExpectedPromptAttempt          int
	ExpectedBindingEpoch           int64
	ExpectedBindingAttemptID       string
	ExpectedBindingSessionID       string
	ExpectedBindingProfileRef      string
	ExpectedBindingPolicyDigest    string
	ExpectedBindingCreationDigest  string
	FailureCode                    string
	FailedAt                       time.Time
	PrepareSuccessor               bool
	SuccessorBindingEpoch          int64
	SuccessorBindingAttemptID      string
	SuccessorSessionID             string
	CreationProfileRef             string
	PolicySpecDigest               string
	SuccessorCreationDigest        string
	SuccessorCreatedAt             time.Time
}

// SettleStoppedBindingCreationRequest closes the post-effect race for a binding stopped while creating.
type SettleStoppedBindingCreationRequest struct {
	Key                  BindingKey
	ExpectedBindingEpoch int64
	SessionID            string
}

// ActivateBindingRequest atomically swaps one creating run-owned attempt into active state.
type ActivateBindingRequest struct {
	Key                  BindingKey
	CheckpointKey        *TurnKey
	ExpectedBindingEpoch int64
	ExpectedControlEpoch int64
	GrantID              int64
	ActivatedAt          time.Time
}

// CloseBindingRequest closes one exact active binding without adopting another epoch.
type CloseBindingRequest struct {
	Key                  BindingKey
	ExpectedBindingEpoch int64
	ClosedAt             time.Time
}

// BindingStore owns continuous-session binding CAS and audit history.
type BindingStore interface {
	GetOrCreateSessionBinding(context.Context, GetOrCreateBindingRequest) (SessionBinding, error)
	PrepareSessionBindingAttempt(context.Context, PrepareBindingAttemptRequest) (SessionBinding, error)
	AdvanceBindingCreationFailure(context.Context, AdvanceBindingCreationFailureRequest) error
	SettleStoppedSessionBindingCreation(context.Context, SettleStoppedBindingCreationRequest) (bool, error)
	FinalizeSessionBindingCreation(context.Context, ActivateBindingRequest) (SessionBinding, bool, error)
	ActivateSessionBinding(context.Context, ActivateBindingRequest) (SessionBinding, error)
	CloseSessionBinding(context.Context, CloseBindingRequest) error
	GetActiveSessionBinding(context.Context, BindingKey) (SessionBinding, error)
	GetSessionBindingAttempt(context.Context, BindingKey, int64) (SessionBinding, error)
}
