package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func goalParamStringFields(node dsl.Node) []namedString {
	params, err := decodeGoalNodeParams(node.Params)
	if err != nil {
		return nil
	}
	fields := []namedString{
		{name: "params.agent", value: params.Agent},
		{name: "params.objective", value: params.Objective},
		{name: "params.environment.worktree_ref", value: params.Environment.WorktreeRef},
		{name: "params.environment.directory", value: params.Environment.Directory},
	}
	if runtime, exists := node.Params["runtime"]; exists {
		fields = append(fields, paramValueStringFields("params.runtime", runtime, nil)...)
	}
	for idx, criterion := range params.Judge {
		fields = append(
			fields,
			criterionStringFields(fmt.Sprintf("params.judge[%d]", idx), criterion)...,
		)
	}
	return fields
}
