package remoteproto

import (
	"net/netip"
	"net/url"
	"time"
)

const (
	HostedTunnelActive          = "active"
	HostedTunnelEntitlementOnly = "entitlement_only"
	HostedRouteReady            = "ready"
	HostedRouteOff              = "off"
	BrowserAuthorizePath        = "/v1/browser/authorize"
	BrowserTokenPath            = "/v1/browser/token"
	BrowserCallbackPath         = "/remote/v1/browser/callback"
	BrowserSessionPath          = "/remote/v1/browser/session"
	BrowserStatusPath           = "/remote/v1/browser/status"
	BrowserPeersPath            = "/remote/v1/browser/peers"
	BrowserApprovalPath         = "/remote/v1/browser/approvals/{id}"
	BrowserStreamTicketsPath    = "/remote/v1/stream-tickets"
)

type BrowserSessionRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
}

type BrowserTokenRequest struct {
	BrowserSessionRequest
	JKT string `json:"jkt"`
}

type GrantResponse struct {
	Grant     string    `json:"grant"`
	ExpiresAt time.Time `json:"expires_at"`
}

type BrowserStatus struct {
	State string `json:"state"`
}

type BrowserSessionResponse struct {
	SessionID string  `json:"session_id"`
	ExpiresAt int64   `json:"expires_at"`
	Scopes    []Scope `json:"scopes"`
}

type BrowserApprovalStatus struct {
	ApprovalID string    `json:"approval_id"`
	State      string    `json:"state"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type BrowserPeersResponse struct {
	Peers []HostedPeer `json:"peers"`
}

type BrowserStreamTicketRequest struct {
	Path string `json:"path"`
}

type BrowserStreamTicketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

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
	return (&url.URL{Scheme: httpScheme, Host: host.String()}).String(), true
}

type HostedTunnel struct {
	Hostname       string `json:"hostname"`
	ConnectorToken string `json:"connector_token"`
	Mode           string `json:"mode"`
}
