package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrSessionNotFound reports that a persisted session row does not exist.
	ErrSessionNotFound = errors.New("store: session not found")
	// ErrSessionAttachLocked reports that another holder owns a live attach lease.
	ErrSessionAttachLocked = errors.New("store: session attach locked")
	// ErrSessionNotAttachable reports that a session is not eligible for attach/resume.
	ErrSessionNotAttachable = errors.New("store: session not attachable")
	// ErrSessionParticipationMismatch rejects a same-ID refresh with different immutable Network participation.
	ErrSessionParticipationMismatch = errors.New("store: session participation mismatch")
)

type StopReason string

const (
	StopCompleted      StopReason = "completed"
	StopUserCanceled   StopReason = "user_canceled"
	StopMaxIterations  StopReason = "max_iterations"
	StopLoopDetected   StopReason = "loop_detected"
	StopTimeout        StopReason = "timeout"
	StopBudgetExceeded StopReason = "budget_exceeded"
	StopError          StopReason = "error"
	StopAgentCrashed   StopReason = "agent_crashed"
	StopHookStopped    StopReason = "hook_stopped"
	StopShutdown       StopReason = "shutdown"
)

// ValidStopReason reports whether r is a supported stop reason enum member.
func ValidStopReason(r StopReason) bool {
	switch r {
	case StopCompleted,
		StopUserCanceled,
		StopMaxIterations,
		StopLoopDetected,
		StopTimeout,
		StopBudgetExceeded,
		StopError,
		StopAgentCrashed,
		StopHookStopped,
		StopShutdown:
		return true
	default:
		return false
	}
}

// SessionInfo is the canonical session index row stored in the global database.
type SessionInfo struct {
	ID          string
	Name        string
	AgentName   string
	Provider    string
	WorkspaceID string
	*SessionNetworkState
	SessionType      string
	Lineage          *SessionLineage
	State            string
	ACPSessionID     *string
	StopReason       StopReason
	StopDetail       string
	Failure          *SessionFailure
	Liveness         *SessionLivenessMeta
	Sandbox          *SessionSandboxMeta
	SoulSnapshotID   string
	SoulDigest       string
	ParentSoulDigest string
	AttachedTo       string
	AttachExpiresAt  *time.Time
	TranscriptEpoch  int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Validate ensures the session record contains the required fields.
func (s SessionInfo) Validate() error {
	if err := requireField(s.ID, "session id"); err != nil {
		return err
	}
	if err := requireField(s.AgentName, "session agent name"); err != nil {
		return err
	}
	if err := requireField(s.WorkspaceID, "session workspace id"); err != nil {
		return err
	}
	if err := requireField(s.State, "session state"); err != nil {
		return err
	}
	if err := ValidateSessionLineage(s.ID, s.Lineage); err != nil {
		return err
	}
	if err := validateSessionStopReason(s.StopReason); err != nil {
		return err
	}
	if err := s.Liveness.Validate(); err != nil {
		return err
	}
	if err := validateSessionSoulProvenance(s.SoulSnapshotID, s.SoulDigest); err != nil {
		return err
	}
	if s.Failure != nil {
		if err := s.Failure.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// SessionListQuery filters global session index queries.
type SessionListQuery struct {
	ID              string
	State           string
	AgentName       string
	WorkspaceID     string
	SessionType     string
	ParentSessionID string
	RootSessionID   string
	SpawnRole       string
	Resumable       bool
	Sort            string
	Limit           int
}

// SessionTranscriptEpochUpdate advances a session transcript epoch after destructive transcript resets.
type SessionTranscriptEpochUpdate struct {
	SessionID string
	Minimum   int64
}

// Validate ensures the epoch update targets a known session and positive epoch.
func (u SessionTranscriptEpochUpdate) Validate() error {
	if err := requireField(u.SessionID, "session id"); err != nil {
		return err
	}
	if u.Minimum <= 0 {
		return fmt.Errorf("store: transcript epoch minimum must be positive")
	}
	return nil
}

// Validate ensures the query uses sane bounds.
func (q SessionListQuery) Validate() error {
	return requirePositiveLimit(q.Limit, "session limit")
}

// SessionAttachRequest identifies one explicit attach lock acquisition.
type SessionAttachRequest struct {
	SessionID  string
	AttachedTo string
	Now        time.Time
	TTL        time.Duration
}

// Normalize trims the attach request and applies UTC timestamps.
func (r SessionAttachRequest) Normalize() SessionAttachRequest {
	r.SessionID = strings.TrimSpace(r.SessionID)
	r.AttachedTo = strings.TrimSpace(r.AttachedTo)
	if !r.Now.IsZero() {
		r.Now = r.Now.UTC()
	}
	return r
}

// Validate ensures the attach request carries the fields needed for a CAS lock.
func (r SessionAttachRequest) Validate() error {
	normalized := r.Normalize()
	if err := requireField(normalized.SessionID, "session attach session id"); err != nil {
		return err
	}
	if err := requireField(normalized.AttachedTo, "session attach holder"); err != nil {
		return err
	}
	if normalized.TTL <= 0 {
		return fmt.Errorf("store: session attach ttl must be positive")
	}
	return nil
}

// SessionAttach records one acquired attach lock.
type SessionAttach struct {
	SessionID       string
	AttachedTo      string
	AttachExpiresAt time.Time
	AttachedAt      time.Time
}

// SessionStateUpdate updates only the stateful fields of an indexed session.
type SessionStateUpdate struct {
	ID            string
	State         string
	ACPSessionID  *string
	StopReasonSet bool
	StopReason    *string
	StopDetail    string
	FailureSet    bool
	Failure       *SessionFailure
	Liveness      *SessionLivenessMeta
	Sandbox       *SessionSandboxMeta
	UpdatedAt     time.Time
}

// SessionSoulSnapshotUpdate updates the Soul provenance attached to a session.
type SessionSoulSnapshotUpdate struct {
	ID               string
	SoulSnapshotID   string
	SoulDigest       string
	ParentSoulDigest string
	UpdatedAt        time.Time
}

// Validate ensures session Soul provenance is internally consistent.
func (u SessionSoulSnapshotUpdate) Validate() error {
	if err := requireField(u.ID, "session soul update id"); err != nil {
		return err
	}
	return validateSessionSoulProvenance(u.SoulSnapshotID, u.SoulDigest)
}

// Validate ensures the update contains the required fields.
func (u SessionStateUpdate) Validate() error {
	if err := requireField(u.ID, "session update id"); err != nil {
		return err
	}
	if err := requireField(u.State, "session update state"); err != nil {
		return err
	}
	if err := u.Liveness.Validate(); err != nil {
		return err
	}
	if u.StopReasonSet && u.StopReason != nil {
		if err := validateSessionStopReason(StopReason(strings.TrimSpace(*u.StopReason))); err != nil {
			return err
		}
	}
	if u.Failure != nil {
		if err := u.Failure.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateSessionSoulProvenance(snapshotID string, digest string) error {
	hasSnapshotID := strings.TrimSpace(snapshotID) != ""
	hasDigest := strings.TrimSpace(digest) != ""
	if hasSnapshotID && !hasDigest {
		return errors.New("store: session soul digest is required when soul snapshot id is set")
	}
	return nil
}

func validateSessionStopReason(reason StopReason) error {
	if reason == "" {
		return nil
	}
	if !ValidStopReason(reason) {
		return fmt.Errorf("store: invalid session stop reason %q", reason)
	}
	return nil
}

// EventSummary is the global, cross-session observability record for one event.
