package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintWait(node dsl.Node) {
	var params dsl.WaitParams
	if err := node.Params.Decode(&params); err != nil {
		c.add(node.ID, CodeWaitShapeInvalid, "wait params are invalid: %v", err)
		return
	}
	if len(params.Extra) > 0 {
		c.add(node.ID, CodeWaitShapeInvalid, "wait params contain unsupported fields")
	}
	discriminators := 0
	if strings.TrimSpace(params.For) != "" {
		discriminators++
		c.lintLifecycleDuration(node.ID, "wait.for", params.For)
	}
	if strings.TrimSpace(params.Until) != "" {
		discriminators++
	}
	if params.Event != nil {
		discriminators++
		if strings.TrimSpace(params.Event.Kind) == "" {
			c.add(node.ID, CodeWaitShapeInvalid, "wait event kind is required")
		}
	}
	if discriminators != 1 {
		c.add(node.ID, CodeWaitShapeInvalid, "wait params must declare exactly one of for, until, or event")
	}
	if params.AheadArrival != "" && !dsl.IsKnownWaitAheadArrival(params.AheadArrival) {
		c.add(node.ID, CodeWaitShapeInvalid, "wait ahead_arrival %q is not in the closed enum", params.AheadArrival)
	}
	if params.Expires == nil {
		return
	}
	if strings.TrimSpace(params.Expires.After) == "" {
		c.add(node.ID, CodeWaitShapeInvalid, "wait expires.after is required")
	} else {
		c.lintLifecycleDuration(node.ID, "wait.expires.after", params.Expires.After)
	}
	if len(params.Expires.Extra) > 0 {
		c.add(node.ID, CodeWaitShapeInvalid, "wait expires contains unsupported fields")
	}
	c.lintEffectLists(node.ID, [][]dsl.EffectSpec{params.Expires.Escalate})
	if params.Expires.Route != "" && !containsNodeID(c.adjacency[node.ID], params.Expires.Route) {
		c.add(
			node.ID,
			CodeErrorRouteBackward,
			"wait expires.route %q must be a direct forward edge",
			params.Expires.Route,
		)
	}
	if len(params.Expires.Escalate) == 0 && params.Expires.Route == "" {
		c.warn(node.ID, CodeWaitExpiryWithoutPath, "wait expiry has neither escalate effects nor route")
	}
}
