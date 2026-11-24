package cloudclient

import (
	"context"
	"crypto/ed25519"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/compozy/compozy/internal/remoteproto"
)

type ComputerLinkRequest struct {
	Name              string          `json:"name"`
	Platform          string          `json:"platform"`
	Version           string          `json:"version"`
	ComputerJWK       remoteproto.JWK `json:"computer_jwk"`
	TLSRootSHA256     string          `json:"tls_root_sha256"`
	InstallationID    string          `json:"installation_id,omitempty"`
	ExpectedAccountID string          `json:"expected_account_id,omitempty"`
	LinkToken         string          `json:"link_token,omitempty"`
}

type ComputerLink struct {
	ComputerID remoteproto.ComputerID `json:"computer_id"`
	AccountID  string                 `json:"account_id"`
	Email      string                 `json:"email"`
}

func (c *Client) Origin() string { return strings.TrimRight(c.base.String(), "/") }

func (c *Client) JWKS(ctx context.Context) (remoteproto.JWKS, error) {
	var keys remoteproto.JWKS
	err := c.json(ctx, http.MethodGet, "/.well-known/jwks.json", nil, &keys, "", nil, false)
	return keys, err
}

func (c *Client) LinkComputer(
	ctx context.Context, request ComputerLinkRequest, device Credentials, key ed25519.PrivateKey,
) (ComputerLink, error) {
	var result ComputerLink
	if request.LinkToken != "" {
		device, key = Credentials{}, nil
	}
	err := c.computerJSON(ctx, http.MethodPost, "/v1/computers/link", request, &result, device, key)
	return result, err
}

func (c *Client) UpdateRemoteAccess(
	ctx context.Context, id remoteproto.ComputerID, key ed25519.PrivateKey, enabled bool, endpoints []string,
) error {
	if endpoints == nil {
		endpoints = []string{}
	}
	body := struct {
		Enabled   bool     `json:"remote_enabled"`
		Endpoints []string `json:"endpoints"`
	}{enabled, endpoints}
	return c.computerJSON(ctx, http.MethodPatch, computerPath(id), body, nil, Credentials{}, key)
}

func (c *Client) UnlinkComputer(ctx context.Context, id remoteproto.ComputerID, device Credentials) error {
	return c.deviceJSON(ctx, device, http.MethodDelete, computerPath(id), nil, nil)
}

func (c *Client) UnlinkComputerWithKey(ctx context.Context, id remoteproto.ComputerID, key ed25519.PrivateKey) error {
	return c.computerJSON(ctx, http.MethodDelete, computerPath(id), nil, nil, Credentials{}, key)
}

func (c *Client) RevalidateComputer(
	ctx context.Context, id remoteproto.ComputerID, key ed25519.PrivateKey, installationID string,
) error {
	return c.computerJSON(ctx, http.MethodPost, computerPath(id)+"/revalidate",
		remoteproto.ComputerRevalidation{InstallationID: installationID}, nil, Credentials{}, key)
}

func computerPath(id remoteproto.ComputerID) string {
	return "/v1/computers/" + url.PathEscape(string(id))
}

const hostedTunnelRequestTimeout = 25 * time.Second

func (c *Client) computerJSON(
	ctx context.Context, method, path string, input, output any, device Credentials, key ed25519.PrivateKey,
) error {
	return c.computerJSONWithin(ctx, requestTimeout, method, path, input, output, device, key)
}

func (c *Client) computerJSONWithin(
	ctx context.Context,
	timeout time.Duration,
	method, path string,
	input, output any,
	device Credentials,
	key ed25519.PrivateKey,
) error {
	request, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	response, err := c.open(request, method, path, input, device.AccessToken, device.Key, true, "application/json", key)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	defer drain(request, response.Body)
	if output != nil {
		if err := json.UnmarshalRead(io.LimitReader(response.Body, maxBody), output); err != nil {
			return unavailable()
		}
	}
	return nil
}

func (c *Client) OpenComputerControl(ctx context.Context, id remoteproto.ComputerID) (*websocket.Conn, error) {
	c.mu.Lock()
	backoff := c.backoff
	c.mu.Unlock()
	if backoff != nil && c.now().Before(backoff.RetryAt) {
		return nil, backoff
	}
	dial, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	target := c.Origin() + "/v1/computers/control?computer_id=" + url.QueryEscape(string(id))
	connection, response, err := websocket.Dial(dial, target, &websocket.DialOptions{HTTPClient: c.http})
	if err != nil {
		if response != nil && response.Body != nil {
			defer response.Body.Close()
			defer drain(dial, response.Body)
			return nil, c.readFailure(response)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, unavailable()
	}
	connection.SetReadLimit(maxBody)
	return connection, nil
}

func (c *Client) ProvisionHostedTunnel(
	ctx context.Context, id remoteproto.ComputerID, key ed25519.PrivateKey, listener string,
) (remoteproto.HostedTunnel, error) {
	var tunnel remoteproto.HostedTunnel
	err := c.computerJSONWithin(ctx, hostedTunnelRequestTimeout, http.MethodPost, computerPath(id)+"/tunnel",
		remoteproto.HostedTunnelRequest{Listener: listener}, &tunnel, Credentials{}, key)
	return tunnel, err
}

func (c *Client) DeleteHostedTunnel(ctx context.Context, id remoteproto.ComputerID, key ed25519.PrivateKey) error {
	return c.computerJSONWithin(ctx, hostedTunnelRequestTimeout, http.MethodDelete, computerPath(id)+"/tunnel",
		nil, nil, Credentials{}, key)
}

func (c *Client) RedeemBrowserCode(
	ctx context.Context, key ed25519.PrivateKey, input remoteproto.BrowserTokenRequest,
) (remoteproto.GrantResponse, error) {
	var result remoteproto.GrantResponse
	err := c.computerJSON(ctx, http.MethodPost, remoteproto.BrowserRedemptionPath, input, &result, Credentials{}, key)
	return result, err
}
