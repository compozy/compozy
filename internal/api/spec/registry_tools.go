package spec

import "github.com/compozy/compozy/internal/api/contract"

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
		Parameters: withProfileSelector(
			queryParam("workspace_id", "Effective workspace id", false),
			queryParam(specWorkspaceKey, "Effective workspace reference", false),
			queryParam("session_id", "Effective session id", false),
			queryParam("agent_name", "Effective agent name", false),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsResponse{}},
			profileToolResponse(400, "Invalid profile selection"),
			profileToolResponse(404, specProfileNotFoundDescription),
			profileToolResponse(409, "Profile selection conflict"),
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
		Parameters:  withProfileSelector(),
		RequestBody: contract.ToolSearchRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolsResponse{}},
			profileOrToolResponse(400, "Malformed search request or invalid profile selection"),
			profileToolResponse(404, specProfileNotFoundDescription),
			profileToolResponse(409, "Profile selection conflict"),
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
		Parameters: withProfileSelector(
			pathParam("id", "Canonical tool id"),
			queryParam("workspace_id", "Effective workspace id", false),
			queryParam(specWorkspaceKey, "Effective workspace reference", false),
			queryParam("session_id", "Effective session id", false),
			queryParam("agent_name", "Effective agent name", false),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolResponse{}},
			profileOrToolResponse(400, "Invalid tool id or profile selection"),
			profileOrToolResponse(404, "Tool or profile not found"),
			profileToolResponse(409, "Profile selection conflict"),
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
		Parameters:  withProfileSelector(pathParam("id", "Canonical tool id")),
		RequestBody: contract.ToolApprovalRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.ToolApprovalResponse{}},
			profileOrToolResponse(400, "Invalid approval request or profile selection"),
			{Status: 403, Description: "Approval denied", Body: contract.ToolErrorResponse{}},
			profileOrToolResponse(404, "Tool or profile not found"),
			profileToolResponse(409, "Profile selection conflict"),
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
		Parameters:  withProfileSelector(pathParam("id", "Canonical tool id")),
		RequestBody: contract.ToolInvokeRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Completed", Body: contract.ToolInvokeResponse{}},
			{Status: 202, Description: "Approval required", Body: contract.ToolErrorResponse{}},
			profileOrToolResponse(400, "Invalid invocation request or profile selection"),
			{Status: 403, Description: "Invocation denied", Body: contract.ToolErrorResponse{}},
			profileOrToolResponse(404, "Tool or profile not found"),
			profileOrToolResponse(409, "Tool or profile conflict"),
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

func profileToolResponse(status int, description string) ResponseSpec {
	return ResponseSpec{Status: status, Description: description, Body: contract.ProfileErrorPayload{}}
}

func profileOrToolResponse(status int, description string) ResponseSpec {
	return ResponseSpec{
		Status: status, Description: description,
		Bodies: responseBodiesOf(
			responseBodyOf[contract.ProfileErrorPayload](),
			responseBodyOf[contract.ToolErrorResponse](),
		),
	}
}
