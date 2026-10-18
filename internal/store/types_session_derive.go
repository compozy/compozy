package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrSessionDerivationExists reports a receipt already recorded for the same
// (workspace_id, idempotency_key); the losing registration writes nothing.
var ErrSessionDerivationExists = errors.New("store: session derivation receipt already exists")

// Derived-session seed modes and first-prompt states recorded in an outcome.
const (
	SessionDerivationSeedReplay     = "replay"
	SessionDerivationSeedNativeFork = "native_fork"

	SessionDerivationFirstPromptStaged   = "staged"
	SessionDerivationFirstPromptAdmitted = "admitted"
)

// SessionDerivationOutcome is the immutable answer recorded for one derive request.
type SessionDerivationOutcome struct {
	Seed                 string `json:"seed"`
	NativeState          string `json:"native_state,omitempty"`
	ACPSessionID         string `json:"acp_session_id,omitempty"`
	NativeForkError      string `json:"native_fork_error,omitempty"`
	OriginMessageID      string `json:"origin_message_id,omitempty"`
	OriginAgentName      string `json:"origin_agent_name,omitempty"`
	ThroughTurnID        string `json:"through_turn_id"`
	ReplayMessageCount   int    `json:"replay_message_count"`
	ReplayBytes          int    `json:"replay_bytes"`
	Truncated            bool   `json:"truncated"`
	OmittedCount         int    `json:"omitted_count"`
	SourceTurnInProgress bool   `json:"source_turn_in_progress"`
	SourceEpoch          int64  `json:"source_epoch"`
	SourceGeneration     int64  `json:"source_generation"`
	SourceMaxSequence    int64  `json:"source_max_sequence"`
	FirstPrompt          string `json:"first_prompt"`
	FirstAdmissionKey    string `json:"first_admission_key,omitempty"`
}

// Validate checks the outcome enums recorded with a receipt.
func (o SessionDerivationOutcome) Validate() error {
	switch o.Seed {
	case SessionDerivationSeedReplay, SessionDerivationSeedNativeFork:
	default:
		return fmt.Errorf("store: unsupported session derivation seed %q", o.Seed)
	}
	switch o.FirstPrompt {
	case SessionDerivationFirstPromptStaged, SessionDerivationFirstPromptAdmitted:
	default:
		return fmt.Errorf("store: unsupported session derivation first prompt state %q", o.FirstPrompt)
	}
	if o.ReplayMessageCount < 0 || o.ReplayBytes < 0 || o.OmittedCount < 0 {
		return errors.New("store: session derivation outcome counters cannot be negative")
	}
	return nil
}

// SessionDerivationReceipt is the durable, never-deleted record of one derive request.
type SessionDerivationReceipt struct {
	WorkspaceID        string
	ProfileID          string
	IdempotencyKey     string
	RequestFingerprint string
	SourceSessionID    string
	ChildSessionID     string
	Kind               LineageKind
	Outcome            SessionDerivationOutcome
	CreatedAt          time.Time
	ChildDeletedAt     *time.Time
}

// Validate checks the receipt identity, kind, and outcome.
func (r SessionDerivationReceipt) Validate() error {
	for _, field := range []struct {
		value string
		name  string
	}{
		{r.WorkspaceID, "session derivation workspace id"},
		{r.ProfileID, "session derivation profile id"},
		{r.IdempotencyKey, "session derivation idempotency key"},
		{r.RequestFingerprint, "session derivation request fingerprint"},
		{r.SourceSessionID, "session derivation source session id"},
		{r.ChildSessionID, "session derivation child session id"},
	} {
		if err := requireField(field.value, field.name); err != nil {
			return err
		}
	}
	if !r.Kind.Derived() {
		return fmt.Errorf("store: session derivation kind must be continue or fork, got %q", r.Kind)
	}
	if strings.TrimSpace(r.SourceSessionID) == strings.TrimSpace(r.ChildSessionID) {
		return errors.New("store: session derivation child cannot be its own source")
	}
	return r.Outcome.Validate()
}

// SessionDerivationStore persists derive receipts together with the child registration.
type SessionDerivationStore interface {
	// RegisterDerivedSession writes the child's catalog row, its creation identity, and
	// the receipt with its immutable outcome in ONE transaction. A receipt with the same
	// (workspace_id, idempotency_key) makes the whole transaction fail with
	// ErrSessionDerivationExists; nothing is written on that path.
	RegisterDerivedSession(
		ctx context.Context,
		info SessionInfo,
		identity SessionCreationIdentity,
		receipt SessionDerivationReceipt,
	) error
	// SessionDerivationReceipt loads one receipt by its workspace-scoped key.
	SessionDerivationReceipt(
		ctx context.Context,
		workspaceID string,
		idempotencyKey string,
	) (SessionDerivationReceipt, bool, error)
	// MarkDerivationChildDeleted is called by the session delete path; the receipt survives.
	MarkDerivationChildDeleted(ctx context.Context, childSessionID string, at time.Time) error
}
