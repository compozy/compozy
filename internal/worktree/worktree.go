package worktree

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/redact"
)

type State string

const (
	StatePending   State = "pending"
	StateReady     State = "ready"
	StateFailed    State = "failed"
	StateRemoving  State = "removing"
	StateMissing   State = "missing"
	StateRemoved   State = "removed"
	StateDismissed State = "dismissed"
)

type Origin string

const (
	OriginManual  Origin = "manual"
	OriginPerRun  Origin = "per_run"
	OriginAdopted Origin = "adopted"
)

type PendingPhase string

const (
	PhaseNone     PendingPhase = ""
	PhaseBranch   PendingPhase = "branch"
	PhaseCheckout PendingPhase = "checkout"
	PhaseCopy     PendingPhase = "copy"
	PhaseSetup    PendingPhase = "setup"
)

type SetupState string

const (
	SetupNone   SetupState = "none"
	SetupOK     SetupState = "ok"
	SetupFailed SetupState = "failed"
)

type Worktree struct {
	ID            string
	WorkspaceID   string
	Name          string
	Branch        string
	Path          string
	GitDir        string
	State         State
	PendingPhase  PendingPhase
	Origin        Origin
	SetupState    SetupState
	SetupError    string
	BaseRef       string
	CreatedBranch bool
	RunNamespace  string
	CreatedHead   string
	RunID         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Status struct {
	WorktreeID  string
	Branch      *string
	Detached    *bool
	HeadSHA     *string
	DirtyFiles  *int
	Insertions  *int
	Deletions   *int
	HasUpstream *bool
	Ahead       *int
	AheadOfBase *int
	Behind      *int
	ReadError   string
	RefreshedAt *time.Time
}

// IsStale reports whether the cached Git snapshot is older than the consumer's
// freshness window. A missing timestamp is always unknown and therefore stale.
func (s Status) IsStale(now time.Time, ttl time.Duration) bool {
	return timestampIsStale(s.RefreshedAt, now, ttl)
}

type ForgeStatus struct {
	WorktreeID string
	Provider   string
	PRNumber   *int
	PRState    *string
	PRURL      string
	Merged     *bool
	FetchedAt  *time.Time
}

// IsStale reports whether the cached forge snapshot is older than the
// consumer's freshness window. It never performs network I/O.
func (s ForgeStatus) IsStale(now time.Time, ttl time.Duration) bool {
	return timestampIsStale(s.FetchedAt, now, ttl)
}

func timestampIsStale(observedAt *time.Time, now time.Time, ttl time.Duration) bool {
	if observedAt == nil || ttl <= 0 {
		return true
	}
	return !now.Before(observedAt.Add(ttl))
}

type ExitOperation struct {
	ID          string
	WorkspaceID string
	WorktreeID  string
	Action      string
	State       string
	StartedAt   time.Time
	FinishedAt  *time.Time
}

type PendingUpdate struct {
	Phase         PendingPhase
	Branch        string
	GitDir        string
	BaseRef       string
	CreatedBranch bool
	RunNamespace  string
	CreatedHead   string
}

// Store owns workspace-predicated persistence for the worktree domain.
type Store interface {
	Insert(context.Context, Worktree) error
	Get(context.Context, string, string) (*Worktree, error)
	GetByPath(context.Context, string, string) (*Worktree, error)
	List(context.Context, string) ([]Worktree, error)
	ListRecoverable(context.Context) ([]Worktree, error)
	DeletePending(context.Context, string, string) error
	DeleteRunMaterialization(context.Context, string, string, string) error
	UpdatePending(context.Context, string, string, PendingUpdate) error
	CompleteCreate(context.Context, string, string, SetupState, string, time.Time) error
	CompareAndSwapState(context.Context, string, string, State, State, time.Time) (bool, error)
	SetState(context.Context, string, string, State, time.Time) error
	SaveStatus(context.Context, string, string, Status) error
	GetStatus(context.Context, string, string) (*Status, error)
	SaveForgeStatus(context.Context, string, string, ForgeStatus) error
	GetForgeStatus(context.Context, string, string) (*ForgeStatus, error)
	InsertExitOperation(context.Context, ExitOperation) error
	FinishExitOperation(context.Context, string, string, string, string, time.Time) (bool, error)
	FailRunningExitOperations(context.Context, time.Time) error
}

var (
	ErrGitUnavailable         = errors.New("worktree_git_unavailable")
	ErrGitVersionUnsupported  = errors.New("worktree_git_version_unsupported")
	ErrWorkspaceNotGitBacked  = errors.New("workspace_not_git_backed")
	ErrNameTaken              = errors.New("worktree_name_taken")
	ErrPathExists             = errors.New("worktree_path_exists")
	ErrBranchHeld             = errors.New("branch_held_by_worktree")
	ErrBranchCheckedOutAtRoot = errors.New("branch_checked_out_at_root")
	ErrBaseRefNotFound        = errors.New("base_ref_not_found")
	ErrRepoHasNoCommits       = errors.New("repo_has_no_commits")
	ErrNotFound               = errors.New("worktree_not_found")
	ErrNotReady               = errors.New("worktree_not_ready")
	ErrPending                = errors.New("worktree_pending")
	ErrMissing                = errors.New("worktree_missing")
	ErrRefInvalid             = errors.New("worktree_ref_invalid")
	ErrAdoptionMainCheckout   = errors.New("adoption_main_checkout")
	ErrAdoptionForeignRepo    = errors.New("adoption_foreign_repository")
	ErrAdoptionUnreadable     = errors.New("adoption_unreadable")
	ErrOperationInProgress    = errors.New("worktree_operation_in_progress")
	ErrSessionActive          = errors.New("worktree_session_active")
	ErrStatusUnreadable       = errors.New("worktree_status_unreadable")
	ErrDirtyRequiresForce     = errors.New("worktree_dirty_requires_force")
	ErrUnpushedRequiresForce  = errors.New("worktree_unpushed_requires_force")
	ErrSafetyCheckFailed      = errors.New("worktree_safety_check_failed")
	ErrRemovalFailed          = errors.New("worktree_removal_failed")
	ErrPerRunMaterialization  = errors.New("per_run_materialization_failed")
	ErrConfigInvalid          = errors.New("worktree_config_invalid")
	ErrDeniedByHook           = errors.New("worktree_denied_by_hook")
	ErrNotPending             = errors.New("worktree_not_pending")
	ErrForgeUnavailable       = errors.New("forge_unavailable")
	ErrForge                  = errors.New("forge_error")
)

type RefusalError struct {
	Cause  error
	Detail string
}

func (e *RefusalError) Error() string {
	if e == nil || e.Cause == nil {
		return "worktree operation refused"
	}
	if e.Detail == "" {
		return e.Cause.Error()
	}
	return fmt.Sprintf("%s: %s", e.Cause, e.Detail)
}

func (e *RefusalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func refusal(cause error, detail string) error {
	return &RefusalError{Cause: cause, Detail: redact.String(detail)}
}
