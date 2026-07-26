package situation

import (
	"context"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *Service) populateSessionRuntimeContext(
	ctx context.Context,
	info *session.Info,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
	workspaceID string,
	payload *contract.AgentContextPayload,
) error {
	taskContext, channelContext, activeChannel, err := s.taskAndChannelContext(ctx, info.ID, workspaceSnapshot)
	if err != nil {
		return err
	}
	payload.Task = taskContext
	if info.NetworkParticipation.Mode != participation.ModeLive {
		return nil
	}
	payload.CoordinationChannel = channelContext
	payload.InboxSummary, payload.PeerRoster, err = s.networkSections(
		ctx,
		info.ID,
		workspaceID,
		firstTrimmed(activeChannel, info.NetworkParticipation.ChannelID),
		coordinationChannelID(channelContext),
	)
	return err
}

func startupSessionPayload(startup session.StartupPromptContext) contract.AgentSessionPayload {
	payload := contract.AgentSessionPayload{
		ID:                           strings.TrimSpace(startup.SessionID),
		Name:                         strings.TrimSpace(startup.SessionName),
		Type:                         startup.SessionType,
		State:                        session.StateStarting,
		ResolvedNetworkParticipation: normalizedSessionParticipation(startup.NetworkParticipation),
		CreatedAt:                    startup.CreatedAt.UTC(),
		UpdatedAt:                    startup.UpdatedAt.UTC(),
	}
	return payload
}

func normalizedSessionParticipation(spec participation.Spec) *participation.Spec {
	if spec == (participation.Spec{}) {
		spec = participation.LocalSpec()
	}
	return participation.CloneSpec(spec)
}
