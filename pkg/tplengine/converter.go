package tplengine

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// ValueConverter provides methods to convert between different value formats
type ValueConverter struct{}

// YAMLToJSON converts a YAML node to a JSON value
func (c *ValueConverter) YAMLToJSON(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("invalid argument: YAML node is nil")
	}

	switch node.Kind {
	case yaml.ScalarNode:
		return c.scalarToJSON(node)
	case yaml.MappingNode:
		return c.mappingToJSON(node)
	case yaml.SequenceNode:
		return c.sequenceToJSON(node)
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			return c.YAMLToJSON(node.Content[0])
		}
		return nil, nil
	case yaml.AliasNode:
		if node.Alias == nil {
			return nil, fmt.Errorf("invalid argument: YAML alias node has nil target")
		}
		return c.YAMLToJSON(node.Alias)
	default:
		return nil, nil
	}
}

// JSONToYAML converts a JSON value to a YAML node
func (c *ValueConverter) JSONToYAML(value any) (*yaml.Node, error) {
	switch v := value.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}, nil
	case bool:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!bool",
			Value: fmt.Sprintf("%t", v),
		}, nil
	case int:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!int",
			Value: strconv.Itoa(v),
		}, nil
	case int64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!int",
			Value: strconv.FormatInt(v, 10),
		}, nil
	case float64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!float",
			Value: strconv.FormatFloat(v, 'g', -1, 64),
		}, nil
	case string:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: v,
		}, nil
	case []any:
		return c.sliceToYAML(v)
	case map[string]any:
		return c.mapToYAML(v)
	default:
		return nil, fmt.Errorf("invalid argument: unsupported type for YAML conversion: %T", value)
	}
}

// sliceToYAML converts a slice to a YAML sequence node
func (c *ValueConverter) sliceToYAML(slice []any) (*yaml.Node, error) {
	content := make([]*yaml.Node, len(slice))
	node := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Content: content,
	}

	for i, item := range slice {
		itemNode, err := c.JSONToYAML(item)
		if err != nil {
			return nil, err
		}
		content[i] = itemNode
	}

	return node, nil
}

// mapToYAML converts a map to a YAML mapping node
func (c *ValueConverter) mapToYAML(m map[string]any) (*yaml.Node, error) {
	content := make([]*yaml.Node, 0, len(m)*2)
	node := &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     "!!map",
		Content: content,
	}

	for key, val := range m {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: key,
		}
		valNode, err := c.JSONToYAML(val)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valNode)
	}

	return node, nil
}

// Helper methods for YAMLToJSON conversion
func (c *ValueConverter) scalarToJSON(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("invalid argument: scalar node is nil")
	}

	switch node.Tag {
	case "!!null":
		return nil, nil
	case "!!bool":
		return node.Value == trueString, nil
	case "!!int":
		return strconv.ParseInt(node.Value, 10, 64)
	case "!!float":
		return strconv.ParseFloat(node.Value, 64)
	default:
		return node.Value, nil
	}
}

func (c *ValueConverter) mappingToJSON(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("invalid argument: mapping node is nil")
	}

	result := make(map[string]any)

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			return nil, fmt.Errorf("invalid argument: invalid YAML mapping node: odd number of elements")
		}

		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("invalid argument: non-string key in YAML mapping")
		}

		key := keyNode.Value
		value, err := c.YAMLToJSON(valueNode)
		if err != nil {
			return nil, err
		}

		result[key] = value
	}

	return result, nil
}

func (c *ValueConverter) sequenceToJSON(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("invalid argument: sequence node is nil")
	}

	result := make([]any, len(node.Content))

	for i, item := range node.Content {
		value, err := c.YAMLToJSON(item)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}

	return result, nil
}

// YAMLNodeToJSON converts a YAML node to a JSON string
func YAMLNodeToJSON(node *yaml.Node) (string, error) {
	if node == nil {
		return "", fmt.Errorf("invalid argument: YAML node is nil")
	}

	converter := &ValueConverter{}
	jsonValue, err := converter.YAMLToJSON(node)
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(jsonValue)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// JSONToYAMLNode converts a JSON string to a YAML node
func JSONToYAMLNode(jsonStr string) (*yaml.Node, error) {
	if jsonStr == "" {
		return nil, fmt.Errorf("invalid argument: JSON string is empty")
	}

	var jsonValue any
	if err := json.Unmarshal([]byte(jsonStr), &jsonValue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	converter := &ValueConverter{}
	return converter.JSONToYAML(jsonValue)
}
