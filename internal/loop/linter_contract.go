package loop

import "github.com/compozy/compozy/internal/loop/dsl"

func (c *lintContext) lintContractShape() {
	c.lintMetrics()
	for _, state := range c.def.Contract.TerminalStates {
		if !dsl.IsKnownTerminalState(state) {
			c.add("", CodeUnknownTerminalState, "terminal state %q is not in the closed enum", state)
		}
	}
}
