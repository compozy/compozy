package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultWindow = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDWindowActivate: SingleResultMinimal,
	toolspkg.ToolIDWindowClose:    SingleResultMinimal,
	toolspkg.ToolIDWindowFloat:    SingleResultMinimal,
	toolspkg.ToolIDWindowFocus:    SingleResultMinimal,
	toolspkg.ToolIDWindowGroup:    SingleResultMinimal,
	toolspkg.ToolIDWindowList:     SingleResultMinimal,
	toolspkg.ToolIDWindowMove:     SingleResultMinimal,
	toolspkg.ToolIDWindowNavigate: SingleResultMinimal,
	toolspkg.ToolIDWindowOpen:     SingleResultMinimal,
	toolspkg.ToolIDWindowPin:      SingleResultMinimal,
	toolspkg.ToolIDWindowReopen:   SingleResultMinimal,
	toolspkg.ToolIDWindowReorder:  SingleResultMinimal,
	toolspkg.ToolIDWindowResize:   SingleResultMinimal,
	toolspkg.ToolIDWindowSwap:     SingleResultMinimal,
	toolspkg.ToolIDWindowZoom:     SingleResultMinimal,
}
