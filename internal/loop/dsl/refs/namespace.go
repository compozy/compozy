// Package refs validates loop template and CEL references against one namespace.
package refs

import (
	"fmt"
	"strings"
)

const (
	// CodeUnknownReference identifies an unknown namespace root or symbol.
	CodeUnknownReference = "unknown_reference"
	// CodeUnresolvablePath identifies a known symbol with a missing child path.
	CodeUnresolvablePath = "unresolvable_path"
	// CodeItemOutsideFanout identifies item/index usage outside fan-out scope.
	CodeItemOutsideFanout = "item_outside_fanout"
	// CodeConditionNotBool identifies a CEL expression that does not return bool.
	CodeConditionNotBool = "condition_not_bool"
)

const (
	namespaceInputs     = "inputs"
	namespaceNodes      = "nodes"
	namespaceItem       = "item"
	namespaceIndex      = "index"
	namespaceTrigger    = "trigger"
	namespaceGeneration = "generation"
	namespaceEvent      = "event"
)

// Schema is a permissive JSON-schema-compatible shape used for path validation.
type Schema map[string]any

// Namespace is the single reference namespace consumed by templates and CEL.
type Namespace struct {
	Inputs       map[string]Schema
	Nodes        map[string]NodeSchema
	AllowFanout  bool
	AllowTrigger bool
	AllowEvent   bool
}

// NodeSchema exposes the referenceable output/status shape for one node.
type NodeSchema struct {
	Output    Schema
	HasOutput bool
}

// Reference records one resolved template or CEL path.
type Reference struct {
	Raw  string
	Path []string
}

// Error reports a namespace validation failure.
type Error struct {
	Code    string
	Path    []string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, strings.Join(e.Path, "."))
}

// ValidatePath checks a dotted path against the namespace.
func (n Namespace) ValidatePath(path []string) *Error {
	if len(path) == 0 {
		return nil
	}

	switch path[0] {
	case namespaceInputs:
		return n.validateInputPath(path)
	case namespaceNodes:
		return n.validateNodePath(path)
	case namespaceItem, namespaceIndex:
		if !n.AllowFanout {
			return newPathError(
				CodeItemOutsideFanout,
				path,
				"fan-out scoped symbol %q is not available here",
				path[0],
			)
		}
		return nil
	case namespaceTrigger:
		if !n.AllowTrigger {
			return newPathError(CodeUnknownReference, path, "trigger payload is not available here")
		}
		return nil
	case namespaceGeneration:
		if len(path) > 1 {
			return newPathError(CodeUnresolvablePath, path, "generation is a scalar")
		}
		return nil
	case namespaceEvent:
		if !n.AllowEvent {
			return newPathError(CodeUnknownReference, path, "event payload is not available here")
		}
		return nil
	default:
		return newPathError(CodeUnknownReference, path, "unknown reference root %q", path[0])
	}
}

func (n Namespace) validateInputPath(path []string) *Error {
	if len(path) < 2 {
		return newPathError(CodeUnknownReference, path, "input reference must name an input")
	}
	schema, ok := n.Inputs[path[1]]
	if !ok {
		return newPathError(CodeUnknownReference, path, "unknown input %q", path[1])
	}
	return validateSchemaPath(schema, path[2:], path)
}

func (n Namespace) validateNodePath(path []string) *Error {
	if len(path) < 3 {
		return newPathError(CodeUnknownReference, path, "node reference must name a node and field")
	}
	node, ok := n.Nodes[path[1]]
	if !ok {
		return newPathError(CodeUnknownReference, path, "unknown node %q", path[1])
	}
	switch path[2] {
	case "status":
		if len(path) > 3 {
			return newPathError(CodeUnresolvablePath, path, "node status is a scalar")
		}
		return nil
	case "output":
		if !node.HasOutput {
			return nil
		}
		return validateSchemaPath(node.Output, path[3:], path)
	default:
		return newPathError(CodeUnknownReference, path, "unknown node field %q", path[2])
	}
}

func validateSchemaPath(schema Schema, path []string, original []string) *Error {
	if len(path) == 0 {
		return nil
	}
	if len(schema) == 0 {
		return newPathError(CodeUnresolvablePath, original, "schema has no child fields")
	}

	current := any(map[string]any(schema))
	for idx, segment := range path {
		next, ok := childSchema(current, segment)
		if !ok {
			return newPathError(
				CodeUnresolvablePath,
				original[:len(original)-len(path)+idx+1],
				"schema path %q is not declared",
				strings.Join(original, "."),
			)
		}
		current = next
	}
	return nil
}

func childSchema(current any, segment string) (any, bool) {
	switch typed := current.(type) {
	case Schema:
		return childSchema(map[string]any(typed), segment)
	case map[string]any:
		if properties, ok := typed["properties"]; ok {
			if child, found := childFromProperties(properties, segment); found {
				return child, true
			}
		}
		if child, ok := typed[segment]; ok {
			return child, true
		}
		if typed["type"] == "array" {
			if items, ok := typed["items"]; ok {
				return childSchema(items, segment)
			}
		}
		return nil, false
	case map[string]Schema:
		child, ok := typed[segment]
		return child, ok
	case []any:
		if len(typed) == 0 {
			return nil, false
		}
		return childSchema(typed[0], segment)
	case string:
		return nil, false
	default:
		return nil, false
	}
}

func childFromProperties(properties any, segment string) (any, bool) {
	switch typed := properties.(type) {
	case map[string]any:
		child, ok := typed[segment]
		return child, ok
	case map[string]Schema:
		child, ok := typed[segment]
		return child, ok
	default:
		return nil, false
	}
}

func newPathError(code string, path []string, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Path:    append([]string(nil), path...),
		Message: fmt.Sprintf(format, args...),
	}
}
