package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/hooks"
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
		kind := hooks.HookEvent(strings.TrimSpace(params.Event.Kind))
		if kind == "" {
			c.add(node.ID, CodeWaitShapeInvalid, "wait event kind is required")
		} else if kind.Validate() != nil {
			c.add(
				node.ID,
				CodeWatchEventsKindUnknown,
				"wait event kind %q is not in the hook catalog",
				params.Event.Kind,
			)
		} else if _, ok := SupportedWatchEvents()[kind]; !ok {
			c.add(
				node.ID,
				CodeWatchEventsKindUnsupported,
				"wait event kind %q is not supported; supported kinds: %s",
				kind,
				strings.Join(sortedSupportedWatchEventKinds(), ", "),
			)
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
	c.lintWaitExpiry(node, params.Expires, "wait")
}

func (c *lintContext) lintGateExpiry(node dsl.Node) {
	if node.Expires == nil {
		return
	}
	c.lintWaitExpiry(node, node.Expires, "gate")
}

func (c *lintContext) lintWaitExpiry(node dsl.Node, expiry *dsl.WaitExpiry, owner string) {
	if expiry == nil {
		return
	}
	if strings.TrimSpace(expiry.After) == "" {
		c.add(node.ID, CodeWaitShapeInvalid, "%s expires.after is required", owner)
	} else {
		c.lintLifecycleDuration(node.ID, owner+".expires.after", expiry.After)
	}
	if len(expiry.Extra) > 0 {
		c.add(node.ID, CodeWaitShapeInvalid, "%s expires contains unsupported fields", owner)
	}
	c.lintEffectLists(node.ID, [][]dsl.EffectSpec{expiry.Escalate})
	if expiry.Route != "" && !containsNodeID(c.adjacency[node.ID], expiry.Route) {
		c.add(
			node.ID,
			CodeErrorRouteBackward,
			"%s expires.route %q must be a direct forward edge",
			owner,
			expiry.Route,
		)
	}
	if len(expiry.Escalate) == 0 && expiry.Route == "" {
		c.warn(node.ID, CodeWaitExpiryWithoutPath, "%s expiry has neither escalate effects nor route", owner)
	}
}
