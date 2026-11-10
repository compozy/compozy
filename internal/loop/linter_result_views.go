package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/tools"
)

const (
	CodeResultViewNotLiteral        = "result_view_not_literal"
	CodeResultViewFieldRequiresFull = "result_view_field_requires_full"
)

func (c *lintContext) lintToolResultView(node dsl.Node, snapshot ToolSchemaSnapshot) {
	if snapshot.FullOutputSchemaDigest == "" && !strings.HasPrefix(node.Kind, "compozy__") {
		return
	}
	if value, exists := node.Params[tools.ResultViewInputKey]; exists {
		selector, ok := value.(string)
		switch {
		case strings.Contains(selector, "{{"):
			c.add(node.ID, CodeResultViewNotLiteral, "node %q params.result_view must be a literal %q or %q",
				node.ID, tools.ResultViewCompact, tools.ResultViewFull)
		case snapshot.FullOutputSchemaDigest == "":
			c.add(
				node.ID,
				string(tools.ReasonSchemaInvalid),
				"tool %q returns a single result and does not accept %q",
				node.Kind,
				tools.ResultViewInputKey,
			)
		case !ok || (selector != string(tools.ResultViewCompact) && selector != string(tools.ResultViewFull)):
			c.add(node.ID, string(tools.ReasonSchemaInvalid), "tool %q input %q must be one of %q, %q",
				node.Kind, tools.ResultViewInputKey, tools.ResultViewCompact, tools.ResultViewFull)
		}
	}
	if snapshot.FullOutputSchemaDigest == "" || len(node.Produces) == 0 {
		return
	}
	raw, _ := snapshot.OutputSchemaFor(nodeResultView(node))
	schema, err := schemaFromJSON(raw)
	if err != nil {
		return
	}
	namespace := refs.Namespace{Nodes: map[string]refs.NodeSchema{
		string(node.ID): {
			Output:          schema,
			HasOutput:       true,
			AllowOpenOutput: nodeResultView(node) == tools.ResultViewFull,
		},
	}}
	for _, path := range declaredOutputPaths(convertSchema(node.Produces)) {
		path = append([]string{namespaceNodesKey, string(node.ID), namespaceOutputKey}, path...)
		if err := namespace.ValidatePath(path); err != nil {
			if !c.addToolResultViewReferenceError(node.ID, node, err) {
				c.addRefsError(node.ID, err)
			}
		} else {
			c.warnUnknownOutputReferences(
				node.ID,
				[]refs.Reference{{Raw: strings.Join(path, "."), Path: path}},
				namespace,
			)
		}
	}
}

func (c *lintContext) toolAllowsOpenOutput(node dsl.Node) bool {
	if node.Class != dsl.NodeClassAction || len(node.Produces) > 0 || c.linter.tools == nil {
		return false
	}
	snapshot, ok := c.linter.tools.Snapshot(node.Kind)
	return ok && snapshot.FullOutputSchemaDigest != "" && nodeResultView(node) == tools.ResultViewFull
}

func (c *lintContext) addResultViewReferenceError(consumer dsl.NodeID, refErr *refs.Error) bool {
	path := nodeOutputReferencePath(refErr.ReferencePath)
	if len(path) < 4 {
		return false
	}
	node, ok := c.nodeByID[dsl.NodeID(path[1])]
	if !ok || len(node.Produces) > 0 {
		return false
	}
	return c.addToolResultViewReferenceError(consumer, node, refErr)
}

func (c *lintContext) addToolResultViewReferenceError(
	consumer dsl.NodeID, node dsl.Node, refErr *refs.Error,
) bool {
	path := nodeOutputReferencePath(refErr.ReferencePath)
	if refErr.Code != refs.CodeUnresolvablePath || len(path) < 4 || c.linter.tools == nil ||
		node.Class != dsl.NodeClassAction || nodeResultView(node) == tools.ResultViewFull {
		return false
	}
	snapshot, ok := c.linter.tools.Snapshot(node.Kind)
	if !ok || snapshot.FullOutputSchemaDigest == "" {
		return false
	}
	raw, _ := snapshot.OutputSchemaFor(tools.ResultViewFull)
	full, err := schemaFromJSON(raw)
	if err != nil {
		return false
	}
	namespace := refs.Namespace{Nodes: map[string]refs.NodeSchema{
		path[1]: {Output: full, HasOutput: true, AllowOpenOutput: true},
	}}
	if namespace.ValidatePath(path) != nil {
		return false
	}
	c.errors = append(c.errors, LintError{
		NodeID:   consumer,
		Path:     strings.Join(refErr.ReferencePath, "."),
		Code:     CodeResultViewFieldRequiresFull,
		Severity: SeverityError,
		Message: fmt.Sprintf(
			"node %q reads %s, which the compact result of %s does not include; fix: add `result_view: full` to the params of node %q",
			consumer,
			strings.Join(path, "."),
			node.Kind,
			node.ID,
		),
	})
	return true
}

func declaredOutputPaths(value any) [][]string {
	if schema, ok := value.(refs.Schema); ok {
		value = map[string]any(schema)
	}
	if items, ok := value.([]any); ok {
		if len(items) == 0 {
			return nil
		}
		return declaredOutputPaths(items[0])
	}
	properties, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	if properties[jsonSchemaTypeKey] == jsonSchemaArrayType {
		return declaredOutputPaths(properties[jsonSchemaItemsKey])
	}
	if declared, ok := properties[jsonSchemaPropertiesKey].(map[string]any); ok {
		properties = declared
	} else if _, ok := properties[jsonSchemaTypeKey]; ok {
		return nil
	}
	var paths [][]string
	for name, value := range properties {
		children := declaredOutputPaths(value)
		if len(children) == 0 {
			paths = append(paths, []string{name})
			continue
		}
		for _, child := range children {
			paths = append(paths, append([]string{name}, child...))
		}
	}
	return paths
}

func (c *lintContext) warnUnknownOutputReferences(
	nodeID dsl.NodeID,
	references []refs.Reference,
	namespace refs.Namespace,
) {
	for _, reference := range references {
		path := nodeOutputReferencePath(reference.Path)
		if len(path) < 3 {
			continue
		}
		node, ok := namespace.Nodes[path[1]]
		if !ok || (node.HasOutput && !node.OutputPathUnproven(path[3:])) {
			continue
		}
		c.warn(
			nodeID,
			refs.CodeUnresolvablePath,
			"node %q has no declared output schema; path %q cannot be proven",
			path[1],
			reference.Raw,
		)
	}
}

func nodeOutputReferencePath(path []string) []string {
	if len(path) > 0 && (path[0] == "previous" || path[0] == "best") {
		path = path[1:]
	}
	if len(path) < 3 || path[0] != namespaceNodesKey || path[2] != namespaceOutputKey {
		return nil
	}
	return path
}
