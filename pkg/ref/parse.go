package ref

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// String Parsing
// -----------------------------------------------------------------------------

var refPattern = regexp.MustCompile(`^(?:(?P<source>.+?)::)?(?P<path>[^!]*)(?:!(?P<mode>.+))?$`)

// parseStringRef parses a reference string into a Ref.
func parseStringRef(refStr string) (*Ref, error) {
	ref := &Ref{Mode: ModeMerge}
	matches := refPattern.FindStringSubmatch(refStr)
	if matches == nil {
		return nil, errors.New("invalid reference format: " + refStr)
	}
	source := matches[1]
	ref.Path = matches[2]
	if mode := matches[3]; mode != "" {
		ref.Mode = Mode(mode)
		if ref.Mode != ModeMerge && ref.Mode != ModeReplace && ref.Mode != ModeAppend {
			return nil, errors.New("invalid mode: " + string(ref.Mode))
		}
	}
	switch {
	case source == "$global":
		ref.Type = TypeGlobal
	case isFileSource(source):
		ref.Type = TypeFile
		ref.File = source
	default:
		ref.Type = TypeProperty
		if source != "" {
			ref.Path = source
		}
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
		switch key {
		case "type":
			ref.Type = Type(value)
		case "path":
			ref.Path = value
		case "mode":
			ref.Mode = Mode(value)
		case "file":
			ref.File = value
		default:
			return nil, &Error{
				Message: fmt.Sprintf("unknown field '%s' for $ref", key),
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
	if err := validateRefFields(ref, node); err != nil {
		return nil, err
	}
	return ref, nil
}

// validateRefFields validates the fields of a Ref based on its type.
func validateRefFields(ref *Ref, node *yaml.Node) error {
	switch ref.Type {
	case TypeProperty:
		if ref.File != "" {
			return &Error{
				Message: "property type cannot have file field",
				Line:    node.Line,
				Column:  node.Column,
			}
		}
		if ref.Path == "" {
			return &Error{
				Message: "path is required for property type",
				Line:    node.Line,
				Column:  node.Column,
			}
		}
	case TypeFile:
		if ref.File == "" {
			return &Error{
				Message: "file type requires file field",
				Line:    node.Line,
				Column:  node.Column,
			}
		}
		if !isFileSource(ref.File) {
			return &Error{
				Message: fmt.Sprintf("invalid file path: %s", ref.File),
				Line:    node.Line,
				Column:  node.Column,
			}
		}
	case TypeGlobal:
		if ref.File != "" {
			return &Error{
				Message: "global type cannot have file field",
				Line:    node.Line,
				Column:  node.Column,
			}
		}
	default:
		return &Error{
			Message: fmt.Sprintf("unknown ref type: %s", ref.Type),
			Line:    node.Line,
			Column:  node.Column,
		}
	}
	if ref.Mode != "" && ref.Mode != ModeMerge && ref.Mode != ModeReplace && ref.Mode != ModeAppend {
		return &Error{
			Message: fmt.Sprintf("invalid mode: %s", ref.Mode),
			Line:    node.Line,
			Column:  node.Column,
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
		yamlData, err := yaml.Marshal(v)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal ref")
		}
		var node yaml.Node
		if err := yaml.Unmarshal(yamlData, &node); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal ref")
		}
		return ParseRef(&node)
	default:
		return nil, nil
	}
}
