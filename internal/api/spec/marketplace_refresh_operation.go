package spec

import "github.com/compozy/compozy/internal/api/contract"

func marketplaceRefreshOperation() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/marketplace/refresh",
		OperationID: "refreshMarketplaceCatalog",
		Summary:     "Refresh the extension catalog",
		Tags:        []string{specMarketplaceKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MarketplaceRefreshResponse{}},
			{Status: 400, Description: "Unsupported catalog selector", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Marketplace catalog is not configured", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
