package heartbeat

import (
	"encoding/json"

	"fmt"
	"strings"
	"time"
)

// SnapshotFromResolved creates a persistence row from a resolved Heartbeat policy.
func SnapshotFromResolved(
	id string,
	workspaceID string,
	agentName string,
	resolved *ResolvedPolicy,
	createdAt time.Time,
) (Snapshot, error) {
	if resolved == nil {
		return Snapshot{}, fmt.Errorf("%w: resolved policy is required", ErrInvalidSnapshot)
	}
	frontmatter, err := json.Marshal(resolved.Frontmatter)
	if err != nil {
		return Snapshot{}, fmt.Errorf("heartbeat: marshal snapshot frontmatter: %w", err)
	}
	diagnostics, err := DiagnosticsJSON(resolved.Diagnostics)
	if err != nil {
		return Snapshot{}, err
	}
	envelope := SnapshotEnvelope{
		SchemaVersion:    firstPositive(resolved.SchemaVersion, snapshotSchemaVersion),
		Present:          resolved.Present,
		Active:           resolved.Active,
		Valid:            resolved.Valid,
		Summary:          resolved.Summary,
		GuidanceMarkdown: resolved.GuidanceMarkdown,
		Preferences:      resolved.Preferences,
		ConfigProvenance: resolved.ConfigProvenance,
		Prompt:           resolved.Prompt,
		Status:           resolved.Status,
		Diagnostics:      cloneDiagnostics(resolved.Diagnostics),
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return Snapshot{}, fmt.Errorf("heartbeat: marshal snapshot envelope: %w", err)
	}
	configDigest := firstNonEmpty(resolved.ConfigDigest, resolved.ConfigProvenance.Digest)
	digest := strings.TrimSpace(resolved.Digest)
	if digest == "" && !resolved.Present && resolved.Valid {
		digest = digestAbsentPolicy(id, resolved.SourcePath, configDigest)
	}
	snapshot := Snapshot{
		ID:              id,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		SourcePath:      resolved.SourcePath,
		SchemaVersion:   envelope.SchemaVersion,
		Digest:          digest,
		ConfigDigest:    configDigest,
		Body:            resolved.GuidanceMarkdown,
		FrontmatterJSON: frontmatter,
		ResolvedJSON:    encoded,
		DiagnosticsJSON: diagnostics,
		CreatedAt:       createdAt,
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot.Normalize(), nil
}

// ResolvedEnvelope decodes the structured JSON envelope stored with the snapshot.
func (s Snapshot) ResolvedEnvelope() (SnapshotEnvelope, error) {
	normalized := s.Normalize()
	var envelope SnapshotEnvelope
	if err := json.Unmarshal(normalized.ResolvedJSON, &envelope); err != nil {
		return SnapshotEnvelope{}, fmt.Errorf("%w: decode resolved_json: %w", ErrInvalidSnapshot, err)
	}
	if envelope.SchemaVersion != snapshotSchemaVersion {
		return SnapshotEnvelope{}, fmt.Errorf(
			"%w: unsupported resolved schema version %d",
			ErrInvalidSnapshot,
			envelope.SchemaVersion,
		)
	}
	return envelope, nil
}

// DiagnosticsJSON encodes redacted validation diagnostics for snapshot and revision storage.
func DiagnosticsJSON(diagnostics []Diagnostic) (json.RawMessage, error) {
	encoded, err := json.Marshal(cloneDiagnostics(diagnostics))
	if err != nil {
		return nil, fmt.Errorf("heartbeat: marshal diagnostics: %w", err)
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
	s.ConfigDigest = strings.TrimSpace(s.ConfigDigest)
	if s.SchemaVersion == 0 {
		s.SchemaVersion = snapshotSchemaVersion
	}
	if len(s.FrontmatterJSON) == 0 {
		s.FrontmatterJSON = json.RawMessage(`{}`)
	}
	if len(s.ResolvedJSON) == 0 {
		s.ResolvedJSON = json.RawMessage(`{}`)
	}
	if len(s.DiagnosticsJSON) == 0 {
		s.DiagnosticsJSON = json.RawMessage(`[]`)
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
	case normalized.SchemaVersion <= 0:
		return fmt.Errorf("%w: schema version is required", ErrInvalidSnapshot)
	case normalized.Digest == "":
		return fmt.Errorf("%w: digest is required", ErrInvalidSnapshot)
	case normalized.ConfigDigest == "":
		return fmt.Errorf("%w: config digest is required", ErrInvalidSnapshot)
	case !json.Valid(normalized.FrontmatterJSON):
		return fmt.Errorf("%w: frontmatter_json must be valid JSON", ErrInvalidSnapshot)
	case !json.Valid(normalized.ResolvedJSON):
		return fmt.Errorf("%w: resolved_json must be valid JSON", ErrInvalidSnapshot)
	case !json.Valid(normalized.DiagnosticsJSON):
		return fmt.Errorf("%w: diagnostics_json must be valid JSON", ErrInvalidSnapshot)
	case normalized.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidSnapshot)
	}
	return nil
}

// Validate ensures the snapshot query uses sane bounds.
func (q SnapshotListQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("%w: invalid snapshot limit %d", ErrInvalidSnapshot, q.Limit)
	}
	return nil
}
