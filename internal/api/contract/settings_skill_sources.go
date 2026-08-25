package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// OptionalStringList preserves absent, null, empty, and populated JSON list states.
type OptionalStringList struct {
	Present bool     `json:"-"`
	Null    bool     `json:"-"`
	Value   []string `json:"-"`
}

// UnmarshalJSON records whether the field was explicitly cleared or replaced.
func (o *OptionalStringList) UnmarshalJSON(data []byte) error {
	if o == nil {
		return errors.New("optional string list is nil")
	}
	o.Present = true
	o.Null = bytes.Equal(bytes.TrimSpace(data), []byte("null"))
	o.Value = nil
	if o.Null {
		return nil
	}
	if err := json.Unmarshal(data, &o.Value); err != nil {
		return fmt.Errorf("decode string list: %w", err)
	}
	return nil
}

// MarshalJSON writes null or the replacement list for a present field.
func (o OptionalStringList) MarshalJSON() ([]byte, error) {
	if o.Null || !o.Present {
		return []byte("null"), nil
	}
	data, err := json.Marshal(o.Value)
	if err != nil {
		return nil, fmt.Errorf("encode string list: %w", err)
	}
	return data, nil
}

// SettingsSkillsOverridePayload carries independent workspace source overrides.
type SettingsSkillsOverridePayload struct {
	Sources       OptionalStringList `json:"sources,omitempty"`
	CustomSources OptionalStringList `json:"custom_sources,omitempty"`
}

// MarshalJSON omits untouched overlay fields while preserving explicit null.
func (p SettingsSkillsOverridePayload) MarshalJSON() ([]byte, error) {
	fields := make(map[string]OptionalStringList, 2)
	if p.Sources.Present {
		fields["sources"] = p.Sources
	}
	if p.CustomSources.Present {
		fields["custom_sources"] = p.CustomSources
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode skills override: %w", err)
	}
	return data, nil
}

// SkillSourceValidationErrorPayload is one portable skill-source policy failure.
type SkillSourceValidationErrorPayload struct {
	Code           string   `json:"code"`
	Message        string   `json:"message"`
	Field          string   `json:"field,omitempty"`
	Path           string   `json:"path,omitempty"`
	ExistingSource string   `json:"existing_source,omitempty"`
	Valid          []string `json:"valid,omitempty"`
	Suggestion     string   `json:"suggestion,omitempty"`
}

// SkillSourceValidationErrorResponse wraps a portable skill-source failure.
type SkillSourceValidationErrorResponse struct {
	Error SkillSourceValidationErrorPayload `json:"error"`
}

// MarshalJSON emits the scope-specific skills request shape without a zero-value sibling object.
func (r UpdateSettingsSkillsRequest) MarshalJSON() ([]byte, error) {
	if r.Override != nil {
		data, err := json.Marshal(struct {
			Override SettingsSkillsOverridePayload `json:"override"`
		}{Override: *r.Override})
		if err != nil {
			return nil, fmt.Errorf("encode workspace skills update: %w", err)
		}
		return data, nil
	}
	data, err := json.Marshal(struct {
		Config SettingsSkillsConfigPayload `json:"config"`
	}{Config: r.Config})
	if err != nil {
		return nil, fmt.Errorf("encode skills update: %w", err)
	}
	return data, nil
}
