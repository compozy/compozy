package spec

import "github.com/compozy/compozy/internal/api/contract"

func registryToolsetOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        "/api/toolsets",
		OperationID: "listToolsets",
		Summary:     "List named toolsets and expansion status",
		Tags:        []string{specToolsetsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			queryParam("workspace_id", "Effective workspace id", false),
			queryParam(specWorkspaceKey, "Effective workspace reference", false),
			queryParam("session_id", "Effective session id", false),
			queryParam("agent_name", "Effective agent name", false),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsetsResponse{}},
			profileToolResponse(400, "Invalid profile selection"),
			profileToolResponse(404, specProfileNotFoundDescription),
			profileToolResponse(409, "Profile selection conflict"),
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
			Parameters: withProfileSelector(
				pathParam("id", "Canonical toolset id"),
				queryParam("workspace_id", "Effective workspace id", false),
				queryParam(specWorkspaceKey, "Effective workspace reference", false),
				queryParam("session_id", "Effective session id", false),
				queryParam("agent_name", "Effective agent name", false),
			),
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.ToolsetResponse{}},
				profileOrToolResponse(400, "Invalid toolset id or profile selection"),
				profileOrToolResponse(404, "Toolset or profile not found"),
				profileToolResponse(409, "Profile selection conflict"),
				{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
				{Status: 503, Description: "Toolset registry unavailable", Body: contract.ErrorPayload{}},
			},
		}}
}
