package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultWorkspace = map[toolspkg.ToolID]SingleResultReason{
	"compozy__workspace_archive":        SingleResultMinimal,
	"compozy__workspace_folder_create":  SingleResultMinimal,
	"compozy__workspace_folder_delete":  SingleResultMinimal,
	"compozy__workspace_folder_list":    SingleResultMinimal,
	"compozy__workspace_folder_rename":  SingleResultMinimal,
	"compozy__workspace_folder_reorder": SingleResultMinimal,
	"compozy__workspace_info":           SingleResultMinimal,
	"compozy__workspace_list":           SingleResultMinimal,
	"compozy__workspace_move":           SingleResultMinimal,
	"compozy__workspace_reorder":        SingleResultMinimal,
	"compozy__workspace_unarchive":      SingleResultMinimal,
}
