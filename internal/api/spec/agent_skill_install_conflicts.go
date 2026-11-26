package spec

import "github.com/compozy/compozy/internal/api/contract"

func agentSkillInstallConflictOperations() []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/agent-skill-install-conflicts",
			OperationID: "listAgentSkillInstallConflicts",
			Summary:     "List skills left with two versions by an external edit (experimental)",
			Parameters: []ParameterSpec{
				queryParam("workspace", "Workspace reference; omitted lists every workspace of the profile", false),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Install conflicts", Body: contract.AgentSkillInstallConflictsResponse{}},
			},
		},
		{
			Method:      httpMethodPost,
			Path:        "/api/agent-skill-install-conflicts/{op_id}/resolve",
			OperationID: "resolveAgentSkillInstallConflict",
			Summary:     "Keep the current or previous version of a conflicted skill as the operator (experimental)",
			Parameters: []ParameterSpec{
				pathParam("op_id", "Conflicted operation ID"),
				queryParam("workspace", "Workspace reference; omitted searches every workspace of the profile", false),
			},
			RequestBody: contract.AgentSkillInstallConflictResolveRequest{},
			Responses: []ResponseSpec{
				{
					Status: 200, Description: "Resolved conflict",
					Body: contract.AgentSkillInstallConflictResolveResponse{},
				},
			},
		},
	}
}
