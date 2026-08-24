package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const terminalRunCommandIntentVersion = 2

// TerminalRunCommandKind identifies one durable non-lease terminal intent.
type TerminalRunCommandKind string

const (
	TerminalRunCommandCompleted      TerminalRunCommandKind = "completed"
	TerminalRunCommandFailed         TerminalRunCommandKind = "failed"
	TerminalRunCommandCanceled       TerminalRunCommandKind = "canceled"
	TerminalRunCommandNeedsAttention TerminalRunCommandKind = "needs_attention"
)

// TerminalRunCommandPhase records how far the external stop effect progressed.
type TerminalRunCommandPhase string

const (
	TerminalRunCommandPhaseAdmitted      TerminalRunCommandPhase = "admitted"
	TerminalRunCommandPhaseStopRequested TerminalRunCommandPhase = "stop_requested"
	TerminalRunCommandPhaseStopConfirmed TerminalRunCommandPhase = "stop_confirmed"
)

type terminalRunCancellationIntent struct {
	Request              CancelRun `json:"request"`
	PropagatedFromTaskID string    `json:"propagated_from_task_id,omitempty"`
	ReconcileTask        bool      `json:"reconcile_task"`
}

type terminalRunRecoveryIntent struct {
	Recovery          RunBootRecovery `json:"recovery"`
	PreviousStatus    RunStatus       `json:"previous_status"`
	PreviousSessionID string          `json:"previous_session_id,omitempty"`
}

type terminalRunCommandIntent struct {
	Version       int                            `json:"version"`
	EventIDs      []string                       `json:"event_ids"`
	Result        json.RawMessage                `json:"result,omitempty"`
	Failure       *RunFailure                    `json:"failure,omitempty"`
	Cancellation  *terminalRunCancellationIntent `json:"cancellation,omitempty"`
	Diagnostic    string                         `json:"diagnostic,omitempty"`
	StopRequired  bool                           `json:"stop_required"`
	StopReason    StopReason                     `json:"stop_reason,omitempty"`
	RecoveryAudit *terminalRunRecoveryIntent     `json:"recovery_audit,omitempty"`
}

// TerminalRunCommand is an opaque durable receipt plus immutable terminal intent.
type TerminalRunCommand struct {
	commandID   string
	taskID      string
	workspaceID string
	kind        TerminalRunCommandKind
	phase       TerminalRunCommandPhase
	fence       RunMutationFence
	intent      terminalRunCommandIntent
	actor       ActorContext
	*terminalRunCommandTimestamps
}

type terminalRunCommandTimestamps struct {
	commandAt  time.Time
	admittedAt time.Time
	updatedAt  time.Time
}

// TerminalRunCommandRecord is the store-facing serialized representation.
type TerminalRunCommandRecord struct {
	CommandID        string
	RunID            string
	TaskID           string
	WorkspaceID      string
	Kind             TerminalRunCommandKind
	Phase            TerminalRunCommandPhase
	SourceStatus     RunStatus
	SourceSession    string
	SourceClaimHash  string
	SourceLeaseUntil time.Time
	IntentJSON       json.RawMessage
	ActorJSON        json.RawMessage
	CommandAt        time.Time
	AdmittedAt       time.Time
	UpdatedAt        time.Time
}

func newTerminalRunCommand(
	commandID string,
	previous Run,
	kind TerminalRunCommandKind,
	intent terminalRunCommandIntent,
	actor ActorContext,
	commandAt time.Time,
) (TerminalRunCommand, error) {
	command := TerminalRunCommand{
		commandID:   strings.TrimSpace(commandID),
		taskID:      strings.TrimSpace(previous.TaskID),
		workspaceID: strings.TrimSpace(previous.WorkspaceID),
		kind:        kind,
		phase:       TerminalRunCommandPhaseAdmitted,
		fence:       NewRunMutationFence(previous),
		intent:      cloneTerminalRunCommandIntent(intent),
		actor:       actor,
		terminalRunCommandTimestamps: &terminalRunCommandTimestamps{
			commandAt: commandAt.UTC(), admittedAt: commandAt.UTC(), updatedAt: commandAt.UTC(),
		},
	}
	if err := command.validate(); err != nil {
		return TerminalRunCommand{}, err
	}
	return command, nil
}

