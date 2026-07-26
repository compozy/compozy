package loop

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/tools"
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
	case harvestKindChannelResult:
	default:
		c.add(node.ID, CodeInvalidHarvest, "harvest kind %q is not supported", kind)
		return
	}
	if strings.TrimSpace(node.Kind) != tools.ToolIDNetworkSend.String() {
		c.add(
			node.ID,
			CodeInvalidHarvest,
			"channel_result harvest requires %s action kind, got %q",
			tools.ToolIDNetworkSend,
			node.Kind,
		)
	}
	window := strings.TrimSpace(node.Harvest.Window)
	if window == "" {
		c.add(node.ID, CodeInvalidHarvest, "channel_result harvest requires window")
	} else if parsed, err := time.ParseDuration(window); err != nil || parsed <= 0 {
		c.add(node.ID, CodeInvalidHarvest, "channel_result harvest window must be a positive duration")
	}
	if err := validateChannelResultContentRule(node.Harvest.ContentRule); err != nil {
		c.add(node.ID, CodeInvalidHarvest, "channel_result harvest content_rule is invalid: %v", err)
	}
}
