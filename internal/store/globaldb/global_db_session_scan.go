package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

type sessionInfoRow struct {
	session                store.SessionInfo
	name                   sql.NullString
	worktreeID             sql.NullString
	acpOptionsJSON         string
	speedResolutionJSON    string
	runtimeRecoveryJSON    string
	selectedProvider       string
	selectedModel          string
	selectedReasoning      string
	selectedSpeed          string
	selectedACPOptionsJSON string
	networkSpecJSON        string
	networkMode            string
	networkChannel         sql.NullString
	networkSource          string
	sessionType            string
	parentSessionID        sql.NullString
	rootSessionID          sql.NullString
	spawnDepth             int
	spawnRole              sql.NullString
	ttlExpiresAt           sql.NullString
	autoStopOnParent       bool
	notifyCreator          bool
	spawnBudgetJSON        string
	permissionPolicyJSON   string
	archivedAt             sql.NullString
	acpSessionID           sql.NullString
	stopReason             sql.NullString
	stopDetail             sql.NullString
	failureKind            sql.NullString
	failureSummary         string
	crashBundlePath        string
	subprocessPID          int
	subprocessStartedAt    sql.NullString
	lastUpdateAt           sql.NullString
	stallState             string
	stallReason            string
	activityJSON           string
	attachedTo             string
	attachExpiresAt        sql.NullString
	transcriptEpoch        int64
	pendingPermissionCount int
	pendingClarifyCount    int
	attentionRevision      int64
	lastSettledRevision    int64
	lastSeenRevision       int64
	lastSeenAt             sql.NullString
	attentionChangedAt     sql.NullString
	soulSnapshotID         sql.NullString
	soulDigest             string
	parentSoulDigest       string
	envID                  string
	envBackend             string
	envProfile             string
	envInstance            string
	envState               string
	envProviderStateJSON   string
	envLastSyncAt          sql.NullString
	envLastSyncError       string
	createdAtRaw           string
	updatedAtRaw           string
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
	session.Model = strings.TrimSpace(row.session.Model)
	session.ReasoningEffort = strings.TrimSpace(row.session.ReasoningEffort)
	session.Speed = speedpkg.Speed(strings.TrimSpace(string(row.session.Speed)))
	if err := applySessionACPOptionsScan(&session, &row); err != nil {
		return store.SessionInfo{}, err
	}
	session.RuntimeStatus = row.session.RuntimeStatus
	session.RuntimeTransition = row.session.RuntimeTransition
	session.RuntimeFailure = strings.TrimSpace(row.session.RuntimeFailure)
	session.RuntimeGeneration = row.session.RuntimeGeneration
	if err := applySessionRuntimeScan(&session, &row); err != nil {
		return store.SessionInfo{}, err
	}
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
	if worktreeID := store.NullString(row.worktreeID); worktreeID != nil {
		session.WorktreeID = *worktreeID
	}
	session.SessionType = store.NormalizeSessionType(row.sessionType)
	lineage, err := scanSessionLineage(
		session.ID,
		row.parentSessionID,
		row.rootSessionID,
		row.spawnDepth,
		row.spawnRole,
		row.ttlExpiresAt,
		row.autoStopOnParent,
		row.notifyCreator,
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
	if err := session.Validate(); err != nil {
		return store.SessionInfo{}, err
	}
	return session, nil
}

func applySessionRuntimeScan(session *store.SessionInfo, row *sessionInfoRow) error {
	if err := store.ValidateSessionRuntime(
		session.RuntimeStatus,
		session.RuntimeTransition,
		session.RuntimeFailure,
	); err != nil {
		return err
	}
	resolution, err := decodeSessionSpeedResolution(row.speedResolutionJSON)
	if err != nil {
		return err
	}
	session.SpeedResolution = resolution
	recovery, err := decodeSessionRuntimeRecovery(row.runtimeRecoveryJSON)
	if err != nil {
		return err
	}
	session.SetRuntimeRecovery(recovery)
	if err := store.ValidateSessionRuntimeRecovery(
		session.RuntimeStatus,
		session.RuntimeGeneration,
		session.RuntimeRecoveryValue(),
	); err != nil {
		return err
	}
	return nil
}

func decodeSessionRuntimeRecovery(raw string) (*store.SessionRuntimeRecovery, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var recovery store.SessionRuntimeRecovery
	if err := json.Unmarshal([]byte(trimmed), &recovery); err != nil {
		return nil, fmt.Errorf("store: decode session runtime recovery: %w", err)
	}
	return &recovery, nil
}

func decodeSessionSpeedResolution(raw string) (*speedpkg.Resolution, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var resolution speedpkg.Resolution
	if err := json.Unmarshal([]byte(trimmed), &resolution); err != nil {
		return nil, fmt.Errorf("store: decode session speed resolution: %w", err)
	}
	if err := speedpkg.ValidateResolution(resolution); err != nil {
		return nil, fmt.Errorf("store: validate session speed resolution: %w", err)
	}
	return &resolution, nil
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
	var attachExpiresAt *time.Time
	if row.attachExpiresAt.Valid && strings.TrimSpace(row.attachExpiresAt.String) != "" {
		parsed, parseErr := store.ParseTimestamp(row.attachExpiresAt.String)
		if parseErr != nil {
			return fmt.Errorf("store: parse session attach expires at: %w", parseErr)
		}
		attachExpiresAt = &parsed
	}
	session.SetAttach(strings.TrimSpace(row.attachedTo), attachExpiresAt)
	session.TranscriptEpoch = row.transcriptEpoch
	if err := populateSessionAttentionScanParts(session, row); err != nil {
		return err
	}
	if row.archivedAt.Valid && strings.TrimSpace(row.archivedAt.String) != "" {
		archivedAt, parseErr := store.ParseTimestamp(row.archivedAt.String)
		if parseErr != nil {
			return fmt.Errorf("store: parse session archived at: %w", parseErr)
		}
		session.ArchivedAt = &archivedAt
	}

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
		&row.session.ProfileID,
		&row.name,
		&row.session.AgentName,
		&row.session.Provider,
		&row.session.Model,
		&row.session.ReasoningEffort,
		&row.session.Speed,
		&row.acpOptionsJSON,
		&row.speedResolutionJSON,
		&row.session.RuntimeStatus,
		&row.session.RuntimeTransition,
		&row.session.RuntimeFailure,
		&row.session.RuntimeGeneration,
		&row.runtimeRecoveryJSON,
		&row.selectedProvider,
		&row.selectedModel,
		&row.selectedReasoning,
		&row.selectedSpeed,
		&row.selectedACPOptionsJSON,
		&row.session.RuntimeSelectionRevision,
		&row.session.WorkspaceID,
		&row.worktreeID,
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
		&row.notifyCreator,
		&row.spawnBudgetJSON,
		&row.permissionPolicyJSON,
		&row.session.State,
		&row.archivedAt,
		&row.acpSessionID,
		&row.stopReason,
		&row.session.StopEscalated, &row.session.StopVerificationFailed,
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
		&row.attachedTo, &row.attachExpiresAt,
		&row.transcriptEpoch,
		&row.pendingPermissionCount,
		&row.pendingClarifyCount,
		&row.attentionRevision,
		&row.lastSettledRevision,
		&row.lastSeenRevision,
		&row.lastSeenAt,
		&row.attentionChangedAt,
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
		&row.createdAtRaw, &row.updatedAtRaw,
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
	notifyCreator bool,
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
		NotifyCreator:    notifyCreator,
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
