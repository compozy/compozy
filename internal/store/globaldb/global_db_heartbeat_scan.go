package globaldb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
)

func scanHeartbeatSnapshot(scanner rowScanner) (heartbeat.Snapshot, error) {
	var (
		snapshot        heartbeat.Snapshot
		frontmatterJSON string
		resolvedJSON    string
		diagnosticsJSON string
		createdRaw      string
	)
	if err := scanner.Scan(
		&snapshot.ID,
		&snapshot.WorkspaceID,
		&snapshot.AgentName,
		&snapshot.SourcePath,
		&snapshot.SchemaVersion,
		&snapshot.Digest,
		&snapshot.ConfigDigest,
		&snapshot.Body,
		&frontmatterJSON,
		&resolvedJSON,
		&diagnosticsJSON,
		&createdRaw,
	); err != nil {
		return heartbeat.Snapshot{}, err
	}
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return heartbeat.Snapshot{}, fmt.Errorf("store: parse heartbeat snapshot created_at: %w", err)
	}
	snapshot.FrontmatterJSON = []byte(frontmatterJSON)
	snapshot.ResolvedJSON = []byte(resolvedJSON)
	snapshot.DiagnosticsJSON = []byte(diagnosticsJSON)
	snapshot.CreatedAt = createdAt
	if err := snapshot.Validate(); err != nil {
		return heartbeat.Snapshot{}, err
	}
	return snapshot.Normalize(), nil
}

func scanHeartbeatRevision(scanner rowScanner) (heartbeat.Revision, error) {
	var (
		revision       heartbeat.Revision
		operation      string
		previousDigest sql.NullString
		newDigest      sql.NullString
		newSnapshotID  sql.NullString
		body           sql.NullString
		actorKind      string
		createdRaw     string
	)
	if err := scanner.Scan(
		&revision.ID,
		&revision.WorkspaceID,
		&revision.AgentName,
		&revision.SourcePath,
		&operation,
		&previousDigest,
		&newDigest,
		&newSnapshotID,
		&body,
		&actorKind,
		&revision.ActorID,
		&createdRaw,
	); err != nil {
		return heartbeat.Revision{}, err
	}
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return heartbeat.Revision{}, fmt.Errorf("store: parse heartbeat revision created_at: %w", err)
	}
	revision.Operation = heartbeat.RevisionOperation(operation)
	revision.PreviousDigest = heartbeatNullStringValue(previousDigest)
	revision.NewDigest = heartbeatNullStringValue(newDigest)
	revision.NewSnapshotID = heartbeatNullStringValue(newSnapshotID)
	revision.Body = heartbeatNullTextValue(body)
	revision.ActorKind = heartbeat.ActorKind(actorKind)
	revision.CreatedAt = createdAt
	if err := revision.Validate(); err != nil {
		return heartbeat.Revision{}, err
	}
	return revision.Normalize(), nil
}

func scanSessionHealth(scanner rowScanner) (heartbeat.SessionHealth, error) {
	var (
		health              heartbeat.SessionHealth
		state               string
		healthStatus        string
		activePrompt        int
		attachable          int
		eligibleForWake     int
		ineligibilityReason sql.NullString
		lastActivityAt      sql.NullString
		lastPresenceAt      sql.NullString
		lastError           sql.NullString
		updatedRaw          string
	)
	if err := scanner.Scan(
		&health.SessionID,
		&health.WorkspaceID,
		&health.AgentName,
		&state,
		&healthStatus,
		&activePrompt,
		&attachable,
		&eligibleForWake,
		&ineligibilityReason,
		&lastActivityAt,
		&lastPresenceAt,
		&lastError,
		&updatedRaw,
	); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	if err := assignNullableHeartbeatTimestamp(&health.LastActivityAt, lastActivityAt); err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health last_activity_at: %w", err)
	}
	if err := assignNullableHeartbeatTimestamp(&health.LastPresenceAt, lastPresenceAt); err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health last_presence_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(updatedRaw)
	if err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health updated_at: %w", err)
	}
	health.State = heartbeat.SessionHealthState(state)
	health.Health = heartbeat.SessionHealthStatus(healthStatus)
	health.ActivePrompt = activePrompt != 0
	health.Attachable = attachable != 0
	health.EligibleForWake = eligibleForWake != 0
	health.IneligibilityReason = heartbeatNullStringValue(ineligibilityReason)
	health.LastError = heartbeatNullStringValue(lastError)
	health.UpdatedAt = updatedAt
	if err := health.Validate(); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	return health.Normalize(), nil
}

