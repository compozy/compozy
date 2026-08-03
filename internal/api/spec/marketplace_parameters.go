package spec

import "github.com/compozy/compozy/internal/api/contract"

func marketplaceKindPathParam() ParameterSpec {
	parameter := pathParam("kind", "Marketplace kind")
	parameter.Enum = contract.MarketplaceKindValues()
	return parameter
}
