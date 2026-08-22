package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func upsertSessionParams(record *sessionCatalogRecord) (sqlcgen.UpsertSessionParams, error) {
	session := record.session
	speedResolutionJSON, network, err := sessionCatalogRuntimeProjections(session)
	if err != nil {
		return sqlcgen.UpsertSessionParams{}, err
	}
	runtimeRecoveryJSON, err := encodeSessionRuntimeRecovery(session.RuntimeRecovery)
	if err != nil {
		return sqlcgen.UpsertSessionParams{}, err
	}
	params := sqlcgen.UpsertSessionParams{
		ID:                       session.ID,
		Name:                     nullableSessionString(session.Name),
		AgentName:                session.AgentName,
		Provider:                 strings.TrimSpace(session.Provider),
		Model:                    strings.TrimSpace(session.Model),
		ReasoningEffort:          strings.TrimSpace(session.ReasoningEffort),
		Speed:                    string(session.Speed),
		SpeedResolutionJson:      speedResolutionJSON,
		RuntimeStatus:            string(session.RuntimeStatus),
		RuntimeTransition:        string(session.RuntimeTransition),
		RuntimeFailure:           strings.TrimSpace(session.RuntimeFailure),
		RuntimeGeneration:        session.RuntimeGeneration,
		RuntimeRecoveryJson:      runtimeRecoveryJSON,
		SelectedProvider:         selectedRuntimeProvider(session.SelectedRuntime),
		SelectedModel:            selectedRuntimeModel(session.SelectedRuntime),
		SelectedReasoningEffort:  selectedRuntimeReasoningEffort(session.SelectedRuntime),
		SelectedSpeed:            selectedRuntimeSpeed(session.SelectedRuntime),
		RuntimeSelectionRevision: session.RuntimeSelectionRevision,
		WorkspaceID:              session.WorkspaceID,
		WorktreeID:               nullableSessionString(session.WorktreeID),
		SessionType:              store.NormalizeSessionType(session.SessionType),
		NetworkSpecJson:          network.JSON,
		NetworkMode:              network.Mode,
		NetworkChannel:           network.Channel,
		NetworkSource:            network.Source,
		State:                    session.State,
		AcpSessionID:             nullableSessionStringPointer(session.ACPSessionID),
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
	}
	applySessionCatalogLineage(&params, record)
	return params, nil
}

func applySessionCatalogLineage(params *sqlcgen.UpsertSessionParams, record *sessionCatalogRecord) {
	lineage := record.lineage
	params.ParentSessionID = nullableSessionString(lineage.ParentSessionID)
	params.RootSessionID = nullableSessionString(lineage.RootSessionID)
	params.SpawnDepth = int64(lineage.SpawnDepth)
	params.SpawnRole = nullableSessionString(lineage.SpawnRole)
	params.TtlExpiresAt = nullableSessionTimePointer(lineage.TTLExpiresAt)
	params.AutoStopOnParent = lineage.AutoStopOnParent
	params.NotifyCreator = lineage.NotifyCreator
	params.SpawnBudgetJson = record.spawnBudgetJSON
	params.PermissionPolicyJson = record.permissionPolicyJSON
}

func sessionCatalogRuntimeProjections(
	session store.SessionInfo,
) (string, participationSnapshotFields, error) {
	speedResolutionJSON, err := encodeSessionSpeedResolution(session.SpeedResolution)
	if err != nil {
		return "", participationSnapshotFields{}, err
	}
	network, err := encodeParticipationSnapshot(session.WorkspaceID, session.NetworkSpecSnapshot())
	if err != nil {
		return "", participationSnapshotFields{}, err
	}
	return speedResolutionJSON, network, nil
}

func encodeSessionSpeedResolution(value *speedpkg.Resolution) (string, error) {
	if value == nil {
		return "", nil
	}
	if err := speedpkg.ValidateResolution(*value); err != nil {
		return "", fmt.Errorf("store: validate session speed resolution: %w", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("store: encode session speed resolution: %w", err)
	}
	return string(encoded), nil
}

func encodeSessionRuntimeRecovery(value *store.SessionRuntimeRecovery) (string, error) {
	if value == nil {
		return "", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("store: encode session runtime recovery: %w", err)
	}
	return string(encoded), nil
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
