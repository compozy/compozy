package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
)

// UnmarshalJSON preserves whether mode was supplied and rejects identity bytes
// that encoding/json would otherwise replace with the Unicode replacement rune.
func (r *BridgeDeliveryTargetInput) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("api contract: bridge delivery target must contain valid UTF-8")
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*r = BridgeDeliveryTargetInput{}
		return nil
	}

	type wireTarget BridgeDeliveryTargetInput
	var decoded wireTarget
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("api contract: decode bridge delivery target: %w", err)
	}

	if err := inspectBridgeTargetJSONFields(data, func(name string, raw json.RawMessage) error {
		switch {
		case strings.EqualFold(name, "mode"):
			return validateBridgeTargetModeJSON(raw)
		case strings.EqualFold(name, "bridge_instance_id"):
			return validateBridgeTargetJSONString(raw, "bridge instance id")
		case strings.EqualFold(name, "peer_id"):
			return validateBridgeTargetJSONString(raw, "peer id")
		case strings.EqualFold(name, "thread_id"):
			return validateBridgeTargetJSONString(raw, "thread id")
		case strings.EqualFold(name, "group_id"):
			return validateBridgeTargetJSONString(raw, "group id")
		}
		return nil
	}); err != nil {
		return err
	}

	*r = BridgeDeliveryTargetInput(decoded)
	return nil
}

func inspectBridgeTargetJSONFields(
	data []byte,
	inspect func(string, json.RawMessage) error,
) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("api contract: read bridge delivery target object: %w", err)
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return errors.New("api contract: bridge delivery target must be a JSON object")
	}
	for decoder.More() {
		member, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("api contract: read bridge delivery target member: %w", err)
		}
		name, ok := member.(string)
		if !ok {
			return errors.New("api contract: bridge delivery target member name must be a string")
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return fmt.Errorf("api contract: read bridge delivery target member %q: %w", name, err)
		}
		if err := inspect(name, raw); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("api contract: close bridge delivery target object: %w", err)
	}
	return nil
}

func validateBridgeTargetModeJSON(raw json.RawMessage) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return bridgepkg.DeliveryMode("").Validate()
	}
	var mode bridgepkg.DeliveryMode
	if err := json.Unmarshal(raw, &mode); err != nil {
		return fmt.Errorf("api contract: bridge delivery target mode must be a string: %w", err)
	}
	return mode.Validate()
}

func validateBridgeTargetJSONString(raw json.RawMessage, label string) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("api contract: bridge delivery target %s must be a string: %w", label, err)
	}
	if err := validateJSONSurrogatePairs(raw); err != nil {
		return fmt.Errorf("api contract: bridge delivery target %s must contain valid Unicode: %w", label, err)
	}
	return nil
}

func validateJSONSurrogatePairs(raw []byte) error {
	for index := 1; index < len(raw)-1; index++ {
		if raw[index] != '\\' {
			continue
		}
		index++
		if index >= len(raw)-1 || raw[index] != 'u' {
			continue
		}

		codeUnit, ok := decodeJSONHexCodeUnit(raw[index+1:])
		if !ok {
			return errors.New("invalid Unicode escape")
		}
		index += 4
		switch {
		case codeUnit >= 0xD800 && codeUnit <= 0xDBFF:
			if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
				return errors.New("unpaired high surrogate")
			}
			low, valid := decodeJSONHexCodeUnit(raw[index+3:])
			if !valid || low < 0xDC00 || low > 0xDFFF {
				return errors.New("unpaired high surrogate")
			}
			index += 6
		case codeUnit >= 0xDC00 && codeUnit <= 0xDFFF:
			return errors.New("unpaired low surrogate")
		}
	}
	return nil
}

func decodeJSONHexCodeUnit(raw []byte) (uint16, bool) {
	if len(raw) < 4 {
		return 0, false
	}
	var value uint16
	for _, char := range raw[:4] {
		value <<= 4
		switch {
		case char >= '0' && char <= '9':
			value += uint16(char - '0')
		case char >= 'a' && char <= 'f':
			value += uint16(char-'a') + 10
		case char >= 'A' && char <= 'F':
			value += uint16(char-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}
