package session

import (
	"context"
	"fmt"
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

// persistSessionLifecycleState is the lifecycle transition choke point. Callers
// mutate the in-memory Session first, then this function durably writes
// meta.json before projecting the same snapshot into the global catalog. If the
// catalog write fails, meta.json remains the recovery source and boot repair
// reconciles the catalog from it.
func (m *Manager) persistSessionLifecycleState(ctx context.Context, session *Session, register bool) error {
	if session == nil {
		return fmt.Errorf("session: session is required")
	}
	session.persistMu.Lock()
	defer session.persistMu.Unlock()
	return m.persistSessionLifecycleStateLocked(ctx, session, register)
}

func (m *Manager) persistSessionLifecycleStateLocked(ctx context.Context, session *Session, register bool) error {
	if err := m.writeMeta(session); err != nil {
		return err
	}
	if m.sessionCatalog == nil {
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, session.Info()))
		return nil
	}
	info := session.Info()
	if register {
		meta := session.Meta()
		if identity := creationIdentityFromMeta(meta); identity != nil && m.creationStore != nil {
			if err := m.registerSessionCreation(ctx, info, meta, *identity); err != nil {
				return err
			}
			m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
			return nil
		}
		if err := m.sessionCatalog.RegisterSession(ctx, sessionCatalogInfoFromRuntime(info)); err != nil {
			return fmt.Errorf("session: register catalog state for %q: %w", session.ID, err)
		}
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
		return nil
	}
	if err := m.sessionCatalog.UpdateSessionState(ctx, sessionCatalogStateUpdate(info)); err != nil {
		return fmt.Errorf("session: update catalog state for %q: %w", session.ID, err)
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return nil
}

func (m *Manager) persistSessionIdentitySnapshot(
	ctx context.Context,
	metaPath string,
	meta store.SessionMeta,
	info *Info,
) error {
	if err := store.WriteSessionMeta(metaPath, meta); err != nil {
		return fmt.Errorf("session: write meta for %q: %w", meta.ID, err)
	}
	if m.sessionCatalog == nil {
		return nil
	}
	if err := m.sessionCatalog.RegisterSession(ctx, sessionCatalogInfoFromRuntime(info)); err != nil {
		return fmt.Errorf("session: update catalog identity for %q: %w", meta.ID, err)
	}
	return nil
}

func (m *Manager) persistSessionMetadataOnly(session *Session) error {
	if session == nil {
		return fmt.Errorf("session: session is required")
	}
	session.persistMu.Lock()
	defer session.persistMu.Unlock()
	return m.writeMeta(session)
}

func (m *Manager) persistSessionCatalogFromMeta(ctx context.Context, meta store.SessionMeta) error {
	return m.persistSessionCatalogSnapshot(ctx, meta, sessionInfoFromMeta(meta))
}

func (m *Manager) persistSessionCatalogSnapshot(
	ctx context.Context,
	meta store.SessionMeta,
	info *Info,
) error {
	if m == nil || m.sessionCatalog == nil {
		return nil
	}
	if info == nil {
		return nil
	}
	if identity := creationIdentityFromMeta(meta); identity != nil && m.creationStore != nil {
		return m.registerSessionCreation(ctx, info, meta, *identity)
	}
	if err := m.sessionCatalog.RegisterSession(ctx, sessionCatalogInfoFromRuntime(info)); err != nil {
		return fmt.Errorf("session: register catalog state from metadata for %q: %w", meta.ID, err)
	}
	return nil
}

func (m *Manager) registerSessionCreation(
	ctx context.Context,
	info *Info,
	meta store.SessionMeta,
	identity store.SessionCreationIdentity,
) error {
	if m.creationStore == nil || meta.CreationProfile == nil {
		return fmt.Errorf("session: creation store/profile is unavailable for %q", meta.ID)
	}
	profileRef, err := m.creationStore.PutSessionCreationProfile(ctx, *meta.CreationProfile)
	if err != nil {
		return fmt.Errorf("session: persist creation profile for %q: %w", meta.ID, err)
	}
	if profileRef != identity.CreationProfileRef {
		return fmt.Errorf("session: creation profile ref changed for %q", meta.ID)
	}
	if _, err := m.creationStore.RegisterSessionWithCreationIdentity(
		ctx,
		sessionCatalogInfoFromRuntime(info),
		identity,
	); err != nil {
		return fmt.Errorf("session: register creation identity for %q: %w", meta.ID, err)
	}
	return nil
}

