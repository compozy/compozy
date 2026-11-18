package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultAgent = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDAgentActivity:           SingleResultCallerBounded,
	toolspkg.ToolIDAgentDraftInstructions:  SingleResultContent,
	toolspkg.ToolIDAgentHeartbeatWake:      SingleResultMinimal,
	toolspkg.ToolIDAgentInstanceAway:       SingleResultContent,
	toolspkg.ToolIDAgentInstanceCheck:      SingleResultMinimal,
	toolspkg.ToolIDAgentInstanceCheckIns:   SingleResultCallerBounded,
	toolspkg.ToolIDAgentInstanceSeen:       SingleResultMinimal,
	toolspkg.ToolIDAgentInstanceOccurrence: SingleResultMinimal,
	toolspkg.ToolIDAgentInstanceReports:    SingleResultCallerBounded,
	toolspkg.ToolIDAgentMemoryAdd:          SingleResultMinimal,
	toolspkg.ToolIDAgentMemoryRemove:       SingleResultMinimal,
	toolspkg.ToolIDAgentMemoryReplace:      SingleResultMinimal,
	toolspkg.ToolIDAgentMemorySearch:       SingleResultContent,
	toolspkg.ToolIDAgentSkillPropose:       SingleResultMinimal,
	toolspkg.ToolIDAgentSkillReviews:       SingleResultCallerBounded,
}
