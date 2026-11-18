package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultGoal = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDGoalReport: SingleResultMinimal,
}
