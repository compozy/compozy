package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintEffectLists(nodeID dsl.NodeID, lists [][]dsl.EffectSpec) {
	for _, effects := range lists {
		for _, effect := range effects {
			c.lintEffect(nodeID, effect)
		}
	}
}

func (c *lintContext) lintEffect(nodeID dsl.NodeID, effect dsl.EffectSpec) {
	toolID := strings.TrimSpace(effect.Tool)
	hasTool := toolID != ""
	hasEmit := effect.Emit != nil
	if hasTool == hasEmit || hasEmit && (strings.TrimSpace(effect.Emit.Kind) == "" || len(effect.With) > 0) {
		c.add(nodeID, CodeEffectShapeInvalid, "effect must declare exactly one valid emit or tool call")
		return
	}
	if hasTool && c.linter.tools != nil {
		if _, ok := c.linter.tools.Snapshot(toolID); !ok {
			c.warn(nodeID, CodeEffectToolUnknown, "effect tool %q is not resolvable", toolID)
		}
	}
}
