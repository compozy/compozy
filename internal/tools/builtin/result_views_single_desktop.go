package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultDesktop = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDDesktopCreate:  SingleResultMinimal,
	toolspkg.ToolIDDesktopDelete:  SingleResultMinimal,
	toolspkg.ToolIDDesktopList:    SingleResultMinimal,
	toolspkg.ToolIDDesktopReorder: SingleResultMinimal,
	toolspkg.ToolIDDesktopSwitch:  SingleResultMinimal,
	toolspkg.ToolIDDesktopUpdate:  SingleResultMinimal,
}
