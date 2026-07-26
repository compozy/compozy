package spec

import "github.com/compozy/agh/internal/api/contract"

const networkCoordinationUnavailableDescription = "Network coordination is unavailable"

func networkCoordinationOperations() []OperationSpec {
	return append(networkCoordinationStateOperations(), networkUsageOperation())
}

func networkCoordinationStateOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/network-coordination",
			OperationID: "getNetworkCoordination",
			Summary:     "Get scoped network coordination settings, invitation state, and eligibility",
			Tags:        []string{specNetworkKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				requiredCoordinationScopeQueryParam(),
				queryParam("task_id", "Task id required for task scope", false),
				queryParam("run_id", "Run id used for daemon-owned invitation eligibility", false),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.NetworkCoordinationResponse{}},
				{Status: 400, Description: "Invalid coordination scope", Body: contract.ErrorPayload{}},
				{Status: 403, Description: "Coordination scope is not authorized", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: networkCoordinationUnavailableDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPut,
			Path:        "/api/workspaces/{workspace_id}/network-coordination",
			OperationID: "putNetworkCoordination",
			Summary:     "Compare-and-set future-run network coordination for one scope",
			Tags:        []string{specNetworkKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("workspace_id", "Workspace id")},
			RequestBody: contract.PutNetworkCoordinationRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.NetworkCoordinationResponse{}},
				{Status: 400, Description: "Invalid coordination request", Body: contract.ErrorPayload{}},
				{Status: 403, Description: "Coordination mutation is not authorized", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{
					Status:      409,
					Description: "Stale revision or Network participation unavailable",
					Body:        contract.ErrorPayload{},
				},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: networkCoordinationUnavailableDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPut,
			Path:        "/api/workspaces/{workspace_id}/network-coordination/invitation",
			OperationID: "putNetworkCoordinationInvitation",
			Summary:     "Dismiss or reset the coordination invitation for a scope",
			Tags:        []string{specNetworkKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
			},
			RequestBody: contract.PutNetworkCoordinationInvitationRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.NetworkCoordinationResponse{}},
				{Status: 400, Description: "Invalid invitation request", Body: contract.ErrorPayload{}},
				{Status: 403, Description: "Invitation mutation is not authorized", Body: contract.ErrorPayload{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Stale coordination revision", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: networkCoordinationUnavailableDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}

func networkUsageOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/usage",
		OperationID: "getNetworkUsage",
		Summary:     "Get workspace-scoped network wake usage from the ledger",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			enumQueryParam(
				"owner_kind",
				"Filter by participation owner kind; requires owner_id",
				[]string{"session", "task_run", "loop_run", "automation_run"},
			),
			queryParam("owner_id", "Filter by participation owner id; requires owner_kind", false),
			queryParam("run_id", "Filter by task or loop run id; mutually exclusive with owner", false),
			queryParam("channel", "Filter by Network channel", false),
			queryParam("cursor", "Continue from an opaque query-bound usage cursor", false),
			intQueryParam("limit", "Maximum usage details to return; defaults to 50 and cannot exceed 200"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkUsageResponse{}},
			{Status: 400, Description: "Invalid usage query or cursor", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Network usage is unavailable", Body: contract.ErrorPayload{}},
		},
	}
}

func requiredCoordinationScopeQueryParam() ParameterSpec {
	parameter := enumQueryParam("scope", "Coordination scope", []string{specWorkspaceKey, "task"})
	parameter.Required = true
	return parameter
}
