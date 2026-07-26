package spec

import "github.com/compozy/agh/internal/api/contract"

func marketplaceKindOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/marketplace/{kind}",
		OperationID: "browseMarketplaceKind",
		Summary:     "Browse or search one marketplace kind",
		Tags:        []string{specMarketplaceKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			marketplaceKindPathParam(),
			queryParam("q", "Optional kind search query", false),
			intQueryParam("limit", "Maximum results from 1 to 100"),
			queryParam("cursor", "Opaque next_cursor from the previous page", false),
			enumQueryParam(
				specScopeKey,
				"Installed-state projection scope",
				[]string{specGlobalKey, specWorkspaceKey},
			),
			queryParam("workspace_id", "Required for workspace installed-state projection", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MarketplaceKindResponse{}},
			{Status: 400, Description: "Invalid marketplace browse request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Marketplace kind not found", Body: contract.ErrorPayload{}},
			{
				Status: 503, Description: "Marketplace kind dependency is not configured",
				Body: contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
