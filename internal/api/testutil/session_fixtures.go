package testutil

import (
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

func NewSessionInfo(id string) *session.Info {
	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	return &session.Info{
		ID:                   id,
		Name:                 "demo",
		AgentName:            "coder",
		WorkspaceID:          "ws-workspace",
		Workspace:            "/workspace",
		NetworkParticipation: participation.LocalSpec(),
		State:                session.StateActive,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func NewSession(id string) *session.Session {
	info := NewSessionInfo(id)
	return &session.Session{
		ID:                   info.ID,
		Name:                 info.Name,
		AgentName:            info.AgentName,
		WorkspaceID:          info.WorkspaceID,
		Workspace:            info.Workspace,
		NetworkParticipation: info.NetworkParticipation,
		State:                info.State,
		CreatedAt:            info.CreatedAt,
		UpdatedAt:            info.UpdatedAt,
	}
}
