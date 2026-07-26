package loop

import "github.com/compozy/agh/internal/loop/dsl"

func (c *lintContext) lintSubLoopBody(node dsl.Node) {
	nested := c.def
	nested.Graph = *node.Body
	nestedCtx := newLintContext(nested, c.linter)
	nestedCtx.indexGraph()
	nestedCtx.lintNodeIDs()
	nestedCtx.lintKindsAndSchemas()
	nestedCtx.lintGraphShape()
	nestedCtx.lintReferences()
	for _, lintError := range nestedCtx.errors {
		if lintError.NodeID == "" {
			lintError.NodeID = node.ID
		}
		c.errors = append(c.errors, lintError)
	}
}
