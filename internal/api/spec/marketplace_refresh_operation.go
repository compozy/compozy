package spec

import "github.com/compozy/agh/internal/api/contract"

func marketplaceRefreshOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/marketplace/refresh",
		OperationID: "refreshMarketplaceCatalog",
		Summary:     "Refresh all or one feed-backed marketplace kind",
		Tags:        []string{specMarketplaceKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(
				"kind",
				"Optional feed-backed kind",
				[]string{specMCPKey, specExtensionKey, specSkillKey},
			),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MarketplaceRefreshResponse{}},
			{Status: 400, Description: "Invalid or derived refresh kind", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Marketplace catalog is not configured", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
