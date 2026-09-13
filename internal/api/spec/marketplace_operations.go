package spec

import "github.com/compozy/compozy/internal/api/contract"

func marketplaceOperations() []OperationSpec {
	return append([]OperationSpec{
		marketplaceListOperation(),
		marketplaceCatalogEntryOperation(),
		marketplaceRefreshOperation(),
	}, marketplaceSourceOperations()...)
}

func marketplaceListOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/marketplace",
		OperationID: "listMarketplace",
		Summary:     "List the extension catalog with source state and a content revision",
		Tags:        []string{specMarketplaceKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("q", "Optional catalog search query", false),
			intQueryParam("limit", "Maximum results from 1 to 100"),
			queryParam("cursor", "Opaque next_cursor from the previous page", false),
			enumQueryParam(
				specScopeKey,
				"Installed-state projection scope",
				[]string{specGlobalKey, specProfileKey, specWorkspaceKey},
			),
			queryParam(specProfileKey, "Required for profile installed-state projection", false),
			queryParam("workspace_id", "Required for workspace installed-state projection", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MarketplaceListResponse{}},
			{Status: 400, Description: "Invalid marketplace browse request", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Catalog content changed; restart pagination",
				Body:        contract.MarketplaceCursorStalePayload{},
			},
			{
				Status: 503, Description: "Catalog dependency is not configured",
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func marketplaceCatalogEntryOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/marketplace/entries/{entry_id}",
		OperationID: "getMarketplaceCatalogEntry",
		Summary:     "Get a catalog entry or an explicitly selected installed extension",
		Tags:        []string{specMarketplaceKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("source", "Catalog source name; defaults to the Compozy catalog", false),
			pathParam("entry_id", "Stable URL-safe marketplace entry id"),
			queryParam(
				"installed_name",
				"Exact installed extension identity",
				false,
			),
			enumQueryParam(
				specScopeKey,
				"Installed-state projection scope",
				[]string{specGlobalKey, specProfileKey, specWorkspaceKey},
			),
			queryParam(specProfileKey, "Required for profile installed-state projection", false),
			queryParam("workspace_id", "Required for workspace installed-state projection", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MarketplaceEntryResponse{}},
			{Status: 400, Description: "Invalid marketplace detail request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Catalog entry not found", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Package bytes differ from the listed digest",
				Body:        contract.ExtensionOperationErrorPayload{},
			},
			{
				Status: 503, Description: "Marketplace detail dependency is not configured",
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
