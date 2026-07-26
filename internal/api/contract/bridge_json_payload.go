package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

// ErrUnsafeBridgeProviderConfigPayload reports raw credentials in bridge provider config JSON.
var ErrUnsafeBridgeProviderConfigPayload = bridgeContractError(
	"bridge provider config contains unsafe token or secret data",
)

// BridgeProviderConfigPayload carries provider-owned runtime configuration.
type BridgeProviderConfigPayload json.RawMessage

// MarshalJSON preserves the compact raw JSON representation of provider config.
func (p BridgeProviderConfigPayload) MarshalJSON() ([]byte, error) {
	return marshalBridgeJSONPayload(
		json.RawMessage(p),
		"bridge provider config",
		validateBridgeProviderConfigPayload,
	)
}

// UnmarshalJSON validates provider config before storing its compact representation.
func (p *BridgeProviderConfigPayload) UnmarshalJSON(data []byte) error {
	normalized, err := normalizeBridgeJSONPayload(data, "bridge provider config", validateBridgeProviderConfigPayload)
	if err != nil {
		return err
	}
	*p = BridgeProviderConfigPayload(normalized)
	return nil
}

// BridgeProgressConfigPayload is the public progress block under delivery_defaults.
type BridgeProgressConfigPayload struct {
	ToolProgress bridgepkg.ProgressMode     `json:"tool_progress"`
	Grouping     bridgepkg.ProgressGrouping `json:"grouping"`
	Typing       bool                       `json:"typing"`
	Reactions    bool                       `json:"reactions"`
}

// Validate reports whether the progress values belong to the public taxonomy.
func (p BridgeProgressConfigPayload) Validate() error {
	return p.ProgressConfig().Validate()
}

// ProgressConfig converts the public payload to the daemon-owned configuration.
func (p BridgeProgressConfigPayload) ProgressConfig() bridgepkg.ProgressConfig {
	return bridgepkg.ProgressConfig{
		ToolProgress: p.ToolProgress,
		Grouping:     p.Grouping,
		Typing:       p.Typing,
		Reactions:    p.Reactions,
	}
}

// BridgeProgressConfigPayloadFromConfig converts effective daemon settings to the public shape.
func BridgeProgressConfigPayloadFromConfig(config bridgepkg.ProgressConfig) BridgeProgressConfigPayload {
	return BridgeProgressConfigPayload{
		ToolProgress: config.ToolProgress,
		Grouping:     config.Grouping,
		Typing:       config.Typing,
		Reactions:    config.Reactions,
	}
}

// BridgeDeliveryDefaultsPayload carries common target/progress fields plus
// provider-owned string defaults in one normalized object.
type BridgeDeliveryDefaultsPayload json.RawMessage

// MarshalJSON validates and emits the canonical delivery-default representation.
func (p BridgeDeliveryDefaultsPayload) MarshalJSON() ([]byte, error) {
	normalized, err := normalizeBridgeDeliveryDefaultsPayload(json.RawMessage(p))
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return []byte("null"), nil
	}
	return normalized, nil
}

// UnmarshalJSON validates delivery defaults before storing their canonical representation.
func (p *BridgeDeliveryDefaultsPayload) UnmarshalJSON(data []byte) error {
	normalized, err := normalizeBridgeDeliveryDefaultsPayload(data)
	if err != nil {
		return err
	}
	*p = BridgeDeliveryDefaultsPayload(normalized)
	return nil
}

func marshalBridgeJSONPayload(
	value json.RawMessage,
	label string,
	validate func(json.RawMessage) error,
) ([]byte, error) {
	normalized, err := normalizeBridgeJSONPayload(value, label, validate)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return []byte("null"), nil
	}
	return normalized, nil
}

func normalizeBridgeJSONPayload(
	value json.RawMessage,
	label string,
	validate func(json.RawMessage) error,
) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("%s must be valid JSON", label)
	}

	var compacted bytes.Buffer
	if err := json.Compact(&compacted, trimmed); err != nil {
		return nil, fmt.Errorf("compact %s: %w", label, err)
	}
	normalized := compacted.Bytes()
	if validate != nil {
		if err := validate(normalized); err != nil {
			return nil, err
		}
	}
	return normalized, nil
}

func validateBridgeProviderConfigPayload(value json.RawMessage) error {
	if isJSONNull(value) {
		return nil
	}

	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return fmt.Errorf("bridge provider config must decode as JSON object or null: %w", err)
	}
	if _, ok := decoded.(map[string]any); !ok {
		return errors.New("bridge provider config must be a JSON object or null")
	}
	if containsUnsafePublicContractJSON(value) {
		return ErrUnsafeBridgeProviderConfigPayload
	}
	return nil
}

func normalizeBridgeDeliveryDefaultsPayload(value json.RawMessage) (json.RawMessage, error) {
	normalized, err := normalizeBridgeJSONPayload(value, "bridge delivery defaults", nil)
	if err != nil {
		return nil, err
	}
	return bridgepkg.NormalizeDeliveryDefaultsJSON(normalized)
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}
