package network

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrWorkNotFound reports that no current work matched the
	// lifecycle message being applied.
	ErrWorkNotFound = errors.New("network: work not found")
	// ErrWorkActorNotAllowed reports a lifecycle actor outside the
	// initiator/target pair.
	ErrWorkActorNotAllowed = errors.New("network: work actor not allowed")
	// ErrInvalidStateTransition reports an impossible lifecycle transition.
	ErrInvalidStateTransition = errors.New("network: invalid work state transition")
	// ErrWorkContainerMismatch reports that a work_id was continued from a
	// different conversation container than the one that opened it.
	ErrWorkContainerMismatch = errors.New("network: work container mismatch")
	// ErrWorkClosed reports a message for a terminal work that must
	// be rejected instead of reopening the work.
	ErrWorkClosed = errors.New("network: work closed")
)

// Work tracks one directed work inside one channel.
type Work struct {
	ID         string
	Ref        ConversationRef
	Initiator  string
	Target     string
	State      WorkState
	CreatedAt  time.Time
	UpdatedAt  time.Time
	TerminalAt *time.Time
}

// Validate reports whether the work carries a usable identity and state.
func (i Work) Validate() error {
	if i.ID == "" {
		return fmt.Errorf("%w: work id is required", ErrMissingField)
	}
	if err := ValidateWorkID(i.ID); err != nil {
		return fmt.Errorf("validate work id: %w", err)
	}
	if err := i.Ref.Validate(); err != nil {
		return fmt.Errorf("validate work conversation: %w", err)
	}
	if i.Initiator == "" {
		return fmt.Errorf("%w: work initiator is required", ErrMissingField)
	}
	if err := ValidatePeerID(i.Initiator); err != nil {
		return fmt.Errorf("%w: work initiator", err)
	}
	if i.Target == "" {
		return fmt.Errorf("%w: work target is required", ErrMissingField)
	}
	if err := ValidatePeerID(i.Target); err != nil {
		return fmt.Errorf("%w: work target", err)
	}
	if err := i.State.Validate(); err != nil {
		return fmt.Errorf("validate work state: %w", err)
	}
	if i.TerminalAt != nil && !IsTerminalState(i.State) {
		return fmt.Errorf("%w: terminal_at requires terminal work state", ErrInvalidField)
	}
	return nil
}

// IsTerminalState reports whether the state is terminal under the RFC.
func IsTerminalState(state WorkState) bool {
	switch state {
	case WorkStateCompleted, WorkStateFailed, WorkStateCanceled:
		return true
	default:
		return false
	}
}

// IsParticipant reports whether the peer owns the work lifecycle.
func (i Work) IsParticipant(peerID string) bool {
	return peerID == i.Initiator || peerID == i.Target
}

func (i Work) counterparty(peerID string) (string, bool) {
	switch peerID {
	case i.Initiator:
		return i.Target, true
	case i.Target:
		return i.Initiator, true
	default:
		return "", false
	}
}

// LifecycleAction explains how a lifecycle helper handled one message.
type LifecycleAction string

const (
	LifecycleActionOpened     LifecycleAction = "opened"
	LifecycleActionAdvanced   LifecycleAction = "advanced"
	LifecycleActionUnchanged  LifecycleAction = "unchanged"
	LifecycleActionIgnored    LifecycleAction = "ignored"
	LifecycleActionRejectWork LifecycleAction = "reject_work"
)

// LifecycleResult is the reusable lifecycle decision surface for router code.
type LifecycleResult struct {
	Work       Work
	Action     LifecycleAction
	ReasonCode *ReasonCode
}

