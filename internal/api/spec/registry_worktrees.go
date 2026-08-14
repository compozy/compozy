package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specAPIWorktreesPath           = "/api/workspaces/{workspace_id}/worktrees"
	specAPIWorktreePath            = specAPIWorktreesPath + "/{worktree_id}"
	specAPIWorktreeStatusPath      = specAPIWorktreePath + "/status"
	specAPIWorktreeExitPath        = specAPIWorktreePath + "/exit"
	specAPIWorktreeStreamPath      = specAPIWorktreePath + "/stream"
	specAPIWorktreeCatalogPath     = "/api/worktrees/catalog-stream"
	worktreeUnavailableDescription = "Worktree service is unavailable"
	worktreeNotFoundDescription    = "Worktree or workspace not found"
)

func registryWorktreeOperations() []OperationSpec {
	return []OperationSpec{
		listWorktreesOperationSpec(),
		createWorktreeOperationSpec(),
		adoptWorktreeOperationSpec(),
		inspectWorktreeOperationSpec(),
		worktreeStatusOperationSpec(),
		worktreeExitPlanOperationSpec(),
		runWorktreeExitActionOperationSpec(),
		cancelWorktreeExitActionOperationSpec(),
		cancelWorktreeOperationSpec(),
		removeWorktreeOperationSpec(),
		dismissWorktreeOperationSpec(),
		streamWorktreeOperationSpec(),
		streamWorktreeCatalogOperationSpec(),
	}
}

func listWorktreesOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIWorktreesPath, OperationID: "listWorktrees",
		Summary: "List registered and discovered worktrees", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: append(worktreeWorkspaceParams(), boolQueryParam("refresh", "Refresh Git discovery and status")),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorktreesResponse{}},
			{Status: 400, Description: "Invalid worktree list query", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: worktreeUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func createWorktreeOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIWorktreesPath, OperationID: "createWorktree",
		Summary: "Accept a worktree creation", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: worktreeWorkspaceParams(),
		RequestBody: contract.CreateWorktreeRequest{},
		Responses: append(worktreeMutationResponses(),
			ResponseSpec{Status: 202, Description: specAcceptedDescription, Body: contract.WorktreeResponse{}},
		),
	}
}

func adoptWorktreeOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIWorktreesPath + "/adopt", OperationID: "adoptWorktree",
		Summary: "Adopt an existing linked worktree", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: worktreeWorkspaceParams(),
		RequestBody: contract.AdoptWorktreeRequest{},
		Responses: append(worktreeMutationResponses(),
			ResponseSpec{Status: 200, Description: "OK", Body: contract.WorktreeResponse{}},
		),
	}
}

func inspectWorktreeOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIWorktreePath, OperationID: "inspectWorktree",
		Summary: "Inspect one worktree", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: worktreeRouteParams(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorktreeInspectionResponse{}},
			{Status: 404, Description: worktreeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: worktreeUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func worktreeStatusOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIWorktreeStatusPath, OperationID: "getWorktreeStatus",
		Summary: "Read or refresh worktree status", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: append(worktreeRouteParams(),
			boolQueryParam("refresh", "Refresh local Git status"),
			boolQueryParam("forge", "Refresh the serving forge provider"),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorktreeStatusResponse{}},
			{Status: 400, Description: "Invalid status query", Body: contract.ErrorPayload{}},
			{Status: 404, Description: worktreeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Worktree status is unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 502, Description: "Forge provider error", Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Worktree or forge service is unavailable", Body: contract.ErrorPayload{}},
		},
	}
}

func worktreeExitPlanOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIWorktreeExitPath, OperationID: "getWorktreeExitPlan",
		Summary: "Read the worktree exit plan", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS}, Parameters: worktreeRouteParams(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorktreeExitPlanResponse{}},
			{Status: 404, Description: worktreeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Worktree exit plan is unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: worktreeUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func runWorktreeExitActionOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIWorktreeExitPath + "/actions",
		OperationID: "runWorktreeExitAction", Summary: "Run a worktree exit action",
		Tags: []string{specWorktreesKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: worktreeRouteParams(), RequestBody: contract.RunWorktreeExitActionRequest{},
		Responses: append(
			worktreeExitMutationResponses(),
			ResponseSpec{
				Status:      202,
				Description: specAcceptedDescription,
				Body:        contract.WorktreeExitOperationResponse{},
			},
		),
	}
}

func cancelWorktreeExitActionOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIWorktreeExitPath + "/cancel",
		OperationID: "cancelWorktreeExitAction", Summary: "Cancel a worktree exit action",
		Tags: []string{specWorktreesKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: worktreeRouteParams(), RequestBody: contract.CancelWorktreeExitActionRequest{},
		Responses: append(worktreeExitMutationResponses(),
			ResponseSpec{Status: 204, Description: specNoContentDescription},
		),
	}
}

func worktreeExitMutationResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 400, Description: "Invalid worktree exit request", Body: contract.ErrorPayload{}},
		{Status: 404, Description: worktreeNotFoundDescription, Body: contract.ErrorPayload{}},
		{Status: 409, Description: "Worktree exit action conflict", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		{Status: 502, Description: "Forge provider error", Body: contract.ErrorPayload{}},
		{Status: 503, Description: "Worktree or forge service is unavailable", Body: contract.ErrorPayload{}},
	}
}

func cancelWorktreeOperationSpec() OperationSpec {
	return worktreeNoContentOperation(
		httpMethodPost, specAPIWorktreePath+"/cancel", "cancelWorktreeCreate", "Cancel a pending worktree creation",
	)
}

func removeWorktreeOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodDelete, Path: specAPIWorktreePath, OperationID: "removeWorktree",
		Summary: "Remove a worktree", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: append(worktreeRouteParams(), boolQueryParam("force", "Confirm destructive removal")),
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 400, Description: "Invalid removal query", Body: contract.ErrorPayload{}},
			{Status: 404, Description: worktreeNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Removal requires another decision",
				Body:        contract.WorktreeRemovalRefusalPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: worktreeUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func dismissWorktreeOperationSpec() OperationSpec {
	return worktreeNoContentOperation(
		httpMethodPost, specAPIWorktreePath+"/dismiss", "dismissWorktree", "Dismiss a worktree tombstone",
	)
}

func streamWorktreeOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIWorktreeStreamPath, OperationID: "streamWorktree",
		Summary: "Stream durable worktree lifecycle events", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: append(worktreeRouteParams(),
			afterSequenceQueryParam("Replay events after this durable sequence"),
			optionalHeaderParam("Last-Event-ID", "Resume after this durable sequence"),
		),
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Canonical worktree event stream",
				Body:        map[string]any{},
				ContentType: specContentTypeEventStream,
			},
			{Status: 400, Description: "Invalid stream cursor", Body: contract.ErrorPayload{}},
			{Status: 404, Description: worktreeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Worktree stream is unavailable", Body: contract.ErrorPayload{}},
		},
	}
}

func streamWorktreeCatalogOperationSpec() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIWorktreeCatalogPath, OperationID: "streamWorktreeCatalog",
		Summary: "Stream worktree catalog changes across workspaces", Tags: []string{specWorktreesKey},
		Transports: []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Workspace-identified worktree catalog event stream",
				Body: contract.WorktreeCatalogEventPayload{}, ContentType: specContentTypeEventStream},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Worktree catalog stream is unavailable", Body: contract.ErrorPayload{}},
		},
	}
}

func worktreeNoContentOperation(method, path, operationID, summary string) OperationSpec {
	return OperationSpec{
		Method: method, Path: path, OperationID: operationID, Summary: summary,
		Tags: []string{specWorktreesKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: worktreeRouteParams(),
		Responses: append(worktreeMutationResponses(),
			ResponseSpec{Status: 204, Description: specNoContentDescription},
		),
	}
}

func worktreeMutationResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 400, Description: "Invalid worktree request", Body: contract.ErrorPayload{}},
		{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
		{Status: 404, Description: worktreeNotFoundDescription, Body: contract.ErrorPayload{}},
		{Status: 409, Description: "Worktree operation conflict", Body: contract.ErrorPayload{}},
		{Status: 422, Description: "Invalid worktree configuration", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		{Status: 503, Description: worktreeUnavailableDescription, Body: contract.ErrorPayload{}},
	}
}

func worktreeWorkspaceParams() []ParameterSpec {
	return []ParameterSpec{pathParam("workspace_id", "Workspace id or path")}
}

func worktreeRouteParams() []ParameterSpec {
	return []ParameterSpec{
		pathParam("workspace_id", "Workspace id or path"),
		pathParam("worktree_id", "Worktree id, name, or path"),
	}
}
