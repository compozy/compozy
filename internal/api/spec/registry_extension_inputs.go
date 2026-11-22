package spec

import "github.com/compozy/compozy/internal/api/contract"

func extensionInputsParameters() []ParameterSpec {
	return []ParameterSpec{
		pathParam("name", "Extension name"),
		queryParam("scope", "Installation scope: global or workspace", false),
		queryParam("workspace", specExtensionWorkspaceParamDescription, false),
	}
}

func getExtensionInputsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNameInputsPath,
		AppAccess:   AppAccessDenied,
		OperationID: "getExtensionInputs",
		Summary:     "Read declared extension inputs and their setup state without values",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  extensionInputsParameters(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionInputsPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ExtensionOperationErrorPayload{}},
		},
	}
}

func setExtensionInputsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPIExtensionsNameInputsPath,
		AppAccess:   AppAccessDenied,
		OperationID: "setExtensionInputs",
		Summary:     "Write extension input values for one installed instance",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  extensionInputsParameters(),
		RequestBody: contract.SetExtensionInputsRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionInputsPayload{}},
			{Status: 400, Description: "Invalid request", Body: contract.ExtensionOperationErrorPayload{}},
			{
				Status:      403,
				Description: "Forbidden, or an agent tried to overwrite inputs that are already set",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ExtensionOperationErrorPayload{}},
			{Status: 422, Description: "Invalid input value", Body: contract.ExtensionOperationErrorPayload{}},
		},
	}
}
