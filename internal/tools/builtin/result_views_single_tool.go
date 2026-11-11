package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultTool = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDToolApprovalsRevoke: SingleResultMinimal,
	toolspkg.ToolIDToolArtifactRead:    SingleResultContent,
	toolspkg.ToolIDToolInfo:            SingleResultContent,
}
