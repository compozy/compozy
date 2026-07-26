package soul

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	snapshotProfileSchemaVersion = 1
	configDigestPrefix           = "agh.soul.config.v1\n"
)

var (
	// ErrSnapshotNotFound reports a missing persisted Soul snapshot.
	ErrSnapshotNotFound = errors.New("soul: snapshot not found")
	// ErrRevisionNotFound reports a missing persisted Soul authoring revision.
	ErrRevisionNotFound = errors.New("soul: revision not found")
	// ErrInvalidSnapshot reports a malformed persisted Soul snapshot.
	ErrInvalidSnapshot = errors.New("soul: invalid snapshot")
	// ErrInvalidRevision reports a malformed persisted Soul authoring revision.
	ErrInvalidRevision = errors.New("soul: invalid revision")
)

// RevisionAction describes a managed SOUL.md authoring mutation.
type RevisionAction string

const (
	// RevisionActionPut records a create or update mutation.
	RevisionActionPut RevisionAction = "put"
	// RevisionActionDelete records a managed delete mutation.
	RevisionActionDelete RevisionAction = "delete"
	// RevisionActionRollback records a managed rollback mutation.
	RevisionActionRollback RevisionAction = "rollback"
)

// Snapshot is the immutable storage row for a resolved Soul profile.
type Snapshot struct {
	ID          string
	WorkspaceID string
	AgentName   string
	SourcePath  string
	Digest      string
	ProfileJSON json.RawMessage
	Body        string
	Truncated   bool
	CreatedAt   time.Time
}

// SnapshotProfile is the structured JSON envelope stored in Snapshot.ProfileJSON.
type SnapshotProfile struct {
	SchemaVersion    int               `json:"schema_version"`
	Present          bool              `json:"present"`
	Active           bool              `json:"active"`
	Valid            bool              `json:"valid"`
	Profile          Profile           `json:"profile"`
	Compact          CompactProjection `json:"compact"`
	ReadModel        ReadModel         `json:"read_model"`
	ConfigProvenance ConfigProvenance  `json:"config_provenance"`
	Diagnostics      []Diagnostic      `json:"diagnostics,omitempty"`
}

// ConfigProvenance captures the Soul config values that shaped a snapshot.
type ConfigProvenance struct {
	Digest                 string `json:"digest"`
	Source                 string `json:"source,omitempty"`
	Enabled                bool   `json:"enabled"`
	MaxBodyBytes           int64  `json:"max_body_bytes"`
	ContextProjectionBytes int64  `json:"context_projection_bytes"`
}

// SnapshotListQuery filters persisted Soul snapshot rows.
type SnapshotListQuery struct {
	WorkspaceID string
	AgentName   string
	Digest      string
	Limit       int
}

// Revision is one append-only managed SOUL.md authoring history row.
type Revision struct {
	ID              string
	WorkspaceID     string
	AgentName       string
	SourcePath      string
	Action          RevisionAction
	PreviousDigest  string
	NewDigest       string
	Body            string
	DiagnosticsJSON json.RawMessage
	ActorKind       string
	ActorID         string
	OriginKind      string
	OriginRef       string
	CreatedAt       time.Time
}

// RevisionListQuery filters managed Soul authoring revision history.
type RevisionListQuery struct {
	WorkspaceID string
	AgentName   string
	Action      RevisionAction
	Limit       int
}

// RollbackLookup selects the prior revision body used by managed rollback.
type RollbackLookup struct {
	WorkspaceID string
	AgentName   string
	RevisionID  string
}

// NewConfigProvenance returns deterministic config provenance for a resolved Soul.
func NewConfigProvenance(config aghconfig.SoulConfig, source string) (ConfigProvenance, error) {
	canonical := struct {
		Enabled                bool  `json:"enabled"`
		MaxBodyBytes           int64 `json:"max_body_bytes"`
		ContextProjectionBytes int64 `json:"context_projection_bytes"`
	}{
		Enabled:                config.Enabled,
		MaxBodyBytes:           config.MaxBodyBytes,
		ContextProjectionBytes: config.ContextProjectionBytes,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return ConfigProvenance{}, fmt.Errorf("soul: marshal config provenance: %w", err)
	}
	sum := sha256.Sum256([]byte(configDigestPrefix + string(encoded)))
	return ConfigProvenance{
		Digest:                 "sha256:" + hex.EncodeToString(sum[:]),
		Source:                 strings.TrimSpace(source),
		Enabled:                config.Enabled,
		MaxBodyBytes:           config.MaxBodyBytes,
		ContextProjectionBytes: config.ContextProjectionBytes,
	}, nil
}

