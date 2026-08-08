package bridges

import (
	"net/url"

	"net/netip"

	bridgecontract "github.com/compozy/compozy/internal/bridges/contract"
)

// WebhookPublicURL returns the configured full external provider callback URL.
func WebhookPublicURL(instance BridgeInstance) (string, error) {
	return bridgecontract.WebhookPublicURL(BridgeInstanceToContract(instance))
}

// WebhookLocalTarget returns the validated loopback destination for gateway callback proxying.
func WebhookLocalTarget(instance BridgeInstance) (*url.URL, error) {
	return bridgecontract.WebhookLocalTarget(BridgeInstanceToContract(instance))
}

// ValidateWebhookDestinationIP rejects any address that is not safe for a
// public callback reachability probe.
func ValidateWebhookDestinationIP(ip netip.Addr) error {
	return bridgecontract.ValidateWebhookDestinationIP(ip)
}
