package spec

import "github.com/compozy/agh/internal/api/contract"

func registryHookOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        "/api/hooks/catalog",
		OperationID: "getHookCatalog",
		Summary:     "List the resolved hook catalog",
		Tags:        []string{specHooksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam(specWorkspaceKey, "Workspace id or path", false),
			queryParam(specAgentKey, "Agent name", false),
			enumQueryParam("event", "Hook event name", hookEventValues()),
			enumQueryParam("source", "Hook source", hookSourceValues()),
			enumQueryParam("mode", "Hook mode", hookModeValues()),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.HookCatalogResponse{}},
			{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodGet,
			Path:        "/api/workspaces/{workspace_id}/hooks/runs",
			OperationID: "getHookRuns",
			Summary:     "List hook run history for one session",
			Tags:        []string{specHooksKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("workspace_id", "Workspace id"),
				queryParam("session", "Session id", true),
				enumQueryParam("event", "Hook event name", hookEventValues()),
				enumQueryParam("outcome", "Hook execution outcome", hookOutcomeValues()),
				dateTimeQueryParam("since", "Only runs recorded since this timestamp"),
				intQueryParam("last", "Maximum number of records to return"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.HookRunsResponse{}},
				{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/hooks/events",
			OperationID: "getHookEvents",
			Summary:     "List supported hook taxonomy metadata",
			Tags:        []string{specHooksKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				enumQueryParam("family", "Hook event family", hookEventFamilyValues()),
				boolQueryParam("sync_only", "Only return sync-eligible events"),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.HookEventsResponse{}},
				{Status: 400, Description: specInvalidFilterDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		}}
}