// SnapshotFromResolved creates a persistence row from a resolved Soul profile.
func SnapshotFromResolved(
	id string,
	workspaceID string,
	agentName string,
	resolved *ResolvedSoul,
	configProvenance ConfigProvenance,
	createdAt time.Time,
) (Snapshot, error) {
	if resolved == nil {
		return Snapshot{}, fmt.Errorf("%w: resolved soul is required", ErrInvalidSnapshot)
	}
	profile := SnapshotProfile{
		SchemaVersion:    snapshotProfileSchemaVersion,
		Present:          resolved.Present,
		Active:           resolved.Active,
		Valid:            resolved.Valid,
		Profile:          resolved.Profile,
		Compact:          resolved.Compact,
		ReadModel:        resolved.ReadModel,
		ConfigProvenance: configProvenance,
		Diagnostics:      clonePersistenceDiagnostics(resolved.Diagnostics),
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		return Snapshot{}, fmt.Errorf("soul: marshal snapshot profile: %w", err)
	}
	snapshot := Snapshot{
		ID:          id,
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		SourcePath:  firstNonEmpty(resolved.SourcePath, resolved.Profile.SourcePath, resolved.ReadModel.SourcePath),
		Digest:      firstNonEmpty(resolved.Digest, resolved.Profile.Digest, resolved.ReadModel.Digest),
		ProfileJSON: encoded,
		Body:        firstNonEmpty(resolved.Profile.Body, resolved.ReadModel.Body),
		Truncated:   resolved.Profile.Truncated || resolved.ReadModel.Truncated || resolved.Compact.Truncated,
		CreatedAt:   createdAt,
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot.Normalize(), nil
}

// ProfileEnvelope decodes the structured JSON envelope stored with the snapshot.
func (s Snapshot) ProfileEnvelope() (SnapshotProfile, error) {
	normalized := s.Normalize()
	if len(normalized.ProfileJSON) == 0 {
		return SnapshotProfile{}, fmt.Errorf("%w: profile_json is required", ErrInvalidSnapshot)
	}
	return decodeSnapshotProfileJSON(normalized.ProfileJSON)
}

func decodeSnapshotProfileJSON(raw json.RawMessage) (SnapshotProfile, error) {
	var profile SnapshotProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return SnapshotProfile{}, fmt.Errorf("%w: decode profile_json: %w", ErrInvalidSnapshot, err)
	}
	if profile.SchemaVersion != snapshotProfileSchemaVersion {
		return SnapshotProfile{}, fmt.Errorf(
			"%w: unsupported profile schema version %d",
			ErrInvalidSnapshot,
			profile.SchemaVersion,
		)
	}
	return profile, nil
}

func validateRevisionDiagnosticsJSON(raw json.RawMessage) error {
	var diagnostics []Diagnostic
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		return fmt.Errorf("%w: decode diagnostics_json: %w", ErrInvalidRevision, err)
	}
	return nil
}

// DiagnosticsJSON encodes redacted validation diagnostics for revision storage.
func DiagnosticsJSON(diagnostics []Diagnostic) (json.RawMessage, error) {
	encoded, err := json.Marshal(clonePersistenceDiagnostics(diagnostics))
	if err != nil {
		return nil, fmt.Errorf("soul: marshal diagnostics: %w", err)
	}
	return encoded, nil
}

// Normalize trims metadata fields and applies JSON defaults.
func (s Snapshot) Normalize() Snapshot {
	s.ID = strings.TrimSpace(s.ID)
	s.WorkspaceID = strings.TrimSpace(s.WorkspaceID)
	s.AgentName = strings.TrimSpace(s.AgentName)
	s.SourcePath = strings.TrimSpace(s.SourcePath)
	s.Digest = strings.TrimSpace(s.Digest)
	if len(s.ProfileJSON) == 0 {
		s.ProfileJSON = json.RawMessage(`{}`)
	}
	return s
}

