package globaldb

import (
	"database/sql"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func upsertSessionParams(record sessionCatalogRecord) (sqlcgen.UpsertSessionParams, error) {
	session := record.session
	lineage := record.lineage
	network, err := encodeParticipationSnapshot(session.WorkspaceID, session.NetworkSpecSnapshot())
	if err != nil {
		return sqlcgen.UpsertSessionParams{}, err
	}
	return sqlcgen.UpsertSessionParams{
		ID:              session.ID,
		Name:            nullableSessionString(session.Name),
		AgentName:       session.AgentName,
		Provider:        strings.TrimSpace(session.Provider),
		WorkspaceID:     session.WorkspaceID,
		SessionType:     store.NormalizeSessionType(session.SessionType),
		NetworkSpecJson: network.JSON,
		NetworkMode:     network.Mode,
		NetworkChannel:  network.Channel,
		NetworkSource:   network.Source,
		State:           session.State,
		ParentSessionID: nullableSessionString(lineage.ParentSessionID),
		RootSessionID:   nullableSessionString(lineage.RootSessionID),
		SpawnDepth:      int64(lineage.SpawnDepth),
		SpawnRole: nullableSessionString(
			lineage.SpawnRole,
		),
		TtlExpiresAt:         nullableSessionTimePointer(lineage.TTLExpiresAt),
		AutoStopOnParent:     lineage.AutoStopOnParent,
		SpawnBudgetJson:      record.spawnBudgetJSON,
		PermissionPolicyJson: record.permissionPolicyJSON,
		AcpSessionID:         nullableSessionStringPointer(session.ACPSessionID),
		StopReason: nullableSessionString(
			string(session.StopReason),
		),
		StopDetail: nullableSessionString(session.StopDetail),
		FailureKind: nullableSessionFailureKind(
			session.Failure,
		),
		FailureSummary: sessionFailureSummary(session.Failure),
		CrashBundlePath: sessionCrashBundlePath(
			session.Failure,
		),
		SubprocessPid:       int64(sessionLivenessPID(session.Liveness)),
		SubprocessStartedAt: nullableSessionLivenessStartedAt(session.Liveness),
		LastUpdateAt:        nullableSessionLivenessLastUpdateAt(session.Liveness),
		StallState: sessionLivenessStallState(
			session.Liveness,
		),
		StallReason:     sessionLivenessStallReason(session.Liveness),
		ActivityJson:    record.activityJSON,
		TranscriptEpoch: session.TranscriptEpoch,
		SoulSnapshotID: nullableSessionString(
			session.SoulSnapshotID,
		),
		SoulDigest:       strings.TrimSpace(session.SoulDigest),
		ParentSoulDigest: strings.TrimSpace(session.ParentSoulDigest),
		SandboxID:        sessionSandboxID(session.Sandbox),
		SandboxBackend:   sessionSandboxBackend(session.Sandbox),
		SandboxProfile:   sessionSandboxProfile(session.Sandbox),
		SandboxInstanceID: sessionSandboxInstanceID(
			session.Sandbox,
		),
		SandboxState:             sessionSandboxState(session.Sandbox),
		SandboxProviderStateJson: sessionSandboxProviderStateJSON(session.Sandbox),
		SandboxLastSyncAt:        nullableSessionSandboxLastSyncAt(session.Sandbox),
		SandboxLastSyncError:     sessionSandboxLastSyncError(session.Sandbox),
		CreatedAt: store.FormatTimestamp(
			session.CreatedAt,
		),
		UpdatedAt: store.FormatTimestamp(session.UpdatedAt),
	}, nil
}

func nullableSessionString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func nullableSessionStringPointer(value *string) sql.NullString {
	return store.SQLNullStringPointer(value)
}

func nullableSessionTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func nullableSessionTimePointer(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return nullableSessionTime(*value)
}

func nullableSessionFailureKind(failure *store.SessionFailure) sql.NullString {
	if failure == nil {
		return sql.NullString{}
	}
	return nullableSessionString(string(failure.Normalize().Kind))
}

func nullableSessionLivenessStartedAt(meta *store.SessionLivenessMeta) sql.NullString {
	if meta == nil || meta.SubprocessStartedAt == nil {
		return sql.NullString{}
	}
	return nullableSessionTime(*meta.SubprocessStartedAt)
}

func nullableSessionLivenessLastUpdateAt(meta *store.SessionLivenessMeta) sql.NullString {
	if meta == nil || meta.LastUpdateAt == nil {
		return sql.NullString{}
	}
	return nullableSessionTime(*meta.LastUpdateAt)
}

func nullableSessionSandboxLastSyncAt(meta *store.SessionSandboxMeta) sql.NullString {
	if meta == nil || meta.LastSyncAt == nil {
		return sql.NullString{}
	}
	return nullableSessionTime(*meta.LastSyncAt)
}
