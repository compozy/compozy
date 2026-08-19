package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintAsk(node dsl.Node) {
	var params dsl.AskParams
	if err := node.Params.Decode(&params); err != nil {
		c.add(node.ID, CodeWaitShapeInvalid, "ask params are invalid: %v", err)
		return
	}
	if len(params.Extra) > 0 {
		c.add(node.ID, CodeWaitShapeInvalid, "ask params contain unsupported fields")
	}
	if strings.TrimSpace(params.Prompt) == "" {
		c.add(node.ID, CodeWaitShapeInvalid, "ask prompt is required")
	}
	if len(params.Expect) == 0 {
		c.add(node.ID, CodeAskExpectRequired, "ask expect is required and defines the answer shape")
	} else {
		c.lintEntityKindAnnotations(node.ID, map[string]any(params.Expect))
	}
	if params.Responders != nil && !params.Responders.Agents.Valid() {
		c.add(node.ID, CodeResponderPolicyInvalid, "responders.agents must be allow or deny")
	}
}
