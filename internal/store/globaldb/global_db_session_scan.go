package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

type sessionInfoRow struct {
	session              store.SessionInfo
	name                 sql.NullString
	networkSpecJSON      string
	networkMode          string
	networkChannel       sql.NullString
	networkSource        string
	sessionType          string
	parentSessionID      sql.NullString
	rootSessionID        sql.NullString
	spawnDepth           int
	spawnRole            sql.NullString
	ttlExpiresAt         sql.NullString
	autoStopOnParent     bool
	spawnBudgetJSON      string
	permissionPolicyJSON string
	acpSessionID         sql.NullString
	stopReason           sql.NullString
	stopDetail           sql.NullString
	failureKind          sql.NullString
	failureSummary       string
	crashBundlePath      string
	subprocessPID        int
	subprocessStartedAt  sql.NullString
	lastUpdateAt         sql.NullString
	stallState           string
	stallReason          string
	activityJSON         string
	attachedTo           string
	attachExpiresAt      sql.NullString
	transcriptEpoch      int64
	soulSnapshotID       sql.NullString
	soulDigest           string
	parentSoulDigest     string
	envID                string
	envBackend           string
	envProfile           string
	envInstance          string
	envState             string
	envProviderStateJSON string
	envLastSyncAt        sql.NullString
	envLastSyncError     string
	createdAtRaw         string
	updatedAtRaw         string
}

func scanSessionInfo(scanner rowScanner) (store.SessionInfo, error) {
	row, err := scanSessionInfoRow(scanner)
	if err != nil {
		return store.SessionInfo{}, err
	}

	session := row.session
	if row.name.Valid {
		session.Name = row.name.String
	}
	session.Provider = strings.TrimSpace(row.session.Provider)
	networkSpec, err := decodeParticipationSnapshot(
		session.WorkspaceID,
		row.networkSpecJSON,
		row.networkMode,
		row.networkChannel,
		row.networkSource,
	)
	if err != nil {
		return store.SessionInfo{}, err
	}
	session.SetNetworkSpec(networkSpec)
	session.SessionType = store.NormalizeSessionType(row.sessionType)
	lineage, err := scanSessionLineage(
		session.ID,
		row.parentSessionID,
		row.rootSessionID,
		row.spawnDepth,
		row.spawnRole,
		row.ttlExpiresAt,
		row.autoStopOnParent,
		row.spawnBudgetJSON,
		row.permissionPolicyJSON,
	)
	if err != nil {
		return store.SessionInfo{}, err
	}
	session.Lineage = lineage
	session.ACPSessionID = store.NullString(row.acpSessionID)
	if soulSnapshotID := store.NullString(row.soulSnapshotID); soulSnapshotID != nil {
		session.SoulSnapshotID = *soulSnapshotID
	}
	session.SoulDigest = strings.TrimSpace(row.soulDigest)
	session.ParentSoulDigest = strings.TrimSpace(row.parentSoulDigest)
	if reason := store.NullString(row.stopReason); reason != nil {
		session.StopReason = store.StopReason(*reason)
	}
	if detail := store.NullString(row.stopDetail); detail != nil {
		session.StopDetail = *detail
	}
	if err := populateSessionScanParts(&session, &row); err != nil {
		return store.SessionInfo{}, err
	}
	return session, nil
}

