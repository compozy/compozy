package loop

import (
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

func compileSubLoopBodies(
	resolved *ResolvedDefinition,
	def dsl.Definition,
	tools ToolSchemaSource,
) error {
	for _, node := range def.Graph.Nodes {
		if err := compileSubLoopBody(resolved, def, node, "", tools); err != nil {
			return err
		}
	}
	return nil
}

func compileSubLoopBody(
	resolved *ResolvedDefinition,
	parent dsl.Definition,
	node dsl.Node,
	prefix string,
	tools ToolSchemaSource,
) error {
	if !isControlKind(node, dsl.ControlSubLoop) || node.Body == nil {
		return nil
	}
	qualifiedParentID := qualifySubLoopNodeID(prefix, node.ID)
	nested := parent
	nested.Graph = *node.Body
	nestedCtx := newLintContext(nested, &DefinitionLinter{tools: tools})
	nestedCtx.indexGraphTrusted()
	for _, child := range node.Body.Nodes {
		namespace := nestedCtx.namespace(nestedCtx.inFanoutScope(child.ID), nestedCtx.hasTriggerStart())
		qualifiedChild := child
		qualifiedChild.ID = qualifySubLoopNodeID(string(qualifiedParentID), child.ID)
		if err := compileNode(resolved, qualifiedChild, namespace, nestedCtx); err != nil {
			return fmt.Errorf("compile sub-loop %s node %s: %w", qualifiedParentID, child.ID, err)
		}
		if child.Class == dsl.NodeClassAction && !dsl.IsReservedActionKind(child.Kind) && tools != nil {
			if snapshot, ok := tools.Snapshot(child.Kind); ok {
				resolved.ToolSchemas[child.Kind] = snapshot
			}
		}
		if err := compileSubLoopBody(resolved, nested, child, string(qualifiedParentID), tools); err != nil {
			return err
		}
	}
	return nil
}
