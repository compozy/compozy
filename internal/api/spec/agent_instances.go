package spec

import "github.com/compozy/compozy/internal/api/contract"

func agentInstanceOperations() []OperationSpec {
	operations := []OperationSpec{
		{
			Method:      "GET",
			Path:        "/api/agent-instances/{id}/occurrences/{occurrence}",
			OperationID: "getAgentInstanceOccurrence",
			Summary:     "Read an instance check occurrence",
			Parameters:  []ParameterSpec{pathParam("id", "Instance ID"), pathParam("occurrence", "Occurrence ID")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Occurrence", Body: contract.AgentInstanceOccurrenceResponse{}},
			},
		},
		{
			Method:      "PATCH",
			Path:        "/api/agent-instances/{id}",
			OperationID: "updateAgentInstance",
			Summary:     "Update instance checks",
			Parameters:  []ParameterSpec{pathParam("id", "Instance ID")},
			RequestBody: contract.AgentInstanceUpdateRequest{},
			Responses:   []ResponseSpec{{Status: 200, Description: "Updated", Body: contract.AgentInstanceResponse{}}},
		},
		{
			Method:      "POST",
			Path:        "/api/agent-instances/{id}/check",
			OperationID: "checkAgentInstance",
			Summary:     "Check an agent instance",
			Parameters:  []ParameterSpec{pathParam("id", "Instance ID")},
			RequestBody: contract.AgentInstanceCheckRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Gate or dry-run result", Body: contract.AgentInstanceCheckResponse{}},
				{Status: 202, Description: "Check admitted", Body: contract.AgentInstanceCheckResponse{}},
			},
		},
		{
			Method:      "GET",
			Path:        "/api/agents/{name}/activity",
			OperationID: "getAgentActivity",
			Summary:     "Read agent activity across authorized workspaces",
			Parameters: []ParameterSpec{
				pathParam("name", "Agent name"),
				intQueryParam("days", "Number of days, from 1 to 365; defaults to 30"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Scoped activity", Body: contract.AgentActivityResponse{}},
			},
		},
		{
			Method:      "POST",
			Path:        "/api/agent-instances",
			OperationID: "createAgentInstance",
			Summary:     "Create a stays-on agent instance",
			RequestBody: contract.AgentInstanceCreateRequest{},
			Responses: []ResponseSpec{
				{Status: 201, Description: "Created", Body: contract.AgentInstanceResponse{}},
				{Status: 200, Description: "Existing instance", Body: contract.AgentInstanceResponse{}},
			},
		},
		{
			Method:      "GET",
			Path:        "/api/agent-instances",
			OperationID: "listAgentInstances",
			Summary:     "List agent instances",
			Parameters: []ParameterSpec{
				queryParam("workspace", "Workspace reference", false),
				queryParam("all_workspaces", "Include all workspaces", false),
				queryParam("agent", "Agent name", false),
				queryParam("state", "Instance state", false),
				queryParam("cursor", "Pagination cursor", false),
				queryParam("limit", "Page size", false),
			},
			Responses: []ResponseSpec{{Status: 200, Description: "OK", Body: contract.AgentInstanceListResponse{}}},
		},
		{
			Method:      "POST",
			Path:        "/api/agent-instances/adopt",
			OperationID: "adoptAgentInstance",
			Summary:     "Adopt a root session as an instance",
			RequestBody: contract.AgentInstanceAdoptRequest{},
			Responses:   []ResponseSpec{{Status: 201, Description: "Created", Body: contract.AgentInstanceResponse{}}},
		},
	}
	for _, verb := range []struct{ method, path, id, summary string }{
		{"GET", "", "getAgentInstance", "Get an agent instance"},
		{"POST", "/pause", "pauseAgentInstance", "Pause an agent instance"},
		{"POST", "/resume", "resumeAgentInstance", "Resume an agent instance"},
		{"POST", "/ensure-main", "ensureAgentInstanceMain", "Ensure the main session exists"},
		{"DELETE", "", "archiveAgentInstance", "Archive an agent instance"},
		{"GET", "/reports", "listAgentInstanceReports", "List durable instance reports"},
		{"GET", "/away", "getAgentInstanceAway", "Read the operator's instance away digest"},
		{"POST", "/away", "markAgentInstanceSeen", "Mark the instance seen by the local operator"},
		{"GET", "/check-ins", "listAgentInstanceCheckIns", "List instance wake events"},
	} {
		operation := OperationSpec{
			Method:      verb.method,
			Path:        "/api/agent-instances/{id}" + verb.path,
			OperationID: verb.id,
			Summary:     verb.summary,
			Parameters:  []ParameterSpec{pathParam("id", "Instance ID")},
			Responses:   []ResponseSpec{{Status: 200, Description: "OK", Body: contract.AgentInstanceResponse{}}},
		}
		if verb.method == "DELETE" {
			operation.Responses = []ResponseSpec{{Status: 204, Description: "Archived"}}
		}
		if verb.path == "/reports" {
			operation.Parameters = append(
				operation.Parameters,
				queryParam("state", "Filter by pending, reserved, delivered, or indeterminate", false),
				queryParam("cursor", "Pagination cursor", false),
				queryParam("limit", "Page size", false),
			)
			operation.Responses = []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.AgentInstanceReportsResponse{}},
			}
		}

		if verb.path == "/away" {
			var body any = contract.AgentInstanceAwayResponse{}
			if verb.method == "POST" {
				body = contract.AgentInstanceSeenResponse{}
			}
			operation.Responses = []ResponseSpec{{Status: 200, Description: "OK", Body: body}}
		}
		if verb.path == "/check-ins" {
			operation.Parameters = append(
				operation.Parameters,
				queryParam("cursor", "Pagination cursor", false),
				intQueryParam("limit", "Page size from 1 to 100; defaults to 50"),
			)
			operation.Responses = []ResponseSpec{
				{
					Status:      200,
					Description: "Wake events, empty when instance wakes are unavailable",
					Body:        contract.AgentInstanceCheckInsResponse{},
				},
			}
		}
		operations = append(operations, operation)
	}
	for i := range operations {
		operations[i].AppAccess = AppAccessDenied
		operations[i].Tags = []string{specAgentsKey}
		operations[i].Transports = []Transport{TransportHTTP, TransportUDS}
		operations[i].Parameters = withProfileSelector(operations[i].Parameters...)
		for _, status := range []int{403, 404, 409, 422, 429, 500, 503} {
			operations[i].Responses = append(
				operations[i].Responses,
				ResponseSpec{Status: status, Description: "Request failed", Body: contract.ErrorPayload{}},
			)
		}
	}
	return operations
}
