package spec

import "github.com/compozy/agh/internal/api/contract"

func registryAutomationOperations() []OperationSpec {
	return []OperationSpec{
		listAutomationJobsOperationSpec(),
		createAutomationJobOperationSpec(),
		getAutomationJobOperationSpec(),
		updateAutomationJobOperationSpec(),
		deleteAutomationJobOperationSpec(),
		triggerAutomationJobOperationSpec(),
		listAutomationJobRunsOperationSpec(),
		listAutomationTriggersOperationSpec(),
		createAutomationTriggerOperationSpec(),
		getAutomationTriggerOperationSpec(),
		updateAutomationTriggerOperationSpec(),
		deleteAutomationTriggerOperationSpec(),
		listAutomationTriggerRunsOperationSpec(),
		listAutomationRunsOperationSpec(),
		getAutomationRunOperationSpec(),
		deliverGlobalWebhookOperationSpec(),
		deliverWorkspaceWebhookOperationSpec(),
	}
}
func listAutomationJobsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/jobs",
		OperationID: "listAutomationJobs",
		Summary:     "List automation jobs",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Filter by automation scope", automationScopeValues()),
			queryParam("workspace_id", "Filter by workspace id", false),
			enumQueryParam("source", "Filter by job source", automationSourceValues()),
			boolQueryParam("enabled", "Filter by enabled state"),
			queryParam("q", "Search jobs by name, agent, prompt, scope, source, or schedule", false),
			queryParam("cursor", "Continue after this automation job cursor", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.JobsResponse{}},
			{Status: 400, Description: "Invalid automation filter", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createAutomationJobOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/automation/jobs",
		OperationID: "createAutomationJob",
		Summary:     "Create an automation job",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateJobRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.JobResponse{}},
			{Status: 400, Description: "Invalid automation job request", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Automation job conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getAutomationJobOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIAutomationJobsIDPath,
		OperationID: "getAutomationJob",
		Summary:     "Get one automation job",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation job id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.JobResponse{}},
			{Status: 404, Description: specAutomationJobNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateAutomationJobOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPIAutomationJobsIDPath,
		OperationID: "updateAutomationJob",
		Summary:     "Update one automation job",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation job id"),
		},
		RequestBody: contract.UpdateJobRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.JobResponse{}},
			{Status: 400, Description: "Invalid automation job update", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAutomationJobNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Automation job conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteAutomationJobOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIAutomationJobsIDPath,
		OperationID: "deleteAutomationJob",
		Summary:     "Delete one automation job",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation job id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 400, Description: "Invalid automation job delete request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAutomationJobNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func triggerAutomationJobOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/automation/jobs/{id}/trigger",
		OperationID: "triggerAutomationJob",
		Summary:     "Trigger one automation job immediately",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation job id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.RunResponse{}},
			{Status: 404, Description: specAutomationJobNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Automation run conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listAutomationJobRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/jobs/{id}/runs",
		OperationID: "listAutomationJobRuns",
		Summary:     "List run history for one automation job",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation job id"),
			enumQueryParam("status", "Filter by run status", automationRunStatusValues()),
			dateTimeQueryParam("since", "Only runs started since this timestamp"),
			dateTimeQueryParam("until", "Only runs started before this timestamp"),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.RunsResponse{}},
			{Status: 400, Description: specInvalidAutomationRunFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAutomationJobNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listAutomationTriggersOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/triggers",
		OperationID: "listAutomationTriggers",
		Summary:     "List automation triggers",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Filter by automation scope", automationScopeValues()),
			queryParam("workspace_id", "Filter by workspace id", false),
			enumQueryParam("source", "Filter by trigger source", automationSourceValues()),
			boolQueryParam("enabled", "Filter by enabled state"),
			queryParam("event", "Filter by trigger event", false),
			queryParam("q", "Search triggers by definition or filter fields", false),
			queryParam("cursor", "Continue after this automation trigger cursor", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TriggersResponse{}},
			{Status: 400, Description: "Invalid automation filter", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createAutomationTriggerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/automation/triggers",
		OperationID: "createAutomationTrigger",
		Summary:     "Create an automation trigger",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateTriggerRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.TriggerResponse{}},
			{Status: 400, Description: "Invalid automation trigger request", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Automation trigger conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getAutomationTriggerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIAutomationTriggersIDPath,
		OperationID: "getAutomationTrigger",
		Summary:     "Get one automation trigger",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation trigger id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TriggerResponse{}},
			{Status: 404, Description: specAutomationTriggerNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateAutomationTriggerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPIAutomationTriggersIDPath,
		OperationID: "updateAutomationTrigger",
		Summary:     "Update one automation trigger",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation trigger id"),
		},
		RequestBody: contract.UpdateTriggerRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.TriggerResponse{}},
			{Status: 400, Description: "Invalid automation trigger update", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAutomationTriggerNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Automation trigger conflict", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteAutomationTriggerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIAutomationTriggersIDPath,
		OperationID: "deleteAutomationTrigger",
		Summary:     "Delete one automation trigger",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Automation trigger id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 400, Description: "Invalid automation trigger delete request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAutomationTriggerNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listAutomationTriggerRunsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/automation/triggers/{id}/runs",
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
			{Status: 400, Description: specInvalidAutomationRunFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAutomationTriggerNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
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
			{Status: 400, Description: specInvalidAutomationRunFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
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
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deliverGlobalWebhookOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/webhooks/global/{endpoint}",
		OperationID: "deliverGlobalWebhook",
		Summary:     "Deliver one global automation webhook",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP},
		Parameters: []ParameterSpec{
			pathParam("endpoint", "Webhook endpoint slug and id"),
			headerParam("X-AGH-Webhook-Timestamp", "Signed webhook timestamp"),
			headerParam("X-AGH-Webhook-Signature", "Signed webhook HMAC signature"),
		},
		RequestBody: map[string]any{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WebhookDeliveryResponse{}},
			{Status: 400, Description: "Invalid webhook request", Body: contract.ErrorPayload{}},
			{Status: 401, Description: "Webhook authentication failed", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Webhook trigger not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deliverWorkspaceWebhookOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/webhooks/workspaces/{workspace_id}/{endpoint}",
		OperationID: "deliverWorkspaceWebhook",
		Summary:     "Deliver one workspace-scoped automation webhook",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("endpoint", "Webhook endpoint slug and id"),
			headerParam("X-AGH-Webhook-Timestamp", "Signed webhook timestamp"),
			headerParam("X-AGH-Webhook-Signature", "Signed webhook HMAC signature"),
		},
		RequestBody: map[string]any{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WebhookDeliveryResponse{}},
			{Status: 400, Description: "Invalid webhook request", Body: contract.ErrorPayload{}},
			{Status: 401, Description: "Webhook authentication failed", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Webhook trigger not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specAutomationManagerIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
