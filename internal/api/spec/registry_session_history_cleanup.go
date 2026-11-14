package spec

import "github.com/compozy/compozy/internal/api/contract"

const specSessionHistoryCleanupPath = "/api/sessions/history-cleanup"

func sessionHistoryCleanupOperations() []OperationSpec {
	return []OperationSpec{
		planSessionHistoryCleanupOperationSpec(),
		getSessionHistoryCleanupOperationSpec(),
		stopSessionHistoryCleanupOperationSpec(),
	}
}

// planSessionHistoryCleanupOperationSpec previews or starts the deletion of the
// selected profile's archived history; the run continues in the daemon.
func planSessionHistoryCleanupOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specSessionHistoryCleanupPath,
		AppAccess:   AppAccessDenied,
		OperationID: "planSessionHistoryCleanup",
		Summary: "Preview (dry_run) or start the background deletion of archived sessions " +
			"archived before a cutoff: whole threads, or only their child sessions or subagents",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(),
		RequestBody: contract.SessionHistoryCleanupRequest{},
		Responses: append(sessionHistoryCleanupResponses(),
			ResponseSpec{
				Status:      200,
				Description: "Dry run: the sessions the cleanup would delete",
				Body:        contract.SessionHistoryCleanupPreviewResponse{},
			},
			ResponseSpec{
				Status:      202,
				Description: "Accepted: the cleanup runs in the background; follow it with getSessionHistoryCleanup",
				Body:        contract.SessionHistoryCleanupStartResponse{},
			},
			ResponseSpec{Status: 400, Description: "Invalid cleanup request", Body: contract.ErrorPayload{}},
			ResponseSpec{
				Status:      409,
				Description: "A cleanup is already running for this profile",
				Body:        contract.ErrorPayload{},
			},
		),
	}
}

func getSessionHistoryCleanupOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specSessionHistoryCleanupPath,
		AppAccess:   AppAccessDenied,
		OperationID: "getSessionHistoryCleanup",
		Summary:     "Get the selected profile's running or most recent archived-history cleanup",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(),
		Responses: append(sessionHistoryCleanupResponses(), ResponseSpec{
			Status: 200, Description: "OK", Body: contract.SessionHistoryCleanupRunResponse{},
		}),
	}
}

func stopSessionHistoryCleanupOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specSessionHistoryCleanupPath + "/stop",
		AppAccess:   AppAccessDenied,
		OperationID: "stopSessionHistoryCleanup",
		Summary:     "Stop the selected profile's running cleanup; sessions already deleted stay deleted",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(),
		Responses: append(sessionHistoryCleanupResponses(),
			ResponseSpec{Status: 200, Description: "OK", Body: contract.SessionHistoryCleanupRunResponse{}},
			ResponseSpec{
				Status:      404,
				Description: "No cleanup is running for this profile",
				Body:        contract.ErrorPayload{},
			},
		),
	}
}

func sessionHistoryCleanupResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 403, Description: agentIdentityDeniedDescription, Body: contract.ErrorPayload{}},
		{
			Status:      503,
			Description: specServiceUnavailableDependentServiceMissingDescription,
			Body:        contract.ErrorPayload{},
		},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
}
