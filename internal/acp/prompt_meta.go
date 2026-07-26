package acp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// PromptTurnSourceUser identifies a daemon prompt that originated from the
	// user-facing prompt surfaces.
	PromptTurnSourceUser = "user"
	// PromptTurnSourceNetwork identifies a daemon prompt that originated from an
	// AGH network envelope delivery.
	PromptTurnSourceNetwork = "network"
	// PromptTurnSourceSynthetic identifies a daemon-owned prompt turn injected by
	// internal runtime code.
	PromptTurnSourceSynthetic = "synthetic"
)

// PromptMeta carries structured, transport-stable metadata for one ACP prompt.
type PromptMeta struct {
	TurnSource string               `json:"turn_source,omitempty"`
	Network    *PromptNetworkMeta   `json:"network,omitempty"`
	Synthetic  *PromptSyntheticMeta `json:"synthetic,omitempty"`
	Judge      *PromptJudgeMeta     `json:"judge,omitempty"`
	System     *PromptSystemMeta    `json:"system,omitempty"`
}

// PromptSystemMeta captures daemon-owned prompt delivery metadata.
type PromptSystemMeta struct {
	PromptDelivery string `json:"prompt_delivery,omitempty"`
}

// PromptNetworkMeta captures stable AGH network envelope correlation fields.
type PromptNetworkMeta struct {
	MessageID             string   `json:"message_id,omitempty"`
	Kind                  string   `json:"kind,omitempty"`
	Channel               string   `json:"channel,omitempty"`
	Surface               string   `json:"surface,omitempty"`
	ThreadID              string   `json:"thread_id,omitempty"`
	DirectID              string   `json:"direct_id,omitempty"`
	From                  string   `json:"from,omitempty"`
	To                    string   `json:"to,omitempty"`
	Mentions              []string `json:"mentions,omitempty"`
	WorkID                string   `json:"work_id,omitempty"`
	ReplyTo               string   `json:"reply_to,omitempty"`
	TraceID               string   `json:"trace_id,omitempty"`
	CausationID           string   `json:"causation_id,omitempty"`
	Trust                 string   `json:"trust,omitempty"`
	DeliveryMode          string   `json:"delivery_mode,omitempty"`
	PromptSizeBytes       int64    `json:"prompt_size_bytes,omitempty"`
	EstimatedPromptTokens int64    `json:"estimated_prompt_tokens,omitempty"`
}

// Normalize returns a trimmed copy of the prompt metadata.
func (m PromptMeta) Normalize() PromptMeta {
	normalized := PromptMeta{
		TurnSource: strings.TrimSpace(m.TurnSource),
	}
	if m.Network != nil {
		network := m.Network.Normalize()
		if !network.IsZero() {
			normalized.Network = &network
		}
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
		normalized.Network == nil &&
		normalized.Synthetic == nil &&
		normalized.Judge == nil &&
		normalized.System == nil
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
	switch normalized.TurnSource {
	case "", PromptTurnSourceUser:
		if normalized.Network != nil || normalized.Synthetic != nil {
			return errors.New("acp: user prompt metadata cannot include network or synthetic fields")
		}
		return nil
	case PromptTurnSourceNetwork:
		if normalized.Synthetic != nil {
			return errors.New("acp: network prompt metadata cannot include synthetic fields")
		}
		if normalized.Judge != nil {
			return errors.New("acp: network prompt metadata cannot include judge fields")
		}
		return nil
	case PromptTurnSourceSynthetic:
		if normalized.Network != nil {
			return errors.New("acp: synthetic prompt metadata cannot include network fields")
		}
		if normalized.Judge != nil {
			return errors.New("acp: synthetic prompt metadata cannot include judge fields")
		}
		if normalized.Synthetic == nil {
			return errors.New("acp: synthetic prompt metadata requires synthetic fields")
		}
		return normalized.Synthetic.Validate()
	default:
		return fmt.Errorf("acp: invalid prompt turn source %q", normalized.TurnSource)
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
		return fmt.Errorf("acp: invalid system prompt delivery %q", normalized.PromptDelivery)
	}
}

// Normalize returns a trimmed copy of the network metadata.
func (m PromptNetworkMeta) Normalize() PromptNetworkMeta {
	return PromptNetworkMeta{
		MessageID:             strings.TrimSpace(m.MessageID),
		Kind:                  strings.TrimSpace(m.Kind),
		Channel:               strings.TrimSpace(m.Channel),
		Surface:               strings.TrimSpace(m.Surface),
		ThreadID:              strings.TrimSpace(m.ThreadID),
		DirectID:              strings.TrimSpace(m.DirectID),
		From:                  strings.TrimSpace(m.From),
		To:                    strings.TrimSpace(m.To),
		Mentions:              normalizePromptMetaStrings(m.Mentions),
		WorkID:                strings.TrimSpace(m.WorkID),
		ReplyTo:               strings.TrimSpace(m.ReplyTo),
		TraceID:               strings.TrimSpace(m.TraceID),
		CausationID:           strings.TrimSpace(m.CausationID),
		Trust:                 strings.TrimSpace(m.Trust),
		DeliveryMode:          strings.TrimSpace(m.DeliveryMode),
		PromptSizeBytes:       max(m.PromptSizeBytes, 0),
		EstimatedPromptTokens: max(m.EstimatedPromptTokens, 0),
	}
}

// IsZero reports whether the network metadata carries any fields.
func (m PromptNetworkMeta) IsZero() bool {
	normalized := m.Normalize()
	return normalized.MessageID == "" &&
		normalized.Kind == "" &&
		normalized.Channel == "" &&
		normalized.Surface == "" &&
		normalized.ThreadID == "" &&
		normalized.DirectID == "" &&
		normalized.From == "" &&
		normalized.To == "" &&
		len(normalized.Mentions) == 0 &&
		normalized.WorkID == "" &&
		normalized.ReplyTo == "" &&
		normalized.TraceID == "" &&
		normalized.CausationID == "" &&
		normalized.Trust == "" &&
		normalized.DeliveryMode == "" &&
		normalized.PromptSizeBytes == 0 &&
		normalized.EstimatedPromptTokens == 0
}

func normalizePromptMetaStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
