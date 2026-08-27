package spec

import "github.com/compozy/compozy/internal/api/contract"

const callServiceUnavailableDescription = "Call service is not configured"

func registryCallOperations() []OperationSpec {
	operations := callOperationsForPrefix("/api", "")
	return append(operations, callOperationsForPrefix("/api/workspaces/{workspace_id}", "Workspace")...)
}

func callOperationsForPrefix(prefix, operationSuffix string) []OperationSpec {
	workspaceParams := []ParameterSpec(nil)
	if operationSuffix != "" {
		workspaceParams = []ParameterSpec{pathParam("workspace_id", "Workspace id")}
	}
	callPath := prefix + "/calls"
	callIDPath := callPath + "/{call_id}"
	messagePath := prefix + "/messages"
	operations := []OperationSpec{
		{
			Method: httpMethodPost, Path: callPath, OperationID: "createCall" + operationSuffix,
			Summary: "Create one agent call or a bounded batch", Tags: []string{specCallsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(workspaceParams...), RequestBody: contract.CreateCallRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Batch accepted", Body: []contract.CallBatchItemPayload{}},
				{Status: 201, Description: specCreatedDescription, Body: contract.CallCreatePayload{}},
				{Status: 403, Description: "Call target or workspace denied", Body: contract.CallErrorResponse{}},
				{Status: 404, Description: "Agent or target not found", Body: contract.CallErrorResponse{}},
				{Status: 409, Description: "Call identity conflict", Body: contract.CallErrorResponse{}},
				{Status: 410, Description: "Call target expired", Body: contract.CallErrorResponse{}},
				{Status: 422, Description: "Invalid call request", Body: contract.CallErrorResponse{}},
				{Status: 503, Description: callServiceUnavailableDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method: httpMethodGet, Path: callPath, OperationID: "listCalls" + operationSuffix,
			Summary: "List profile-owned calls", Tags: []string{specCallsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(append(workspaceParams,
				queryParam("state", "Filter by call state", false),
				boolQueryParam("attention", "Only return unresolved call attention causes"),
				queryParam("caller", "Filter by caller id", false),
				queryParam("child_session_id", "Filter by receiving child session id", false),
				queryParam("root_session_id", "Filter by governed root session id", false),
				queryParam("agent", "Filter by target agent name", false),
				queryParam("cursor", "Opaque continuation cursor", false),
				intQueryParam("limit", "Page size from 1 to 200"),
			)...),
			Responses: callReadResponses(contract.CallsResponse{}),
		},
		{
			Method: httpMethodGet, Path: callIDPath, OperationID: "getCall" + operationSuffix,
			Summary: "Get one profile-owned call", Tags: []string{specCallsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(append(workspaceParams, pathParam("call_id", "Call id"))...),
			Responses:  callReadResponses(contract.CallPayload{}),
		},
		{
			Method: httpMethodGet, Path: callIDPath + "/prompt", OperationID: "getCallPrompt" + operationSuffix,
			Summary: "Fetch one complete authored call prompt", Tags: []string{specCallsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(append(workspaceParams, pathParam("call_id", "Call id"))...),
			Responses:  callReadResponses(contract.CallPromptResponse{}),
		},
		{
			Method: httpMethodGet, Path: callIDPath + "/result", OperationID: "getCallResult" + operationSuffix,
			Summary: "Fetch one complete call result", Tags: []string{specCallsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(append(workspaceParams, pathParam("call_id", "Call id"))...),
			Responses:  callReadResponses(contract.CallResultResponse{}),
		},
		{
			Method: httpMethodGet, Path: callIDPath + "/superseded", OperationID: "getCallSuperseded" + operationSuffix,
			Summary: "Fetch preserved superseded call evidence", Tags: []string{specCallsKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(append(workspaceParams, pathParam("call_id", "Call id"))...),
			Responses:  callReadResponses(contract.CallSupersededResponse{}),
		},
		callMutationOperation(callIDPath+"/cancel", "cancelCall"+operationSuffix,
			"Cancel one call idempotently", workspaceParams, contract.CancelCallRequest{}, true,
			contract.CancelCallResponse{}),
		callMutationOperation(callIDPath+"/await", "awaitCall"+operationSuffix,
			"Await one call for a bounded interval", workspaceParams, contract.AwaitCallsRequest{}, true,
			contract.AwaitCallsResponse{}),
		callMutationOperation(callIDPath+"/publish", "publishCall"+operationSuffix,
			"Publish one completed call to Compozy Network", workspaceParams, contract.PublishCallRequest{}, false,
			contract.PublishCallResponse{}),
	}
	return append(operations, callMessageOperations(messagePath, operationSuffix, workspaceParams)...)
}

func callMessageOperations(
	messagePath, operationSuffix string,
	workspaceParams []ParameterSpec,
) []OperationSpec {
	return []OperationSpec{
		{
			Method: httpMethodPost, Path: messagePath, OperationID: "sendCallMessage" + operationSuffix,
			Summary: "Send one inert message to a child session", Tags: []string{specMessagesKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(workspaceParams...), RequestBody: contract.SendCallMessageRequest{},
			Responses: []ResponseSpec{
				{Status: 202, Description: specAcceptedDescription, Body: contract.SendCallMessageResponse{}},
				{Status: 403, Description: "Message target or workspace denied", Body: contract.CallErrorResponse{}},
				{Status: 409, Description: "Message target blocked or duplicate", Body: contract.CallErrorResponse{}},
				{Status: 410, Description: "Message target expired", Body: contract.CallErrorResponse{}},
				{Status: 413, Description: "Message is too large", Body: contract.CallErrorResponse{}},
				{Status: 422, Description: "Invalid message request", Body: contract.CallErrorResponse{}},
				{Status: 429, Description: "Message rate limit exceeded", Body: contract.CallErrorResponse{}},
				{Status: 503, Description: callServiceUnavailableDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method: httpMethodGet, Path: messagePath, OperationID: "listCallMessages" + operationSuffix,
			Summary: "List profile-owned call messages", Tags: []string{specMessagesKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(append(workspaceParams,
				queryParam("session", "Filter by recipient session id", false),
				queryParam("cursor", "Opaque continuation cursor", false),
				intQueryParam("limit", "Page size from 1 to 200"),
			)...),
			Responses: callReadResponses(contract.CallMessagesResponse{}),
		},
		{
			Method: httpMethodGet, Path: messagePath + "/{message_id}", OperationID: "getCallMessage" + operationSuffix,
			Summary: "Get one profile-owned call message", Tags: []string{specMessagesKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileScope(append(workspaceParams, pathParam("message_id", "Message id"))...),
			Responses:  callReadResponses(contract.CallMessagePayload{}),
		},
	}
}

func callMutationOperation(
	path, operationID, summary string,
	workspaceParams []ParameterSpec,
	request any,
	requestOptional bool,
	response any,
) OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: path, OperationID: operationID, Summary: summary, Tags: []string{specCallsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  append(withProfileSelector(workspaceParams...), pathParam("call_id", "Call id")),
		RequestBody: request, RequestBodyOptional: requestOptional,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: response},
			{Status: 403, Description: "Call mutation denied", Body: contract.CallErrorResponse{}},
			{Status: 404, Description: "Call not found", Body: contract.CallErrorResponse{}},
			{Status: 409, Description: "Call state conflict", Body: contract.CallErrorResponse{}},
			{Status: 410, Description: "Call target expired", Body: contract.CallErrorResponse{}},
			{Status: 422, Description: "Invalid call request", Body: contract.CallErrorResponse{}},
			{Status: 503, Description: callServiceUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func callReadResponses(body any) []ResponseSpec {
	return []ResponseSpec{
		{Status: 200, Description: "OK", Body: body},
		{Status: 404, Description: "Call or message not found", Body: contract.CallErrorResponse{}},
		{Status: 409, Description: "Call result is not settled", Body: contract.CallErrorResponse{}},
		{Status: 422, Description: "Invalid call read request", Body: contract.CallErrorResponse{}},
		{Status: 503, Description: callServiceUnavailableDescription, Body: contract.ErrorPayload{}},
	}
}
