package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultWorktree = map[toolspkg.ToolID]SingleResultReason{
	"compozy__worktree_create":  SingleResultMinimal,
	"compozy__worktree_inspect": SingleResultMinimal,
	"compozy__worktree_remove":  SingleResultMinimal,
}
