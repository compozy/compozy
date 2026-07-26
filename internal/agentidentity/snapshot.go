package agentidentity

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

// SessionSnapshot is the daemon-authoritative session subset needed for identity validation.
type SessionSnapshot struct {
	ID                   string
	Name                 string
	AgentName            string
	Provider             string
	Model                string
	WorkspaceID          string
	WorkspacePath        string
	NetworkParticipation *participation.Spec
	Type                 session.Type
	Lineage              *store.SessionLineage
	State                session.State
	SoulSnapshotID       string
	SoulDigest           string
	ParentSoulDigest     string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// SessionLookup loads a daemon-authoritative session snapshot by session ID.
type SessionLookup func(context.Context, string) (SessionSnapshot, error)

// SessionSnapshotFromInfo converts the runtime session read model into a validation snapshot.
func SessionSnapshotFromInfo(info *session.Info) SessionSnapshot {
	if info == nil {
		return SessionSnapshot{}
	}
	return SessionSnapshot{
		ID:                   info.ID,
		Name:                 info.Name,
		AgentName:            info.AgentName,
		Provider:             info.Provider,
		Model:                info.Model,
		WorkspaceID:          info.WorkspaceID,
		WorkspacePath:        info.Workspace,
		NetworkParticipation: participation.CloneSpec(info.NetworkParticipation),
		Type:                 info.Type,
		Lineage:              store.CloneSessionLineage(info.Lineage),
		State:                info.State,
		SoulSnapshotID:       info.SoulSnapshotID,
		SoulDigest:           info.SoulDigest,
		ParentSoulDigest:     info.ParentSoulDigest,
		CreatedAt:            info.CreatedAt,
		UpdatedAt:            info.UpdatedAt,
	}
}

// NetworkSpecSnapshot returns the caller session's immutable participation snapshot.
func (s SessionSnapshot) NetworkSpecSnapshot() participation.Spec {
	if s.NetworkParticipation == nil {
		return participation.LocalSpec()
	}
	return *s.NetworkParticipation
}

func normalizeSessionSnapshot(snapshot SessionSnapshot) SessionSnapshot {
	snapshot.ID = strings.TrimSpace(snapshot.ID)
	snapshot.Name = strings.TrimSpace(snapshot.Name)
	snapshot.AgentName = strings.TrimSpace(snapshot.AgentName)
	snapshot.Provider = strings.TrimSpace(snapshot.Provider)
	snapshot.Model = strings.TrimSpace(snapshot.Model)
	snapshot.WorkspaceID = strings.TrimSpace(snapshot.WorkspaceID)
	snapshot.WorkspacePath = strings.TrimSpace(snapshot.WorkspacePath)
	snapshot.SoulSnapshotID = strings.TrimSpace(snapshot.SoulSnapshotID)
	snapshot.SoulDigest = strings.TrimSpace(snapshot.SoulDigest)
	snapshot.ParentSoulDigest = strings.TrimSpace(snapshot.ParentSoulDigest)
	return snapshot
}
