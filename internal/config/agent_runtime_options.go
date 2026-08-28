package config

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/runtimeoption"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

// ACPOptionSelection identifies one provider-advertised ACP option default.
// Exactly one of ValueID or BoolValue must be set.
type ACPOptionSelection = runtimeoption.Selection

// AgentRuntimeDefaults keeps optional speed and ACP selections compact while
// its embedded fields preserve the flat agent wire format.
type AgentRuntimeDefaults struct {
	Speed      speedpkg.Speed       `json:"speed,omitempty"       yaml:"speed,omitempty"       toml:"speed,omitempty"`
	ACPOptions []ACPOptionSelection `json:"acp_options,omitempty" yaml:"acp_options,omitempty" toml:"acp_options,omitempty"`
}

// NormalizeACPOptionSelections validates, copies, and sorts ACP option defaults by ID.
func NormalizeACPOptionSelections(selections []ACPOptionSelection) ([]ACPOptionSelection, error) {
	return normalizeACPOptionSelectionsAt("agent.acp_options", selections)
}

func normalizeACPOptionSelectionsAt(path string, selections []ACPOptionSelection) ([]ACPOptionSelection, error) {
	return runtimeoption.NormalizeSelections(path, selections)
}

func validateACPOptionSelections(path string, selections []ACPOptionSelection) error {
	_, err := normalizeACPOptionSelectionsAt(path, selections)
	return err
}

// CloneACPOptionSelections returns an independent copy of ACP option defaults.
func CloneACPOptionSelections(selections []ACPOptionSelection) []ACPOptionSelection {
	return runtimeoption.CloneSelections(selections)
}

func canonicalACPOptionSelections(selections []ACPOptionSelection) []ACPOptionSelection {
	return runtimeoption.CanonicalSelections(selections)
}

func validateAgentSpeed(value speedpkg.Speed, path string) error {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return nil
	}
	if _, err := speedpkg.Parse(trimmed); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func normalizeAgentSpeed(value speedpkg.Speed) speedpkg.Speed {
	return speedpkg.Speed(strings.TrimSpace(string(value)))
}

// SpeedValue returns the authored speed without exposing compact pointer storage.
func (a AgentDef) SpeedValue() speedpkg.Speed {
	if a.AgentRuntimeDefaults == nil {
		return ""
	}
	return a.Speed
}

// SetSpeed stores one normalized authored speed.
func (a *AgentDef) SetSpeed(value speedpkg.Speed) {
	setAgentRuntimeSpeed(&a.AgentRuntimeDefaults, value)
}

// ACPOptionsValue returns the authored typed ACP selections.
func (a AgentDef) ACPOptionsValue() []ACPOptionSelection {
	if a.AgentRuntimeDefaults == nil {
		return nil
	}
	return a.ACPOptions
}

// SetACPOptions stores an ownership-safe authored selection set.
func (a *AgentDef) SetACPOptions(selections []ACPOptionSelection) {
	setAgentRuntimeACPOptions(&a.AgentRuntimeDefaults, selections)
}

// SpeedValue returns the resolved speed without exposing compact pointer storage.
func (a *ResolvedAgent) SpeedValue() speedpkg.Speed {
	if a == nil || a.AgentRuntimeDefaults == nil {
		return ""
	}
	return a.Speed
}

// SetSpeed stores one normalized resolved speed.
func (a *ResolvedAgent) SetSpeed(value speedpkg.Speed) {
	setAgentRuntimeSpeed(&a.AgentRuntimeDefaults, value)
}

// ACPOptionsValue returns the resolved typed ACP selections.
func (a *ResolvedAgent) ACPOptionsValue() []ACPOptionSelection {
	if a == nil || a.AgentRuntimeDefaults == nil {
		return nil
	}
	return a.ACPOptions
}

// SetACPOptions stores an ownership-safe resolved selection set.
func (a *ResolvedAgent) SetACPOptions(selections []ACPOptionSelection) {
	setAgentRuntimeACPOptions(&a.AgentRuntimeDefaults, selections)
}

func setAgentRuntimeSpeed(state **AgentRuntimeDefaults, value speedpkg.Speed) {
	normalized := normalizeAgentSpeed(value)
	if *state == nil {
		if normalized == "" {
			return
		}
		*state = &AgentRuntimeDefaults{}
	}
	(*state).Speed = normalized
	clearEmptyAgentRuntimeDefaults(state)
}

func setAgentRuntimeACPOptions(state **AgentRuntimeDefaults, selections []ACPOptionSelection) {
	cloned := CloneACPOptionSelections(selections)
	if *state == nil {
		if len(cloned) == 0 {
			return
		}
		*state = &AgentRuntimeDefaults{}
	}
	(*state).ACPOptions = cloned
	clearEmptyAgentRuntimeDefaults(state)
}

func clearEmptyAgentRuntimeDefaults(state **AgentRuntimeDefaults) {
	if *state != nil && (*state).Speed == "" && len((*state).ACPOptions) == 0 {
		*state = nil
	}
}
