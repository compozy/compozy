package core_test

import "github.com/compozy/agh/internal/network/participation"

func resolvedParticipationChannelID(spec *participation.Spec) string {
	if spec == nil {
		return ""
	}
	return spec.ChannelID
}