func sessionCatalogInfoFromRuntime(info *Info) store.SessionInfo {
	if info == nil {
		return store.SessionInfo{}
	}
	result := store.SessionInfo{
		ID:                       info.ID,
		Name:                     info.Name,
		AgentName:                info.AgentName,
		Provider:                 info.Provider,
		Model:                    info.Model,
		ReasoningEffort:          info.ReasoningEffort,
		Speed:                    info.Speed,
		SpeedResolution:          speedpkg.CloneResolution(info.SpeedResolution),
		RuntimeStatus:            info.RuntimeStatus,
		RuntimeTransition:        info.RuntimeTransition,
		RuntimeFailure:           info.RuntimeFailure,
		SelectedRuntime:          storeSessionRuntimeSelection(info.SelectedRuntime),
		RuntimeSelectionRevision: info.RuntimeSelectionRevision,
		WorkspaceID:              info.WorkspaceID,
		WorktreeID:               info.WorktreeID,
		SessionType:              string(info.Type),
		Lineage:                  store.CloneSessionLineage(info.Lineage),
		State:                    string(info.State),
		ACPSessionID:             stringPointer(info.ACPSessionID),
		StopReason:               info.StopReason,
		StopDetail:               info.StopDetail,
		Failure:                  store.CloneSessionFailure(info.Failure),
		Liveness:                 store.CloneSessionLivenessMeta(info.Liveness),
		Sandbox:                  cloneSessionSandboxMeta(info.Sandbox),
		SoulSnapshotID:           strings.TrimSpace(info.SoulSnapshotID),
		SoulDigest:               strings.TrimSpace(info.SoulDigest),
		ParentSoulDigest:         strings.TrimSpace(info.ParentSoulDigest),
		TranscriptEpoch:          info.TranscriptEpoch,
		ArchivedAt:               cloneTimePointer(info.ArchivedAt),
		CreatedAt:                info.CreatedAt,
		UpdatedAt:                info.UpdatedAt,
	}
	result.SetNetworkSpec(info.NetworkParticipation)
	return result
}

func sessionCatalogStateUpdate(info *Info) store.SessionStateUpdate {
	if info == nil {
		return store.SessionStateUpdate{}
	}
	return store.SessionStateUpdate{
		ID:                       info.ID,
		State:                    string(info.State),
		ACPSessionID:             stringPointer(info.ACPSessionID),
		RuntimeSet:               true,
		Provider:                 info.Provider,
		Model:                    info.Model,
		ReasoningEffort:          info.ReasoningEffort,
		Speed:                    info.Speed,
		SpeedResolution:          speedpkg.CloneResolution(info.SpeedResolution),
		RuntimeStatus:            info.RuntimeStatus,
		RuntimeTransition:        info.RuntimeTransition,
		RuntimeFailure:           info.RuntimeFailure,
		SelectedRuntimeSet:       true,
		SelectedRuntime:          storeSessionRuntimeSelection(info.SelectedRuntime),
		RuntimeSelectionRevision: info.RuntimeSelectionRevision,
		StopReasonSet:            true,
		StopReason:               stringPointer(string(info.StopReason)),
		StopDetail:               info.StopDetail,
		FailureSet:               true,
		Failure:                  store.CloneSessionFailure(info.Failure),
		Liveness:                 store.CloneSessionLivenessMeta(info.Liveness),
		Sandbox:                  cloneSessionSandboxMeta(info.Sandbox),
		UpdatedAt:                info.UpdatedAt,
	}
}
