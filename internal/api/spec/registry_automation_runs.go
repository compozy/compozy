package spec

import "github.com/compozy/compozy/internal/api/contract"

func listAutomationRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/runs",
		OperationID: "listAutomationRuns",
		Summary:     "List automation runs",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
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