func populateSessionScanParts(session *store.SessionInfo, row *sessionInfoRow) error {
	failure := store.SessionFailure{
		Summary:         strings.TrimSpace(row.failureSummary),
		CrashBundlePath: strings.TrimSpace(row.crashBundlePath),
	}
	if kind := store.NullString(row.failureKind); kind != nil {
		failure.Kind = store.FailureKind(*kind)
	}
	if !failure.IsZero() {
		if err := failure.Validate(); err != nil {
			return err
		}
		session.Failure = &failure
	}
	liveness, err := scanSessionLiveness(
		row.subprocessPID,
		row.subprocessStartedAt,
		row.lastUpdateAt,
		row.stallState,
		row.stallReason,
		row.activityJSON,
	)
	if err != nil {
		return err
	}
	session.Liveness = liveness
	sandbox, err := scanSessionSandbox(
		row.envID,
		row.envBackend,
		row.envProfile,
		row.envInstance,
		row.envState,
		row.envProviderStateJSON,
		row.envLastSyncAt,
		row.envLastSyncError,
	)
	if err != nil {
		return err
	}
	session.Sandbox = sandbox
	session.AttachedTo = strings.TrimSpace(row.attachedTo)
	if row.attachExpiresAt.Valid && strings.TrimSpace(row.attachExpiresAt.String) != "" {
		attachExpiresAt, parseErr := store.ParseTimestamp(row.attachExpiresAt.String)
		if parseErr != nil {
			return fmt.Errorf("store: parse session attach expires at: %w", parseErr)
		}
		session.AttachExpiresAt = &attachExpiresAt
	}
	session.TranscriptEpoch = row.transcriptEpoch

	createdAt, updatedAt, err := parseSessionInfoTimestamps(row.createdAtRaw, row.updatedAtRaw)
	if err != nil {
		return err
	}
	session.CreatedAt = createdAt
	session.UpdatedAt = updatedAt
	return nil
}

func scanSessionInfoRow(scanner rowScanner) (sessionInfoRow, error) {
	var row sessionInfoRow
	if err := scanner.Scan(
		&row.session.ID,
		&row.name,
		&row.session.AgentName,
		&row.session.Provider,
		&row.session.WorkspaceID,
		&row.networkSpecJSON,
		&row.networkMode,
		&row.networkChannel,
		&row.networkSource,
		&row.sessionType,
		&row.parentSessionID,
		&row.rootSessionID,
		&row.spawnDepth,
		&row.spawnRole,
		&row.ttlExpiresAt,
		&row.autoStopOnParent,
		&row.spawnBudgetJSON,
		&row.permissionPolicyJSON,
		&row.session.State,
		&row.acpSessionID,
		&row.stopReason,
		&row.stopDetail,
		&row.failureKind,
		&row.failureSummary,
		&row.crashBundlePath,
		&row.subprocessPID,
		&row.subprocessStartedAt,
		&row.lastUpdateAt,
		&row.stallState,
		&row.stallReason,
		&row.activityJSON,
		&row.attachedTo,
		&row.attachExpiresAt,
		&row.transcriptEpoch,
		&row.soulSnapshotID,
		&row.soulDigest,
		&row.parentSoulDigest,
		&row.envID,
		&row.envBackend,
		&row.envProfile,
		&row.envInstance,
		&row.envState,
		&row.envProviderStateJSON,
		&row.envLastSyncAt,
		&row.envLastSyncError,
		&row.createdAtRaw,
		&row.updatedAtRaw,
	); err != nil {
		return sessionInfoRow{}, fmt.Errorf("store: scan session info: %w", err)
	}
	return row, nil
}

func scanSessionSandbox(
	sandboxID string,
	backend string,
	profile string,
	instanceID string,
	state string,
	providerStateJSON string,
	lastSyncAt sql.NullString,
	lastSyncError string,
) (*store.SessionSandboxMeta, error) {
	sandboxID = strings.TrimSpace(sandboxID)
	backend = strings.TrimSpace(backend)
	profile = strings.TrimSpace(profile)
	instanceID = strings.TrimSpace(instanceID)
	state = strings.TrimSpace(state)
	providerStateJSON = strings.TrimSpace(providerStateJSON)
	if sandboxID == "" &&
		backend == "" &&
		profile == "" &&
		instanceID == "" &&
		state == "" &&
		providerStateJSON == "" {
		return nil, nil
	}

	meta := &store.SessionSandboxMeta{
		SandboxID:  sandboxID,
		Backend:    backend,
		Profile:    profile,
		InstanceID: instanceID,
		State:      state,
	}
	if providerStateJSON != "" {
		meta.ProviderState = []byte(providerStateJSON)
	}
	if lastSyncAt.Valid && strings.TrimSpace(lastSyncAt.String) != "" {
		parsed, err := store.ParseTimestamp(lastSyncAt.String)
		if err != nil {
			return nil, fmt.Errorf("store: parse session sandbox last sync at: %w", err)
		}
		meta.LastSyncAt = &parsed
	}
	meta.LastSyncError = strings.TrimSpace(lastSyncError)
	return meta, nil
}

