package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultLayout = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDLayoutApply:       SingleResultMinimal,
	toolspkg.ToolIDLayoutArrange:     SingleResultMinimal,
	toolspkg.ToolIDLayoutBalance:     SingleResultMinimal,
	toolspkg.ToolIDLayoutExport:      SingleResultContent,
	toolspkg.ToolIDLayoutFrameResize: SingleResultMinimal,
	toolspkg.ToolIDLayoutRedo:        SingleResultMinimal,
	toolspkg.ToolIDLayoutResize:      SingleResultMinimal,
	toolspkg.ToolIDLayoutUndo:        SingleResultMinimal,
	toolspkg.ToolIDLayoutValidate:    SingleResultContent,
}
