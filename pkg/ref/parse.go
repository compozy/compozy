package ref

import (
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// String Parsing
// -----------------------------------------------------------------------------

// parseStringRef parses a reference string into a Ref.
func parseStringRef(refStr string) (*Ref, error) {
	if refStr == "" {
		return &Ref{Type: TypeProperty, Mode: ModeMerge}, nil
	}

	ref := &Ref{Mode: ModeMerge}

	// Extract mode if present
	modeIdx := strings.LastIndex(refStr, "!")
	if modeIdx != -1 && modeIdx < len(refStr)-1 {
		mode := refStr[modeIdx+1:]
		if err := validateMode(mode); err != nil {
			return nil, err
		}
		ref.Mode = Mode(mode)
		refStr = refStr[:modeIdx]
	}

	// Handle global reference
	if strings.HasPrefix(refStr, "$global") {
		ref.Type = TypeGlobal
		if strings.HasPrefix(refStr, "$global::") {
			ref.Path = strings.TrimPrefix(refStr, "$global::")
		}
		return ref, nil
	}

	// Check for file reference (contains :: or is a file path)
	if idx := strings.Index(refStr, "::"); idx != -1 {
		source := refStr[:idx]
		ref.Path = refStr[idx+2:]

		if isFileSource(source) {
			ref.Type = TypeFile
			ref.File = source
		} else {
			// If source is not a file, it's a property reference with the full path
			ref.Type = TypeProperty
			ref.Path = refStr
		}
	} else if isFileSource(refStr) {
		// Just a file reference without path
		ref.Type = TypeFile
		ref.File = refStr
	} else {
		// Simple property reference
		ref.Type = TypeProperty
		ref.Path = refStr
	}

	return ref, nil
}

// isFileSource checks if the source is a file path or URL.
func isFileSource(source string) bool {
	return strings.HasPrefix(source, "./") ||
		strings.HasPrefix(source, "../") ||
		strings.HasPrefix(source, "/") ||
		strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://")
}

// validateMode validates that the mode is one of the allowed values.
func validateMode(mode string) error {
	switch Mode(mode) {
	case ModeMerge, ModeReplace, ModeAppend:
		return nil
	default:
		return errors.Errorf("invalid mode: %s", mode)
	}
}

// -----------------------------------------------------------------------------
// YAML Node Parsing
// -----------------------------------------------------------------------------

// ParseRef parses a YAML node into a Ref.
func ParseRef(node *yaml.Node) (*Ref, error) {
	if node.Kind == yaml.ScalarNode {
		return parseStringRef(node.Value)
	}
	if node.Kind != yaml.MappingNode {
		return nil, &Error{
			Message: "$ref must be a string or object",
			Line:    node.Line,
			Column:  node.Column,
		}
	}

	ref := &Ref{Mode: ModeMerge}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1].Value

		if err := setRefField(ref, key, value); err != nil {
			return nil, &Error{
				Message: err.Error(),
				Line:    node.Content[i].Line,
				Column:  node.Content[i].Column,
			}
		}
	}

	if ref.Type == "" {
		return nil, &Error{
			Message: "type is required for object form $ref",
			Line:    node.Line,
			Column:  node.Column,
		}
	}

	if err := validateRefFields(ref); err != nil {
		return nil, &Error{
			Message: err.Error(),
			Line:    node.Line,
			Column:  node.Column,
		}
	}

	return ref, nil
}

// setRefField sets a field on the Ref struct based on key-value pair.
func setRefField(ref *Ref, key, value string) error {
	switch key {
	case "type":
		switch value {
		case "property", string(TypeFile), "global":
			ref.Type = Type(value)
		default:
			return errors.Errorf("unknown ref type: %s", value)
		}
	case "path":
		ref.Path = value
	case "mode":
		if err := validateMode(value); err != nil {
			return err
		}
		ref.Mode = Mode(value)
	case "file":
		ref.File = value
	default:
		return errors.Errorf("unknown field '%s' for $ref", key)
	}
	return nil
}

// validateRefFields validates the fields of a Ref based on its type.
func validateRefFields(ref *Ref) error {
	switch ref.Type {
	case TypeProperty:
		if ref.File != "" {
			return errors.New("property type cannot have file field")
		}
		if ref.Path == "" {
			return errors.New("path is required for property type")
		}
	case TypeFile:
		if ref.File == "" {
			return errors.New("file type requires file field")
		}
		if !isFileSource(ref.File) {
			return errors.Errorf("invalid file path: %s", ref.File)
		}
	case TypeGlobal:
		if ref.File != "" {
			return errors.New("global type cannot have file field")
		}
	}
	return nil
}

// parseRefValue parses a reference value (string or object) into a Ref.
func parseRefValue(refValue any) (*Ref, error) {
	switch v := refValue.(type) {
	case string:
		return parseStringRef(v)
	case map[string]any:
		// Convert map to Ref struct
		ref := &Ref{Mode: ModeMerge}

		if typeVal, ok := v["type"].(string); ok {
			if err := setRefField(ref, "type", typeVal); err != nil {
				return nil, err
			}
		} else {
			ref.Type = TypeProperty // Default type
		}

		if path, ok := v["path"].(string); ok {
			ref.Path = path
		}

		if file, ok := v["file"].(string); ok {
			ref.File = file
		}

		if mode, ok := v["mode"].(string); ok {
			if err := setRefField(ref, "mode", mode); err != nil {
				return nil, err
			}
		}

		if err := validateRefFields(ref); err != nil {
			return nil, err
		}

		return ref, nil
	default:
		return nil, nil
	}
}
