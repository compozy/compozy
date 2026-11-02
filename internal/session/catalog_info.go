package session

import (
	"context"
	"fmt"
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func sessionInfoFromCatalog(info *store.SessionInfo) *Info {
	attention := info.AttentionSnapshot()
	return &Info{
		InstanceID:    info.InstanceID,
		InitiatorKind: info.InitiatorKind, InitiatorID: info.InitiatorID,
		VerdictOnly:              info.VerdictOnly,
		ID:                       strings.TrimSpace(info.ID),
		ProfileID:                strings.TrimSpace(info.ProfileID),
		Name:                     strings.TrimSpace(info.Name),
		AgentName:                strings.TrimSpace(info.AgentName),
		Provider:                 strings.TrimSpace(info.Provider),
		Model:                    strings.TrimSpace(info.Model),
		ReasoningEffort:          strings.TrimSpace(info.ReasoningEffort),
		Speed:                    info.Speed,
		ACPOptions:               ACPOptionSelectionsFromStore(info.ACPOptionsValue()),
		SpeedResolution:          speedpkg.CloneResolution(info.SpeedResolution),
		RuntimeStatus:            info.RuntimeStatus,
		RuntimeTransition:        info.RuntimeTransition,
		RuntimeFailure:           strings.TrimSpace(info.RuntimeFailure),
		RuntimeGeneration:        info.RuntimeGeneration,
		RuntimeRecovery:          info.RuntimeRecoveryValue(),
		SelectedRuntime:          runtimeSelectionFromSessionStore(info.SelectedRuntime),
		RuntimeSelectionRevision: info.RuntimeSelectionRevision,
		WorkspaceID:              strings.TrimSpace(info.WorkspaceID),
		EffectiveWorkspaceAccess: strings.TrimSpace(info.EffectiveWorkspaceAccess),
		WorktreeID:               strings.TrimSpace(info.WorktreeID),
		Type:                     Type(strings.TrimSpace(info.SessionType)),
		Lineage:                  store.CloneSessionLineage(info.Lineage),
		State:                    State(strings.TrimSpace(info.State)),
		StopReason:               info.StopReason,
		StopEscalated:            info.StopEscalated,
		StopVerificationFailed:   info.StopVerificationFailed,
		StopDetail:               strings.TrimSpace(info.StopDetail),
		Failure:                  store.CloneSessionFailure(info.Failure),
		ACPSessionID:             stringValue(info.ACPSessionID),
		Liveness:                 store.CloneSessionLivenessMeta(info.Liveness),
		SoulSnapshotID:           strings.TrimSpace(info.SoulSnapshotID),
		SoulDigest:               strings.TrimSpace(info.SoulDigest),
		ParentSoulDigest:         strings.TrimSpace(info.ParentSoulDigest),
		AttachedTo:               strings.TrimSpace(info.AttachedToValue()),
		AttachExpiresAt:          cloneTimePointer(info.AttachExpiresAtValue()),
		TranscriptEpoch:          info.TranscriptEpoch,
		PendingPermissionCount:   attention.PendingPermissionCount,
		PendingClarifyCount:      attention.PendingClarifyCount,
		AttentionRevision:        attention.AttentionRevision,
		LastSettledRevision:      attention.LastSettledRevision,
		LastSeenRevision:         attention.LastSeenRevision,
		LastSeenAt:               cloneTimePointer(attention.LastSeenAt),
		AttentionChangedAt:       cloneTimePointer(attention.AttentionChangedAt),
		EndedEarlyTurnID:         attention.EndedEarlyTurnID,
		ArchivedAt:               cloneTimePointer(info.ArchivedAt),
		Later:                    store.CloneSessionLater(info.LaterState()),
		LaterReturnedAt:          cloneTimePointer(info.LaterReturnedAtValue()),
		CreatedAt:                info.CreatedAt,
		UpdatedAt:                info.UpdatedAt,
	}
}

func (m *Manager) populateInstanceMetadata(ctx context.Context, info *Info) error {
	if m.sessionCatalog == nil {
		return nil
	}
	rows, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
		ID:        info.ID,
		ReadScope: store.ReadScope{AllProfiles: true},
		Archive:   store.SessionArchiveInclude,
	})
	if err != nil {
		return fmt.Errorf("session: read instance binding: %w", err)
	}
	if len(rows) > 0 {
		info.InstanceID = rows[0].InstanceID
	}
	return nil
}
