package loop

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// EffectiveChecksJSON stores immutable resolved check configuration compactly.
type EffectiveChecksJSON string

func newEffectiveChecksJSON(value json.RawMessage) *EffectiveChecksJSON {
	compact := EffectiveChecksJSON(string(value))
	return &compact
}

func effectiveChecksBytes(value *EffectiveChecksJSON) json.RawMessage {
	if value == nil {
		return nil
	}
	return value.Bytes()
}

// Bytes returns the raw JSON representation expected by JSON consumers.
func (v EffectiveChecksJSON) Bytes() json.RawMessage {
	if v == "" {
		return nil
	}
	return json.RawMessage(v)
}

// MarshalJSON preserves the raw-object wire contract.
func (v EffectiveChecksJSON) MarshalJSON() ([]byte, error) {
	if v == "" {
		return []byte("null"), nil
	}
	raw := []byte(v)
	if !json.Valid(raw) {
		return nil, fmt.Errorf("loop: marshal enabled checks: invalid JSON")
	}
	return raw, nil
}

// UnmarshalJSON validates and stores one immutable raw JSON value.
func (v *EffectiveChecksJSON) UnmarshalJSON(data []byte) error {
	if v == nil {
		return fmt.Errorf("loop: unmarshal enabled checks into nil receiver")
	}
	raw := bytes.TrimSpace(data)
	if bytes.Equal(raw, []byte("null")) {
		*v = ""
		return nil
	}
	if !json.Valid(raw) {
		return fmt.Errorf("loop: unmarshal enabled checks: invalid JSON")
	}
	*v = EffectiveChecksJSON(string(raw))
	return nil
}