// OpenWork opens a new work from the first directed message.
func OpenWork(env Envelope, at time.Time) (Work, error) {
	if env.Kind != KindSay && env.Kind != KindCapability {
		return Work{}, fmt.Errorf("%w: opening message kind=%q", ErrInvalidField, env.Kind)
	}
	if env.To == nil {
		return Work{}, fmt.Errorf("%w: opening message to is required", ErrMissingField)
	}
	if env.WorkID == nil {
		return Work{}, fmt.Errorf("%w: opening message work_id is required", ErrMissingField)
	}
	ref, err := ConversationRefFromEnvelope(env)
	if err != nil {
		return Work{}, err
	}

	if at.IsZero() {
		at = time.Now().UTC()
	}

	work := Work{
		ID:        *env.WorkID,
		Ref:       ref,
		Initiator: env.From,
		Target:    *env.To,
		State:     WorkStateSubmitted,
		CreatedAt: at,
		UpdatedAt: at,
	}

	if err := work.Validate(); err != nil {
		return Work{}, fmt.Errorf("validate opened work: %w", err)
	}

	return work, nil
}

// ApplyWorkEnvelope applies one validated lifecycle envelope to the
// current work state and returns the router-facing decision.
func ApplyWorkEnvelope(current *Work, env Envelope, at time.Time) (LifecycleResult, error) {
	at = normalizeWorkTime(at)

	if current == nil {
		return openWorkResult(env, at)
	}

	work, err := validateWorkEnvelope(*current, env)
	if err != nil {
		return LifecycleResult{}, err
	}
	if result, terminal := terminalWorkResult(work); terminal {
		return result, nil
	}

	return applyActiveWorkEnvelope(work, env, at)
}

func normalizeWorkTime(at time.Time) time.Time {
	if at.IsZero() {
		return time.Now().UTC()
	}
	return at
}

func openWorkResult(env Envelope, at time.Time) (LifecycleResult, error) {
	opened, err := OpenWork(env, at)
	if err != nil {
		if env.Kind != KindSay && env.Kind != KindCapability {
			return LifecycleResult{}, fmt.Errorf("%w: kind=%q", ErrWorkNotFound, env.Kind)
		}
		return LifecycleResult{}, err
	}
	return LifecycleResult{
		Work:   opened,
		Action: LifecycleActionOpened,
	}, nil
}

func validateWorkEnvelope(current Work, env Envelope) (Work, error) {
	if err := current.Validate(); err != nil {
		return Work{}, fmt.Errorf("validate current work: %w", err)
	}
	if err := validateWorkIdentity(current, env); err != nil {
		return Work{}, err
	}
	if !current.IsParticipant(env.From) {
		return Work{}, fmt.Errorf("%w: from=%q", ErrWorkActorNotAllowed, env.From)
	}
	if err := validateWorkDirection(current, env); err != nil {
		return Work{}, err
	}
	return current, nil
}

func validateWorkIdentity(current Work, env Envelope) error {
	if env.WorkID == nil || *env.WorkID != current.ID {
		return fmt.Errorf(
			"%w: work_id=%v current=%q",
			ErrInvalidField,
			env.WorkID,
			current.ID,
		)
	}
	ref, err := ConversationRefFromEnvelope(env)
	if err != nil {
		return err
	}
	if ref.ContainerKey() != current.Ref.ContainerKey() {
		return fmt.Errorf(
			"%w: work_id=%q",
			ErrWorkContainerMismatch,
			current.ID,
		)
	}
	return nil
}

func validateWorkDirection(current Work, env Envelope) error {
	if env.Kind != KindSay && env.Kind != KindCapability {
		return nil
	}
	if env.To == nil {
		return fmt.Errorf("%w: %s to is required", ErrMissingField, env.Kind)
	}

	expectedTarget, ok := current.counterparty(env.From)
	if !ok || *env.To != expectedTarget {
		return fmt.Errorf(
			"%w: from=%q to=%q expected_to=%q",
			ErrWorkActorNotAllowed,
			env.From,
			*env.To,
			expectedTarget,
		)
	}
	return nil
}

