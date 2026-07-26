package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AutonomyReasonCode is the stable machine-readable reason for session-bound
// autonomy lease lookup failures.
type AutonomyReasonCode string

const (
	AutonomySessionRequired  AutonomyReasonCode = "AUTONOMY_SESSION_REQUIRED"
	AutonomyNoActiveLease    AutonomyReasonCode = "AUTONOMY_NO_ACTIVE_LEASE"
	AutonomyForeignRun       AutonomyReasonCode = "AUTONOMY_FOREIGN_RUN"
	AutonomyLeaseExpired     AutonomyReasonCode = "AUTONOMY_LEASE_EXPIRED"
	AutonomyLeaseAlreadyHeld AutonomyReasonCode = "AUTONOMY_LEASE_ALREADY_HELD"
)

// AutonomyError carries the deterministic reason for a session-bound autonomy rejection.
type AutonomyError struct {
	Reason AutonomyReasonCode
	Err    error
}

// Error returns the redacted public error string.
func (e *AutonomyError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		return fmt.Sprintf("task: autonomy lease rejected: %s", e.Reason)
	}
	return fmt.Sprintf("task: autonomy lease rejected: %s: %s", e.Reason, e.Err.Error())
}

// Unwrap returns the wrapped task-domain sentinel.
func (e *AutonomyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// AutonomyReasonOf extracts a deterministic autonomy reason code from an error.
func AutonomyReasonOf(err error) (AutonomyReasonCode, bool) {
	var autonomyErr *AutonomyError
	if errors.As(err, &autonomyErr) && autonomyErr.Reason != "" {
		return autonomyErr.Reason, true
	}
	return "", false
}

// AutonomyLeaseAuthority resolves the internal lease credential for the calling
// session without exposing the raw claim token at public boundaries.
type AutonomyLeaseAuthority interface {
	LookupActiveRunForSession(
		ctx context.Context,
		sessionID string,
		runID string,
	) (AutonomyLeaseHandle, error)
}

// AutonomyLeaseStore is the narrowed internal store surface required by the
// session-bound autonomy lookup.
type AutonomyLeaseStore interface {
	ListAutonomyLeaseHandles(ctx context.Context, sessionID string) ([]AutonomyLeaseHandle, error)
}

// AutonomyLeaseHandle is the internal-only active lease handle used to call the
// existing token-fenced lease writers.
type AutonomyLeaseHandle struct {
	RunID           string
	TaskID          string
	RunKind         RunKind
	WorkspaceID     string
	TargetSessionID string
	OwnerKey        string
	SessionID       string
	Status          RunStatus
	ClaimedBy       *ActorIdentity
	ClaimToken      string
	ClaimTokenHash  string
	LeaseUntil      time.Time
	HeartbeatAt     time.Time
}

func autonomyError(reason AutonomyReasonCode, cause error, format string, args ...any) error {
	detail := strings.TrimSpace(fmt.Sprintf(format, args...))
	if detail == "" {
		return &AutonomyError{Reason: reason, Err: cause}
	}
	return &AutonomyError{Reason: reason, Err: fmt.Errorf("%w: %s", cause, detail)}
}

func isAutonomyLeaseStatusActive(status RunStatus) bool {
	switch status.Normalize() {
	case TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
		return true
	default:
		return false
	}
}
