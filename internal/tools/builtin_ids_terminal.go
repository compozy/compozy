package tools

// Integrated-terminal native tool IDs. Registration is intentionally deferred
// until the runtime and web surfaces co-ship.
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
