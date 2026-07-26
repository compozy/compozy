package spec

import (
	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

const automationSuggestionsPath = "/api/workspaces/{workspace_id}/automation/suggestions"

func automationSuggestionOperationSpecs() []OperationSpec {
	return []OperationSpec{
		automationSuggestionListOperationSpec(),
		automationSuggestionAcceptOperationSpec(),
		automationSuggestionDismissOperationSpec(),
	}
}

func automationSuggestionListOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        automationSuggestionsPath,
		OperationID: "listAutomationSuggestions",
		Summary:     "List consent-first automation suggestions for one workspace",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			enumQueryParam("status", "Filter by suggestion status", automationSuggestionStatusValues()),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AutomationSuggestionsResponse{}},
			{Status: 400, Description: "Invalid suggestion status", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status: 503, Description: specAutomationManagerIsNotConfiguredDescription,
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func automationSuggestionAcceptOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        automationSuggestionsPath + "/{suggestion_id}/accept",
		OperationID: "acceptAutomationSuggestion",
		Summary:     "Accept one automation suggestion and create its Job",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("suggestion_id", "Automation suggestion id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AutomationSuggestionAcceptanceResponse{}},
			{Status: 400, Description: "Invalid suggested Job", Body: contract.ErrorPayload{}},
			{
				Status: 404, Description: "Workspace or automation suggestion not found",
				Body: contract.ErrorPayload{},
			},
			{
				Status: 409, Description: "Automation suggestion is already resolved",
				Body: contract.ErrorPayload{},
			},
			{
				Status: 503, Description: specAutomationManagerIsNotConfiguredDescription,
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func automationSuggestionDismissOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        automationSuggestionsPath + "/{suggestion_id}/dismiss",
		OperationID: "dismissAutomationSuggestion",
		Summary:     "Durably dismiss one automation suggestion",
		Tags:        []string{specAutomationKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("suggestion_id", "Automation suggestion id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AutomationSuggestionResponse{}},
			{
				Status: 404, Description: "Workspace or automation suggestion not found",
				Body: contract.ErrorPayload{},
			},
			{
				Status: 409, Description: "Automation suggestion is already resolved",
				Body: contract.ErrorPayload{},
			},
			{
				Status: 503, Description: specAutomationManagerIsNotConfiguredDescription,
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func automationSuggestionStatusValues() []string {
	return []string{
		string(automationpkg.SuggestionStatusPending),
		string(automationpkg.SuggestionStatusAccepted),
		string(automationpkg.SuggestionStatusDismissed),
	}
}
