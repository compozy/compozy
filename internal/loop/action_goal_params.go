package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// MaterializeGoalParams resolves one Goal node against its runtime namespace.
func MaterializeGoalParams(node dsl.Node, namespace map[string]any) (dsl.GoalParams, error) {
	rendered, err := renderNodeParamsExcept(
		node,
		namespace,
		map[string]struct{}{outputSchemaParamKey: {}},
	)
	if err != nil {
		return dsl.GoalParams{}, fmt.Errorf("materialize Goal params: %w", err)
	}
	var params dsl.GoalParams
	if err := dsl.NodeParams(rendered).Decode(&params); err != nil {
		return dsl.GoalParams{}, fmt.Errorf("decode materialized Goal params: %w", err)
	}
	return params, nil
}
