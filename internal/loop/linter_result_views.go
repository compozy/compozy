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
			Output:    schema,
			HasOutput: true, OutputJSONSchema: true,
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
			c.checkReferenceContracts(
				node.ID,
				[]refs.Reference{{Raw: strings.Join(path, "."), Path: path}},
				namespace,
			)
		}
	}
}

func (c *lintContext) allowsOpenOutput(node dsl.Node) bool {
	if isToolAction(node) {
		if len(node.Produces) > 0 || c.linter.tools == nil {
			return false
		}
		snapshot, ok := c.linter.tools.Snapshot(node.Kind)
		return ok && snapshot.FullOutputSchemaDigest != "" && nodeResultView(node) == tools.ResultViewFull
	}
	if node.Class == dsl.NodeClassSource {
		return false
	}
	if authored := authoredOutputSchema(node); authored != nil {
		return isFullJSONSchema(map[string]any(authored))
	}

	return false
}

func (c *lintContext) addResultViewReferenceError(consumer dsl.NodeID, refErr *refs.Error) bool {
	path := nodeOutputReferencePath(refErr.ReferencePath)
	if len(path) < 4 {
		return false
	}
	node, ok := c.nodeByID[dsl.NodeID(path[1])]
	if !ok {
		return false
	}

	if len(node.Produces) > 0 {
		declared, valid := normalizedReferenceSchema(node.Produces)
		declaration := refs.Namespace{
			Nodes: map[string]refs.NodeSchema{
				path[1]: {
					Output:           declared,
					HasOutput:        valid,
					OutputJSONSchema: true,
					AllowOpenOutput:  isFullJSONSchema(map[string]any(convertSchema(node.Produces))),
				},
			},
		}
		if !valid || declaration.ValidatePath(path) != nil {
			return false
		}
	}
	return c.addToolResultViewReferenceError(consumer, node, refErr)
}

func (c *lintContext) addToolResultViewReferenceError(
	consumer dsl.NodeID, node dsl.Node, refErr *refs.Error,
) bool {
	path := nodeOutputReferencePath(refErr.ReferencePath)
	if refErr.Code != refs.CodeUnresolvablePath || len(path) < 4 || c.linter.tools == nil ||
		!isToolAction(node) || nodeResultView(node) == tools.ResultViewFull {
		return false
	}
	snapshot, ok := c.linter.tools.Snapshot(node.Kind)
	if !ok || snapshot.FullOutputSchemaDigest == "" {
		return false
	}

	compactRaw, _ := snapshot.OutputSchemaFor(tools.ResultViewCompact)
	compact, compactErr := schemaFromJSON(compactRaw)
	if compactErr == nil {
		compactNamespace := refs.Namespace{
			Nodes: map[string]refs.NodeSchema{path[1]: {Output: compact, HasOutput: true, OutputJSONSchema: true}},
		}
		if compactNamespace.ValidatePath(path) == nil {
			return false
		}
	}
	raw, _ := snapshot.OutputSchemaFor(tools.ResultViewFull)
	full, err := schemaFromJSON(raw)
	if err != nil {
		return false
	}
	namespace := refs.Namespace{Nodes: map[string]refs.NodeSchema{
		path[1]: {Output: full, HasOutput: true, OutputJSONSchema: true, AllowOpenOutput: true},
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
		if producer, ok := c.nodeByID[dsl.NodeID(path[1])]; ok && c.rejectToolReference(nodeID, producer, reference) {
			continue
		}
		node, ok := namespace.Nodes[path[1]]
		if !ok || (node.HasOutput && !node.OutputPathUnproven(path[3:])) {
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

func (c *lintContext) rejectToolReference(consumer dsl.NodeID, node dsl.Node, reference refs.Reference) bool {
	if !isToolAction(node) || c.linter.tools == nil {
		return false
	}
	schema, ok := c.toolOutputSchema(node)
	if !ok {
		return false
	}
	snapshot, _ := c.linter.tools.Snapshot(node.Kind)
	namespace := refs.Namespace{
		Nodes: map[string]refs.NodeSchema{
			string(node.ID): {
				Output:           schema,
				HasOutput:        true,
				OutputJSONSchema: true,
				AllowOpenOutput:  nodeResultView(node) == tools.ResultViewFull || snapshot.FullOutputSchemaDigest == "",
			},
		},
	}
	if err := namespace.ValidatePath(reference.Path); err != nil {
		if !c.addToolResultViewReferenceError(consumer, node, err) {
			c.addRefsError(consumer, err)
		}
		return true
	}
	return false
}
