package spec

import "github.com/compozy/compozy/internal/api/contract"

func agentMemoryOperations() []OperationSpec {
	operations := make([]OperationSpec, 0, 9)
	for _, item := range []struct {
		method, suffix, id, summary string
		request, response           any
	}{
		{httpMethodGet, "", "getAgentMemory", "Inspect agent memory", nil, contract.AgentMemoryDocument{}},
		{"PUT", "", "putAgentMemory", "Replace memory with a digest check", contract.AgentMemoryWriteRequest{}, contract.AgentMemoryDocument{}},
		{httpMethodPost, agentMemoryEntriesSuffix, "addAgentMemory", "Add a dated memory entry", contract.AgentMemoryAddRequest{}, contract.AgentMemoryDocument{}},
		{httpMethodPatch, agentMemoryEntriesSuffix, "replaceAgentMemory", "Replace matching memory text", contract.AgentMemoryReplaceRequest{}, contract.AgentMemoryDocument{}},
		{httpMethodDelete, agentMemoryEntriesSuffix, "removeAgentMemory", "Remove matching memory entries", contract.AgentMemoryRemoveRequest{}, contract.AgentMemoryRemoveResponse{}},
		{httpMethodPost, "/search", "searchAgentMemory", "Search agent notes and accessible sessions", contract.AgentMemorySearchRequest{}, contract.AgentMemorySearchResponse{}},
		{httpMethodPost, "/validate", "validateAgentMemory", "Validate proposed memory content", contract.AgentMemoryValidateRequest{}, contract.AgentMemoryValidateResponse{}},
		{httpMethodGet, "/history", "listAgentMemoryHistory", "List the last fifty memory revisions", nil, contract.AgentMemoryHistoryResponse{}},
		{httpMethodPost, "/rollback", "rollbackAgentMemory", "Restore a memory revision", contract.AgentMemoryRollbackRequest{}, contract.AgentMemoryDocument{}},
	} {
		operations = append(operations, OperationSpec{
			Method:      item.method,
			Path:        "/api/agents/{name}/memory" + item.suffix,
			OperationID: item.id,
			Stability:   schemaStabilityExperimental,
			Summary:     item.summary + " (experimental)",
			Tags:        []string{"agents"},
			AppAccess:   AppAccessDenied,
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("name", "Agent name"),
				queryParam("workspace", "Workspace selecting the agent definition", false),
			),
			RequestBody: item.request,
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: item.response},
				{Status: 400, Description: "Invalid request or profile selection", Body: contract.ErrorPayload{}},
				{Status: 401, Description: "Caller identity is missing or stale", Body: contract.ErrorPayload{}},
				{Status: 403, Description: "Caller cannot access this agent memory", Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Agent, workspace or revision not found", Body: contract.ErrorPayload{}},
				{
					Status:      409,
					Description: "agent_memory_conflict: reload the current digest",
					Body:        contract.ErrorPayload{},
				},
				{
					Status:      422,
					Description: "agent_not_stays_on, agent_memory_entry_too_large, agent_memory_budget_exceeded or invalid input",
					Body:        contract.ErrorPayload{},
				},
				{Status: 500, Description: "Memory persistence failed", Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Memory service unavailable", Body: contract.ErrorPayload{}},
			},
		})
	}
	return operations
}

const agentMemoryEntriesSuffix = "/entries"
