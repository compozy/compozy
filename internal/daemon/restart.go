package daemon

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	// RestartOperationEnvKey carries the restart operation id from the helper to the replacement daemon.
	RestartOperationEnvKey = "AGH_INTERNAL_RESTART_OPERATION_ID"

	defaultRestartPollInterval  = 100 * time.Millisecond
	defaultRestartReleaseWait   = 15 * time.Second
	defaultRestartReadyWait     = 20 * time.Second
	defaultRestartExitDrainWait = 500 * time.Millisecond
)

var (
	// ErrRestartOperationNotFound reports a missing persisted restart operation.
	ErrRestartOperationNotFound           = errors.New("daemon: restart operation not found")
	errReplacementDaemonExitedBeforeReady = errors.New("daemon: replacement daemon exited before ready")
	errInvalidRestartTransition           = errors.New("daemon: invalid restart transition")
	errRestartNotRunning                  = errors.New("daemon: restart requires a running daemon")
)

// RestartStatus is the durable lifecycle state of one daemon restart operation.
type RestartStatus string

const (
	RestartStatusPending        RestartStatus = "pending"
	RestartStatusStopping       RestartStatus = "stopping"
	RestartStatusWaitingRelease RestartStatus = "waiting_release"
	RestartStatusStarting       RestartStatus = "starting"
	RestartStatusReady          RestartStatus = "ready"
	RestartStatusFailed         RestartStatus = "failed"
)

// RestartOperation is the persisted restart status record stored under ~/.agh/restarts/.
type RestartOperation struct {
	OperationID        string        `json:"operation_id"`
	Status             RestartStatus `json:"status"`
	OldPID             int           `json:"old_pid"`
	OldStartedAt       time.Time     `json:"old_started_at"`
	OldSocketPath      string        `json:"old_socket_path"`
	NewPID             int           `json:"new_pid,omitempty"`
	ActiveSessionCount int           `json:"active_session_count"`
	FailureReason      string        `json:"failure_reason,omitempty"`
	StartedAt          time.Time     `json:"started_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
	CompletedAt        *time.Time    `json:"completed_at,omitempty"`
}

// Validate ensures the persisted restart operation is structurally usable.
func (o RestartOperation) Validate() error {
	if err := o.validateBaseFields(); err != nil {
		return err
	}
	if err := o.validateStatus(); err != nil {
		return err
	}
	return o.validateLifecycleFields()
}

func (o RestartOperation) validateBaseFields() error {
	switch {
	case strings.TrimSpace(o.OperationID) == "":
		return errors.New("daemon: restart operation id is required")
	case strings.ContainsAny(o.OperationID, `/\`):
		return fmt.Errorf("daemon: restart operation id %q contains path separators", o.OperationID)
	case o.OldPID <= 0:
		return fmt.Errorf("daemon: restart old pid must be positive: %d", o.OldPID)
	case o.ActiveSessionCount < 0:
		return fmt.Errorf("daemon: restart active session count must be non-negative: %d", o.ActiveSessionCount)
	case o.OldStartedAt.IsZero():
		return errors.New("daemon: restart old start time is required")
	case strings.TrimSpace(o.OldSocketPath) == "":
		return errors.New("daemon: restart old socket path is required")
	case o.StartedAt.IsZero():
		return errors.New("daemon: restart started_at is required")
	case o.UpdatedAt.IsZero():
		return errors.New("daemon: restart updated_at is required")
	case o.NewPID < 0:
		return fmt.Errorf("daemon: restart new pid must be non-negative: %d", o.NewPID)
	default:
		return nil
	}
}

func (o RestartOperation) validateStatus() error {
	switch o.Status {
	case RestartStatusPending,
		RestartStatusStopping,
		RestartStatusWaitingRelease,
		RestartStatusStarting,
		RestartStatusReady,
		RestartStatusFailed:
		return nil
	default:
		return fmt.Errorf("daemon: unsupported restart status %q", o.Status)
	}
}

func (o RestartOperation) validateLifecycleFields() error {
	switch o.Status {
	case RestartStatusReady:
		return o.validateReadyFields()
	case RestartStatusFailed:
		return o.validateFailedFields()
	default:
		return o.validateInFlightFields()
	}
}

func (o RestartOperation) validateReadyFields() error {
	if o.NewPID <= 0 {
		return errors.New("daemon: ready restart operation requires new_pid")
	}
	if o.CompletedAt == nil || o.CompletedAt.IsZero() {
		return errors.New("daemon: ready restart operation requires completed_at")
	}
	if strings.TrimSpace(o.FailureReason) != "" {
		return errors.New("daemon: ready restart operation cannot carry failure_reason")
	}
	return nil
}

func (o RestartOperation) validateFailedFields() error {
	if strings.TrimSpace(o.FailureReason) == "" {
		return errors.New("daemon: failed restart operation requires failure_reason")
	}
	if o.CompletedAt == nil || o.CompletedAt.IsZero() {
		return errors.New("daemon: failed restart operation requires completed_at")
	}
	if o.NewPID != 0 {
		return errors.New("daemon: failed restart operation must not set new_pid")
	}
	return nil
}

func (o RestartOperation) validateInFlightFields() error {
	if o.NewPID != 0 {
		return errors.New("daemon: restart operation must not set new_pid before ready")
	}
	if o.CompletedAt != nil {
		return errors.New("daemon: non-terminal restart operation must not set completed_at")
	}
	if strings.TrimSpace(o.FailureReason) != "" {
		return errors.New("daemon: non-terminal restart operation must not set failure_reason")
	}
	return nil
}

func (o RestartOperation) terminal() bool {
	return o.Status == RestartStatusReady || o.Status == RestartStatusFailed
}

func (o RestartOperation) hasFreshDaemonInfo(info Info) bool {
	if err := info.Validate(); err != nil {
		return false
	}
	return info.PID != o.OldPID || !info.StartedAt.Equal(o.OldStartedAt)
}

type restartTransition struct {
	status        RestartStatus
	failureReason string
	newPID        int
}

type restartStore struct {
	homePaths aghconfig.HomePaths
	now       func() time.Time
}

type restartProcess interface {
	PID() int
	Wait() error
}

type detachedStartRequest struct {
	binary  string
	args    []string
	sandbox []string
	logPath string
}

type detachedStartFunc func(context.Context, detachedStartRequest) (restartProcess, error)
