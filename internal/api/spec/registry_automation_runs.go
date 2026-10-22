package spec

import "github.com/compozy/compozy/internal/api/contract"

func listAutomationRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/runs",
		AppAccess:   AppAccessAllowed,
		OperationID: "listAutomationRuns",
		Summary:     "List automation runs",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("workspace_id", "Filter by source project; @user selects User", false),
			boolQueryParam("all_workspaces", "Read every run, including unknown attribution"),
			workspaceIDsQueryParam("Source project IDs or @user; excludes unknown attribution"),
			queryParam("cursor", "Opaque source-filter-bound cursor", false),
			queryParam("job_id", "Filter by automation job id", false),
			queryParam("trigger_id", "Filter by automation trigger id", false),
			enumQueryParam("status", "Filter by run status", automationRunStatusValues()),
			dateTimeQueryParam("since", "Only runs started since this timestamp"),
			dateTimeQueryParam("until", "Only runs started before this timestamp"),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.RunsResponse{}},
			{
				Status:      400,
				Description: specInvalidAutomationRunFilterDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      503,
				Description: specAutomationManagerIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	}
}

func getAutomationRunOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/runs/{id}",
		AppAccess:   AppAccessAllowed,
		OperationID: "getAutomationRun",
		Summary:     "Get one automation run",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation run id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.RunResponse{}},
			{Status: 404, Description: "Automation run not found", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specAutomationManagerIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	}
}

func listAutomationTriggerRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/triggers/{id}/runs",
		AppAccess:   AppAccessAllowed,
		OperationID: "listAutomationTriggerRuns",
		Summary:     "List run history for one automation trigger",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation trigger id"),
			enumQueryParam("status", "Filter by run status", automationRunStatusValues()),
			dateTimeQueryParam("since", "Only runs started since this timestamp"),
			dateTimeQueryParam("until", "Only runs started before this timestamp"),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.RunsResponse{}},
			{
				Status:      400,
				Description: specInvalidAutomationRunFilterDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      404,
				Description: specAutomationTriggerNotFoundDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      503,
				Description: specAutomationManagerIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	}
}
