package remoteproto

import (
	"net/netip"
	"net/url"
)

const (
	HostedTunnelActive          = "active"
	HostedTunnelEntitlementOnly = "entitlement_only"
	BrowserStatusPath           = "/remote/v1/browser/status"
)

type HostedTunnelRequest struct {
	// Listener is the loopback origin that receives this computer's hosted traffic.
	Listener string `json:"listener,omitempty"`
}

func (r HostedTunnelRequest) Origin() (string, bool) {
	address := r.Listener
	if address == "" {
		address = "127.0.0.1:47824"
	}
	host, err := netip.ParseAddrPort(address)
	if err != nil || !host.Addr().IsLoopback() || host.Port() == 0 || host.Addr().Zone() != "" {
		return "", false
	}
	return (&url.URL{Scheme: "http", Host: host.String()}).String(), true
}

type HostedTunnel struct {
	Hostname       string `json:"hostname"`
	ConnectorToken string `json:"connector_token"`
	Mode           string `json:"mode"`
}