func newCompletionTerminalRunCommand(
	commandID string,
	eventIDs []string,
	previous Run,
	result json.RawMessage,
	actor ActorContext,
	commandAt time.Time,
) (TerminalRunCommand, error) {
	return newTerminalRunCommand(commandID, previous, TerminalRunCommandCompleted, terminalRunCommandIntent{
		Version:      terminalRunCommandIntentVersion,
		EventIDs:     append([]string(nil), eventIDs...),
		Result:       cloneRawJSON(result),
		StopRequired: strings.TrimSpace(previous.SessionID) != "",
		StopReason:   StopReasonCompleted,
	}, actor, commandAt)
}

func newFailureTerminalRunCommand(
	commandID string,
	eventIDs []string,
	previous Run,
	failure RunFailure,
	actor ActorContext,
	commandAt time.Time,
	stopSession bool,
	recoveryAudit *bootRecoveryAudit,
) (TerminalRunCommand, error) {
	intent := terminalRunCommandIntent{
		Version:      terminalRunCommandIntentVersion,
		EventIDs:     append([]string(nil), eventIDs...),
		Failure:      &RunFailure{Error: failure.Error, Metadata: cloneRawJSON(failure.Metadata)},
		StopRequired: stopSession && strings.TrimSpace(previous.SessionID) != "",
		StopReason:   StopReasonFailed,
	}
	if recoveryAudit != nil {
		intent.RecoveryAudit = &terminalRunRecoveryIntent{
			Recovery:          recoveryAudit.recovery,
			PreviousStatus:    recoveryAudit.previousStatus,
			PreviousSessionID: recoveryAudit.previousSessionID,
		}
	}
	return newTerminalRunCommand(
		commandID,
		previous,
		TerminalRunCommandFailed,
		intent,
		actor,
		commandAt,
	)
}

func newCancellationTerminalRunCommand(
	commandID string,
	eventIDs []string,
	previous Run,
	req CancelRun,
	actor ActorContext,
	commandAt time.Time,
	opts cancelRunOptions,
) (TerminalRunCommand, error) {
	status := previous.Status.Normalize()
	activeSession := (status == TaskRunStatusStarting || status == TaskRunStatusRunning) &&
		strings.TrimSpace(previous.SessionID) != ""
	return newTerminalRunCommand(commandID, previous, TerminalRunCommandCanceled, terminalRunCommandIntent{
		Version:  terminalRunCommandIntentVersion,
		EventIDs: append([]string(nil), eventIDs...),
		Cancellation: &terminalRunCancellationIntent{
			Request: CancelRun{
				Reason:   req.Reason,
				Metadata: cloneRawJSON(req.Metadata),
			},
			PropagatedFromTaskID: opts.propagatedFromTaskID,
			ReconcileTask:        opts.reconcileTask,
		},
		StopRequired: activeSession,
		StopReason:   StopReasonCancellation,
	}, actor, commandAt)
}

func newNeedsAttentionTerminalRunCommand(
	commandID string,
	eventIDs []string,
	previous Run,
	diagnostic string,
	actor ActorContext,
	commandAt time.Time,
) (TerminalRunCommand, error) {
	return newTerminalRunCommand(commandID, previous, TerminalRunCommandNeedsAttention, terminalRunCommandIntent{
		Version:    terminalRunCommandIntentVersion,
		EventIDs:   append([]string(nil), eventIDs...),
		Diagnostic: strings.TrimSpace(diagnostic),
	}, actor, commandAt)
}

// RestoreTerminalRunCommand validates and restores one durable store record.
func RestoreTerminalRunCommand(record TerminalRunCommandRecord) (TerminalRunCommand, error) {
	var intent terminalRunCommandIntent
	if err := decodeTerminalRunCommandJSON("intent", record.IntentJSON, &intent); err != nil {
		return TerminalRunCommand{}, err
	}
	var actor ActorContext
	if err := decodeTerminalRunCommandJSON("actor", record.ActorJSON, &actor); err != nil {
		return TerminalRunCommand{}, err
	}
	command := TerminalRunCommand{
		commandID:   record.CommandID,
		taskID:      record.TaskID,
		workspaceID: record.WorkspaceID,
		kind:        record.Kind,
		phase:       record.Phase,
		fence: NewRunMutationFence(Run{
			ID:             record.RunID,
			TaskID:         record.TaskID,
			WorkspaceID:    record.WorkspaceID,
			Status:         record.SourceStatus,
			SessionID:      record.SourceSession,
			ClaimTokenHash: record.SourceClaimHash,
			LeaseUntil:     record.SourceLeaseUntil,
		}),
		intent: cloneTerminalRunCommandIntent(intent),
		actor:  actor,
		terminalRunCommandTimestamps: &terminalRunCommandTimestamps{
			commandAt: record.CommandAt.UTC(), admittedAt: record.AdmittedAt.UTC(), updatedAt: record.UpdatedAt.UTC(),
		},
	}
	if err := command.validate(); err != nil {
		return TerminalRunCommand{}, err
	}
	return command, nil
}

