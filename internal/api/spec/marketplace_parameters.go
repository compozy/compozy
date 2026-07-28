package spec

import "github.com/compozy/compozy/internal/api/contract"

func marketplaceKindPathParam() ParameterSpec {
	parameter := pathParam("kind", "Marketplace kind")
	parameter.Enum = []string{
		contract.MarketplaceKindMCP,
		contract.MarketplaceKindExtension,
		contract.MarketplaceKindSkill,
		contract.MarketplaceKindBundle,
	}
	return parameter
}
