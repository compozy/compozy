package contract

import (
	"maps"

	"github.com/compozy/agh/internal/hooks"
)

func networkParticipationNamedHookTypes() map[string]NamedType {
	return map[string]NamedType{
		"NetworkParticipationPreResolvePayload": {
			Name:  "NetworkParticipationPreResolvePayload",
			Value: hooks.NetworkParticipationPreResolvePayload{},
		},
		"NetworkParticipationPreResolvePatch": {
			Name:  "NetworkParticipationPreResolvePatch",
			Value: hooks.NetworkParticipationPreResolvePatch{},
		},
		"NetworkParticipationResolvedPayload": {
			Name:  "NetworkParticipationResolvedPayload",
			Value: hooks.NetworkParticipationResolvedPayload{},
		},
		"NetworkParticipationResolvedPatch": {
			Name:  "NetworkParticipationResolvedPatch",
			Value: hooks.NetworkParticipationResolvedPatch{},
		},
	}
}

func mergeNamedHookTypes(
	base map[string]NamedType,
	overlays ...map[string]NamedType,
) map[string]NamedType {
	for _, overlay := range overlays {
		maps.Copy(base, overlay)
	}
	return base
}
