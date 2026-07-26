package task

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	redactpkg "github.com/compozy/agh/internal/redact"
)

const (
	leaseHandoffKey   = "handoff"
	leaseStatusKey    = "status"
	runEvidenceIDKey  = "run_id"
	taskEvidenceIDKey = "task_id"
)

const (
	// DefaultRunLeaseDuration is the conservative lease duration used when a caller omits one.
	DefaultRunLeaseDuration = 5 * time.Minute
	// MaxRunLeaseDuration is the MVP guardrail for a single task-run lease extension.
	MaxRunLeaseDuration = 24 * time.Hour

	claimTokenRandomBytes = 32
	claimTokenHashPrefix  = "sha256:"
	// LeaseRecoveryExhaustedReason identifies a run whose shared attempt budget cannot grant another claim.
	LeaseRecoveryExhaustedReason = "lease_recovery_exhausted"
)

var defaultCoordinationMessageKinds = []string{
	leaseStatusKey,
	"request",
	"reply",
	"blocker",
	leaseHandoffKey,
	"result",
	"review_request",
}

var canonicalTaskIDTokenPattern = regexp.MustCompile(`\btask-[A-Za-z0-9][A-Za-z0-9_-]*\b`)

// ClaimCriteria captures the atomic next-work filters for one claiming session.
type ClaimCriteria struct {
	RunID                string         `json:"run_id,omitempty"`
	Scope                Scope          `json:"scope,omitempty"`
	WorkspaceID          string         `json:"workspace_id,omitempty"`
	RunKind              RunKind        `json:"run_kind,omitempty"`
	TargetSessionID      string         `json:"target_session_id,omitempty"`
	ClaimerSessionID     string         `json:"claimer_session_id"`
	ClaimedBy            *ActorIdentity `json:"claimed_by,omitempty"`
	AgentName            string         `json:"agent_name,omitempty"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
	PriorityMin          int            `json:"priority_min,omitempty"`
	ParticipationChannel string         `json:"participation_channel,omitempty"`
	// WorkspaceActiveRunCap is trusted Service policy and never caller-controlled wire input.
	WorkspaceActiveRunCap int `json:"-"`
	// CallerNetworkParticipation is trusted hook context, never run-selection input.
	CallerNetworkParticipation *participation.Spec  `json:"-"`
	Soul                       *SoulClaimProvenance `json:"soul,omitempty"`
	LeaseDuration              time.Duration        `json:"lease_duration"`
	Now                        time.Time            `json:"now"`
}

// SoulClaimProvenance captures pre-resolved session Soul data at claim time.
type SoulClaimProvenance struct {
	SnapshotID string    `json:"snapshot_id,omitempty"`
	Digest     string    `json:"digest,omitempty"`
	AgentName  string    `json:"agent_name,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
}

// CoordinationChannelMetadata is the safe channel display metadata returned with a claim.
type CoordinationChannelMetadata struct {
	ID                  string    `json:"id"`
	DisplayName         string    `json:"display_name"`
	Purpose             string    `json:"purpose,omitempty"`
	WorkspaceID         string    `json:"workspace_id,omitempty"`
	TaskID              string    `json:"task_id,omitempty"`
	RunID               string    `json:"run_id,omitempty"`
	WorkflowID          string    `json:"workflow_id,omitempty"`
	AllowedMessageKinds []string  `json:"allowed_message_kinds,omitempty"`
	LastActivityAt      time.Time `json:"last_activity_at"`
}

// ClaimResult is the successful synchronous claim result. ClaimToken is raw and must not cross public surfaces.
type ClaimResult struct {
	Task                *Task                        `json:"task,omitempty"`
	Run                 Run                          `json:"run"`
	ClaimToken          string                       `json:"claim_token"`
	LeaseUntil          time.Time                    `json:"lease_until"`
	CoordinationChannel *CoordinationChannelMetadata `json:"coordination_channel,omitempty"`
}

// LeaseHeartbeat captures a token-fenced lease extension request.
type LeaseHeartbeat struct {
	RunID         string        `json:"run_id"`
	ClaimToken    string        `json:"claim_token"`
	LeaseDuration time.Duration `json:"lease_duration"`
	Now           time.Time     `json:"now"`
	TokensUsed    int64         `json:"tokens_used,omitempty"`
	Actor         ActorContext  `json:"-"`
}

// LeaseRelease captures a token-fenced release request.
type LeaseRelease struct {
	RunID      string       `json:"run_id"`
	ClaimToken string       `json:"claim_token"`
	Reason     string       `json:"reason,omitempty"`
	Now        time.Time    `json:"now"`
	Actor      ActorContext `json:"-"`
}

