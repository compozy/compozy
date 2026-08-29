package core

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"

	"github.com/compozy/compozy/internal/network/participation"

	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"

	"github.com/compozy/compozy/internal/store"

	"github.com/compozy/compozy/internal/workref"
)

const (
	maxDiagnosticPayloadBytes = 2048
)

// SessionPayloadFromInfo converts a session info snapshot into the shared session payload.
func SessionPayloadFromInfo(info *session.Info) contract.SessionPayload {
	return sessionPayloadFromInfoAt(info, time.Now().UTC(), false)
}

// SessionPayloadForAgentFromInfo retains capability atoms for authorized native agent tools.
func SessionPayloadForAgentFromInfo(info *session.Info) contract.SessionPayload {
	return sessionPayloadFromInfoAt(info, time.Now().UTC(), true)
}

func sessionPayloadFromInfoAt(info *session.Info, now time.Time, includePermissionAtoms bool) contract.SessionPayload {
	payload := contract.SessionPayload{}
	if info == nil {
		return payload
	}

	ref := workref.NewPath(info.WorkspaceID, info.Workspace)
	lineage := contract.SessionLineagePayloadForOperatorFromStore(info.Lineage)
	if includePermissionAtoms {
		lineage = contract.SessionLineagePayloadFromStore(info.Lineage)
	}
	payload = contract.SessionPayload{
		ID:        info.ID,
		ProfileID: strings.TrimSpace(info.ProfileID),
		Name:      info.Name,
		AgentName: info.AgentName,
		Runtime: contract.SessionRuntimePayload{
			Status:            info.RuntimeStatus,
			Transition:        info.RuntimeTransition,
			Failure:           diagnostics.RedactAndBound(info.RuntimeFailure, maxDiagnosticPayloadBytes),
			Generation:        info.RuntimeGeneration,
			Recovery:          sessionRuntimeRecoveryPayload(info.RuntimeRecovery),
			Selected:          contract.PromptRuntimeSelectionPayloadFromSelection(info.SelectedRuntime),
			SelectionRevision: info.RuntimeSelectionRevision,
			ACPSessionID:      info.ACPSessionID,
		},
		WorkspaceID:                  ref.WorkspaceID,
		WorkspacePath:                ref.WorkspacePath,
		WorktreeID:                   strings.TrimSpace(info.WorktreeID),
		ResolvedNetworkParticipation: participation.CloneSpec(info.NetworkParticipation),
		Type:                         info.Type,
		State:                        info.State,
		ChildState:                   sessionChildState(info),
		Badge:                        session.BadgeForInfo(info),
		Attachable:                   session.AttachableForInfo(info, now),
		AttachedTo:                   strings.TrimSpace(info.AttachedTo),
		AttachExpiresAt:              cloneTimePtr(info.AttachExpiresAt),
		TranscriptEpoch:              info.TranscriptEpoch,
		AttentionChangedAt:           cloneTimePtr(info.AttentionChangedAt),
		PendingInteractions:          PendingInteractionPayloadsFromStore(info.PendingInteractions),
		ArchivedAt:                   cloneTimePtr(info.ArchivedAt),
		StopReason:                   info.StopReason,
		StopDetail:                   info.StopDetail,
		Failure:                      SessionFailurePayloadFromStore(info.Failure),
		AvailableCommands:            availableCommandPayloads(info.AdvertisedCommands),
		Lineage:                      lineage,
		CreatedAt:                    info.CreatedAt,
		UpdatedAt:                    info.UpdatedAt,
	}
	if runtime := runtimeSelectionPayloadFromInfo(info); runtime != nil {
		payload.Runtime.Effective = runtime
	}
	if caps := contract.ACPCapsPayloadFromACP(info.ACPCaps, info.ACPCapsKnown); caps != nil {
		payload.Runtime.ACPCaps = caps
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
func SessionPayloadFromStoreInfo(info *store.SessionInfo) contract.SessionPayload {
	state := session.State(strings.TrimSpace(info.State))
	attention := info.AttentionSnapshot()
	converted := &session.Info{
		ID:                       strings.TrimSpace(info.ID),
		ProfileID:                strings.TrimSpace(info.ProfileID),
		Name:                     strings.TrimSpace(info.Name),
		AgentName:                strings.TrimSpace(info.AgentName),
		RuntimeStatus:            info.RuntimeStatus,
		RuntimeTransition:        info.RuntimeTransition,
		RuntimeFailure:           strings.TrimSpace(info.RuntimeFailure),
		RuntimeGeneration:        info.RuntimeGeneration,
		RuntimeRecovery:          info.RuntimeRecoveryValue(),
		Provider:                 strings.TrimSpace(info.Provider),
		Model:                    strings.TrimSpace(info.Model),
		ReasoningEffort:          strings.TrimSpace(info.ReasoningEffort),
		Speed:                    info.Speed,
		ACPOptions:               session.ACPOptionSelectionsFromStore(info.ACPOptionsValue()),
		SpeedResolution:          speedpkg.CloneResolution(info.SpeedResolution),
		SelectedRuntime:          runtimeSelectionFromStoreInfo(info.SelectedRuntime),
		RuntimeSelectionRevision: info.RuntimeSelectionRevision,
		WorkspaceID:              strings.TrimSpace(info.WorkspaceID),
		WorktreeID:               strings.TrimSpace(info.WorktreeID),
		NetworkParticipation:     info.NetworkSpecSnapshot(),
		Type:                     session.Type(strings.TrimSpace(info.SessionType)),
		State:                    state,
		StopReason:               info.StopReason,
		StopDetail:               strings.TrimSpace(info.StopDetail),
		Failure:                  store.CloneSessionFailure(info.Failure),
		ACPSessionID:             stringPointerValue(info.ACPSessionID),
		Lineage:                  store.NormalizeSessionLineage(info.ID, info.Lineage),
		Liveness:                 store.CloneSessionLivenessMeta(info.Liveness),
		Sandbox:                  cloneStoreSessionSandboxMeta(info.Sandbox),
		SoulSnapshotID:           strings.TrimSpace(info.SoulSnapshotID),
		SoulDigest:               strings.TrimSpace(info.SoulDigest),
		ParentSoulDigest:         strings.TrimSpace(info.ParentSoulDigest),
		AttachedTo:               strings.TrimSpace(info.AttachedTo),
		AttachExpiresAt:          cloneTimePtr(info.AttachExpiresAtValue()),
		TranscriptEpoch:          info.TranscriptEpoch,
		PendingPermissionCount:   attention.PendingPermissionCount,
		PendingClarifyCount:      attention.PendingClarifyCount,
		AttentionRevision:        attention.AttentionRevision,
		LastSettledRevision:      attention.LastSettledRevision,
		LastSeenRevision:         attention.LastSeenRevision,
		LastSeenAt:               cloneTimePtr(attention.LastSeenAt),
		AttentionChangedAt:       cloneTimePtr(attention.AttentionChangedAt),
		ArchivedAt:               cloneTimePtr(info.ArchivedAtValue()),
		ParkedAt:                 cloneTimePtr(info.ParkedAtValue()),
		IdleExpiresAt:            cloneTimePtr(info.IdleExpiresAtValue()),
		DrainingAt:               cloneTimePtr(info.DrainingAtValue()),
		CreatedAt:                info.CreatedAt,
		UpdatedAt:                info.UpdatedAt,
	}
	return SessionPayloadFromInfo(converted)
}

func sessionChildState(info *session.Info) contract.SessionChildState {
	if info == nil || info.Lineage == nil || strings.TrimSpace(info.Lineage.ParentSessionID) == "" {
		return ""
	}
	if info.ParkedAt != nil {
		return contract.SessionChildStateParked
	}
	if info.State == session.StateStopped {
		return contract.SessionChildStateGone
	}
	return contract.SessionChildStateRunning
}

// PendingInteractionPayloadsFromStore converts canonical records into sanitized wire payloads.
func PendingInteractionPayloadsFromStore(values []store.PendingInteraction) []contract.PendingInteractionPayload {
	result := make([]contract.PendingInteractionPayload, 0, len(values))
	for _, value := range values {
		value = store.SanitizePendingInteraction(value)
		result = append(result, contract.PendingInteractionPayload{
			InteractionID:     strings.TrimSpace(value.InteractionID),
			Kind:              strings.TrimSpace(value.Kind),
			ProviderRequestID: strings.TrimSpace(value.ProviderRequestID),
			TurnID:            strings.TrimSpace(value.TurnID),
			Title:             strings.TrimSpace(value.Title),
			Choices:           append([]string(nil), value.Payload.Choices...),
			Decisions:         append([]string(nil), value.Payload.Decisions...),
			Status:            strings.TrimSpace(value.Status),
			CreatedAt:         value.CreatedAt.UTC(),
			ResolvedAt:        cloneTimePtr(value.ResolvedAt),
			Resolution:        strings.TrimSpace(value.Resolution),
			ResolvedBy:        strings.TrimSpace(value.ResolvedBy),
		})
	}
	return result
}

func runtimeSelectionPayloadFromInfo(info *session.Info) *contract.RuntimeSelectionPayload {
	if info == nil || !runtimeHasEffectiveSelection(info) || strings.TrimSpace(info.Provider) == "" {
		return nil
	}
	return &contract.RuntimeSelectionPayload{
		Provider:        strings.TrimSpace(info.Provider),
		Model:           strings.TrimSpace(info.Model),
		ReasoningEffort: contract.ReasoningEffort(strings.TrimSpace(info.ReasoningEffort)),
		Speed:           info.Speed,
		ACPOptions:      contract.ACPOptionSelectionPayloadsFromACP(info.ACPOptions),
		SpeedResolution: speedpkg.CloneResolution(info.SpeedResolution),
	}
}

func runtimeHasEffectiveSelection(info *session.Info) bool {
	if info == nil {
		return false
	}
	switch info.RuntimeStatus {
	case session.RuntimeStatusReady, session.RuntimeStatusReconfiguring,
		session.RuntimeStatusRecovering, session.RuntimeStatusFailed:
		return true
	case session.RuntimeStatusUnbound:
		return info.RuntimeTransition != session.RuntimeTransitionNone
	default:
		return false
	}
}

func sessionRuntimeRecoveryPayload(value *store.SessionRuntimeRecovery) *contract.SessionRuntimeRecoveryPayload {
	if value == nil {
		return nil
	}
	return &contract.SessionRuntimeRecoveryPayload{
		Attempt:       value.Attempt,
		MaxAttempts:   value.MaxAttempts,
		Generation:    value.Generation,
		StartedAt:     value.StartedAt.UTC(),
		LastAttemptAt: value.LastAttemptAt.UTC(),
		NextAttemptAt: cloneTimePtr(value.NextAttemptAt),
		LastError:     diagnostics.RedactAndBound(value.LastError, maxDiagnosticPayloadBytes),
	}
}

func runtimeSelectionFromStoreInfo(selection *store.SessionRuntimeSelection) *session.RuntimeSelection {
	if selection == nil {
		return nil
	}
	normalized := store.NormalizeSessionRuntimeSelection(selection)
	return &session.RuntimeSelection{
		Provider:        normalized.Provider,
		Model:           normalized.Model,
		ReasoningEffort: normalized.ReasoningEffort,
		Speed:           normalized.Speed,
		ACPOptions:      session.ACPOptionSelectionsFromStore(normalized.ACPOptions),
	}
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
	return sessionPayloadsFromInfos(infos, false)
}

// SessionPayloadsForAgentFromInfos retains capability atoms for authorized native agent tools.
func SessionPayloadsForAgentFromInfos(infos []*session.Info) []contract.SessionPayload {
	return sessionPayloadsFromInfos(infos, true)
}

func sessionPayloadsFromInfos(infos []*session.Info, includePermissionAtoms bool) []contract.SessionPayload {
	payload := make([]contract.SessionPayload, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		payload = append(payload, sessionPayloadFromInfoAt(info, time.Now().UTC(), includePermissionAtoms))
	}
	return payload
}
