package spec

import "github.com/compozy/compozy/internal/api/contract"

func heartbeatAuthorityOperations() []OperationSpec {
	operations := make([]OperationSpec, 0, 3)
	for _, verb := range []string{"show", authorityApproveAction, "revoke"} {
		method, path, id := httpMethodGet, "/api/agents/{name}/heartbeat/authority", "getAgentHeartbeatAuthority"
		response := ResponseSpec{
			Status:      200,
			Description: "Check-in authority",
			Body:        contract.HeartbeatAuthorityPayload{},
		}

		if verb == authorityApproveAction {
			method, path, id = httpMethodPost, path+"/approve", "approveAgentHeartbeatAuthority"
			response = ResponseSpec{Status: 204, Description: "Authority approved"}
		}
		if verb == "revoke" {
			method, path, id = httpMethodDelete, path+"/approve", "revokeAgentHeartbeatAuthority"
			response = ResponseSpec{Status: 204, Description: "Authority revoked"}
		}

		operation := OperationSpec{
			Method:      method,
			Path:        path,
			AppAccess:   AppAccessDenied,
			OperationID: id,
			Summary:     verb + " check-in authority",
			Tags:        []string{authoredContextAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("name", "Agent name"),
				queryParam("workspace_id", "Workspace id", false),
			),
			Responses: []ResponseSpec{
				response,
				{Status: 403, Description: operatorRequiredDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Authority changed", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 422, Description: "Invalid authority", Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Authority storage unavailable", Body: contract.ErrorPayload{}},
			},
		}
		if verb == authorityApproveAction {
			operation.RequestBody = contract.HeartbeatScriptApprovalRequest{}
		}
		operations = append(operations, operation)
	}
	return operations
}

const (
	authorityApproveAction      = "approve"
	operatorRequiredDescription = "Operator required"
)