// LeaseCompletion captures a token-fenced successful terminal transition.
type LeaseCompletion struct {
	RunID          string    `json:"run_id"`
	ClaimToken     string    `json:"claim_token"`
	Result         RunResult `json:"result"`
	CreatedTaskIDs []string  `json:"created_task_ids,omitempty"`
	TokensUsed     int64     `json:"tokens_used,omitempty"`
	Now            time.Time `json:"now"`
	Actor          ActorContext
}

// LeaseFailure captures a token-fenced failed terminal transition.
type LeaseFailure struct {
	RunID      string     `json:"run_id"`
	ClaimToken string     `json:"claim_token"`
	Failure    RunFailure `json:"failure"`
	TokensUsed int64      `json:"tokens_used,omitempty"`
	Now        time.Time  `json:"now"`
	Actor      ActorContext
}

// ExpiredLeaseRecovery captures deterministic recovery of stale task-run leases.
type ExpiredLeaseRecovery struct {
	Now    time.Time    `json:"now"`
	Reason string       `json:"reason,omitempty"`
	Limit  int          `json:"limit,omitempty"`
	Actor  ActorContext `json:"-"`
}

// ExpiredLeaseRecoveryResult records one recovered lease and its previous owner state.
type ExpiredLeaseRecoveryResult struct {
	Run                    Run       `json:"run"`
	PreviousRunStatus      RunStatus `json:"previous_run_status"`
	PreviousSessionID      string    `json:"previous_session_id,omitempty"`
	PreviousLeaseUntil     time.Time `json:"previous_lease_until"`
	PreviousClaimTokenHash string    `json:"previous_claim_token_hash,omitempty"`
	Reason                 string    `json:"reason,omitempty"`
	Exhausted              bool      `json:"exhausted,omitempty"`
}

// SessionLeaseRelease captures a daemon-owned structural release for all active
// leases bound to one runtime session.
type SessionLeaseRelease struct {
	SessionID string    `json:"session_id"`
	Reason    string    `json:"reason,omitempty"`
	Now       time.Time `json:"now"`
}

// SessionLeaseReleaseResult records one structurally released session lease.
type SessionLeaseReleaseResult struct {
	Run                    Run       `json:"run"`
	PreviousRunStatus      RunStatus `json:"previous_run_status"`
	PreviousSessionID      string    `json:"previous_session_id,omitempty"`
	PreviousLeaseUntil     time.Time `json:"previous_lease_until"`
	PreviousClaimTokenHash string    `json:"previous_claim_token_hash,omitempty"`
	Reason                 string    `json:"reason,omitempty"`
}

// NewClaimToken generates one raw bearer token for a successful claim response.
func NewClaimToken() (string, error) {
	random := make([]byte, claimTokenRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("task: generate claim token: %w", err)
	}
	return "agh_claim_" + base64.RawURLEncoding.EncodeToString(random), nil
}

// RedactClaimTokens replaces raw claim bearer tokens in free-form strings.
func RedactClaimTokens(value string) string {
	if value == "" {
		return ""
	}
	return redactpkg.ClaimTokens(value)
}

// ClaimTokenHash returns the canonical hash persisted for one raw claim token.
func ClaimTokenHash(rawToken string) (string, error) {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", fmt.Errorf("%w: claim_token is required", ErrValidation)
	}
	sum := sha256.Sum256([]byte(token))
	return claimTokenHashPrefix + hex.EncodeToString(sum[:]), nil
}

// VerifyClaimToken reports whether rawToken hashes to the persisted canonical hash.
func VerifyClaimToken(rawToken string, persistedHash string) bool {
	token := strings.TrimSpace(rawToken)
	hash := canonicalClaimTokenHash(persistedHash)
	if token == "" || hash == "" {
		return false
	}
	sum := sha256.Sum256([]byte(token))
	computed := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}

// Normalize returns a validated claim criteria with default scope, time, and lease duration applied.
func (p SoulClaimProvenance) Validate(path string) error {
	hasSnapshotID := strings.TrimSpace(p.SnapshotID) != ""
	hasDigest := strings.TrimSpace(p.Digest) != ""
	if hasSnapshotID && !hasDigest {
		return fmt.Errorf("%w: %s.digest is required when snapshot_id is set", ErrValidation, path)
	}
	if hasDigest && strings.TrimSpace(p.AgentName) == "" {
		return fmt.Errorf("%w: %s.agent_name is required when digest is set", ErrValidation, path)
	}
	if !p.CapturedAt.IsZero() && p.CapturedAt.Location() != time.UTC {
		return fmt.Errorf("%w: %s.captured_at must be UTC", ErrValidation, path)
	}
	return nil
}

