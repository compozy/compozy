//go:build integration

package cli

import "github.com/compozy/compozy/internal/network/participation"

func resolvedParticipationChannelID(spec *participation.Spec) string {
	if spec == nil {
		return ""
	}
	return spec.ChannelID
}
