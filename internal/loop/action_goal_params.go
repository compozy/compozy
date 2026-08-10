package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const goalJudgeParamKey = "judge"

// MaterializeGoalParams resolves one Goal node against its runtime namespace.
func MaterializeGoalParams(node dsl.Node, namespace map[string]any) (dsl.GoalParams, error) {
	rendered, err := renderNodeParamsExcept(
		node,
		namespace,
		map[string]struct{}{outputSchemaParamKey: {}, goalJudgeParamKey: {}},
	)
	if err != nil {
		return dsl.GoalParams{}, fmt.Errorf("materialize Goal params: %w", err)
	}
	var params dsl.GoalParams
	if err := dsl.NodeParams(rendered).Decode(&params); err != nil {
		return dsl.GoalParams{}, fmt.Errorf("decode materialized Goal params: %w", err)
	}
	for index := range params.Judge {
		criterion, renderErr := renderGateCriterion(node.ID, params.Judge[index], namespace)
		if renderErr != nil {
			return dsl.GoalParams{}, fmt.Errorf("materialize Goal judge: %w", renderErr)
		}
		params.Judge[index] = criterion
	}
	return params, nil
}
