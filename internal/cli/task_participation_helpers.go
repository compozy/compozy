package cli

import "github.com/compozy/compozy/internal/network/participation"

func resolvedParticipationChannel(spec *participation.Spec) string {
	if spec == nil {
		return ""
	}
	return stringOrDash(spec.ChannelID)
}

func resolvedParticipationChannelRaw(spec *participation.Spec) string {
	if spec == nil {
		return ""
	}
	return spec.ChannelID
}
