package contentblock

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Envelope stores one typed content-block payload in canonical JSON form.
type Envelope[T ~string] struct {
	Type T
	Data json.RawMessage
}

// ValidatePayload rejects nil payloads before they reach encoding/json.
func ValidatePayload(block any) error {
	if block == nil {
		return fmt.Errorf("marshal content block: nil payload")
	}

	value := reflect.ValueOf(block)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return fmt.Errorf("marshal content block: nil %T", block)
	}
	return nil
}

// MarshalEnvelope encodes one typed block and returns its canonical envelope data.
func MarshalEnvelope[T ~string](block any) (Envelope[T], error) {
	data, err := json.Marshal(block)
	if err != nil {
		return Envelope[T]{}, fmt.Errorf("marshal content block payload: %w", err)
	}

	var envelope struct {
		Type T `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope[T]{}, fmt.Errorf("decode marshaled content block envelope: %w", err)
	}
	if envelope.Type == "" {
		return Envelope[T]{}, fmt.Errorf("marshal content block payload: missing type")
	}

	return Envelope[T]{
		Type: envelope.Type,
		Data: data,
	}, nil
}

// MarshalEnvelopeJSON preserves the canonical JSON payload stored in an envelope.
func MarshalEnvelopeJSON[T ~string](blockType T, data json.RawMessage) ([]byte, error) {
	if blockType == "" {
		return nil, fmt.Errorf("marshal content block: missing type")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("marshal %s block: missing data", blockType)
	}
	return data, nil
}

// UnmarshalEnvelopeJSON validates one content-block payload and stores its canonical JSON form.
func UnmarshalEnvelopeJSON[T ~string](
	data []byte,
	validate func(T, []byte) error,
) (Envelope[T], error) {
	var envelope struct {
		Type T `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope[T]{}, fmt.Errorf("decode content block envelope: %w", err)
	}
	if envelope.Type == "" {
		return Envelope[T]{}, fmt.Errorf("decode content block envelope: missing type")
	}
	if err := validate(envelope.Type, data); err != nil {
		return Envelope[T]{}, err
	}

	return Envelope[T]{
		Type: envelope.Type,
		Data: append(json.RawMessage(nil), data...),
	}, nil
}

// DecodeBlock unmarshals one typed content-block payload and enforces its expected type.
func DecodeBlock[T any, B ~string](
	data []byte,
	expected B,
	blockType func(T) B,
	normalize func(*T, B),
) (T, error) {
	var block T
	if err := json.Unmarshal(data, &block); err != nil {
		var zero T
		return zero, fmt.Errorf("decode %s block: %w", expected, err)
	}

	if got := blockType(block); got != expected {
		var zero T
		return zero, fmt.Errorf("decode %s block: unexpected type %q", expected, got)
	}

	if normalize != nil {
		normalize(&block, expected)
	}
	return block, nil
}
