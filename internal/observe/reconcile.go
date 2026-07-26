package observe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

// Reconcile scans the sessions directory and reconciles the global session index.
func (o *Observer) Reconcile(ctx context.Context) (store.ReconcileResult, error) {
	recovered, err := o.loadSessionMetadata(ctx)
	if err != nil {
		return store.ReconcileResult{}, err
	}

	sessions := make([]store.SessionInfo, 0, len(recovered))
	creationIndexed := make([]string, 0)
	for _, recoveredSession := range recovered {
		created, err := o.reconcileRecoveredSessionCreation(ctx, recoveredSession)
		if err != nil {
			return store.ReconcileResult{}, err
		}
		if created {
			creationIndexed = append(creationIndexed, recoveredSession.ID)
		}
		sessions = append(sessions, recoveredSession.SessionInfo)
	}

	result, err := o.registry.ReconcileSessions(ctx, sessions)
	if err != nil {
		return store.ReconcileResult{}, fmt.Errorf("observe: reconcile sessions: %w", err)
	}
	result.Indexed = append(result.Indexed, creationIndexed...)
	sort.Strings(result.Indexed)

	return result, nil
}

type recoveredSession struct {
	store.SessionInfo
	creationProfile  *store.SessionCreationProfile
	creationIdentity *store.SessionCreationIdentity
}

func (o *Observer) reconcileRecoveredSessionCreation(
	ctx context.Context,
	recovered recoveredSession,
) (bool, error) {
	if recovered.creationProfile == nil && recovered.creationIdentity == nil {
		return false, nil
	}
	if recovered.creationProfile == nil || recovered.creationIdentity == nil {
		return false, fmt.Errorf("observe: incomplete session creation metadata for %q", recovered.ID)
	}
	creationStore, ok := o.registry.(store.SessionCreationStore)
	if !ok {
		return false, fmt.Errorf("observe: session creation store is unavailable for %q", recovered.ID)
	}
	profileRef, err := creationStore.PutSessionCreationProfile(ctx, *recovered.creationProfile)
	if err != nil {
		return false, fmt.Errorf("observe: persist session creation profile for %q: %w", recovered.ID, err)
	}
	if profileRef != recovered.creationIdentity.CreationProfileRef {
		return false, fmt.Errorf("observe: session creation profile ref changed for %q", recovered.ID)
	}
	registration, err := creationStore.RegisterSessionWithCreationIdentity(
		ctx,
		recovered.SessionInfo,
		*recovered.creationIdentity,
	)
	if err != nil {
		return false, fmt.Errorf("observe: register session creation identity for %q: %w", recovered.ID, err)
	}
	return registration.Created, nil
}

func (o *Observer) loadSessionMetadata(ctx context.Context) ([]recoveredSession, error) {
	entries, err := os.ReadDir(o.homePaths.SessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("observe: read sessions directory %q: %w", o.homePaths.SessionsDir, err)
	}

	sessions := make([]recoveredSession, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		recovered, ok := o.loadRecoveredSession(ctx, entry.Name())
		if ok {
			sessions = append(sessions, recovered)
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ID < sessions[j].ID
	})

	return sessions, nil
}

func (o *Observer) loadRecoveredSession(ctx context.Context, entryName string) (recoveredSession, bool) {
	sessionDir := filepath.Join(o.homePaths.SessionsDir, entryName)
	metaPath := store.SessionMetaFile(sessionDir)
	meta, err := store.ReadSessionMeta(metaPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			o.logger.Warn(
				"observe: skipping unreadable session metadata",
				"session_id", strings.TrimSpace(entryName),
				"path", metaPath,
				"error", err,
			)
		}
		return recoveredSession{}, false
	}

	sessionID := strings.TrimSpace(meta.ID)
	meta, err = session.RepairLegacyProvider(ctx, metaPath, meta, session.LegacyProviderRepairOptions{
		Now:               o.now,
		Logger:            o.logger,
		WorkspaceResolver: o.workspaceResolver,
	})
	if err != nil {
		o.logger.Warn(
			"observe: skipping session with unrecoverable legacy provider metadata",
			"session_id", sessionID,
			"path", metaPath,
			"error", err,
		)
		return recoveredSession{}, false
	}

	return recoveredSessionFromMeta(o.normalizeRecoveredMeta(metaPath, meta)), true
}

func recoveredSessionFromMeta(meta store.SessionMeta) recoveredSession {
	stopReason := store.StopReason("")
	if meta.StopReason != nil {
		stopReason = *meta.StopReason
	}
	recovered := recoveredSession{SessionInfo: store.SessionInfo{
		ID:               meta.ID,
		Name:             meta.Name,
		AgentName:        meta.AgentName,
		Provider:         meta.Provider,
		WorkspaceID:      meta.WorkspaceID,
		SessionType:      meta.SessionType,
		Lineage:          store.CloneSessionLineage(meta.Lineage),
		State:            meta.State,
		ACPSessionID:     meta.ACPSessionID,
		StopReason:       stopReason,
		StopDetail:       meta.StopDetail,
		Failure:          store.CloneSessionFailure(meta.Failure),
		Liveness:         store.CloneSessionLivenessMeta(meta.Liveness),
		Sandbox:          cloneSessionSandboxMeta(meta.Sandbox),
		SoulSnapshotID:   strings.TrimSpace(meta.SoulSnapshotID),
		SoulDigest:       strings.TrimSpace(meta.SoulDigest),
		ParentSoulDigest: strings.TrimSpace(meta.ParentSoulDigest),
		CreatedAt:        meta.CreatedAt,
		UpdatedAt:        meta.UpdatedAt,
	}}
	recovered.SetNetworkSpec(meta.NetworkSpecSnapshot())
	if meta.CreationProfile != nil {
		profile := store.NormalizeSessionCreationProfile(*meta.CreationProfile)
		identity := store.SessionCreationIdentity{
			CreationProfileRef: strings.TrimSpace(meta.CreationProfileRef),
			PolicySpecDigest:   strings.TrimSpace(meta.PolicySpecDigest),
			CreationDigest:     strings.TrimSpace(meta.CreationDigest),
		}
		recovered.creationProfile = &profile
		recovered.creationIdentity = &identity
	}
	return recovered
}

func (o *Observer) normalizeRecoveredMeta(path string, meta store.SessionMeta) store.SessionMeta {
	normalized, changed := session.ClassifyInactiveMetaForRecovery(o.now(), meta)
	if !changed {
		return normalized
	}

	normalized.UpdatedAt = o.now()
	if err := store.WriteSessionMeta(path, normalized); err != nil {
		o.logger.Warn(
			"observe: persist recovered session classification failed",
			"session_id", strings.TrimSpace(meta.ID),
			"path", path,
			"error", err,
		)
		return session.AnnotateUnpersistedRecovery(normalized, err)
	}

	return normalized
}