func scanHeartbeatWakeState(scanner rowScanner) (heartbeat.WakeState, error) {
	var (
		state            heartbeat.WakeState
		policySnapshotID sql.NullString
		lastWakeAt       sql.NullString
		nextAllowedAt    sql.NullString
		lastResult       string
		lastReason       sql.NullString
		updatedRaw       string
	)
	if err := scanner.Scan(
		&state.WorkspaceID,
		&state.AgentName,
		&state.SessionID,
		&policySnapshotID,
		&lastWakeAt,
		&nextAllowedAt,
		&state.CoalescedCount,
		&lastResult,
		&lastReason,
		&updatedRaw,
	); err != nil {
		return heartbeat.WakeState{}, err
	}
	if err := assignNullableHeartbeatTimestamp(&state.LastWakeAt, lastWakeAt); err != nil {
		return heartbeat.WakeState{}, fmt.Errorf("store: parse heartbeat wake state last_wake_at: %w", err)
	}
	if err := assignNullableHeartbeatTimestamp(&state.NextAllowedAt, nextAllowedAt); err != nil {
		return heartbeat.WakeState{}, fmt.Errorf("store: parse heartbeat wake state next_allowed_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(updatedRaw)
	if err != nil {
		return heartbeat.WakeState{}, fmt.Errorf("store: parse heartbeat wake state updated_at: %w", err)
	}
	state.PolicySnapshotID = heartbeatNullStringValue(policySnapshotID)
	state.LastResult = heartbeat.WakeResult(lastResult)
	state.LastReason = heartbeat.WakeReason(heartbeatNullStringValue(lastReason))
	state.UpdatedAt = updatedAt
	if err := state.Validate(); err != nil {
		return heartbeat.WakeState{}, err
	}
	return state.Normalize(), nil
}

func scanHeartbeatWakeEvent(scanner rowScanner) (heartbeat.WakeEvent, error) {
	var (
		event             heartbeat.WakeEvent
		sessionID         sql.NullString
		policySnapshotID  sql.NullString
		source            string
		result            string
		reason            string
		syntheticPromptID sql.NullString
		createdRaw        string
		expiresRaw        string
	)
	if err := scanner.Scan(
		&event.ID,
		&event.WorkspaceID,
		&event.AgentName,
		&sessionID,
		&policySnapshotID,
		&source,
		&result,
		&reason,
		&syntheticPromptID,
		&createdRaw,
		&expiresRaw,
	); err != nil {
		return heartbeat.WakeEvent{}, err
	}
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return heartbeat.WakeEvent{}, fmt.Errorf("store: parse heartbeat wake event created_at: %w", err)
	}
	expiresAt, err := store.ParseTimestamp(expiresRaw)
	if err != nil {
		return heartbeat.WakeEvent{}, fmt.Errorf("store: parse heartbeat wake event expires_at: %w", err)
	}
	event.SessionID = heartbeatNullStringValue(sessionID)
	event.PolicySnapshotID = heartbeatNullStringValue(policySnapshotID)
	event.Source = heartbeat.WakeSource(source)
	event.Result = heartbeat.WakeResult(result)
	event.Reason = heartbeat.WakeReason(reason)
	event.SyntheticPromptID = heartbeatNullStringValue(syntheticPromptID)
	event.CreatedAt = createdAt
	event.ExpiresAt = expiresAt
	if err := event.Validate(); err != nil {
		return heartbeat.WakeEvent{}, err
	}
	return event.Normalize(), nil
}

func heartbeatBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableHeartbeatTimestamp(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func assignNullableHeartbeatTimestamp(target *time.Time, raw sql.NullString) error {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil
	}
	parsed, err := store.ParseTimestamp(raw.String)
	if err != nil {
		return err
	}
	*target = parsed
	return nil
}

func heartbeatNullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func heartbeatNullTextValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