func decodeTerminalRunCommandJSON(path string, raw json.RawMessage, target any) error {
	if !json.Valid(raw) {
		return fmt.Errorf("%w: terminal run command %s must contain valid JSON", ErrValidation, path)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf(
			"task: decode terminal run command %s: %w",
			path,
			errors.Join(ErrValidation, err),
		)
	}
	return nil
}

// Record returns an isolated persistence representation of the command.
func (c TerminalRunCommand) Record() (TerminalRunCommandRecord, error) {
	if err := c.validate(); err != nil {
		return TerminalRunCommandRecord{}, err
	}
	intentJSON, err := json.Marshal(c.intent)
	if err != nil {
		return TerminalRunCommandRecord{}, fmt.Errorf("task: encode terminal run command intent: %w", err)
	}
	actorJSON, err := json.Marshal(c.actor)
	if err != nil {
		return TerminalRunCommandRecord{}, fmt.Errorf("task: encode terminal run command actor: %w", err)
	}
	return TerminalRunCommandRecord{
		CommandID:        c.commandID,
		RunID:            c.fence.RunID(),
		TaskID:           c.taskID,
		WorkspaceID:      c.workspaceID,
		Kind:             c.kind,
		Phase:            c.phase,
		SourceStatus:     c.fence.Status(),
		SourceSession:    c.fence.SessionID(),
		SourceClaimHash:  c.fence.ClaimTokenHash(),
		SourceLeaseUntil: c.fence.LeaseUntil(),
		IntentJSON:       intentJSON,
		ActorJSON:        actorJSON,
		CommandAt:        c.commandAt,
		AdmittedAt:       c.admittedAt,
		UpdatedAt:        c.updatedAt,
	}, nil
}

func (c TerminalRunCommand) validate() error {
	if c.commandID == "" || c.fence.RunID() == "" || c.taskID == "" {
		return fmt.Errorf("%w: terminal run command identity is required", ErrValidation)
	}
	for _, field := range []struct {
		path  string
		value string
	}{
		{path: "terminal_run_command.command_id", value: c.commandID},
		{path: "terminal_run_command.run_id", value: c.fence.RunID()},
		{path: "terminal_run_command.task_id", value: c.taskID},
		{path: "terminal_run_command.workspace_id", value: c.workspaceID},
		{path: "terminal_run_command.session_id", value: c.fence.SessionID()},
		{path: "terminal_run_command.claim_hash", value: c.fence.ClaimTokenHash()},
	} {
		if err := ValidateReferenceSize(field.value, field.path); err != nil {
			return err
		}
	}
	if err := c.fence.Status().Validate("terminal_run_command.source_status"); err != nil {
		return err
	}
	if err := c.actor.Validate(); err != nil {
		return err
	}
	if err := validateTerminalRunCommandCredentialFields(c); err != nil {
		return err
	}
	if c.terminalRunCommandTimestamps == nil {
		return fmt.Errorf("%w: terminal run command timestamps are required", ErrValidation)
	}
	if c.commandAt.IsZero() || c.admittedAt.IsZero() || c.updatedAt.IsZero() {
		return fmt.Errorf("%w: terminal run command timestamps are required", ErrValidation)
	}
	if c.admittedAt.Before(c.commandAt) || c.updatedAt.Before(c.admittedAt) {
		return fmt.Errorf("%w: terminal run command timestamps are out of order", ErrValidation)
	}
	if err := validateTerminalRunCommandPhase(c.phase); err != nil {
		return err
	}
	if err := validateTerminalRunCommandIntent(c.kind, c.intent); err != nil {
		return err
	}
	if err := validateTerminalRunCommandIntentAgainstFence(c); err != nil {
		return err
	}
	if !c.StopRequired() && c.phase != TerminalRunCommandPhaseAdmitted {
		return fmt.Errorf("%w: a command without a stop must remain admitted until settlement", ErrValidation)
	}
	return nil
}

