package acpmock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"os"

	"strings"

	"github.com/compozy/agh/internal/acp"
)

const (
	fixtureUserMessageOpen  = "<user-message>"
	fixtureUserMessageClose = "</user-message>"
	fixtureAllowAlwaysValue = "allow-always"
)

const (
	aghSituationContextOpen           = "<agh-situation-context>"
	aghSituationContextClose          = "</agh-situation-context>"
	availableSkillsOpen               = "<available-skills>"
	availableSkillsClose              = "</available-skills>"
	availableSkillsSelfClosing        = "<available-skills />"
	currentAvailableSkillsOpen        = "<current-available-skills>"
	currentAvailableSkillsClose       = "</current-available-skills>"
	currentAvailableSkillsSelfClosing = "<current-available-skills />"
	currentSkillsCatalogOpeningLine   = "The <current-available-skills> block above is " +
		"the authoritative current skill state for this turn."
	currentSkillsCatalogFinalLine = "If current tool policy denies canonical `agh__skill_view`, " +
		"use `agh skill view <name>` as an operator fallback."
	durableMemoryOpen         = "<turn-recall>"
	durableMemoryClose        = "</turn-recall>"
	inboundBridgePromptPrefix = "Inbound bridge message"
)

// LoadFixture parses and validates one fixture file.
func LoadFixture(path string) (Fixture, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return Fixture{}, errors.New("acpmock: fixture path is required")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return Fixture{}, fmt.Errorf("acpmock: read fixture %q: %w", target, err)
	}
	fixture, err := ParseFixture(data)
	if err != nil {
		return Fixture{}, fmt.Errorf("acpmock: parse fixture %q: %w", target, err)
	}
	return fixture, nil
}

// ParseFixture decodes and validates fixture JSON.
func ParseFixture(data []byte) (Fixture, error) {
	var fixture Fixture
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err == nil || !errors.Is(err, io.EOF) {
		return Fixture{}, errors.New("acpmock: fixture JSON must contain exactly one document")
	}
	if err := fixture.Validate(); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}

// Validate ensures the fixture can drive deterministic ACP scenarios.
func (f Fixture) Validate() error {
	if f.Version != FixtureVersion {
		return fmt.Errorf("acpmock: fixture version %d, want %d", f.Version, FixtureVersion)
	}
	if len(f.Agents) == 0 {
		return errors.New("acpmock: at least one fixture agent is required")
	}

	seen := make(map[string]struct{}, len(f.Agents))
	for idx := range f.Agents {
		name := strings.TrimSpace(f.Agents[idx].Name)
		if name == "" {
			return fmt.Errorf("acpmock: agents[%d].name is required", idx)
		}
		f.Agents[idx].Name = name
		if _, ok := seen[name]; ok {
			return fmt.Errorf("acpmock: duplicate agent %q", name)
		}
		seen[name] = struct{}{}
		agent := f.Agents[idx]
		if err := agent.Validate(fmt.Sprintf("agents[%d]", idx)); err != nil {
			return err
		}
	}

	return nil
}

// Agent returns one named fixture agent.
func (f Fixture) Agent(name string) (AgentFixture, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return AgentFixture{}, errors.New("acpmock: fixture agent name is required")
	}
	for _, agent := range f.Agents {
		if agent.Name == target {
			return agent, nil
		}
	}
	return AgentFixture{}, fmt.Errorf("acpmock: fixture agent %q not found", target)
}

// SelectTurn returns the first turn that matches stable prompt fields.
func (a AgentFixture) SelectTurn(prompt string, meta ...acp.PromptMeta) (TurnFixture, error) {
	input := turnMatchInput{
		UserText: strings.TrimSpace(prompt),
	}
	if len(meta) > 0 {
		input.Meta = meta[0].Normalize()
	}

	for _, turn := range a.Turns {
		if turn.Match.matches(input) {
			return turn, nil
		}
	}

	return TurnFixture{}, fmt.Errorf(
		"acpmock: no turn matched agent %q canonical_prompt %q raw_prompt %q with meta %#v",
		a.Name,
		canonicalUserText(input.UserText),
		input.UserText,
		input.Meta,
	)
}

