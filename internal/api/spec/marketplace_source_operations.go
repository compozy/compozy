package spec

import "github.com/compozy/compozy/internal/api/contract"

func marketplaceSourceOperations() []OperationSpec {
	operations := []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/marketplace/sources",
			OperationID: "listMarketplaceSources",
			Summary:     "List global plugin marketplace sources",
			Responses: []ResponseSpec{
				{Status: 200, Description: "Configured sources", Body: contract.MarketplaceSourcesResponse{}},
			},
		},
		{Method: httpMethodPost, Path: "/api/marketplace/sources", OperationID: "addMarketplaceSource",
			Summary:     "Validate or register a plugin marketplace source",
			Parameters:  []ParameterSpec{boolQueryParam("dry_run", "Inspect packages without registering the source")},
			RequestBody: contract.AddMarketplaceSourceRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Source preview", Body: contract.MarketplaceSourcePreviewPayload{}},
				{Status: 201, Description: "Registered source", Body: contract.MarketplaceSourceResponse{}},
			}},
		{
			Method:      httpMethodPatch,
			Path:        "/api/marketplace/sources/{name}",
			OperationID: "updateMarketplaceSource",
			Summary:     "Enable or disable a plugin marketplace source",
			Parameters:  []ParameterSpec{pathParam("name", "Registered source name")},
			RequestBody: contract.UpdateMarketplaceSourceRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Updated source", Body: contract.MarketplaceSourceResponse{}},
			},
		},
		{Method: httpMethodDelete, Path: "/api/marketplace/sources/{name}", OperationID: "removeMarketplaceSource",
			Summary:    "Remove a custom source without removing installed extensions",
			Parameters: []ParameterSpec{pathParam("name", "Custom source name")},
			Responses:  []ResponseSpec{{Status: 204, Description: "Source removed"}}},
		{
			Method:      httpMethodPost,
			Path:        "/api/marketplace/sources/{name}/refresh",
			OperationID: "refreshMarketplaceSource",
			Summary:     "Refresh one enabled marketplace source",
			Parameters:  []ParameterSpec{pathParam("name", "Registered source name")},
			Responses: []ResponseSpec{
				{
					Status:      200,
					Description: "Current source state, including degraded refresh",
					Body:        contract.MarketplaceSourceResponse{},
				},
			},
		},
	}
	for index := range operations {
		operation := &operations[index]
		operation.Tags = []string{specMarketplaceKey}
		operation.Transports = []Transport{TransportHTTP, TransportUDS}
		operation.Stability = "experimental"
		operation.Responses = append(operation.Responses, marketplaceSourceErrorResponses()...)
	}
	return operations
}

func marketplaceSourceErrorResponses() []ResponseSpec {
	return []ResponseSpec{
		{Status: 400, Description: "Invalid request", Body: contract.ErrorPayload{}},
		{
			Status:      403,
			Description: "Mutation forbidden or preset removal refused",
			Body:        contract.MarketplaceSourceErrorPayload{},
		},
		{Status: 404, Description: "Source not found", Body: contract.MarketplaceSourceErrorPayload{}},
		{
			Status:      409,
			Description: "Source name already exists or is retained by installed extensions",
			Body:        contract.MarketplaceSourceErrorPayload{},
		},
		{
			Status:      422,
			Description: "Source reference, name, or document rejected",
			Body:        contract.MarketplaceSourceErrorPayload{},
		},
		{
			Status:      503,
			Description: "Source unavailable",
			Body:        contract.MarketplaceSourceErrorPayload{},
		},
		{
			Status:      500,
			Description: specInternalServerErrorDescription,
			Body:        contract.MarketplaceSourceErrorPayload{},
		},
	}
}
