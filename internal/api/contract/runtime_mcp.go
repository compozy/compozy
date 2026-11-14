package contract

import "github.com/compozy/compozy/internal/runtimemcp"

type RuntimeMCPServersResponse = runtimemcp.Inventory

type MCPListResponse struct {
	Compozy  []SettingsMCPServerItemPayload `json:"compozy"`
	Runtimes []runtimemcp.Snapshot          `json:"runtimes"`
}