// Validate ensures the agent fixture is usable.
func (a AgentFixture) Validate(path string) error {
	if strings.TrimSpace(a.Provider) == "" {
		return fmt.Errorf("acpmock: %s.provider is required", path)
	}
	if len(a.Turns) == 0 {
		return fmt.Errorf("acpmock: %s.turns must contain at least one turn", path)
	}
	for idx, option := range a.ConfigOptions {
		if err := option.Validate(fmt.Sprintf("%s.config_options[%d]", path, idx)); err != nil {
			return err
		}
	}
	for idx, turn := range a.Turns {
		if err := turn.Validate(fmt.Sprintf("%s.turns[%d]", path, idx)); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures one session config option is deterministic and selectable.
func (o SessionConfigOptionFixture) Validate(path string) error {
	id := strings.TrimSpace(o.ID)
	if id == "" {
		return fmt.Errorf("acpmock: %s.id is required", path)
	}
	name := strings.TrimSpace(o.Name)
	if name == "" {
		return fmt.Errorf("acpmock: %s.name is required", path)
	}
	current := strings.TrimSpace(o.Current)
	if current == "" {
		return fmt.Errorf("acpmock: %s.current is required", path)
	}
	if len(o.Values) == 0 {
		return fmt.Errorf("acpmock: %s.values must contain at least one value", path)
	}
	seen := make(map[string]struct{}, len(o.Values))
	currentFound := false
	for idx, value := range o.Values {
		trimmed := strings.TrimSpace(value.Value)
		if trimmed == "" {
			return fmt.Errorf("acpmock: %s.values[%d].value is required", path, idx)
		}
		if _, exists := seen[trimmed]; exists {
			return fmt.Errorf("acpmock: %s.values[%d].value duplicates %q", path, idx, trimmed)
		}
		seen[trimmed] = struct{}{}
		if trimmed == current {
			currentFound = true
		}
	}
	if !currentFound {
		return fmt.Errorf("acpmock: %s.current %q must be listed in values", path, current)
	}
	return nil
}

// Validate ensures the turn match contains at least one supported stable selector.
func (m TurnMatch) Validate(path string) error {
	normalized := m.Normalize()
	switch normalized.TurnSource {
	case "", acp.PromptTurnSourceUser, acp.PromptTurnSourceNetwork, acp.PromptTurnSourceSynthetic:
	default:
		return fmt.Errorf("acpmock: %s.turn_source %q is invalid", path, normalized.TurnSource)
	}
	if normalized.TurnSource == "" && normalized.UserText == "" && normalized.Network == nil &&
		normalized.Goal == nil && normalized.Judge == nil {
		return fmt.Errorf("acpmock: %s requires at least one stable selector", path)
	}
	if normalized.Network != nil {
		if err := normalized.Network.Validate(path + ".network"); err != nil {
			return err
		}
	}
	if normalized.Goal != nil {
		if err := normalized.Goal.Validate(path + ".goal"); err != nil {
			return err
		}
	}
	if normalized.Judge != nil {
		if err := normalized.Judge.Validate(path + ".judge"); err != nil {
			return err
		}
	}
	return nil
}

// Normalize returns a trimmed copy of the turn matcher.
func (m TurnMatch) Normalize() TurnMatch {
	normalized := TurnMatch{
		TurnSource: strings.TrimSpace(m.TurnSource),
		UserText:   strings.TrimSpace(m.UserText),
	}
	if m.Goal != nil {
		goal := m.Goal.Normalize()
		if !goal.IsZero() {
			normalized.Goal = &goal
		}
	}
	if m.Judge != nil {
		judge := m.Judge.Normalize()
		if !judge.IsZero() {
			normalized.Judge = &judge
		}
	}
	if m.Network != nil {
		network := m.Network.Normalize()
		if !network.IsZero() {
			normalized.Network = &network
		}
	}
	return normalized
}

type turnMatchInput struct {
	UserText string
	Meta     acp.PromptMeta
}

func (m TurnMatch) matches(input turnMatchInput) bool {
	normalized := m.Normalize()
	if normalized.TurnSource != "" && input.Meta.Normalize().TurnSource != normalized.TurnSource {
		return false
	}
	if normalized.UserText != "" && canonicalUserText(input.UserText) != normalized.UserText {
		return false
	}
	if normalized.Network != nil {
		if input.Meta.Network == nil {
			return false
		}
		if !normalized.Network.matches(*input.Meta.Network) {
			return false
		}
	}
	if normalized.Goal != nil {
		meta := input.Meta.Normalize()
		if meta.Synthetic == nil || meta.Synthetic.Goal == nil ||
			!normalized.Goal.matches(*meta.Synthetic.Goal) {
			return false
		}
	}
	if normalized.Judge != nil {
		meta := input.Meta.Normalize()
		if meta.Judge == nil || !normalized.Judge.matches(*meta.Judge) {
			return false
		}
	}
	return true
}
