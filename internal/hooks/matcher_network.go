package hooks

import (
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

const (
	matcherChannelKey             = "channel"
	matcherParticipationModeKey   = "participation_mode"
	matcherParticipationSourceKey = "participation_source"
	matcherWorkStateKey           = "work_state"
)

var emptyNetworkMatcher = &NetworkMatcher{}

func resolvedParticipationChannel(spec *participation.Spec) string {
	if spec == nil {
		return ""
	}
	return strings.TrimSpace(spec.ChannelID)
}

// MatchesNetwork matches network-family observation hooks.
func (m HookMatcher) MatchesNetwork(payload NetworkPayload) bool {
	network := m.network()
	return matchStringField(network.Channel, payload.Channel) &&
		matchStringField(network.Surface, payload.Surface) &&
		matchStringField(network.Kind, payload.Kind) &&
		matchStringField(network.Direction, payload.Direction) &&
		matchStringField(network.WorkState, payload.WorkState)
}

func (m HookMatcher) network() *NetworkMatcher {
	if m.NetworkMatcher == nil {
		return emptyNetworkMatcher
	}
	return m.NetworkMatcher
}

func normalizeNetworkMatcher(matcher *NetworkMatcher) *NetworkMatcher {
	if matcher == nil {
		return nil
	}
	normalized := NetworkMatcher{
		Channel:             strings.TrimSpace(matcher.Channel),
		Surface:             strings.TrimSpace(matcher.Surface),
		Kind:                strings.TrimSpace(matcher.Kind),
		Direction:           strings.TrimSpace(matcher.Direction),
		WorkState:           strings.TrimSpace(matcher.WorkState),
		ParticipationMode:   strings.TrimSpace(matcher.ParticipationMode),
		ParticipationSource: strings.TrimSpace(matcher.ParticipationSource),
	}
	if normalized.empty() {
		return nil
	}
	return &normalized
}

func (m *NetworkMatcher) empty() bool {
	return m.Channel == "" &&
		m.Surface == "" &&
		m.Kind == "" &&
		m.Direction == "" &&
		m.WorkState == "" &&
		m.ParticipationMode == "" &&
		m.ParticipationSource == ""
}

func appendNetworkMatcherFieldNames(fields *[]string, matcher *NetworkMatcher) {
	appendIf := func(name string, present bool) {
		if present {
			*fields = append(*fields, name)
		}
	}

	appendIf(matcherChannelKey, matcher.Channel != "")
	appendIf("surface", matcher.Surface != "")
	appendIf("kind", matcher.Kind != "")
	appendIf("direction", matcher.Direction != "")
	appendIf(matcherWorkStateKey, matcher.WorkState != "")
	appendIf(matcherParticipationModeKey, matcher.ParticipationMode != "")
	appendIf(matcherParticipationSourceKey, matcher.ParticipationSource != "")
}

func validateNetworkMatcherPatterns(matcher *NetworkMatcher) error {
	if matcher == nil {
		return nil
	}
	patterns := []struct {
		field   string
		pattern string
	}{
		{field: matcherChannelKey, pattern: matcher.Channel},
		{field: "surface", pattern: matcher.Surface},
		{field: "kind", pattern: matcher.Kind},
		{field: "direction", pattern: matcher.Direction},
		{field: matcherWorkStateKey, pattern: matcher.WorkState},
		{field: matcherParticipationModeKey, pattern: matcher.ParticipationMode},
		{field: matcherParticipationSourceKey, pattern: matcher.ParticipationSource},
	}
	for _, item := range patterns {
		if err := validateMatcherPattern(item.field, item.pattern); err != nil {
			return err
		}
	}
	return nil
}
