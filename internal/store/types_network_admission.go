package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

const (
	NetworkDispositionDeliver = "deliver"
	NetworkDispositionIgnore  = "ignore"
)

const (
	NetworkWakeTriggerNone    = ""
	NetworkWakeTriggerDirect  = "direct"
	NetworkWakeTriggerMention = "mention"
)

const (
	NetworkWakeSkipNotLive         = "not_live"
	NetworkWakeSkipNotEligible     = "not_eligible"
	NetworkWakeSkipNotAddressed    = "not_addressed"
	NetworkWakeSkipDuplicate       = "duplicate"
	NetworkWakeSkipDepthExceeded   = "depth_exceeded"
	NetworkWakeSkipCoalesced       = "coalesced"
	NetworkWakeSkipInFlight        = "in_flight"
	NetworkWakeSkipBudgetExhausted = "budget_exhausted"
	NetworkWakeSkipNetworkDisabled = "network_disabled"
)

const (
	NetworkWakeStateSucceeded        = "succeeded"
	NetworkWakeStateFailed           = "failed"
	NetworkWakeStateCanceled         = "canceled"
	NetworkWakeStateDeadlineExceeded = "deadline_exceeded"
)

const (
	NetworkWakeUsageActual      = "actual"
	NetworkWakeUsageUnavailable = "usage_unavailable"
)

// NetworkMessageDisposition is one immutable pre-message recipient decision.
type NetworkMessageDisposition struct {
	RecipientSessionID string
	Decision           string
}

// Validate ensures the decision is bound to one durable session identity.
func (d NetworkMessageDisposition) Validate() error {
	if err := requireField(d.RecipientSessionID, "network message disposition recipient_session_id"); err != nil {
		return err
	}
	switch strings.TrimSpace(d.Decision) {
	case NetworkDispositionDeliver, NetworkDispositionIgnore:
		return nil
	default:
		return fmt.Errorf("store: unsupported network message disposition decision %q", d.Decision)
	}
}

// NetworkWakeAdmissionInput carries policy resolved before the acceptance transaction.
type NetworkWakeAdmissionInput struct {
	WorkspaceID        string
	RecipientSessionID string
	OwnerKey           string
	Spec               participation.Spec
	Trigger            string
	Eligible           bool
	Addressed          bool
	RootID             string
	Depth              int
	WakeID             string
	TaskRunID          string
}

// Validate ensures an admission candidate is immutable and fully identified.
func (i NetworkWakeAdmissionInput) Validate() error {
	for field, value := range map[string]string{
		"workspace_id":         i.WorkspaceID,
		"recipient_session_id": i.RecipientSessionID,
		"owner_key":            i.OwnerKey,
	} {
		if err := requireField(value, "network wake admission "+field); err != nil {
			return err
		}
	}
	if i.Depth < 0 {
		return fmt.Errorf("store: network wake admission depth must be zero or positive: %d", i.Depth)
	}
	if err := participation.ValidateSpec(i.Spec); err != nil {
		return fmt.Errorf("store: validate network wake admission participation: %w", err)
	}
	switch strings.TrimSpace(i.Trigger) {
	case NetworkWakeTriggerNone:
		if i.Eligible && i.Addressed {
			return fmt.Errorf("store: addressed eligible network wake admission requires direct or mention trigger")
		}
		return nil
	case NetworkWakeTriggerDirect, NetworkWakeTriggerMention:
		if !i.Eligible || !i.Addressed {
			return fmt.Errorf("store: direct or mention network wake admission must be eligible and addressed")
		}
		for field, value := range map[string]string{
			"root_id":     i.RootID,
			"wake_id":     i.WakeID,
			"task_run_id": i.TaskRunID,
		} {
			if err := requireField(value, "network wake admission "+field); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("store: unsupported network wake trigger %q", i.Trigger)
	}
}

// AcceptNetworkMessageRequest is the complete input to the atomic acceptance boundary.
type AcceptNetworkMessageRequest struct {
	Message      NetworkConversationMessage
	Dispositions []NetworkMessageDisposition
	Admissions   []NetworkWakeAdmissionInput
}

// WakeReservation is durable admission evidence paired one-to-one with a task run.
type WakeReservation struct {
	WakeID           string
	TaskRunID        string
	WorkspaceID      string
	OwnerKey         string
	EnvelopeIDs      []string
	ReservedWallTime string
	CoalesceUntil    time.Time
}

// WakeSkip records why an admission candidate accumulated without a new wake.
type WakeSkip struct {
	RecipientSessionID string
	OwnerKey           string
	Reason             string
	WakeID             string
}

// CommittedNetworkNotification is safe to dispatch only after acceptance commits.
type CommittedNetworkNotification struct {
	WorkspaceID        string
	RecipientSessionID string
	TaskRunID          string
	AcceptanceSeq      int64
	ReadyAt            time.Time
}

// AcceptNetworkMessageResult reports the exact durable acceptance outcome.
type AcceptNetworkMessageResult struct {
	AcceptanceSeq     int64
	AvailabilityEpoch int64
	Duplicate         bool
	Conversation      NetworkConversationWriteResult
	Dispositions      []NetworkMessageDisposition
	Admitted          []WakeReservation
	Skipped           []WakeSkip
	Notify            []CommittedNetworkNotification
}

// NetworkWakeOutcome is the terminal truth applied by the wake runner.
type NetworkWakeOutcome struct {
	State              string
	ActualWallTime     time.Duration
	ActualInputTokens  int64
	ActualOutputTokens int64
	UsageState         string
	Reason             string
}

// Validate rejects non-terminal or internally contradictory settlement facts.
func (o NetworkWakeOutcome) Validate() error {
	switch strings.TrimSpace(o.State) {
	case NetworkWakeStateSucceeded,
		NetworkWakeStateFailed,
		NetworkWakeStateCanceled,
		NetworkWakeStateDeadlineExceeded:
	default:
		return fmt.Errorf("store: unsupported network wake terminal state %q", o.State)
	}
	if o.ActualWallTime < 0 || o.ActualInputTokens < 0 || o.ActualOutputTokens < 0 {
		return fmt.Errorf("store: network wake usage values must be zero or positive")
	}
	switch strings.TrimSpace(o.UsageState) {
	case NetworkWakeUsageActual:
		return nil
	case NetworkWakeUsageUnavailable:
		if o.ActualInputTokens != 0 || o.ActualOutputTokens != 0 {
			return fmt.Errorf("store: unavailable network wake usage cannot carry token values")
		}
		return nil
	default:
		return fmt.Errorf("store: unsupported network wake usage state %q", o.UsageState)
	}
}
