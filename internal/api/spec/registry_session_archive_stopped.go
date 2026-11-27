package spec

import "github.com/compozy/compozy/internal/api/contract"

// archiveStoppedSessionsOperationSpec archives, or previews with dry_run, every
// eligible stopped thread of the selected profile in one scope.
func archiveStoppedSessionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/sessions/archive-stopped",
		AppAccess:   AppAccessDenied,
		OperationID: "archiveStoppedSessions",
		Summary: "Archive every stopped thread of one workspace or all workspaces that is not " +
			"running, waiting on the operator, holding queued input, pinned, in Later, or an always-on " +
			"agent's main session; dry_run reports the same result without archiving",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(),
		RequestBody: contract.SessionArchiveStoppedRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionArchiveStoppedResponse{}},
			{
				Status:      400,
				Description: "Choose exactly one of workspace_id and all_workspaces",
				Body:        contract.ErrorPayload{},
			},
			{Status: 403, Description: agentIdentityDeniedDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Workspace not found or archived", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
