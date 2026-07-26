package spec

import "github.com/compozy/agh/internal/api/contract"

func authoredContextSessionOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/health",
		OperationID: "getSessionHealth",
		Summary:     "Read metadata-only session health and wake eligibility",
		Tags:        []string{authoredContextSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionHealthResponse{}},
			{
				Status:      403,
				Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: authoredContextSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: authoredContextInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/status",
			OperationID: "getSessionStatus",
			Summary:     "Read compact session status and wake eligibility",
			Tags:        []string{authoredContextSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.SessionStatusResponse{}},
				{
					Status:      403,
					Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
					Body:        contract.ErrorPayload{},
				},
				{Status: 404, Description: authoredContextSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{
					Status:      500,
					Description: authoredContextInternalServerErrorDescription,
					Body:        contract.ErrorPayload{},
				},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        authoredContextSessionInspectPath,
			OperationID: "inspectSession",
			Summary:     "Inspect session health, wake audit, and policy correlation metadata",
			Tags:        []string{authoredContextSessionsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				pathParam("session_id", "Session id"),
				boolQueryParam("include_recent_wake_events", "Include recent wake audit rows"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.SessionInspectResponse{}},
				{
					Status:      403,
					Description: authoredContextForbiddenWorkspaceOrPermissionMismatchDescription,
					Body:        contract.ErrorPayload{},
				},
				{Status: 404, Description: authoredContextSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{
					Status:      500,
					Description: authoredContextInternalServerErrorDescription,
					Body:        contract.ErrorPayload{},
				},
			},
		}}
}
