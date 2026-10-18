package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintActionHarvest(node dsl.Node) {
	if node.Harvest == nil {
		return
	}
	kind := strings.TrimSpace(node.Harvest.Kind)
	if kind == "" || kind == harvestKindSync {
		return
	}
	if dsl.IsReservedActionKind(node.Kind) {
		c.add(node.ID, CodeInvalidHarvest, "reserved action kind %q does not support harvest kind %q", node.Kind, kind)
		return
	}
	switch kind {
	case harvestKindEventRange, harvestKindAsync:
		return
	default:
		c.add(node.ID, CodeInvalidHarvest, "harvest kind %q is not supported", kind)
		return
	}
}
