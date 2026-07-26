package extensionpkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

func (h *HostAPIHandler) requireHostAPINetworkParticipant(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	channel string,
) error {
	info, err := h.requireHostAPISessionWorkspace(ctx, workspaceID, sessionID)
	if err != nil {
		return err
	}
	wantedChannel := strings.TrimSpace(channel)
	if info.NetworkParticipation.Mode != participation.ModeLive ||
		strings.TrimSpace(info.NetworkParticipation.ChannelID) != wantedChannel {
		return unavailableRPCError(fmt.Errorf(
			"%w: session=%q channel=%q",
			participation.ErrNotParticipating,
			strings.TrimSpace(sessionID),
			wantedChannel,
		))
	}
	return nil
}
