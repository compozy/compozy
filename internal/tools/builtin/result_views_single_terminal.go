package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultTerminal = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDTerminalClose:        SingleResultMinimal,
	toolspkg.ToolIDTerminalExec:         SingleResultContent,
	toolspkg.ToolIDTerminalList:         SingleResultMinimal,
	toolspkg.ToolIDTerminalOpen:         SingleResultMinimal,
	toolspkg.ToolIDTerminalRead:         SingleResultContent,
	toolspkg.ToolIDTerminalRequestInput: SingleResultMinimal,
	toolspkg.ToolIDTerminalSignal:       SingleResultMinimal,
	toolspkg.ToolIDTerminalWait:         SingleResultContent,
	toolspkg.ToolIDTerminalWrite:        SingleResultMinimal,
}
