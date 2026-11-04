package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

// retiredAgentKind is the v0.5.0 spelling of always_on. v0.5.1 accepted it with a warning and v0.5.2
// removed it (#1239). It stays named here only so each rejection can point at the replacement.
const retiredAgentKind = "stays_on"

// ErrConfigPathRemoved marks a config path whose table was renamed and no longer resolves.
var ErrConfigPathRemoved = errors.New("config: removed config path")

var (
	retiredAgentKindTable = []string{AgentsDirName, retiredAgentKind}
	alwaysOnTable         = []string{AgentsDirName, AgentKindAlwaysOn}
)

// ValidateAgentKind accepts on_demand, always_on, and empty (on_demand). Callers that compare a
// requested kind with the stored one validate it first, so a bad value is not reported as a change.
func ValidateAgentKind(kind string) error {
	switch (AgentDef{Kind: kind}).EffectiveKind() {
	case AgentKindOnDemand, AgentKindAlwaysOn:
		return nil
	case retiredAgentKind:
		return fmt.Errorf(
			"%w (%s was renamed to %s in v0.5.1)",
			ErrAgentKindInvalid,
			retiredAgentKind,
			AgentKindAlwaysOn,
		)
	default:
		return ErrAgentKindInvalid
	}
}

// RejectRemovedConfigPath reports a path under the renamed [agents.stays_on] table and names its
// [agents.always_on] replacement, so config get/set/unset and the config tools do not answer
// "not found" for a key that moved.
func RejectRemovedConfigPath(path []string) error {
	replacement, ok := alwaysOnReplacementPath(path)
	if !ok {
		return nil
	}
	return fmt.Errorf(
		"%w %q: use %q",
		ErrConfigPathRemoved,
		strings.Join(path, "."),
		strings.Join(replacement, "."),
	)
}

// RejectRemovedDottedConfigPath is RejectRemovedConfigPath for a dotted path.
func RejectRemovedDottedConfigPath(raw string) error {
	return RejectRemovedConfigPath(strings.Split(strings.TrimSpace(raw), "."))
}

func rejectRemovedAlwaysOnTableKeys(source string, keys []burnttoml.Key) error {
	for _, key := range sortedTOMLKeys(keys) {
		replacement, ok := alwaysOnReplacementPath(key)
		if !ok {
			continue
		}
		return fmt.Errorf(
			"removed config key %q in %q: use %q",
			key.String(),
			source,
			burnttoml.Key(replacement).String(),
		)
	}
	return nil
}

func alwaysOnReplacementPath(path []string) ([]string, bool) {
	if len(path) < len(retiredAgentKindTable) ||
		!slices.Equal(path[:len(retiredAgentKindTable)], retiredAgentKindTable) {
		return nil, false
	}
	return append(slices.Clone(alwaysOnTable), path[len(retiredAgentKindTable):]...), true
}
