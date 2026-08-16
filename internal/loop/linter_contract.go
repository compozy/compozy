package loop

import (
	"sort"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintContractShape() {
	c.lintMetrics()
	if len(c.def.Contract.NoProgress.Extra) > 0 {
		keys := make([]string, 0, len(c.def.Contract.NoProgress.Extra))
		for key := range c.def.Contract.NoProgress.Extra {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			c.add("", CodeUnknownParameter, "contract.no_progress.%s is not supported", key)
		}
	}
	if !c.def.Contract.StopWhen.OnEvalError.Valid() {
		c.add("", CodeEvalErrorPolicyInvalid, "contract.stop_when.on_eval_error must be fail or exit")
	}
	for _, state := range c.def.Contract.TerminalStates {
		if !dsl.IsKnownTerminalState(state) {
			c.add("", CodeUnknownTerminalState, "terminal state %q is not in the closed enum", state)
		}
	}
	c.lintLifecycleGrammar()
}
