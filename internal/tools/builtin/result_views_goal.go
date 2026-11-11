package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func withGoalResultViews(d toolspkg.Descriptor) toolspkg.Descriptor {
	var compact string
	switch d.ID {
	case toolspkg.ToolIDGoalControl:
		compact = goalcontrolCompactSchema
	case toolspkg.ToolIDGoalGet:
		compact = goalgetCompactSchema
	default:
		return d
	}
	return withResultViews(d, compact, string(d.OutputSchema))
}