// Validate ensures the snapshot can be stored as immutable provenance.
func (s Snapshot) Validate() error {
	normalized := s.Normalize()
	switch {
	case normalized.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidSnapshot)
	case normalized.WorkspaceID == "":
		return fmt.Errorf("%w: workspace id is required", ErrInvalidSnapshot)
	case normalized.AgentName == "":
		return fmt.Errorf("%w: agent name is required", ErrInvalidSnapshot)
	case normalized.SourcePath == "":
		return fmt.Errorf("%w: source path is required", ErrInvalidSnapshot)
	case normalized.Digest == "":
		return fmt.Errorf("%w: digest is required", ErrInvalidSnapshot)
	case !json.Valid(normalized.ProfileJSON):
		return fmt.Errorf("%w: profile_json must be valid JSON", ErrInvalidSnapshot)
	case normalized.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidSnapshot)
	}
	if _, err := decodeSnapshotProfileJSON(normalized.ProfileJSON); err != nil {
		return err
	}
	return nil
}

// Validate ensures the snapshot query uses supported bounds.
func (q SnapshotListQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("soul: invalid snapshot limit %d", q.Limit)
	}
	return nil
}

// Normalize trims metadata fields and applies JSON defaults.
func (r Revision) Normalize() Revision {
	r.ID = strings.TrimSpace(r.ID)
	r.WorkspaceID = strings.TrimSpace(r.WorkspaceID)
	r.AgentName = strings.TrimSpace(r.AgentName)
	r.SourcePath = strings.TrimSpace(r.SourcePath)
	r.Action = RevisionAction(strings.TrimSpace(string(r.Action)))
	r.PreviousDigest = strings.TrimSpace(r.PreviousDigest)
	r.NewDigest = strings.TrimSpace(r.NewDigest)
	r.ActorKind = strings.TrimSpace(r.ActorKind)
	r.ActorID = strings.TrimSpace(r.ActorID)
	r.OriginKind = strings.TrimSpace(r.OriginKind)
	r.OriginRef = strings.TrimSpace(r.OriginRef)
	if len(r.DiagnosticsJSON) == 0 {
		r.DiagnosticsJSON = json.RawMessage(`[]`)
	}
	return r
}

// Validate ensures the authoring revision can be appended.
func (r Revision) Validate() error {
	normalized := r.Normalize()
	switch {
	case normalized.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidRevision)
	case normalized.WorkspaceID == "":
		return fmt.Errorf("%w: workspace id is required", ErrInvalidRevision)
	case normalized.AgentName == "":
		return fmt.Errorf("%w: agent name is required", ErrInvalidRevision)
	case normalized.SourcePath == "":
		return fmt.Errorf("%w: source path is required", ErrInvalidRevision)
	case !validRevisionAction(normalized.Action):
		return fmt.Errorf("%w: unsupported action %q", ErrInvalidRevision, normalized.Action)
	case normalized.Action == RevisionActionDelete && normalized.NewDigest != "":
		return fmt.Errorf("%w: delete revisions must not set new_digest", ErrInvalidRevision)
	case normalized.Action != RevisionActionDelete && normalized.NewDigest == "":
		return fmt.Errorf("%w: %s revisions require new_digest", ErrInvalidRevision, normalized.Action)
	case !json.Valid(normalized.DiagnosticsJSON):
		return fmt.Errorf("%w: diagnostics_json must be valid JSON", ErrInvalidRevision)
	case normalized.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidRevision)
	}
	if err := validateRevisionDiagnosticsJSON(normalized.DiagnosticsJSON); err != nil {
		return err
	}
	return nil
}

// Validate ensures the revision query uses supported bounds.
func (q RevisionListQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("soul: invalid revision limit %d", q.Limit)
	}
	if q.Action != "" && !validRevisionAction(q.Action) {
		return fmt.Errorf("soul: unsupported revision action %q", q.Action)
	}
	return nil
}

// Validate ensures rollback lookup has a deterministic target revision.
func (q RollbackLookup) Validate() error {
	switch {
	case strings.TrimSpace(q.WorkspaceID) == "":
		return fmt.Errorf("soul: rollback workspace id is required")
	case strings.TrimSpace(q.AgentName) == "":
		return fmt.Errorf("soul: rollback agent name is required")
	case strings.TrimSpace(q.RevisionID) == "":
		return fmt.Errorf("soul: rollback revision id is required")
	default:
		return nil
	}
}

func validRevisionAction(action RevisionAction) bool {
	switch action {
	case RevisionActionPut, RevisionActionDelete, RevisionActionRollback:
		return true
	default:
		return false
	}
}

func clonePersistenceDiagnostics(items []Diagnostic) []Diagnostic {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]Diagnostic, len(items))
	copy(cloned, items)
	return cloned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
