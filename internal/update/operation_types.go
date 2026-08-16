package update

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const OperationSchemaVersion = 1

// Target identifies one install track in a host-global update operation.
type Target string

const (
	TargetRuntime Target = "runtime"
	TargetApp     Target = "app"
)

// Actor identifies the public surface that requested a transition.
type Actor string

const (
	ActorCLI    Actor = "cli"
	ActorDaemon Actor = "daemon"
	ActorWeb    Actor = "web"
	ActorShell  Actor = "shell"
)

// WaitingState identifies an operation that is durable but has no live holder.
type WaitingState string

const (
	WaitingNone   WaitingState = ""
	WaitingForApp WaitingState = "waiting-for-app"
)

// OperationPhase is the closed union of runtime and app journal phases.
type OperationPhase string

const (
	PhasePending          OperationPhase = "pending"
	PhaseDownloading      OperationPhase = "downloading"
	PhaseVerifying        OperationPhase = "verifying"
	PhaseSwapping         OperationPhase = "swapping"
	PhaseRestarting       OperationPhase = "restarting"
	PhaseHealthChecking   OperationPhase = "health-checking"
	PhaseFinalized        OperationPhase = "finalized"
	PhaseRolledBack       OperationPhase = "rolled-back"
	PhaseFailed           OperationPhase = "failed"
	PhaseStaged           OperationPhase = "staged"
	PhaseApplying         OperationPhase = "applying"
	PhaseInstallerHandoff OperationPhase = "installer-handoff"
	PhaseRestarted        OperationPhase = "restarted"
	PhaseVerified         OperationPhase = "verified"
)

// Holder is the fenced lease for the sole active executor.
type Holder struct {
	PID                int       `json:"pid"`
	PIDStartTime       time.Time `json:"pid_start_time"`
	Surface            Actor     `json:"surface"`
	ExecutorGeneration string    `json:"executor_generation"`
	LeaseExpiresAt     time.Time `json:"lease_expires_at"`
}

// ArtifactIdentity binds an update step to one verified release artifact.
type ArtifactIdentity struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	ReleaseTag  string `json:"release_tag"`
	Asset       string `json:"asset"`
	Digest      string `json:"digest"`
}

// RuntimeOperationState journals the runtime track.
type RuntimeOperationState struct {
	ArtifactIdentity
	InstallMethod InstallMethod  `json:"install_method"`
	BackupPath    string         `json:"backup_path,omitempty"`
	Phase         OperationPhase `json:"phase"`
}

// AppOperationState journals the desktop app track.
type AppOperationState struct {
	ArtifactIdentity
	AttemptID           string         `json:"attempt_id"`
	Phase               OperationPhase `json:"phase"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	WatchdogDeadline    time.Time      `json:"watchdog_deadline,omitempty"`
}

// Operation is the sole durable mutation journal for one CompozyOS home.
type Operation struct {
	SchemaVersion int                    `json:"schema_version"`
	ID            string                 `json:"operation_id"`
	RequestedBy   Actor                  `json:"requested_by"`
	Revision      int64                  `json:"revision"`
	Targets       []Target               `json:"targets"`
	ActiveTarget  Target                 `json:"active_target,omitempty"`
	Percent       int                    `json:"percent"`
	Runtime       *RuntimeOperationState `json:"runtime,omitempty"`
	App           *AppOperationState     `json:"app,omitempty"`
	Holder        *Holder                `json:"holder"`
	Waiting       WaitingState           `json:"waiting"`
	Deadline      time.Time              `json:"deadline"`
	StartedAt     time.Time              `json:"started_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	LastError     string                 `json:"last_error,omitempty"`
	Outcome       string                 `json:"outcome,omitempty"`
}

// OperationRequest contains release identities already verified at acquisition.
type OperationRequest struct {
	RequestedBy Actor
	Targets     []Target
	Runtime     *RuntimeOperationState
	App         *AppOperationState
	Holder      Holder
	Deadline    time.Time
}

func (t Target) valid() bool { return t == TargetRuntime || t == TargetApp }

func (a Actor) valid() bool {
	return a == ActorCLI || a == ActorDaemon || a == ActorWeb || a == ActorShell
}

func (p OperationPhase) validFor(target Target) bool {
	switch target {
	case TargetRuntime:
		return p == PhasePending || p == PhaseDownloading || p == PhaseVerifying || p == PhaseSwapping ||
			p == PhaseRestarting || p == PhaseHealthChecking || p == PhaseFinalized || p == PhaseRolledBack || p == PhaseFailed
	case TargetApp:
		return p == PhasePending || p == PhaseStaged || p == PhaseApplying || p == PhaseInstallerHandoff ||
			p == PhaseRestarted || p == PhaseVerified || p == PhaseFailed
	default:
		return false
	}
}

func validateOperation(operation *Operation) error {
	if operation == nil {
		return errors.New("update: operation is required")
	}
	if operation.SchemaVersion != OperationSchemaVersion {
		return fmt.Errorf("update: unsupported operation schema version %d", operation.SchemaVersion)
	}
	if strings.TrimSpace(operation.ID) == "" || !operation.RequestedBy.valid() || operation.Revision < 1 {
		return errors.New("update: operation identity is invalid")
	}
	if err := validateTargets(operation.Targets); err != nil {
		return err
	}
	if operation.Runtime != nil && !operation.Runtime.Phase.validFor(TargetRuntime) {
		return fmt.Errorf("update: invalid runtime phase %q", operation.Runtime.Phase)
	}
	if operation.App != nil && !operation.App.Phase.validFor(TargetApp) {
		return fmt.Errorf("update: invalid app phase %q", operation.App.Phase)
	}
	if operation.Holder != nil {
		if operation.Holder.PID <= 0 || operation.Holder.PIDStartTime.IsZero() ||
			!operation.Holder.Surface.valid() || strings.TrimSpace(operation.Holder.ExecutorGeneration) == "" ||
			operation.Holder.LeaseExpiresAt.IsZero() {
			return errors.New("update: operation holder is invalid")
		}
		if operation.Waiting != WaitingNone {
			return errors.New("update: a waiting operation cannot have a holder")
		}
	}
	if operation.Waiting != WaitingNone && operation.Waiting != WaitingForApp {
		return fmt.Errorf("update: invalid waiting state %q", operation.Waiting)
	}
	if operation.Percent < -1 || operation.Percent > 100 {
		return fmt.Errorf("update: percent %d is outside -1..100", operation.Percent)
	}
	return nil
}

func validateTargets(targets []Target) error {
	if len(targets) == 0 || len(targets) > 2 {
		return errors.New("update: one or two targets are required")
	}
	seen := make(map[Target]struct{}, len(targets))
	for index, target := range targets {
		if !target.valid() {
			return fmt.Errorf("update: invalid target %q", target)
		}
		if _, ok := seen[target]; ok {
			return fmt.Errorf("update: duplicate target %q", target)
		}
		seen[target] = struct{}{}
		if target == TargetRuntime && index != 0 {
			return errors.New("update: runtime must be the first target")
		}
	}
	return nil
}
