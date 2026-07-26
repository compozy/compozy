package daemon

import (
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

func daemonTestLiveParticipation(workspaceID, channelID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     strings.TrimSpace(workspaceID),
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       strings.TrimSpace(channelID),
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	}
}

func daemonTestLiveParticipationPtr(workspaceID, channelID string) *participation.Spec {
	spec := daemonTestLiveParticipation(workspaceID, channelID)
	return &spec
}

func daemonTestParticipationFromCreateOpts(opts session.CreateOpts) participation.Spec {
	if opts.ResolvedNetworkParticipation != nil {
		return *opts.ResolvedNetworkParticipation
	}
	if opts.NetworkParticipation == nil || opts.NetworkParticipation.Mode == nil ||
		*opts.NetworkParticipation.Mode != participation.ModeLive {
		return participation.LocalSpec()
	}
	channelID := ""
	if opts.NetworkParticipation.ChannelID != nil {
		channelID = *opts.NetworkParticipation.ChannelID
	}
	return daemonTestLiveParticipation(opts.Workspace, channelID)
}
