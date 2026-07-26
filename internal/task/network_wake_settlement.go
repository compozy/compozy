package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

// NetworkWakeSettlement is the complete token-fenced terminal command for one taskless wake.
type NetworkWakeSettlement struct {
	WorkspaceID     string
	WakeID          string
	TaskRunID       string
	ClaimToken      string
	TargetSessionID string
	OwnerKey        string
	Outcome         store.NetworkWakeOutcome
	Now             time.Time
	Actor           ActorContext
}

// NetworkWakeSettlementResult reports the single committed terminal truth.
type NetworkWakeSettlementResult struct {
	Run       Run
	Outcome   store.NetworkWakeOutcome
	Duplicate bool
}

// Normalize canonicalizes a settlement command before the transaction boundary.
func (s NetworkWakeSettlement) Normalize(defaultNow time.Time) (NetworkWakeSettlement, error) {
	s.WorkspaceID = strings.TrimSpace(s.WorkspaceID)
	s.WakeID = strings.TrimSpace(s.WakeID)
	s.TaskRunID = strings.TrimSpace(s.TaskRunID)
	s.ClaimToken = strings.TrimSpace(s.ClaimToken)
	s.TargetSessionID = strings.TrimSpace(s.TargetSessionID)
	s.OwnerKey = strings.TrimSpace(s.OwnerKey)
	s.Outcome.State = strings.TrimSpace(s.Outcome.State)
	s.Outcome.UsageState = strings.TrimSpace(s.Outcome.UsageState)
	s.Outcome.Reason = strings.TrimSpace(s.Outcome.Reason)
	if s.Outcome.Reason == "" {
		s.Outcome.Reason = defaultNetworkWakeTerminalReason(s.Outcome.State)
	}
	if s.Now.IsZero() {
		s.Now = defaultNow.UTC()
	} else {
		s.Now = s.Now.UTC()
	}
	if err := s.Validate(); err != nil {
		return NetworkWakeSettlement{}, err
	}
	return s, nil
}

func defaultNetworkWakeTerminalReason(state string) string {
	switch strings.TrimSpace(state) {
	case store.NetworkWakeStateFailed:
		return "network wake failed"
	case store.NetworkWakeStateCanceled:
		return "network wake canceled"
	case store.NetworkWakeStateDeadlineExceeded:
		return "network wake deadline exceeded"
	default:
		return ""
	}
}

// Validate rejects incomplete identity, authority, and outcome combinations.
func (s NetworkWakeSettlement) Validate() error {
	for field, value := range map[string]string{
		"workspace_id":      s.WorkspaceID,
		"wake_id":           s.WakeID,
		"task_run_id":       s.TaskRunID,
		"claim_token":       s.ClaimToken,
		"target_session_id": s.TargetSessionID,
		"owner_key":         s.OwnerKey,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: network wake settlement %s is required", ErrValidation, field)
		}
	}
	if s.Now.IsZero() {
		return fmt.Errorf("%w: network wake settlement now is required", ErrValidation)
	}
	if err := s.Actor.Validate(); err != nil {
		return err
	}
	if err := s.Outcome.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return nil
}
