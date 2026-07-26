package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/store"
)

func (m *Manager) readMetaWithContext(ctx context.Context, id string) (store.SessionMeta, error) {
	target, err := normalizeStoredSessionID(id)
	if err != nil {
		return store.SessionMeta{}, err
	}

	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	reader := m.readSessionMeta
	if reader == nil {
		reader = store.ReadSessionMeta
	}
	meta, err := reader(metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.SessionMeta{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
		}
		return store.SessionMeta{}, fmt.Errorf("session: read metadata for %q: %w", target, err)
	}
	if _, ok := m.Get(target); ok || m.isPending(target) {
		return meta, nil
	}
	repaired, err := m.repairInactiveMeta(ctx, metaPath, meta)
	if err != nil {
		return store.SessionMeta{}, err
	}
	return repaired, nil
}

func normalizeStoredSessionID(id string) (string, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return "", errors.New("session: session id is required")
	}
	if filepath.IsAbs(target) || target == "." || target == ".." {
		return "", fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	if hasWindowsDriveRelativePrefix(target) {
		return "", fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	if strings.Contains(target, "/") || strings.Contains(target, `\`) {
		return "", fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	if filepath.Clean(target) != target {
		return "", fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	return target, nil
}

func hasWindowsDriveRelativePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	drive := value[0]
	if (drive < 'a' || drive > 'z') && (drive < 'A' || drive > 'Z') {
		return false
	}
	if len(value) == 2 {
		return true
	}
	return value[2] != '/' && value[2] != '\\'
}

func (m *Manager) sessionInfoFromMeta(ctx context.Context, meta store.SessionMeta) *Info {
	info := sessionInfoFromMeta(meta)
	if m != nil && m.transcriptEpochStore != nil {
		epoch, err := m.transcriptEpochStore.SessionTranscriptEpoch(ctx, meta.ID)
		if err != nil {
			m.logger.Warn(
				"session: read transcript epoch for metadata failed",
				"session_id",
				meta.ID,
				"error",
				err,
			)
		} else {
			info.TranscriptEpoch = epoch
		}
	}
	workspaceRoot, err := m.resolveWorkspaceRoot(ctx, meta.WorkspaceID)
	if err != nil {
		m.logger.Warn(
			"session: resolve workspace root for metadata failed",
			"session_id",
			meta.ID,
			"workspace_id",
			meta.WorkspaceID,
			"error",
			err,
		)
		return info
	}
	info.Workspace = workspaceRoot
	return info
}

func sessionInfoFromMeta(meta store.SessionMeta) *Info {
	return &Info{
		ID:                   meta.ID,
		Name:                 meta.Name,
		AgentName:            meta.AgentName,
		Provider:             meta.Provider,
		Model:                strings.TrimSpace(meta.Model),
		ReasoningEffort:      strings.TrimSpace(meta.ReasoningEffort),
		WorkspaceID:          meta.WorkspaceID,
		NetworkParticipation: meta.NetworkSpecSnapshot(),
		NetworkOwnerKey:      meta.NetworkOwnerKeySnapshot(),
		Type:                 normalizeSessionType(Type(meta.SessionType)),
		Lineage:              store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		State:                State(meta.State),
		StopReason:           sessionMetaStopReason(meta),
		StopDetail:           meta.StopDetail,
		Failure:              store.CloneSessionFailure(meta.Failure),
		ACPSessionID:         stringValue(meta.ACPSessionID),
		AdvertisedCommands:   store.CloneSessionAdvertisedCommands(meta.AdvertisedCommands),
		Liveness:             store.CloneSessionLivenessMeta(meta.Liveness),
		Sandbox:              cloneSessionSandboxMeta(meta.Sandbox),
		SoulSnapshotID:       strings.TrimSpace(meta.SoulSnapshotID),
		SoulDigest:           strings.TrimSpace(meta.SoulDigest),
		ParentSoulDigest:     strings.TrimSpace(meta.ParentSoulDigest),
		CreatedAt:            meta.CreatedAt,
		UpdatedAt:            meta.UpdatedAt,
	}
}

func sortSessionInfos(infos []*Info) []*Info {
	out := make([]*Info, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})

	return out
}
