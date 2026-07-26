package tools

import (
	"encoding/json"
	"strings"
)

const maxSegmentedIDLength = 64

// ToolID is the canonical public tool identity.
type ToolID string

// Validate ensures the tool id follows the canonical grammar.
func (id ToolID) Validate() error {
	return validateSegmentedID("tool_id", string(id))
}

// String returns the canonical string value.
func (id ToolID) String() string {
	return string(id)
}

// Segments returns a copy of the namespace segments.
func (id ToolID) Segments() ([]string, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return strings.Split(string(id), "__"), nil
}

// Namespace returns the leading namespace segment.
func (id ToolID) Namespace() (string, error) {
	segments, err := id.Segments()
	if err != nil {
		return "", err
	}
	return segments[0], nil
}

// MarshalText encodes the validated id.
func (id ToolID) MarshalText() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return []byte(id), nil
}

// UnmarshalText decodes and validates an id.
func (id *ToolID) UnmarshalText(text []byte) error {
	candidate := ToolID(strings.TrimSpace(string(text)))
	if err := candidate.Validate(); err != nil {
		return err
	}
	*id = candidate
	return nil
}

// ToolsetID is the canonical public toolset identity.
type ToolsetID string

// Validate ensures the toolset id follows the canonical grammar.
func (id ToolsetID) Validate() error {
	return validateSegmentedID("toolset_id", string(id))
}

// String returns the canonical string value.
func (id ToolsetID) String() string {
	return string(id)
}

// MarshalText encodes the validated id.
func (id ToolsetID) MarshalText() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return []byte(id), nil
}

// UnmarshalText decodes and validates an id.
func (id *ToolsetID) UnmarshalText(text []byte) error {
	candidate := ToolsetID(strings.TrimSpace(string(text)))
	if err := candidate.Validate(); err != nil {
		return err
	}
	*id = candidate
	return nil
}

func validateSegmentedID(field string, value string) error {
	switch {
	case value == "":
		return NewValidationError(field, ReasonIDEmpty, "id is required")
	case len(value) > maxSegmentedIDLength:
		return NewValidationError(field, ReasonIDTooLong, "id exceeds 64 characters")
	case strings.Contains(value, "___"):
		return NewValidationError(field, ReasonIDReservedConflict, "reserved separator is ambiguous")
	}

	segments := strings.SplitSeq(value, "__")
	for segment := range segments {
		if segment == "" {
			return NewValidationError(field, ReasonIDEmptySegment, "id contains an empty segment")
		}
		if strings.HasPrefix(segment, "_") || strings.HasSuffix(segment, "_") {
			return NewValidationError(field, ReasonIDReservedConflict, "segment uses reserved underscore boundary")
		}
		for i, r := range segment {
			if i == 0 {
				if r < 'a' || r > 'z' {
					return NewValidationError(
						field,
						ReasonIDInvalidFormat,
						"segment must start with a lowercase letter",
					)
				}
				continue
			}
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
				continue
			}
			return NewValidationError(field, ReasonIDInvalidFormat, "segment contains an unsupported character")
		}
	}
	return nil
}

// CanonicalIDSegment normalizes one external name into a provider-safe segment.
func CanonicalIDSegment(raw string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastUnderscore = false
		default:
			if builder.Len() > 0 && !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	segment := strings.Trim(builder.String(), "_")
	if err := validateSegmentedID("segment", segment); err != nil {
		return "", err
	}
	if strings.Contains(segment, "__") {
		return "", NewValidationError("segment", ReasonIDReservedConflict, "segment contains reserved separator")
	}
	return segment, nil
}

// CanonicalToolID builds a ToolID from raw namespace segments.
func CanonicalToolID(namespace string, segments ...string) (ToolID, error) {
	all := append([]string{namespace}, segments...)
	normalized := make([]string, 0, len(all))
	for _, segment := range all {
		canonical, err := CanonicalIDSegment(segment)
		if err != nil {
			return "", err
		}
		normalized = append(normalized, canonical)
	}
	id := ToolID(strings.Join(normalized, "__"))
	if err := id.Validate(); err != nil {
		return "", err
	}
	return id, nil
}

// ValidateJSONObject validates a JSON Schema object payload.
func ValidateJSONObject(field string, raw json.RawMessage, required bool) error {
	if len(raw) == 0 {
		if required {
			return NewValidationError(field, ReasonSchemaInvalid, "schema object is required")
		}
		return nil
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return NewValidationError(field, ReasonSchemaInvalid, err.Error())
	}
	if decoded == nil {
		if required {
			return NewValidationError(field, ReasonSchemaInvalid, "schema must be a JSON object")
		}
		return nil
	}
	if err := validateJSONSchemaDocument("$", decoded); err != nil {
		return NewValidationError(field, ReasonSchemaInvalid, err.Error())
	}
	return nil
}
