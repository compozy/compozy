package spec

import "github.com/compozy/compozy/internal/api/contract"

func agentSkillReviewOperations() []OperationSpec {
	result := []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/agent-skill-reviews",
			OperationID: "listAgentSkillReviews",
			Summary:     "List agent skill reviews (experimental)",
			Parameters: []ParameterSpec{
				queryParam("instance_id", "Instance ID", false),
				queryParam("workspace", "Workspace reference", false),
				queryParam("state", "pending, approved, rejected or reverted", false),
				queryParam("kind", "new or edit", false),
				queryParam("decided_via", "operator or auto_approve", false),
				queryParam("cursor", "Page cursor bound to the filters it was issued for", false),
				intQueryParam("limit", "Page size, 1-100; default 50"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Skill reviews", Body: contract.AgentSkillReviewsResponse{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/agent-skill-reviews/{id}",
			OperationID: "getAgentSkillReview",
			Summary:     "Read exact proposed skill contents (experimental)",
			Parameters: []ParameterSpec{
				pathParam("id", "Review ID"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Skill review", Body: contract.AgentSkillReviewResponse{}},
			},
		},
	}
	for _, decision := range []struct{ path, id string }{
		{authorityApproveAction, "approveAgentSkillReview"},
		{"reject", "rejectAgentSkillReview"},
		{"revert", "revertAgentSkillReview"},
	} {
		result = append(
			result,
			OperationSpec{
				Method:      httpMethodPost,
				Path:        "/api/agent-skill-reviews/{id}/" + decision.path,
				OperationID: decision.id,
				Summary:     "Decide a skill review as the operator (experimental)",
				Parameters:  []ParameterSpec{pathParam("id", "Review ID")},
				RequestBody: contract.AgentSkillReviewDecisionRequest{},
				Responses: []ResponseSpec{
					{Status: 200, Description: "Decided review", Body: contract.AgentSkillReviewResponse{}},
				},
			},
		)
	}
	return result
}
