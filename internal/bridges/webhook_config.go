package bridges

import (
	"net/netip"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// WebhookPublicURL returns the configured full external provider callback URL.
func WebhookPublicURL(instance BridgeInstance) (string, error) {
	return bridgecontract.WebhookPublicURL(BridgeInstanceToContract(instance))
}

// ValidateWebhookDestinationIP rejects any address that is not safe for a
// public callback reachability probe.
func ValidateWebhookDestinationIP(ip netip.Addr) error {
	return bridgecontract.ValidateWebhookDestinationIP(ip)
}
