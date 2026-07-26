package spec

import "github.com/compozy/agh/internal/api/contract"

func registryToolOperations() []OperationSpec {
	return []OperationSpec{
		listToolsOperationSpec(),
		searchToolsOperationSpec(),
		getToolOperationSpec(),
		createToolApprovalOperationSpec(),
		invokeToolOperationSpec(),
		listSessionToolsOperationSpec(),
		searchSessionToolsOperationSpec(),
	}
}
func listToolsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/tools",
		OperationID: "listTools",
		Summary:     "List operator-visible registry tools",
		Tags:        []string{specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("workspace_id", "Effective workspace id", false),
			queryParam(specWorkspaceKey, "Effective workspace reference", false),
			queryParam("session_id", "Effective session id", false),
			queryParam("agent_name", "Effective agent name", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: specToolRegistryUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func searchToolsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tools/search",
		OperationID: "searchTools",
		Summary:     "Search operator-visible registry tools",
		Tags:        []string{specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.ToolSearchRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsResponse{}},
			{Status: 400, Description: "Malformed search request", Body: contract.ToolErrorResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: specToolRegistryUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getToolOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/tools/{id}",
		OperationID: "getTool",
		Summary:     "Get one operator-visible registry tool",
		Tags:        []string{specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Canonical tool id"),
			queryParam("workspace_id", "Effective workspace id", false),
			queryParam(specWorkspaceKey, "Effective workspace reference", false),
			queryParam("session_id", "Effective session id", false),
			queryParam("agent_name", "Effective agent name", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolResponse{}},
			{Status: 400, Description: "Invalid tool id", Body: contract.ToolErrorResponse{}},
			{Status: 404, Description: specToolNotFoundDescription, Body: contract.ToolErrorResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: specToolRegistryUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createToolApprovalOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tools/{id}/approvals",
		OperationID: "createToolApproval",
		Summary:     "Mint a local single-use approval token for one tool invocation",
		Tags:        []string{specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Canonical tool id"),
		},
		RequestBody: contract.ToolApprovalRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.ToolApprovalResponse{}},
			{Status: 400, Description: "Invalid approval request", Body: contract.ToolErrorResponse{}},
			{Status: 403, Description: "Approval denied", Body: contract.ToolErrorResponse{}},
			{Status: 404, Description: specToolNotFoundDescription, Body: contract.ToolErrorResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: "Tool approval service unavailable", Body: contract.ErrorPayload{}},
		},
	}
}
func invokeToolOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/tools/{id}/invoke",
		OperationID: "invokeTool",
		Summary:     "Invoke a registry tool through executable dispatch",
		Tags:        []string{specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Canonical tool id"),
		},
		RequestBody: contract.ToolInvokeRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Completed", Body: contract.ToolInvokeResponse{}},
			{Status: 202, Description: "Approval required", Body: contract.ToolErrorResponse{}},
			{Status: 400, Description: "Invalid invocation request", Body: contract.ToolErrorResponse{}},
			{Status: 403, Description: "Invocation denied", Body: contract.ToolErrorResponse{}},
			{Status: 404, Description: specToolNotFoundDescription, Body: contract.ToolErrorResponse{}},
			{Status: 409, Description: "Tool conflict", Body: contract.ToolErrorResponse{}},
			{Status: 422, Description: "Tool unavailable or not executable", Body: contract.ToolErrorResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 502, Description: "Backend adapter failure", Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: specToolRegistryUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listSessionToolsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/tools",
		OperationID: "listSessionTools",
		Summary:     "List session-callable registry tools",
		Tags:        []string{specSessionsKey, specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			queryParam("agent_name", "Effective agent name", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: specToolRegistryUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func searchSessionToolsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/tools/search",
		OperationID: "searchSessionTools",
		Summary:     "Search session-callable registry tools",
		Tags:        []string{specSessionsKey, specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.ToolSearchRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsResponse{}},
			{Status: 400, Description: "Malformed search request", Body: contract.ToolErrorResponse{}},
			{Status: 500, Description: specInternalDaemonErrorDescription, Body: contract.ToolErrorResponse{}},
			{Status: 503, Description: specToolRegistryUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}
