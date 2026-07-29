package spec

import "github.com/compozy/compozy/internal/api/contract"

func registryExtensionOperations() []OperationSpec {
	return []OperationSpec{
		listExtensionsOperationSpec(),
		installExtensionOperationSpec(),
		getExtensionOperationSpec(),
		updateExtensionOperationSpec(),
		removeExtensionOperationSpec(),
		getExtensionProvenanceOperationSpec(),
		enableExtensionOperationSpec(),
		disableExtensionOperationSpec(),
		devExtensionOperationSpec(),
		reloadDevExtensionOperationSpec(),
		getExtensionLogsOperationSpec(),
	}
}

func devExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIExtensionsDevPath,
		OperationID: "devExtension",
		Summary:     "Link an immutable extension generation to the resolved workspace",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("workspace", "Operator workspace reference; agent callers use trusted session scope", false),
		},
		RequestBody: contract.DevLinkExtensionRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.ExtensionResponse{}},
			{Status: 400, Description: "Invalid dev generation", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func reloadDevExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIExtensionsNameReloadPath,
		OperationID: "reloadDevExtension",
		Summary:     "Reload a workspace extension from an immutable generation",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
			queryParam("workspace", "Operator workspace reference; agent callers use trusted session scope", false),
		},
		RequestBody: contract.ReloadExtensionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionResponse{}},
			{Status: 400, Description: "Invalid generation", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Extension is not dev linked", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func getExtensionLogsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNameLogsPath,
		OperationID: "getExtensionLogs",
		Summary:     "Read or follow redacted extension logs",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
			queryParam("workspace", "Operator workspace reference; omit for the global instance", false),
			queryParam("follow", "Stream new entries as extension_log SSE events", false),
			queryParam("after", "Return entries after this monotonic sequence", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionLogsResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listExtensionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsPath,
		OperationID: "listExtensions",
		Summary:     "List installed extensions",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionsResponse{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func installExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIExtensionsPath,
		OperationID: "installExtension",
		Summary:     "Install a local or marketplace extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.InstallExtensionRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.ExtensionResponse{}},
			{Status: 400, Description: "Invalid install request", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Extension trust decision required", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNamePath,
		OperationID: "getExtension",
		Summary:     "Get one installed extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionResponse{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPIExtensionsNamePath,
		OperationID: "updateExtension",
		Summary:     "Update one marketplace-installed extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
		},
		RequestBody: contract.UpdateExtensionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionUpdateResponse{}},
			{Status: 400, Description: "Invalid update request", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Extension trust decision required", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func removeExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIExtensionsNamePath,
		OperationID: "removeExtension",
		Summary:     "Remove one managed extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionRemoveResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Extension is in use", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getExtensionProvenanceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNameProvenancePath,
		OperationID: "getExtensionProvenance",
		Summary:     "Get extension provenance and trust evidence",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionProvenanceResponse{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func enableExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIExtensionsNameEnablePath,
		OperationID: "enableExtension",
		Summary:     "Enable an installed extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func disableExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIExtensionsNameDisablePath,
		OperationID: "disableExtension",
		Summary:     "Disable an installed extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
