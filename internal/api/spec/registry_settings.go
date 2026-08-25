package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	settingsUpdateUnavailableDescription = "Update surface unavailable"
	specInvalidSettingsScopeDescription  = "Invalid settings scope"
)

func registrySettingsOperations() []OperationSpec {
	return []OperationSpec{
		listSettingsApplyRecordsOperationSpec(),
		reloadSettingsOperationSpec(),
		getSettingsRestartStatusOperationSpec(),
		getSettingsUpdateOperationSpec(),
		applySettingsUpdateOperationSpec(),
		cancelSettingsUpdateOperationSpec(),
		triggerSettingsRestartOperationSpec(),
		getSettingsAutomationOperationSpec(),
		updateSettingsAutomationOperationSpec(),
		listSettingsSandboxesOperationSpec(),
		getSettingsSandboxOperationSpec(),
		putSettingsSandboxOperationSpec(),
		deleteSettingsSandboxOperationSpec(),
		getSettingsGeneralOperationSpec(),
		updateSettingsGeneralOperationSpec(),
		getSettingsPersonaOperationSpec(),
		updateSettingsPersonaOperationSpec(),
		listSettingsHooksOperationSpec(),
		putSettingsHookOperationSpec(),
		deleteSettingsHookOperationSpec(),
		getSettingsHooksExtensionsOperationSpec(),
		updateSettingsHooksExtensionsOperationSpec(),
		listSettingsMCPServersOperationSpec(),
		putSettingsMCPServerOperationSpec(),
		deleteSettingsMCPServerOperationSpec(),
	}
}
func listSettingsApplyRecordsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/apply",
		OperationID: "listSettingsApplyRecords",
		Summary:     "List config apply records for desired and active generation reconciliation",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam("status", "Filter by apply status", configApplyStatusValues()),
			queryParam("actor", "Filter by config apply actor", false),
			intQueryParam("limit", "Maximum number of records to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ConfigApplyRecordsResponse{}},
			{Status: 400, Description: "Invalid apply history filter", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func reloadSettingsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/settings/reload",
		OperationID: "reloadSettings",
		Summary:     "Reconcile config.toml with the daemon active generation",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsRestartStatusOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/actions/restart/{operation_id}",
		OperationID: "getSettingsRestartStatus",
		Summary:     "Get the persisted status for one daemon restart operation",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("operation_id", "Restart operation id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.RestartActionStatus{}},
			{Status: 404, Description: "Restart operation not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsUpdateOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/update",
		OperationID: "getSettingsUpdate",
		Summary:     "Read the current CompozyOS software update status",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsUpdateResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: settingsUpdateUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func applySettingsUpdateOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/settings/update/apply",
		OperationID: "applySettingsUpdate",
		Summary:     "Durably acquire an asynchronous software update operation",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.SettingsUpdateApplyRequest{},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Accepted, blocked, or acquisition-time failure",
				Body:        contract.SettingsUpdateApplyResponse{},
			},
			{Status: 400, Description: "Invalid update target set", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: settingsUpdateUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func cancelSettingsUpdateOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/settings/update/cancel",
		OperationID: "cancelSettingsUpdate",
		Summary:     "Cancel and archive a dormant software update operation",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Canceled or declined with current holder",
				Body:        contract.SettingsUpdateCancelResponse{},
			},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: settingsUpdateUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func triggerSettingsRestartOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/settings/actions/restart",
		OperationID: "triggerSettingsRestart",
		Summary:     "Trigger a daemon restart using the persisted relaunch helper flow",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 202, Description: specAcceptedDescription, Body: contract.RestartActionResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsAutomationOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsAutomationPath,
		OperationID: "getSettingsAutomation",
		Summary:     "Read the automation settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsAutomationResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSettingsAutomationOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsAutomationPath,
		OperationID: "updateSettingsAutomation",
		Summary:     "Update the automation settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsAutomationRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listSettingsSandboxesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/sandboxes",
		OperationID: "listSettingsSandboxes",
		Summary:     "List settings-backed execution sandboxes",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsSandboxesResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsSandboxOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsSandboxesNamePath,
		OperationID: "getSettingsSandbox",
		Summary:     "Read one settings-backed execution sandbox",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Sandbox name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsSandboxResponse{}},
			{Status: 404, Description: "Sandbox not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func putSettingsSandboxOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPISettingsSandboxesNamePath,
		OperationID: "putSettingsSandbox",
		Summary:     "Create or replace one settings-backed execution sandbox",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Sandbox name"),
		},
		RequestBody: contract.PutSettingsSandboxRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: "Invalid sandbox payload", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflicting sandbox change", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteSettingsSandboxOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPISettingsSandboxesNamePath,
		OperationID: "deleteSettingsSandbox",
		Summary:     "Delete one settings-backed execution sandbox overlay",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Sandbox name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Sandbox not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsGeneralOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsGeneralPath,
		OperationID: "getSettingsGeneral",
		Summary:     "Read the general settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsGeneralResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSettingsGeneralOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsGeneralPath,
		OperationID: "updateSettingsGeneral",
		Summary:     "Update the general settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsGeneralRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listSettingsHooksOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/hooks",
		OperationID: "listSettingsHooks",
		Summary:     "List settings-backed hook declarations",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsLayeredParameters(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsHooksResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func putSettingsHookOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPISettingsHooksNamePath,
		OperationID: "putSettingsHook",
		Summary:     "Create or replace one settings-backed hook declaration",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append(
			[]ParameterSpec{pathParam("name", "Hook name")}, settingsLayeredParameters()...,
		),
		RequestBody: contract.PutSettingsHookRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: "Invalid hook payload", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflicting hook change", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteSettingsHookOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPISettingsHooksNamePath,
		OperationID: "deleteSettingsHook",
		Summary:     "Delete one settings-backed hook declaration",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append(
			[]ParameterSpec{pathParam("name", "Hook name")}, settingsLayeredParameters()...,
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Hook not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsHooksExtensionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsHooksExtensionsPath,
		OperationID: "getSettingsHooksExtensions",
		Summary:     "Read the hooks and extensions settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsHooksExtensionsResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSettingsHooksExtensionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsHooksExtensionsPath,
		OperationID: "updateSettingsHooksExtensions",
		Summary:     "Update the hooks and extensions settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsHooksExtensionsRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listSettingsMCPServersOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/mcp-servers",
		OperationID: "listSettingsMCPServers",
		Summary:     "List settings-backed MCP servers",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsLayeredParameters(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsMCPServersResponse{}},
			{Status: 400, Description: specInvalidSettingsScopeDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func putSettingsMCPServerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPISettingsMCPServersNamePath,
		OperationID: "putSettingsMCPServer",
		Summary:     "Create or replace one settings-backed MCP server",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsMCPServerParameters(),
		RequestBody: contract.PutSettingsMCPServerRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: "Invalid MCP server payload", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflicting MCP server change", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteSettingsMCPServerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPISettingsMCPServersNamePath,
		OperationID: "deleteSettingsMCPServer",
		Summary:     "Delete one settings-backed MCP server",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  settingsMCPServerParameters(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: "Invalid MCP server request", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "MCP server or workspace not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflicting MCP server change", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func settingsMCPServerParameters() []ParameterSpec {
	parameters := []ParameterSpec{pathParam("name", "MCP server name")}
	parameters = append(parameters, settingsLayeredParameters()...)
	return append(parameters, enumQueryParam(
		"target",
		"Select the persistence target",
		settingsTargetSelectorValues(),
	))
}
