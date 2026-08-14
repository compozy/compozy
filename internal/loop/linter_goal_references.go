package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func goalParamStringFields(node dsl.Node) []namedString {
	var params dsl.GoalParams
	if err := node.Params.Decode(&params); err != nil {
		return nil
	}
	fields := []namedString{
		{name: "params.agent", value: params.Agent},
		{name: "params.objective", value: params.Objective},
		{name: "params.environment.worktree_ref", value: params.Environment.WorktreeRef},
		{name: "params.environment.directory", value: params.Environment.Directory},
	}
	for idx, criterion := range params.Judge {
		fields = append(
			fields,
			criterionStringFields(fmt.Sprintf("params.judge[%d]", idx), criterion)...,
		)
	}
	return fields
}
