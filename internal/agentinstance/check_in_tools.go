package agentinstance

import "context"

type CheckInToolCall struct {
	Seq         int64  `json:"seq"`
	ToolID      string `json:"tool_id"`
	Backend     string `json:"backend"`
	MCPServer   string `json:"mcp_server,omitempty"`
	Access      string `json:"access"`
	Decision    string `json:"decision"`
	DenyReason  string `json:"deny_reason,omitempty"`
	Outcome     string `json:"outcome"`
	Effect      string `json:"effect"`
	ErrorCode   string `json:"error_code,omitempty"`
	ArgsPreview string `json:"args_preview,omitempty"`
	DurationMS  int64  `json:"duration_ms"`
}
type CheckInToolSummary struct {
	Total       int64 `json:"total"`
	WritesDone  int64 `json:"writes_done"`
	WritesMaybe int64 `json:"writes_maybe"`
	Denied      int64 `json:"denied"`
}
type CheckInToolReader interface {
	CheckInToolCalls(context.Context, string, string) ([]CheckInToolCall, error)
	CheckInOccurrence(context.Context, string, string) (CheckIn, error)
}
