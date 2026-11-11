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
	for _, path := range declaredOutputPaths(convertSchema(node.Produces)) {
		path = append([]string{namespaceNodesKey, string(node.ID), namespaceOutputKey}, path...)
		unproven, err := c.checkToolViewPath(node, path)
		if err != nil {
			c.addToolViewError(node.ID, node, err)
		} else if unproven {
			c.warn(
				node.ID,
				refs.CodeUnresolvablePath,
				"node %q output path %q cannot be proven",
				node.ID,
				strings.Join(path, "."),
			)
		}
	}
}

func (c *lintContext) addResultViewReferenceError(consumer dsl.NodeID, refErr *refs.Error) bool {
	path := nodeOutputReferencePath(refErr.ReferencePath)
	if refErr.Code != refs.CodeUnresolvablePath || len(path) < 4 {
		return false
	}
	node, ok := c.nodeByID[dsl.NodeID(path[1])]
	if !ok || len(node.Produces) > 0 {
		return false
	}
	_, err := c.checkToolViewPath(node, refErr.ReferencePath)
	if err == nil || err.Code != CodeResultViewFieldRequiresFull {
		return false
	}
	c.addToolViewError(consumer, node, err)
	return true
}

func (c *lintContext) addToolViewError(consumer dsl.NodeID, node dsl.Node, refErr *refs.Error) {
	if refErr.Code != CodeResultViewFieldRequiresFull {
		c.addRefsError(consumer, refErr)
		return
	}
	path := nodeOutputReferencePath(refErr.ReferencePath)
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

func (c *lintContext) checkReferenceContracts(
	nodeID dsl.NodeID,
	references []refs.Reference,
	namespace refs.Namespace,
) {
	for _, reference := range references {
		if namespace.InputPathUnproven(reference.Path) {
			c.warn(
				nodeID,
				refs.CodeUnresolvablePath,
				"input %q path %q cannot be proven",
				reference.Path[1],
				strings.Join(reference.Path, "."),
			)
			continue
		}

		path := nodeOutputReferencePath(reference.Path)
		if len(path) < 3 {
			continue
		}
		unproven := false
		if producer, ok := c.nodeByID[dsl.NodeID(path[1])]; ok {
			var err *refs.Error
			unproven, err = c.checkToolViewPath(producer, reference.Path)
			if err != nil {
				c.addToolViewError(nodeID, producer, err)
				continue
			}
		}
		node, ok := namespace.Nodes[path[1]]
		if !ok || (node.HasOutput && !unproven && !node.OutputPathUnproven(path[3:])) {
			continue
		}
		c.warn(
			nodeID,
			refs.CodeUnresolvablePath,
			"node %q output path %q cannot be proven",
			path[1],
			strings.Join(reference.Path, "."),
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

// checkToolViewPath returns unproven acceptance or a rejection; single-result tools have no view bound.
func (c *lintContext) checkToolViewPath(node dsl.Node, original []string) (bool, *refs.Error) {
	path := nodeOutputReferencePath(original)
	if !isToolAction(node) || c.linter.tools == nil || len(path) < 4 {
		return false, nil
	}
	snapshot, ok := c.linter.tools.Snapshot(node.Kind)
	if !ok || snapshot.FullOutputSchemaDigest == "" {
		return false, nil
	}
	raw, _ := snapshot.OutputSchemaFor(nodeResultView(node))
	schema, err := schemaFromJSON(raw)
	if err != nil {
		return false, nil
	}
	selected := refs.NodeSchema{
		Output:           schema,
		HasOutput:        true,
		OutputJSONSchema: true,
		AllowOpenOutput:  nodeResultView(node) == tools.ResultViewFull,
	}
	unproven, refErr := selected.CheckOutputPath(path[3:], original)
	if refErr == nil || nodeResultView(node) == tools.ResultViewFull {
		return unproven, refErr
	}
	full, err := schemaFromJSON(snapshot.FullOutputSchema)
	if err == nil {
		selected.Output = full
		selected.AllowOpenOutput = true
		if _, fullErr := selected.CheckOutputPath(path[3:], original); fullErr == nil {
			refErr.Code = CodeResultViewFieldRequiresFull
		}
	}
	return false, refErr
}
