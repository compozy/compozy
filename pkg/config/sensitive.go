package config

import (
	"encoding/json"
)

// SensitiveString is a string type that redacts its value in logs and JSON.
type SensitiveString string

// String implements the fmt.Stringer interface to prevent accidental logging.
func (s SensitiveString) String() string {
	if s == "" {
		return ""
	}
	return "[REDACTED]"
}

// MarshalJSON implements the json.Marshaler interface.
func (s SensitiveString) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *SensitiveString) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*s = SensitiveString(str)
	return nil
}

// Value returns the actual string value. Use this method only when you
// explicitly need to pass the secret to another service.
func (s SensitiveString) Value() string {
	return string(s)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// This is needed for koanf to properly unmarshal environment variables.
func (s *SensitiveString) UnmarshalText(text []byte) error {
	*s = SensitiveString(text)
	return nil
}
