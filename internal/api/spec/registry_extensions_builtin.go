package spec

import "github.com/compozy/compozy/internal/api/contract"

// Built-in restore operations: the permanently removed built-in listing and the restore action. Both are
// registered beside the other extension operations in registryExtensionOperations.

func restoreExtensionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        specAPIExtensionsNameRestorePath,
		AppAccess:   AppAccessDenied,
		OperationID: "restoreExtension",
		Summary:     "Restore one permanently removed built-in extension",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Extension name"),
		},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Restored at the bundled version, enabled",
				Body:        contract.ExtensionRestorePayload{},
			},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specExtensionNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Not permanently removed, or a non-bundled extension of the same name is installed",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
			{
				Status:      501,
				Description: "Restore is not available on this extension service",
				Body:        contract.ErrorPayload{},
			},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{
				Status:      500,
				Description: "Enrollment failed; the removed state is restored and the tombstone is kept",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
		},
	}
}

func listRemovedBuiltinExtensionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIExtensionsRemovedPath,
		AppAccess:   AppAccessDenied,
		OperationID: "listRemovedBuiltinExtensions",
		Summary:     "List permanently removed built-in extensions",
		Tags:        []string{specExtensionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  []ParameterSpec{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ExtensionRemovedBuiltinsResponse{}},
			{
				Status:      501,
				Description: "Listing is not available on this extension service",
				Body:        contract.ErrorPayload{},
			},
			{Status: 503, Description: specExtensionServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
