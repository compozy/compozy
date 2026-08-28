package tools

// Integrated-terminal native tool IDs registered by the terminal toolset.
const (
	ToolIDTerminalExec         ToolID = "compozy__terminal_exec"
	ToolIDTerminalOpen         ToolID = "compozy__terminal_open"
	ToolIDTerminalWrite        ToolID = "compozy__terminal_write"
	ToolIDTerminalRead         ToolID = "compozy__terminal_read"
	ToolIDTerminalWait         ToolID = "compozy__terminal_wait"
	ToolIDTerminalSignal       ToolID = "compozy__terminal_signal"
	ToolIDTerminalClose        ToolID = "compozy__terminal_close"
	ToolIDTerminalList         ToolID = "compozy__terminal_list"
	ToolIDTerminalRequestInput ToolID = "compozy__terminal_request_input"
	ToolIDTerminalYield        ToolID = "compozy__terminal_yield"
	ToolIDTerminalClaim        ToolID = "compozy__terminal_claim"
)

// IsTerminalTool reports whether the tool requires an active session run identity.
func IsTerminalTool(id ToolID) bool {
	switch id {
	case ToolIDTerminalExec, ToolIDTerminalOpen, ToolIDTerminalWrite, ToolIDTerminalRead,
		ToolIDTerminalWait, ToolIDTerminalSignal, ToolIDTerminalClose, ToolIDTerminalList,
		ToolIDTerminalRequestInput, ToolIDTerminalYield, ToolIDTerminalClaim:
		return true
	default:
		return false
	}
}
