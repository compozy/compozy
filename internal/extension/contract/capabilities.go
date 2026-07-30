package contract

import extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"

// ProvideCapabilityContract binds a daemon-recognized provide surface to its service methods.
type ProvideCapabilityContract struct {
	Provide string
	Methods []string
	Public  bool
}

// ProvideCapabilityContracts returns the daemon capability registry in stable order.
func ProvideCapabilityContracts() []ProvideCapabilityContract {
	provides := extensionprotocol.ProvideCapabilities()
	contracts := make([]ProvideCapabilityContract, 0, len(provides))
	for _, provide := range provides {
		contracts = append(contracts, ProvideCapabilityContract{
			Provide: provide,
			Methods: extensionprotocol.RequiredServiceMethods(provide),
			Public:  provide != extensionprotocol.CapabilityProvideBridgeAdapter,
		})
	}
	return contracts
}
