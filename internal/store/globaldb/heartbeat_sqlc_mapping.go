package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func heartbeatSnapshotFromGenerated(row sqlcgen.AgentHeartbeatSnapshot) (heartbeat.Snapshot, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return heartbeat.Snapshot{}, fmt.Errorf("store: parse heartbeat snapshot created_at: %w", err)
	}
	snapshot := heartbeat.Snapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, AgentName: row.AgentName, SourcePath: row.SourcePath,
		SchemaVersion: int(row.SchemaVersion), Digest: row.Digest, ConfigDigest: row.ConfigDigest,
		Body: row.Body, FrontmatterJSON: []byte(row.FrontmatterJson), ResolvedJSON: []byte(row.ResolvedJson),
		DiagnosticsJSON: []byte(row.DiagnosticsJson), CreatedAt: createdAt,
	}
	if err := snapshot.Validate(); err != nil {
		return heartbeat.Snapshot{}, err
	}
	return snapshot.Normalize(), nil
}

func heartbeatRevisionFromGenerated(row sqlcgen.AgentHeartbeatRevision) (heartbeat.Revision, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return heartbeat.Revision{}, fmt.Errorf("store: parse heartbeat revision created_at: %w", err)
	}
	revision := heartbeat.Revision{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		AgentName:   row.AgentName,
		SourcePath:  row.SourcePath,
		Operation: heartbeat.RevisionOperation(
			row.Operation,
		),
		PreviousDigest: heartbeatNullStringValue(row.PreviousDigest),
		NewDigest:      heartbeatNullStringValue(row.NewDigest),
		NewSnapshotID:  heartbeatNullStringValue(row.NewSnapshotID),
		Body:           heartbeatNullTextValue(row.Body),
		ActorKind:      heartbeat.ActorKind(row.ActorKind),
		ActorID:        row.ActorID,
		CreatedAt:      createdAt,
	}
	if err := revision.Validate(); err != nil {
		return heartbeat.Revision{}, err
	}
	return revision.Normalize(), nil
}

func heartbeatSessionHealthFromGenerated(row sqlcgen.SessionHealth) (heartbeat.SessionHealth, error) {
	health := heartbeat.SessionHealth{
		SessionID: row.SessionID, WorkspaceID: row.WorkspaceID, AgentName: row.AgentName,
		State: heartbeat.SessionHealthState(row.State), Health: heartbeat.SessionHealthStatus(row.Health),
		ActivePrompt: row.ActivePrompt, Attachable: row.Attachable, EligibleForWake: row.EligibleForWake,
		IneligibilityReason: heartbeatNullStringValue(row.IneligibilityReason),
		LastError:           heartbeatNullStringValue(row.LastError),
	}
	if err := assignNullableHeartbeatTimestamp(&health.LastActivityAt, row.LastActivityAt); err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health last_activity_at: %w", err)
	}
	if err := assignNullableHeartbeatTimestamp(&health.LastPresenceAt, row.LastPresenceAt); err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health last_presence_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health updated_at: %w", err)
	}
	health.UpdatedAt = updatedAt
	if err := health.Validate(); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	return health.Normalize(), nil
}

func heartbeatWakeStateFromGenerated(row sqlcgen.AgentHeartbeatWakeState) (heartbeat.WakeState, error) {
	state := heartbeat.WakeState{
		WorkspaceID: row.WorkspaceID, AgentName: row.AgentName, SessionID: row.SessionID,
		PolicySnapshotID: heartbeatNullStringValue(row.PolicySnapshotID), CoalescedCount: int(row.CoalescedCount),
		LastResult: heartbeat.WakeResult(row.LastResult),
		LastReason: heartbeat.WakeReason(heartbeatNullStringValue(row.LastReason)),
	}
	if err := assignNullableHeartbeatTimestamp(&state.LastWakeAt, row.LastWakeAt); err != nil {
		return heartbeat.WakeState{}, fmt.Errorf("store: parse heartbeat wake state last_wake_at: %w", err)
	}
	if err := assignNullableHeartbeatTimestamp(&state.NextAllowedAt, row.NextAllowedAt); err != nil {
		return heartbeat.WakeState{}, fmt.Errorf("store: parse heartbeat wake state next_allowed_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return heartbeat.WakeState{}, fmt.Errorf("store: parse heartbeat wake state updated_at: %w", err)
	}
	state.UpdatedAt = updatedAt
	if err := state.Validate(); err != nil {
		return heartbeat.WakeState{}, err
	}
	return state.Normalize(), nil
}

func heartbeatWakeEventFromGenerated(row sqlcgen.AgentHeartbeatWakeEvent) (heartbeat.WakeEvent, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return heartbeat.WakeEvent{}, fmt.Errorf("store: parse heartbeat wake event created_at: %w", err)
	}
	expiresAt, err := store.ParseTimestamp(row.ExpiresAt)
	if err != nil {
		return heartbeat.WakeEvent{}, fmt.Errorf("store: parse heartbeat wake event expires_at: %w", err)
	}
	event := heartbeat.WakeEvent{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		AgentName:   row.AgentName,
		SessionID: heartbeatNullStringValue(
			row.SessionID,
		),
		PolicySnapshotID:  heartbeatNullStringValue(row.PolicySnapshotID),
		Source:            heartbeat.WakeSource(row.Source),
		Result:            heartbeat.WakeResult(row.Result),
		Reason:            heartbeat.WakeReason(row.Reason),
		SyntheticPromptID: heartbeatNullStringValue(row.SyntheticPromptID),
		CreatedAt:         createdAt,
		ExpiresAt:         expiresAt,
	}
	if err := event.Validate(); err != nil {
		return heartbeat.WakeEvent{}, err
	}
	return event.Normalize(), nil
}

func nullableHeartbeatString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func nullableHeartbeatText(value string) sql.NullString {
	return sql.NullString{String: value, Valid: strings.TrimSpace(value) != ""}
}
