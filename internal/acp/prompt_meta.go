package acp

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// PromptTurnSourceUser identifies a daemon prompt that originated from the
	// user-facing prompt surfaces.
	PromptTurnSourceUser = "user"
	// PromptTurnSourceSynthetic identifies a daemon-owned prompt turn injected by
	// internal runtime code.
	PromptTurnSourceSynthetic = "synthetic"
)

// PromptMeta carries structured, transport-stable metadata for one ACP prompt.
type PromptMeta struct {
	Origin     *PromptOriginMeta    `json:"origin,omitempty"`
	TurnSource string               `json:"turn_source,omitempty"`
	Synthetic  *PromptSyntheticMeta `json:"synthetic,omitempty"`
	Judge      *PromptJudgeMeta     `json:"judge,omitempty"`
	System     *PromptSystemMeta    `json:"system,omitempty"`
}

// PromptSystemMeta captures daemon-owned prompt delivery metadata.
type PromptSystemMeta struct {
	PromptDelivery string `json:"prompt_delivery,omitempty"`
}

// Normalize returns a trimmed copy of the prompt metadata.
func (m PromptMeta) Normalize() PromptMeta {
	normalized := PromptMeta{
		TurnSource: strings.TrimSpace(m.TurnSource),
		Origin:     ClonePromptOriginMeta(m.Origin),
	}
	if m.Synthetic != nil {
		synthetic := m.Synthetic.Normalize()
		if !synthetic.IsZero() {
			normalized.Synthetic = &synthetic
		}
	}
	if m.Judge != nil {
		judge := m.Judge.Normalize()
		if !judge.IsZero() {
			normalized.Judge = &judge
		}
	}
	if m.System != nil {
		system := m.System.Normalize()
		if !system.IsZero() {
			normalized.System = &system
		}
	}
	return normalized
}

// IsZero reports whether the prompt metadata carries any fields.
func (m PromptMeta) IsZero() bool {
	normalized := m.Normalize()
	return normalized.TurnSource == "" &&
		normalized.Synthetic == nil &&
		normalized.Judge == nil &&
		normalized.System == nil && normalized.Origin == nil
}

// ToMap converts normalized prompt metadata to the ACP SDK extensibility map.
func (m PromptMeta) ToMap() (map[string]any, error) {
	normalized := m.Normalize()
	if normalized.IsZero() {
		return nil, nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("acp: encode prompt metadata: %w", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, fmt.Errorf("acp: decode prompt metadata map: %w", err)
	}
	return decoded, nil
}

// Validate ensures the metadata shape is internally consistent.
func (m PromptMeta) Validate() error {
	normalized := m.Normalize()
	if normalized.System != nil {
		if err := normalized.System.Validate(); err != nil {
			return err
		}
	}
	if normalized.Judge != nil {
		if err := normalized.Judge.Validate(); err != nil {
			return err
		}
	}
	if normalized.Origin != nil {
		if err := normalized.Origin.Validate(); err != nil {
			return err
		}
		if normalized.TurnSource == PromptTurnSourceSynthetic {
			return invalidPromptMetadata("acp: synthetic prompt cannot include origin")
		}
	}
	switch normalized.TurnSource {
	case "", PromptTurnSourceUser:
		if normalized.Synthetic != nil {
			return invalidPromptMetadata("acp: user prompt metadata cannot include synthetic fields")
		}
		return nil
	case PromptTurnSourceSynthetic:
		if normalized.Judge != nil {
			return invalidPromptMetadata("acp: synthetic prompt metadata cannot include judge fields")
		}
		if normalized.Synthetic == nil {
			return invalidPromptMetadata("acp: synthetic prompt metadata requires synthetic fields")
		}
		return normalized.Synthetic.Validate()
	default:
		return invalidPromptMetadata(fmt.Sprintf("acp: invalid prompt turn source %q", normalized.TurnSource))
	}
}

// Normalize returns a trimmed copy of the system metadata.
func (m PromptSystemMeta) Normalize() PromptSystemMeta {
	return PromptSystemMeta{
		PromptDelivery: strings.TrimSpace(m.PromptDelivery),
	}
}

// IsZero reports whether the system metadata carries any fields.
func (m PromptSystemMeta) IsZero() bool {
	normalized := m.Normalize()
	return normalized == (PromptSystemMeta{})
}

// Validate ensures the system metadata shape is internally consistent.
func (m PromptSystemMeta) Validate() error {
	normalized := m.Normalize()
	switch SystemPromptDeliveryMode(normalized.PromptDelivery) {
	case "", SystemPromptDeliveryFirstTurnPrefix, SystemPromptDeliveryNative:
		return nil
	default:
		return invalidPromptMetadata(fmt.Sprintf("acp: invalid system prompt delivery %q", normalized.PromptDelivery))
	}
}