func scanSessionLineage(
	sessionID string,
	parentSessionID sql.NullString,
	rootSessionID sql.NullString,
	spawnDepth int,
	spawnRole sql.NullString,
	ttlExpiresAt sql.NullString,
	autoStopOnParent bool,
	spawnBudgetJSON string,
	permissionPolicyJSON string,
) (*store.SessionLineage, error) {
	budget, err := store.DecodeSessionSpawnBudget(spawnBudgetJSON)
	if err != nil {
		return nil, err
	}
	policy, err := store.DecodeSessionPermissionPolicy(permissionPolicyJSON)
	if err != nil {
		return nil, err
	}
	lineage := &store.SessionLineage{
		ParentSessionID:  sessionNullString(parentSessionID),
		RootSessionID:    sessionNullString(rootSessionID),
		SpawnDepth:       spawnDepth,
		SpawnRole:        sessionNullString(spawnRole),
		AutoStopOnParent: autoStopOnParent,
		SpawnBudget:      budget,
		PermissionPolicy: policy,
	}
	if ttlExpiresAt.Valid && strings.TrimSpace(ttlExpiresAt.String) != "" {
		parsed, parseErr := store.ParseTimestamp(ttlExpiresAt.String)
		if parseErr != nil {
			return nil, fmt.Errorf("store: parse session ttl expires at: %w", parseErr)
		}
		lineage.TTLExpiresAt = &parsed
	}
	normalized := store.NormalizeSessionLineage(sessionID, lineage)
	if err := store.ValidateSessionLineage(sessionID, normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func sessionNullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func parseSessionInfoTimestamps(createdAtRaw string, updatedAtRaw string) (time.Time, time.Time, error) {
	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return createdAt, updatedAt, nil
}

func scanSessionLiveness(
	subprocessPID int,
	subprocessStartedAt sql.NullString,
	lastUpdateAt sql.NullString,
	stallState string,
	stallReason string,
	activityJSON string,
) (*store.SessionLivenessMeta, error) {
	if subprocessPID <= 0 &&
		(!subprocessStartedAt.Valid || strings.TrimSpace(subprocessStartedAt.String) == "") &&
		(!lastUpdateAt.Valid || strings.TrimSpace(lastUpdateAt.String) == "") &&
		strings.TrimSpace(stallState) == "" &&
		strings.TrimSpace(stallReason) == "" &&
		strings.TrimSpace(activityJSON) == "" {
		return nil, nil
	}

	meta := &store.SessionLivenessMeta{
		SubprocessPID: subprocessPID,
		StallState:    strings.TrimSpace(stallState),
		StallReason:   strings.TrimSpace(stallReason),
	}
	if subprocessStartedAt.Valid && strings.TrimSpace(subprocessStartedAt.String) != "" {
		parsed, err := store.ParseTimestamp(subprocessStartedAt.String)
		if err != nil {
			return nil, fmt.Errorf("store: parse session subprocess started at: %w", err)
		}
		meta.SubprocessStartedAt = &parsed
	}
	if lastUpdateAt.Valid && strings.TrimSpace(lastUpdateAt.String) != "" {
		parsed, err := store.ParseTimestamp(lastUpdateAt.String)
		if err != nil {
			return nil, fmt.Errorf("store: parse session last update at: %w", err)
		}
		meta.LastUpdateAt = &parsed
	}
	if strings.TrimSpace(activityJSON) != "" {
		activity, err := parseSessionActivityJSON(activityJSON)
		if err != nil {
			return nil, err
		}
		meta.Activity = activity
	}
	if err := meta.Validate(); err != nil {
		return nil, err
	}
	return meta, nil
}

func parseSessionActivityJSON(raw string) (*store.SessionActivityMeta, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var activity store.SessionActivityMeta
	if err := json.Unmarshal([]byte(trimmed), &activity); err != nil {
		return nil, fmt.Errorf("store: parse session activity json: %w", err)
	}
	if err := activity.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate session activity json: %w", err)
	}
	return store.CloneSessionActivityMeta(&activity), nil
}
