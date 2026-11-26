package spec

import "github.com/compozy/compozy/internal/api/contract"

func listExtensionSecretsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsNameSecretsPath,
		AppAccess:   AppAccessDenied,
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
		AppAccess:   AppAccessDenied,
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
		AppAccess:   AppAccessDenied,
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
