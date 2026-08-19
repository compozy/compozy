package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintReview(node dsl.Node) {
	review := node.Review
	if review == nil {
		return
	}
	if node.Class != dsl.NodeClassAction {
		c.add(node.ID, CodeReviewShapeInvalid, "review is valid only on action nodes")
		return
	}
	if len(review.Extra) > 0 {
		c.add(node.ID, CodeReviewShapeInvalid, "review contains unsupported fields")
	}
	if review.Responders != nil && !review.Responders.Agents.Valid() {
		c.add(node.ID, CodeResponderPolicyInvalid, "review responders.agents must be allow or deny")
	}
	seen := make(map[dsl.ReviewDecision]struct{})
	hasRespond := false
	for _, decision := range review.EffectiveDecisions() {
		if !decision.Valid() {
			c.add(node.ID, CodeReviewShapeInvalid, "review decision %q is not supported", decision)
			continue
		}
		if _, exists := seen[decision]; exists {
			c.add(node.ID, CodeReviewShapeInvalid, "review decision %q is duplicated", decision)
			continue
		}
		seen[decision] = struct{}{}
		if decision == dsl.ReviewDecisionRespond {
			hasRespond = true
			if _, ok := c.outputSchema(node); !ok {
				c.add(node.ID, CodeReviewRespondSchemaRequired,
					"review respond requires a declared action output shape")
			}
		}
	}
	if hasRespond {
		if schema, ok := c.outputSchema(node); ok {
			c.lintEntityKindAnnotations(node.ID, map[string]any(schema))
		}
	}
	if review.OnReject == nil {
		return
	}
	if len(review.OnReject.Extra) > 0 || strings.TrimSpace(string(review.OnReject.Route)) == "" {
		c.add(node.ID, CodeReviewShapeInvalid, "review on_reject must declare only route")
		return
	}
	if !containsNodeID(c.adjacency[node.ID], review.OnReject.Route) {
		c.add(node.ID, CodeErrorRouteBackward,
			"review on_reject.route %q must be a direct forward edge", review.OnReject.Route)
	}
}
