package core

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/network/participation"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/store"

	"github.com/compozy/agh/internal/workref"
)

const (
	maxDiagnosticPayloadBytes = 2048
)

// SessionPayloadFromInfo converts a session info snapshot into the shared session payload.
func SessionPayloadFromInfo(info *session.Info) contract.SessionPayload {
	return sessionPayloadFromInfoAt(info, time.Now().UTC())
}

func sessionPayloadFromInfoAt(info *session.Info, now time.Time) contract.SessionPayload {
	payload := contract.SessionPayload{}
	if info == nil {
		return payload
	}

	ref := workref.NewPath(info.WorkspaceID, info.Workspace)
	payload = contract.SessionPayload{
		ID:                           info.ID,
		Name:                         info.Name,
		AgentName:                    info.AgentName,
		Provider:                     info.Provider,
		Model:                        strings.TrimSpace(info.Model),
		ReasoningEffort:              contract.ReasoningEffort(strings.TrimSpace(info.ReasoningEffort)),
		WorkspaceID:                  ref.WorkspaceID,
		WorkspacePath:                ref.WorkspacePath,
		ResolvedNetworkParticipation: participation.CloneSpec(info.NetworkParticipation),
		Type:                         info.Type,
		State:                        info.State,
		Badge:                        session.BadgeForInfo(info),
		Attachable:                   session.AttachableForInfo(info, now),
		AttachedTo:                   strings.TrimSpace(info.AttachedTo),
		AttachExpiresAt:              cloneTimePtr(info.AttachExpiresAt),
		TranscriptEpoch:              info.TranscriptEpoch,
		StopReason:                   info.StopReason,
		StopDetail:                   info.StopDetail,
		Failure:                      SessionFailurePayloadFromStore(info.Failure),
		ACPSessionID:                 info.ACPSessionID,
		AvailableCommands:            availableCommandPayloads(info.AdvertisedCommands),
		Lineage:                      contract.SessionLineagePayloadFromStore(info.Lineage),
		CreatedAt:                    info.CreatedAt,
		UpdatedAt:                    info.UpdatedAt,
	}
	if caps := ACPCapsPayloadFromInfo(info.ACPCaps); caps != nil {
		payload.ACPCaps = caps
	}
	if activity := RuntimeActivityPayloadFromSessionMeta(info.Liveness, now); activity != nil {
		payload.Activity = activity
	}
	if sandbox := SessionSandboxPayloadFromMeta(info.Sandbox); sandbox != nil {
		payload.Sandbox = sandbox
	}
	return payload
}

// SessionPayloadFromStoreInfo converts the persisted session index row into the shared payload.
func SessionPayloadFromStoreInfo(info store.SessionInfo) contract.SessionPayload {
	state := session.State(strings.TrimSpace(info.State))
	converted := &session.Info{
		ID:                   strings.TrimSpace(info.ID),
		Name:                 strings.TrimSpace(info.Name),
		AgentName:            strings.TrimSpace(info.AgentName),
		Provider:             strings.TrimSpace(info.Provider),
		WorkspaceID:          strings.TrimSpace(info.WorkspaceID),
		NetworkParticipation: info.NetworkSpecSnapshot(),
		Type:                 session.Type(strings.TrimSpace(info.SessionType)),
		State:                state,
		StopReason:           info.StopReason,
		StopDetail:           strings.TrimSpace(info.StopDetail),
		Failure:              store.CloneSessionFailure(info.Failure),
		ACPSessionID:         stringPointerValue(info.ACPSessionID),
		Lineage:              store.NormalizeSessionLineage(info.ID, info.Lineage),
		Liveness:             store.CloneSessionLivenessMeta(info.Liveness),
		Sandbox:              cloneStoreSessionSandboxMeta(info.Sandbox),
		SoulSnapshotID:       strings.TrimSpace(info.SoulSnapshotID),
		SoulDigest:           strings.TrimSpace(info.SoulDigest),
		ParentSoulDigest:     strings.TrimSpace(info.ParentSoulDigest),
		AttachedTo:           strings.TrimSpace(info.AttachedTo),
		AttachExpiresAt:      cloneTimePtr(info.AttachExpiresAt),
		TranscriptEpoch:      info.TranscriptEpoch,
		CreatedAt:            info.CreatedAt,
		UpdatedAt:            info.UpdatedAt,
	}
	return SessionPayloadFromInfo(converted)
}

func visibleSessionInfosInternal(infos []*session.Info) []*session.Info {
	visible := make([]*session.Info, 0, len(infos))
	for _, info := range infos {
		if info == nil || isInternalSessionInfo(info.Type, info.Lineage) {
			continue
		}
		visible = append(visible, info)
	}
	return visible
}

