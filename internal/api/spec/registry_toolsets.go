package spec

import "github.com/compozy/agh/internal/api/contract"

func registryToolsetOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        "/api/toolsets",
		OperationID: "listToolsets",
		Summary:     "List named toolsets and expansion status",
		Tags:        []string{specToolsetsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("workspace_id", "Effective workspace id", false),
			queryParam(specWorkspaceKey, "Effective workspace reference", false),
			queryParam("session_id", "Effective session id", false),
			queryParam("agent_name", "Effective agent name", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsetsResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: "Toolset registry unavailable", Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodGet,
			Path:        "/api/toolsets/{id}",
			OperationID: "getToolset",
			Summary:     "Inspect one named toolset expansion",
			Tags:        []string{specToolsetsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("id", "Canonical toolset id"),
				queryParam("workspace_id", "Effective workspace id", false),
				queryParam(specWorkspaceKey, "Effective workspace reference", false),
				queryParam("session_id", "Effective session id", false),
				queryParam("agent_name", "Effective agent name", false),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ToolsetResponse{}},
				{Status: 400, Description: "Invalid toolset id", Body: contract.ToolErrorResponse{}},
				{Status: 404, Description: "Toolset not found", Body: contract.ToolErrorResponse{}},
				{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
				{Status: 503, Description: "Toolset registry unavailable", Body: contract.ErrorPayload{}},
			},
		}}
}
