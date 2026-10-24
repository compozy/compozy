package spec

import "github.com/compozy/compozy/internal/api/contract"

func refreshSessionPullRequestOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/sessions/{session_id}/pull-request/refresh",
		AppAccess:   AppAccessAllowed,
		OperationID: "refreshSessionPullRequest",
		Summary:     "Refresh a session pull request",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(pathParam("session_id", "Session id")),
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Refreshed or retained cached facts",
				Body:        contract.SessionPullRequestResponse{},
			},
			{Status: 404, Description: authoredContextSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Session branch unavailable", Body: contract.SessionPullRequestErrorPayload{}},
			{
				Status:      502,
				Description: "Forge unavailable without cached facts",
				Body:        contract.SessionPullRequestErrorPayload{},
			},
		},
	}
}
