package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

type instanceSessionReader struct {
	sessions workspaceAccessSessionStatus
}

var _ instanceAdoptionReader = instanceSessionReader{}

func (r instanceSessionReader) AdoptionSession(
	ctx context.Context,
	id string,
) (store.SessionInfo, store.SessionPermissionPolicy, error) {
	info, err := r.sessions.Status(ctx, id)
	if err != nil {
		return store.SessionInfo{}, store.SessionPermissionPolicy{}, err
	}
	if info == nil {
		return store.SessionInfo{}, store.SessionPermissionPolicy{}, session.ErrSessionNotFound
	}
	if info.Lineage == nil {
		return store.SessionInfo{}, store.SessionPermissionPolicy{}, errors.New("session permission policy unavailable")
	}
	return store.SessionInfo{
		ID:                       info.ID,
		ProfileID:                info.ProfileID,
		WorkspaceID:              info.WorkspaceID,
		AgentName:                info.AgentName,
		SessionType:              string(info.Type),
		ArchivedAt:               info.ArchivedAt,
		Lineage:                  info.Lineage,
		EffectiveWorkspaceAccess: info.EffectiveWorkspaceAccess,
	}, store.NormalizeSessionPermissionPolicy(info.Lineage.PermissionPolicy), nil
}
