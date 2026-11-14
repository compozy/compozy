package contract

import "github.com/compozy/compozy/internal/runtimemcp"

type RuntimeMCPServersResponse = runtimemcp.Inventory

type MCPListResponse struct {
	CompozyServers []SettingsMCPServerItemPayload `json:"compozy_servers"`
	Runtimes       []runtimemcp.Snapshot          `json:"runtimes"`
}
