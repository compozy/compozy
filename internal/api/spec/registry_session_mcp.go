package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSessionMCPOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/mcp",
		AppAccess:   AppAccessAllowed,
		OperationID: "getSessionMCP",
		Summary:     "Read session MCP availability without discovery",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Session id")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionMCPResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Session MCP read model unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
