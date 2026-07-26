package spec

import "github.com/compozy/agh/internal/api/contract"

func marketplaceEntryOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/marketplace/{kind}/{entry_id}",
		OperationID: "getMarketplaceEntry",
		Summary:     "Get one marketplace entry by stable entry id",
		Tags:        []string{specMarketplaceKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			marketplaceKindPathParam(),
			pathParam("entry_id", "Stable URL-safe marketplace entry id"),
			queryParam(
				"installed_name",
				"Exact installed MCP, extension, or skill identity; rejected for bundles",
				false,
			),
			enumQueryParam(
				specScopeKey,
				"Installed-state projection scope",
				[]string{specGlobalKey, specWorkspaceKey},
			),
			queryParam("workspace_id", "Required for workspace installed-state projection", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MarketplaceEntryResponse{}},
			{Status: 400, Description: "Invalid marketplace detail request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Marketplace kind or entry not found", Body: contract.ErrorPayload{}},
			{
				Status: 503, Description: "Marketplace detail dependency is not configured",
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