func terminalWorkResult(work Work) (LifecycleResult, bool) {
	if !IsTerminalState(work.State) {
		return LifecycleResult{}, false
	}

	reason := ReasonCodeWorkClosed
	return LifecycleResult{
		Work:       work,
		Action:     LifecycleActionRejectWork,
		ReasonCode: &reason,
	}, true
}

func applyActiveWorkEnvelope(work Work, env Envelope, at time.Time) (LifecycleResult, error) {
	switch env.Kind {
	case KindSay, KindCapability:
		return applySayOrCapability(work, at), nil
	case KindReceipt:
		receipt, err := decodeLifecycleBody[ReceiptBody](env, "receipt")
		if err != nil {
			return LifecycleResult{}, err
		}
		return applyReceipt(work, receipt, at)
	case KindTrace:
		trace, err := decodeLifecycleBody[TraceBody](env, "trace")
		if err != nil {
			return LifecycleResult{}, err
		}
		return applyTrace(work, trace, at)
	default:
		return LifecycleResult{}, fmt.Errorf("%w: lifecycle kind=%q", ErrInvalidField, env.Kind)
	}
}

func applySayOrCapability(work Work, at time.Time) LifecycleResult {
	if work.State == WorkStateNeedsInput {
		work.State = WorkStateWorking
		work.UpdatedAt = at
		return LifecycleResult{Work: work, Action: LifecycleActionAdvanced}
	}
	return LifecycleResult{Work: work, Action: LifecycleActionUnchanged}
}

func decodeLifecycleBody[T any](env Envelope, label string) (T, error) {
	var zero T

	body, err := env.DecodeBody()
	if err != nil {
		return zero, fmt.Errorf("decode %s body: %w", label, err)
	}
	typed, ok := body.(T)
	if !ok {
		return zero, fmt.Errorf("%w: expected %s body", ErrInvalidBody, label)
	}
	return typed, nil
}

func applyReceipt(work Work, receipt ReceiptBody, at time.Time) (LifecycleResult, error) {
	switch receipt.Status {
	case ReceiptStatusAccepted, ReceiptStatusDuplicate, ReceiptStatusExpired, ReceiptStatusUnsupported:
		return LifecycleResult{
			Work:   work,
			Action: LifecycleActionUnchanged,
		}, nil
	case ReceiptStatusRejected:
		work = transitionWorkState(work, WorkStateFailed, at)
	case ReceiptStatusCanceled:
		work = transitionWorkState(work, WorkStateCanceled, at)
	default:
		return LifecycleResult{}, fmt.Errorf("%w: receipt status=%q", ErrInvalidStateTransition, receipt.Status)
	}

	return LifecycleResult{
		Work:   work,
		Action: LifecycleActionAdvanced,
	}, nil
}

func applyTrace(work Work, trace TraceBody, at time.Time) (LifecycleResult, error) {
	if !canApplyTrace(work.State, trace.State) {
		return LifecycleResult{}, fmt.Errorf("%w: %s -> %s", ErrInvalidStateTransition, work.State, trace.State)
	}

	updated := work
	updated = transitionWorkState(updated, trace.State, at)

	return LifecycleResult{
		Work:   updated,
		Action: LifecycleActionAdvanced,
	}, nil
}

func transitionWorkState(work Work, next WorkState, at time.Time) Work {
	work.State = next
	work.UpdatedAt = at
	if IsTerminalState(next) {
		terminalAt := at
		work.TerminalAt = &terminalAt
	}
	return work
}

func canApplyTrace(current WorkState, next WorkState) bool {
	switch current {
	case WorkStateSubmitted, WorkStateWorking, WorkStateNeedsInput:
		return canAdvanceOpenWork(next)
	default:
		return false
	}
}

func canAdvanceOpenWork(next WorkState) bool {
	switch next {
	case WorkStateWorking, WorkStateNeedsInput, WorkStateCompleted, WorkStateFailed, WorkStateCanceled:
		return true
	default:
		return false
	}
}