func isInternalSessionInfo(sessionType session.Type, lineage *store.SessionLineage) bool {
	if sessionType == session.SessionTypeDream {
		return true
	}
	normalized := store.NormalizeSessionLineage("", lineage)
	return session.IsInternalSpawnRole(normalized.SpawnRole)
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func cloneStoreSessionSandboxMeta(meta *store.SessionSandboxMeta) *store.SessionSandboxMeta {
	if meta == nil {
		return nil
	}
	cloned := *meta
	cloned.RuntimeAdditionalDirs = append([]string(nil), meta.RuntimeAdditionalDirs...)
	cloned.ProviderState = append([]byte(nil), meta.ProviderState...)
	cloned.SSHAccessExpiresAt = cloneTimePtr(meta.SSHAccessExpiresAt)
	cloned.LastSyncAt = cloneTimePtr(meta.LastSyncAt)
	return &cloned
}

// RuntimeActivityPayloadFromSessionMeta converts persisted session activity metadata into the shared payload.
func RuntimeActivityPayloadFromSessionMeta(
	liveness *store.SessionLivenessMeta,
	now time.Time,
) *contract.RuntimeActivityPayload {
	if liveness == nil || liveness.Activity == nil {
		return nil
	}
	activity := store.CloneSessionActivityMeta(liveness.Activity)
	payload := &contract.RuntimeActivityPayload{
		TurnID:             activity.TurnID,
		TurnSource:         activity.TurnSource,
		TurnStartedAt:      cloneTimePtr(activity.TurnStartedAt),
		DeadlineAt:         nil,
		LastActivityAt:     cloneTimePtr(activity.LastActivityAt),
		LastActivityKind:   activity.LastActivityKind,
		LastActivityDetail: activity.LastActivityDetail,
		CurrentTool:        activity.CurrentTool,
		ToolCallID:         activity.ToolCallID,
		LastProgressAt:     cloneTimePtr(activity.LastProgressAt),
		IterationCurrent:   activity.IterationCurrent,
		IterationMax:       activity.IterationMax,
		IdleSeconds:        store.SessionActivityIdleSeconds(activity, now),
	}
	if !now.IsZero() && activity.TurnStartedAt != nil && !activity.TurnStartedAt.IsZero() {
		elapsed := now.UTC().Sub(activity.TurnStartedAt.UTC())
		if elapsed > 0 {
			payload.ElapsedSeconds = int64(elapsed.Seconds())
			payload.ElapsedMS = elapsed.Milliseconds()
		}
	}
	return payload
}

func runtimeActivityPayloadFromEvent(activity *acp.RuntimeActivity) *contract.RuntimeActivityPayload {
	if activity == nil {
		return nil
	}
	return &contract.RuntimeActivityPayload{
		TurnID:             strings.TrimSpace(activity.TurnID),
		TurnSource:         strings.TrimSpace(activity.TurnSource),
		TurnStartedAt:      cloneTimePtr(activity.TurnStartedAt),
		DeadlineAt:         cloneTimePtr(activity.DeadlineAt),
		LastActivityAt:     cloneTimePtr(activity.LastActivityAt),
		LastActivityKind:   strings.TrimSpace(activity.LastActivityKind),
		LastActivityDetail: strings.TrimSpace(activity.LastActivityDetail),
		CurrentTool:        strings.TrimSpace(activity.CurrentTool),
		ToolCallID:         strings.TrimSpace(activity.ToolCallID),
		LastProgressAt:     cloneTimePtr(activity.LastProgressAt),
		IterationCurrent:   activity.IterationCurrent,
		IterationMax:       activity.IterationMax,
		IdleSeconds:        activity.IdleSeconds,
		ElapsedSeconds:     activity.ElapsedSeconds,
		ElapsedMS:          activity.ElapsedMS,
	}
}

// SessionSandboxPayloadFromMeta converts session sandbox metadata into the shared payload.
func SessionSandboxPayloadFromMeta(meta *store.SessionSandboxMeta) *contract.SessionSandboxPayload {
	if meta == nil {
		return nil
	}
	return &contract.SessionSandboxPayload{
		SandboxID:     strings.TrimSpace(meta.SandboxID),
		Backend:       strings.TrimSpace(meta.Backend),
		Profile:       strings.TrimSpace(meta.Profile),
		State:         strings.TrimSpace(meta.State),
		InstanceID:    strings.TrimSpace(meta.InstanceID),
		LastSyncError: strings.TrimSpace(meta.LastSyncError),
	}
}

// SessionPayloadsFromInfos converts a session list into response payloads.
func SessionPayloadsFromInfos(infos []*session.Info) []contract.SessionPayload {
	payload := make([]contract.SessionPayload, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		payload = append(payload, SessionPayloadFromInfo(info))
	}
	return payload
}
