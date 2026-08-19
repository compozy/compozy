package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrApprovalNotFound        = errors.New("tool approval not found")
	ErrApprovalTerminal        = errors.New("tool approval is already terminal")
	ErrApprovalDispatchFenced  = errors.New("tool approval dispatch is already fenced")
	ErrApprovalInvalid         = errors.New("tool approval is invalid")
	ErrCannotDeferSecrets      = errors.New("cannot_defer_secrets")
	ErrApprovalExecutionFailed = errors.New("tool approval execution failed")
)

type ApprovalTargetKind string

const (
	ApprovalTargetTool     ApprovalTargetKind = "tool"
	ApprovalTargetClientOp ApprovalTargetKind = "client_op"
	ApprovalTargetNavigate ApprovalTargetKind = "navigate"
	ApprovalTargetView     ApprovalTargetKind = "view"
)

type ApprovalOutcome string

const (
	ApprovalPending  ApprovalOutcome = "pending"
	ApprovalApproved ApprovalOutcome = "approved"
	ApprovalDenied   ApprovalOutcome = "denied"
	ApprovalTimedOut ApprovalOutcome = "timeout"
	ApprovalCanceled ApprovalOutcome = "canceled"
)

type ApprovalExecutionStatus string

const (
	ApprovalDispatching ApprovalExecutionStatus = "dispatching"
	ApprovalCompleted   ApprovalExecutionStatus = "completed"
	ApprovalFailed      ApprovalExecutionStatus = "failed"
	ApprovalUncertain   ApprovalExecutionStatus = "uncertain"
)

type ApprovalTarget struct {
	Kind    ApprovalTargetKind `json:"kind"`
	ToolID  ToolID             `json:"tool_id,omitempty"`
	Payload json.RawMessage    `json:"payload,omitempty"`
}

type ApprovalRequest struct {
	WorkspaceID             string          `json:"workspace_id"`
	InvocationID            string          `json:"invocation_id"`
	CommandID               string          `json:"command_id,omitempty"`
	Target                  ApprovalTarget  `json:"target"`
	Args                    json.RawMessage `json:"args"`
	ExpiresAt               time.Time       `json:"expires_at"`
	ContainsSecretArguments bool            `json:"-"`
}

type ApprovalTicket struct {
	ApprovalID   string          `json:"approval_id"`
	InvocationID string          `json:"invocation_id"`
	ExpiresAt    time.Time       `json:"expires_at"`
	Completion   <-chan struct{} `json:"-"`
}

type ApprovalStatus struct {
	ApprovalID      string                  `json:"approval_id"`
	WorkspaceID     string                  `json:"workspace_id"`
	InvocationID    string                  `json:"invocation_id"`
	CommandID       string                  `json:"command_id,omitempty"`
	Target          ApprovalTarget          `json:"target"`
	Args            json.RawMessage         `json:"-"`
	ApprovalStatus  ApprovalOutcome         `json:"approval_status"`
	ExecutionStatus ApprovalExecutionStatus `json:"execution_status,omitempty"`
	Result          json.RawMessage         `json:"result,omitempty"`
	Error           json.RawMessage         `json:"error,omitempty"`
	RequestedAt     time.Time               `json:"requested_at"`
	ExpiresAt       time.Time               `json:"expires_at"`
	ResolvedAt      *time.Time              `json:"resolved_at,omitempty"`
	ExecutedAt      *time.Time              `json:"executed_at,omitempty"`
	ResumeFence     bool                    `json:"-"`
}

type ApprovalCoordinator interface {
	Begin(context.Context, ApprovalRequest) (ApprovalTicket, error)
	Resolve(context.Context, string, ApprovalOutcome) error
	Status(context.Context, string) (ApprovalStatus, error)
	Cancel(context.Context, string) error
	Recover(context.Context) error
	Close() error
}

type ApprovalPendingStore interface {
	CreateApproval(context.Context, string, ApprovalRequest, time.Time) (ApprovalStatus, error)
	GetApproval(context.Context, string) (ApprovalStatus, error)
	ResolveApproval(context.Context, string, ApprovalOutcome, time.Time) (ApprovalStatus, error)
	CompleteApprovalExecution(
		context.Context,
		string,
		ApprovalExecutionStatus,
		json.RawMessage,
		json.RawMessage,
		time.Time,
	) (ApprovalStatus, error)
	ExpireApprovals(context.Context, time.Time) ([]ApprovalStatus, error)
	RecoverDispatchingApprovals(context.Context, time.Time) ([]ApprovalStatus, error)
	ListPendingApprovals(context.Context) ([]ApprovalStatus, error)
}

type ApprovalDispatcher interface {
	DispatchApproval(context.Context, ApprovalStatus) (json.RawMessage, error)
}

func (request ApprovalRequest) validate(now time.Time) error {
	if strings.TrimSpace(request.WorkspaceID) == "" || strings.TrimSpace(request.InvocationID) == "" {
		return fmt.Errorf("%w: workspace_id and invocation_id are required", ErrApprovalInvalid)
	}
	if request.ContainsSecretArguments {
		return ErrCannotDeferSecrets
	}
	if !request.ExpiresAt.After(now) {
		return fmt.Errorf("%w: expires_at must be in the future", ErrApprovalInvalid)
	}
	if !json.Valid(request.Args) {
		return fmt.Errorf("%w: args must be valid JSON", ErrApprovalInvalid)
	}
	if err := request.Target.validate(); err != nil {
		return err
	}
	return nil
}

func (target ApprovalTarget) validate() error {
	switch target.Kind {
	case ApprovalTargetTool:
		if target.ToolID == "" {
			return fmt.Errorf("%w: tool target requires tool_id", ErrApprovalInvalid)
		}
	case ApprovalTargetClientOp, ApprovalTargetNavigate, ApprovalTargetView:
	default:
		return fmt.Errorf("%w: unknown target kind %q", ErrApprovalInvalid, target.Kind)
	}
	if len(target.Payload) > 0 && !json.Valid(target.Payload) {
		return fmt.Errorf("%w: target payload must be valid JSON", ErrApprovalInvalid)
	}
	return nil
}

func validateApprovalOutcome(outcome ApprovalOutcome) error {
	switch outcome {
	case ApprovalApproved, ApprovalDenied, ApprovalTimedOut, ApprovalCanceled:
		return nil
	default:
		return fmt.Errorf("%w: unknown outcome %q", ErrApprovalInvalid, outcome)
	}
}
