package spec

import "github.com/compozy/compozy/internal/api/contract"

func marketplaceOperations() []OperationSpec {
	return []OperationSpec{
		marketplaceListOperation(),
		marketplaceCatalogEntryOperation(),
		marketplaceSearchOperation(),
		marketplaceKindOperation(),
		marketplaceEntryOperation(),
		marketplaceRefreshOperation(),
	}
}

func marketplaceListOperation() OperationSpec {
	operation := marketplaceKindOperation()
	operation.Path = "/api/marketplace"
	operation.OperationID = "listMarketplace"
	operation.Summary = "List the extension catalog with source state and a content revision"
	operation.Parameters = operation.Parameters[1:]
	operation.Responses = []ResponseSpec{
		{
			Status:      200,
			Description: "Catalog page, including degraded cached sources",
			Body:        contract.MarketplaceListResponse{},
		},
		{Status: 400, Description: "Invalid catalog query or cursor scope", Body: contract.ErrorPayload{}},
		{
			Status:      409,
			Description: "Catalog content changed; restart pagination",
			Body:        contract.MarketplaceCursorStalePayload{},
		},
		{Status: 503, Description: "Catalog is not configured", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
	return operation
}

func marketplaceCatalogEntryOperation() OperationSpec {
	operation := marketplaceEntryOperation()
	operation.Path = "/api/marketplace/entries/{entry_id}"
	operation.OperationID = "getMarketplaceCatalogEntry"
	operation.Summary = "Get a catalog entry or an explicitly selected installed extension"
	operation.Parameters = append(
		operation.Parameters[1:],
		queryParam("source", "Catalog source name; defaults to the Compozy catalog", false),
	)
	return operation
}
