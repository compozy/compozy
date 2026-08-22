package spec

import "github.com/compozy/compozy/internal/api/contract"

// specExtensionWorkspaceParamDescription documents the already-supported instance-scope selector
// shared by the extension read and remove operations.
const specExtensionWorkspaceParamDescription = "Operator workspace reference; " +
	"omit for the global instance"

func registryExtensionOperations() []OperationSpec {
	return []OperationSpec{
		listExtensionsOperationSpec(),
		listExtensionCommandsOperationSpec(),
		previewExtensionInstallOperationSpec(),
		installExtensionOperationSpec(),
		getExtensionOperationSpec(),
		getExtensionInventoryOperationSpec(),
		listExtensionSecretsOperationSpec(),
		setExtensionSecretsOperationSpec(),
		deleteExtensionSecretOperationSpec(),
		updateExtensionOperationSpec(),
		removeExtensionOperationSpec(),
		getExtensionProvenanceOperationSpec(),
		listExtensionEnablementOperationSpec(),
		setExtensionEnablementOperationSpec(),
		devExtensionOperationSpec(),
		reloadDevExtensionOperationSpec(),
		getExtensionLogsOperationSpec(),
	}
}

func previewExtensionInstallOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIExtensionsPreviewInstallPath,
		OperationID: "previewExtensionInstall",
		Summary:     "Preview an extension install without changing state",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.InstallExtensionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionInstallPreviewPayload{}},
			{Status: 400, Description: "Invalid install request", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Package validation failure", Body: contract.ExtensionValidationErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func listExtensionCommandsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsCommandsPath,
		OperationID: "listExtensionCommands",
		Summary:     "List active extension-contributed commands",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("extension", "Filter by exact extension name", false),
			queryParam("workspace", "Operator workspace reference; agent callers use trusted session scope", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionCommandsResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
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
			{
				Status:      409,
				Description: "Extension is not dev linked or needs network confirmation",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func getExtensionInventoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNameInventoryPath,
		OperationID: "getExtensionInventory",
		Summary:     "List shipped and live resources for one extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("name", "Extension name")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionInventoryPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func listExtensionSecretsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNameSecretsPath,
		OperationID: "listExtensionSecrets",
		Summary:     "List declared and bound extension environment names",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
			queryParam("workspace", specExtensionWorkspaceParamDescription, false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionSecretsPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func setExtensionSecretsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPIExtensionsNameSecretsPath,
		OperationID: "setExtensionSecrets",
		Summary:     "Set or bind extension environment secrets",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
			queryParam("workspace", specExtensionWorkspaceParamDescription, false),
		},
		RequestBody: contract.SetExtensionSecretsRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionSecretsPayload{}},
			{
				Status:      400,
				Description: "Invalid extension secret binding",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func deleteExtensionSecretOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIExtensionsNameSecretPath,
		OperationID: "deleteExtensionSecret",
		Summary:     "Remove one extension environment secret binding",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
			pathParam("env_name", "Declared environment name"),
			queryParam("workspace", specExtensionWorkspaceParamDescription, false),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: "No Content"},
			{
				Status:      400,
				Description: "Invalid extension secret binding",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
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
			queryParam("follow", "Stream extension_log deltas and atomic extension_log_reset snapshots", false),
			queryParam("after", "Return entries after this sequence within stream_epoch", false),
			queryParam("stream_epoch", "Opaque ring identity; required when after is greater than zero", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionLogsResponse{}},
			{Status: 400, Description: "Invalid extension log cursor", Body: contract.ErrorPayload{}},
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
		Parameters: []ParameterSpec{
			queryParam("workspace", specExtensionWorkspaceParamDescription, false),
			queryParam("profile", "Profile whose effective extension state is returned", false),
		},
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
			{
				Status:      422,
				Description: "Extension trust decision or package validation failure",
				Bodies: responseBodiesOf(
					responseBodyOf[contract.ErrorPayload](),
					responseBodyOf[contract.ExtensionOperationErrorPayload](),
					responseBodyOf[contract.ExtensionValidationErrorPayload](),
				),
			},
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
			queryParam("workspace", specExtensionWorkspaceParamDescription, false),
			queryParam("profile", "Profile whose effective extension state is returned", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionResponse{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func listExtensionEnablementOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNameEnablementPath,
		OperationID: "listExtensionEnablement",
		Summary:     "List one extension's effective state for every profile",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("name", "Extension name")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: []contract.ExtensionEnablementPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func setExtensionEnablementOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPIExtensionsNameEnablementPath,
		OperationID: "setExtensionEnablement",
		Summary:     "Set one extension's effective state for a profile",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{pathParam("name", "Extension name")},
		RequestBody: contract.SetExtensionEnablementRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionEnablementPayload{}},
			{Status: 400, Description: "Invalid profile or enablement state", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
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
			{
				Status:      409,
				Description: "Network confirmation required",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
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
			queryParam("workspace", specExtensionWorkspaceParamDescription, false),
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