// Normalize returns a validated heartbeat request with default time and lease duration applied.
func (h LeaseHeartbeat) Normalize(defaultNow time.Time) (LeaseHeartbeat, error) {
	normalized := h
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.ClaimToken = strings.TrimSpace(normalized.ClaimToken)
	if normalized.LeaseDuration == 0 {
		normalized.LeaseDuration = DefaultRunLeaseDuration
	}
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	if err := normalized.Validate("lease_heartbeat"); err != nil {
		return LeaseHeartbeat{}, err
	}
	return normalized, nil
}

// Validate reports whether the heartbeat request is internally consistent.
func (h LeaseHeartbeat) Validate(path string) error {
	if err := validateLeaseRunToken(h.RunID, h.ClaimToken, path); err != nil {
		return err
	}
	if err := validateLeaseDuration(h.LeaseDuration, nestedPath(path, "lease_duration")); err != nil {
		return err
	}
	if h.Now.IsZero() {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "now"))
	}
	if h.TokensUsed < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "tokens_used"),
			h.TokensUsed,
		)
	}
	return nil
}

// Normalize returns a validated release request with default time applied.
func (r LeaseRelease) Normalize(defaultNow time.Time) (LeaseRelease, error) {
	normalized := r
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.ClaimToken = strings.TrimSpace(normalized.ClaimToken)
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	if err := normalized.Validate("lease_release"); err != nil {
		return LeaseRelease{}, err
	}
	return normalized, nil
}

// Validate reports whether the release request is internally consistent.
func (r LeaseRelease) Validate(path string) error {
	return validateLeaseRunToken(r.RunID, r.ClaimToken, path)
}

// Normalize returns a validated structural session lease release request.
func (r SessionLeaseRelease) Normalize(defaultNow time.Time) (SessionLeaseRelease, error) {
	normalized := r
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	if err := normalized.Validate("session_lease_release"); err != nil {
		return SessionLeaseRelease{}, err
	}
	return normalized, nil
}

// Validate reports whether the structural session lease release is internally consistent.
func (r SessionLeaseRelease) Validate(path string) error {
	if strings.TrimSpace(r.SessionID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "session_id"))
	}
	if r.Now.IsZero() {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "now"))
	}
	return nil
}

// Normalize returns a validated completion request with default time applied.
func (c LeaseCompletion) Normalize(defaultNow time.Time) (LeaseCompletion, error) {
	normalized := c
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.ClaimToken = strings.TrimSpace(normalized.ClaimToken)
	normalized.CreatedTaskIDs = normalizeCreatedTaskIDs(normalized.CreatedTaskIDs)
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	result, err := normalizeRunResult(normalized.Result)
	if err != nil {
		return LeaseCompletion{}, err
	}
	normalized.Result = result
	if err := normalized.Validate("lease_completion"); err != nil {
		return LeaseCompletion{}, err
	}
	return normalized, nil
}

// Validate reports whether the completion request is internally consistent.
func (c LeaseCompletion) Validate(path string) error {
	if err := validateLeaseRunToken(c.RunID, c.ClaimToken, path); err != nil {
		return err
	}
	for idx, taskID := range c.CreatedTaskIDs {
		if strings.TrimSpace(taskID) == "" {
			return fmt.Errorf(
				"%w: %s is required",
				ErrValidation,
				nestedPath(path, fmt.Sprintf("created_task_ids[%d]", idx)),
			)
		}
	}
	if c.TokensUsed < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "tokens_used"),
			c.TokensUsed,
		)
	}
	return c.Result.Validate(nestedPath(path, "result"))
}

// Normalize returns a validated failure request with default time applied.
func (f LeaseFailure) Normalize(defaultNow time.Time) (LeaseFailure, error) {
	normalized := f
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.ClaimToken = strings.TrimSpace(normalized.ClaimToken)
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	if err := normalized.Validate("lease_failure"); err != nil {
		return LeaseFailure{}, err
	}
	return normalized, nil
}

// Validate reports whether the failure request is internally consistent.
func (f LeaseFailure) Validate(path string) error {
	if err := validateLeaseRunToken(f.RunID, f.ClaimToken, path); err != nil {
		return err
	}
	if f.TokensUsed < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "tokens_used"),
			f.TokensUsed,
		)
	}
	return f.Failure.Validate(nestedPath(path, "failure"))
}

// Normalize returns a validated expired-lease recovery request.
func (r ExpiredLeaseRecovery) Normalize(defaultNow time.Time) (ExpiredLeaseRecovery, error) {
	normalized := r
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	if err := normalized.Validate("expired_lease_recovery"); err != nil {
		return ExpiredLeaseRecovery{}, err
	}
	return normalized, nil
}

// Validate reports whether the expired-lease recovery request is internally consistent.
func (r ExpiredLeaseRecovery) Validate(path string) error {
	if r.Now.IsZero() {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "now"))
	}
	if r.Limit < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "limit"),
			r.Limit,
		)
	}
	return nil
}
