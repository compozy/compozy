package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultSkill = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDSkillView: SingleResultContent,
}
