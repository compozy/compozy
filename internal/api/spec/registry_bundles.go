package spec

import "github.com/compozy/agh/internal/api/contract"

func registryBundleOperations() []OperationSpec {
	return []OperationSpec{
		listBundleCatalogOperationSpec(),
		previewBundleActivationOperationSpec(),
		listBundleActivationsOperationSpec(),
		activateBundleOperationSpec(),
		getBundleActivationOperationSpec(),
		updateBundleActivationOperationSpec(),
		deleteBundleActivationOperationSpec(),
		getBundleNetworkSettingsOperationSpec(),
	}
}
func listBundleCatalogOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/bundles/catalog",
		OperationID: "listBundleCatalog",
		Summary:     "List available extension bundle presets",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BundlesCatalogResponse{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func previewBundleActivationOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/bundles/preview",
		OperationID: "previewBundleActivation",
		Summary:     "Preview one bundle activation without mutating runtime resources",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.ActivateBundleRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BundlePreviewResponse{}},
			{Status: 400, Description: "Invalid activation request", Body: contract.ErrorPayload{}},
			{
				Status:      404,
				Description: "Extension, bundle, profile, or workspace not found",
				Body:        contract.ErrorPayload{},
			},
			{Status: 409, Description: specActivationConflictDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid bundle resource reference", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listBundleActivationsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/bundles/activations",
		OperationID: "listBundleActivations",
		Summary:     "List active bundle preset activations",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BundleActivationsResponse{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func activateBundleOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/bundles/activations",
		OperationID: "activateBundle",
		Summary:     "Activate one extension bundle preset",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.ActivateBundleRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.BundleActivationResponse{}},
			{Status: 400, Description: "Invalid activation request", Body: contract.ErrorPayload{}},
			{
				Status:      404,
				Description: "Extension, bundle, profile, or workspace not found",
				Body:        contract.ErrorPayload{},
			},
			{Status: 409, Description: specActivationConflictDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Invalid bundle resource reference", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getBundleActivationOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIBundlesActivationsIDPath,
		OperationID: "getBundleActivation",
		Summary:     "Get one bundle activation",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Bundle activation id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BundleActivationResponse{}},
			{Status: 404, Description: specActivationNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateBundleActivationOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPIBundlesActivationsIDPath,
		OperationID: "updateBundleActivation",
		Summary:     "Update mutable bundle activation overlays",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Bundle activation id"),
		},
		RequestBody: contract.UpdateBundleActivationRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BundleActivationResponse{}},
			{Status: 400, Description: "Invalid update request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specActivationNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specActivationConflictDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteBundleActivationOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIBundlesActivationsIDPath,
		OperationID: "deleteBundleActivation",
		Summary:     "Deactivate one bundle preset and remove owned projected resources",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Bundle activation id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: specActivationNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getBundleNetworkSettingsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/bundles/network/settings",
		OperationID: "getBundleNetworkSettings",
		Summary:     "Get bundle-derived network defaults and declared channels",
		Tags:        []string{specBundlesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.BundleNetworkSettingsResponse{}},
			{Status: 503, Description: specBundleServiceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