func cloneTerminalRunCommandIntent(intent terminalRunCommandIntent) terminalRunCommandIntent {
	cloned := intent
	cloned.EventIDs = append([]string(nil), intent.EventIDs...)
	cloned.Result = cloneRawJSON(intent.Result)
	if intent.Failure != nil {
		failure := *intent.Failure
		failure.Metadata = cloneRawJSON(failure.Metadata)
		cloned.Failure = &failure
	}
	if intent.Cancellation != nil {
		cancellation := *intent.Cancellation
		cancellation.Request.Metadata = cloneRawJSON(cancellation.Request.Metadata)
		cloned.Cancellation = &cancellation
	}
	if intent.RecoveryAudit != nil {
		recovery := *intent.RecoveryAudit
		cloned.RecoveryAudit = &recovery
	}
	return cloned
}

func (c TerminalRunCommand) sameIntent(other TerminalRunCommand) bool {
	if c.kind != other.kind || c.taskID != other.taskID || c.workspaceID != other.workspaceID ||
		c.fence.RunID() != other.fence.RunID() || c.actor != other.actor {
		return false
	}
	leftIntent := cloneTerminalRunCommandIntent(c.intent)
	rightIntent := cloneTerminalRunCommandIntent(other.intent)
	// A retry reserves fresh IDs before consulting durable state. Once a command
	// already exists, its IDs remain authoritative and must not turn the same
	// semantic operation into a conflicting intent.
	leftIntent.EventIDs = nil
	rightIntent.EventIDs = nil
	left, leftErr := json.Marshal(leftIntent)
	right, rightErr := json.Marshal(rightIntent)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

// CommandID reports the private durable receipt identity.
func (c TerminalRunCommand) CommandID() string { return c.commandID }

// RunID reports the command's run identity.
func (c TerminalRunCommand) RunID() string { return c.fence.RunID() }

// TaskID reports the command's task identity.
func (c TerminalRunCommand) TaskID() string { return c.taskID }

// WorkspaceID reports the command's workspace boundary; empty means global scope.
func (c TerminalRunCommand) WorkspaceID() string { return c.workspaceID }

// Kind reports the immutable terminal intent kind.
func (c TerminalRunCommand) Kind() TerminalRunCommandKind { return c.kind }

// Phase reports the durable external-effect phase.
func (c TerminalRunCommand) Phase() TerminalRunCommandPhase { return c.phase }

// Fence reports the exact source run ownership snapshot.
func (c TerminalRunCommand) Fence() RunMutationFence { return c.fence }

// Actor reports the actor authorized when the command was admitted.
func (c TerminalRunCommand) Actor() ActorContext { return c.actor }

// CommandAt reports the immutable command timestamp.
func (c TerminalRunCommand) CommandAt() time.Time {
	if c.terminalRunCommandTimestamps == nil {
		return time.Time{}
	}
	return c.commandAt
}

// StopRequired reports whether settlement depends on a physical session stop.
func (c TerminalRunCommand) StopRequired() bool { return c.intent.StopRequired }

// StopReason reports the requested physical stop classification.
func (c TerminalRunCommand) StopReason() StopReason { return c.intent.StopReason }

func (c TerminalRunCommand) eventID(index int) (string, error) {
	if index < 0 || index >= len(c.intent.EventIDs) {
		return "", fmt.Errorf(
			"%w: terminal run command %q event index %d is unavailable",
			ErrValidation,
			c.commandID,
			index,
		)
	}
	return c.intent.EventIDs[index], nil
}

// Advance returns the next valid durable external-effect phase.
func (c TerminalRunCommand) Advance(
	phase TerminalRunCommandPhase,
	updatedAt time.Time,
) (TerminalRunCommand, error) {
	if c.terminalRunCommandTimestamps == nil || !c.StopRequired() ||
		(c.phase == TerminalRunCommandPhaseAdmitted && phase != TerminalRunCommandPhaseStopRequested) ||
		(c.phase == TerminalRunCommandPhaseStopRequested && phase != TerminalRunCommandPhaseStopConfirmed) ||
		c.phase == TerminalRunCommandPhaseStopConfirmed {
		return TerminalRunCommand{}, fmt.Errorf(
			"%w: terminal run command cannot advance from %q to %q",
			ErrInvalidStatusTransition,
			c.phase,
			phase,
		)
	}
	cloned := c
	if c.terminalRunCommandTimestamps != nil {
		timestamps := *c.terminalRunCommandTimestamps
		cloned.terminalRunCommandTimestamps = &timestamps
	}
	cloned.phase = phase
	cloned.updatedAt = updatedAt.UTC()
	if err := cloned.validate(); err != nil {
		return TerminalRunCommand{}, err
	}
	return cloned, nil
}
