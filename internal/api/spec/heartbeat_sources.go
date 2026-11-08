package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	sourceInternalErrorDescription      = "Internal error"
	sourceStorageUnavailableDescription = "Source storage unavailable"
)

func heartbeatSourceOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/agents/{name}/heartbeat/sources",
			AppAccess:   AppAccessAllowed,
			OperationID: "getAgentHeartbeatSources",
			Summary:     "List check-in source kinds, scripts and effective limits",
			Tags:        []string{authoredContextAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("name", "Agent name"),
				queryParam("workspace_id", "Workspace id", false),
			),
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.HeartbeatSourceCatalogPayload{}},
				{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: sourceInternalErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: sourceStorageUnavailableDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/agents/{name}/heartbeat/scripts/{script}/approve",
			AppAccess:   AppAccessDenied,
			OperationID: "approveAgentHeartbeatScript",
			Summary:     "Approve exact source script bytes (operator only)",
			Tags:        []string{authoredContextAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("name", "Agent name"),
				pathParam("script", "Script name"),
				queryParam("workspace_id", "Workspace id", false),
			),
			RequestBody: contract.HeartbeatScriptApprovalRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Approved", Body: contract.HeartbeatScriptApprovalPayload{}},
				{Status: 403, Description: operatorRequiredDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 422, Description: "Invalid source script request", Body: contract.ErrorPayload{}},
				{Status: 500, Description: sourceInternalErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: sourceStorageUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Script changed", Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodDelete,
			Path:        "/api/agents/{name}/heartbeat/scripts/{script}/approve",
			AppAccess:   AppAccessDenied,
			OperationID: "revokeAgentHeartbeatScript",
			Summary:     "Revoke a source script approval (operator only)",
			Tags:        []string{authoredContextAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("name", "Agent name"),
				pathParam("script", "Script name"),
				queryParam("workspace_id", "Workspace id", false),
			),
			Responses: []ResponseSpec{
				{Status: 204, Description: "Script approval revoked"},
				{Status: 403, Description: operatorRequiredDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 422, Description: "Invalid source script request", Body: contract.ErrorPayload{}},
				{Status: 500, Description: sourceInternalErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: sourceStorageUnavailableDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
