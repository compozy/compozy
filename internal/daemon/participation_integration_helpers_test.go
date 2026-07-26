//go:build integration

package daemon

import (
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

func resolvedParticipationChannelID(spec *participation.Spec) string {
	if spec == nil {
		return ""
	}
	return strings.TrimSpace(spec.ChannelID)
}

func daemonTestNamedParticipationRequest(channelID string) *participation.Request {
	trimmed := strings.TrimSpace(channelID)
	if trimmed == "" {
		return nil
	}
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &trimmed,
	}
}
