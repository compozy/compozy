package spec

import "github.com/compozy/agh/internal/api/contract"

func marketplaceSearchOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/marketplace/search",
		OperationID: "searchMarketplace",
		Summary:     "Search the grouped marketplace catalog",
		Tags:        []string{specMarketplaceKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("q", "Search query; empty returns curated idle slices", false),
			intQueryParam("limit", "Maximum results per kind from 1 to 100"),
			enumQueryParam(
				specScopeKey,
				"Installed-state projection scope",
				[]string{specGlobalKey, specWorkspaceKey},
			),
			queryParam("workspace_id", "Required for workspace installed-state projection", false),
		},
		Responses: []ResponseSpec{
			{
				Status: 200, Description: "OK; individual source failures are returned in kind.error",
				Body: contract.MarketplaceSearchResponse{},
			},
			{Status: 400, Description: "Invalid marketplace search request", Body: contract.ErrorPayload{}},
			{
				Status: 503, Description: "Marketplace discovery dependencies are not configured",
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
