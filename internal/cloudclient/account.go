package cloudclient

import (
	"context"
	"crypto"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/remoteproto"
)

func (c *Client) StartDeviceFlow(ctx context.Context, input remoteproto.DeviceFlowRequest) (
	result remoteproto.DeviceFlowResponse, err error,
) {
	err = c.json(ctx, http.MethodPost, "/v1/device-flow", input, &result, "", nil, false)
	return result, err
}

func (c *Client) CancelDeviceFlow(ctx context.Context, code string) error {
	return c.json(ctx, http.MethodDelete, "/v1/device-flow/"+url.PathEscape(code), nil, nil, "", nil, false)
}

func (c *Client) PollDeviceFlow(
	ctx context.Context,
	code string,
) (result remoteproto.DeviceFlowTokenResponse, err error) {
	input := remoteproto.DeviceFlowTokenRequest{DeviceCode: code}
	err = c.json(ctx, http.MethodPost, "/v1/device-flow/token", input, &result, "", nil, false)
	return result, err
}

func (c *Client) Enroll(ctx context.Context, token string, key crypto.Signer, input remoteproto.DeviceEnrollRequest) (
	result remoteproto.DeviceEnrollResponse, err error,
) {
	err = c.json(ctx, http.MethodPost, "/v1/devices/enroll", input, &result, token, key, false)
	return result, err
}

func (c *Client) Refresh(ctx context.Context, token string, key crypto.Signer) (
	result remoteproto.DeviceSessionTokens, err error,
) {
	err = c.json(ctx, http.MethodPost, "/v1/session/refresh", nil, &result, token, key, false)
	return result, err
}

func (c *Client) Logout(ctx context.Context, token string, key crypto.Signer) error {
	return c.json(ctx, http.MethodPost, "/v1/session/logout", nil, nil, token, key, false)
}

func (c *Client) Account(ctx context.Context, creds Credentials) (result remoteproto.AccountSummary, err error) {
	err = c.deviceJSON(ctx, creds, http.MethodGet, "/v1/account", nil, &result)
	return result, err
}

func (c *Client) Devices(ctx context.Context, creds Credentials) ([]remoteproto.AccountDevice, error) {
	var result struct {
		Devices []remoteproto.AccountDevice `json:"devices"`
	}
	err := c.deviceJSON(ctx, creds, http.MethodGet, "/v1/devices", nil, &result)
	return result.Devices, err
}

func (c *Client) PatchDevice(ctx context.Context, creds Credentials, id string, patch remoteproto.AccountDevicePatch) (
	result remoteproto.AccountDevice, err error,
) {
	err = c.deviceJSON(ctx, creds, http.MethodPatch, "/v1/devices/"+url.PathEscape(id), patch, &result)
	return result, err
}

func (c *Client) RevokeDevice(ctx context.Context, creds Credentials, id string) error {
	return c.deviceJSON(ctx, creds, http.MethodDelete, "/v1/devices/"+url.PathEscape(id), nil, nil)
}

func (c *Client) Approvals(ctx context.Context, creds Credentials) ([]remoteproto.AccountApproval, error) {
	var result struct {
		Approvals []remoteproto.AccountApproval `json:"approvals"`
	}
	err := c.deviceJSON(ctx, creds, http.MethodGet, "/v1/approvals", nil, &result)
	return result.Approvals, err
}

func (c *Client) Approval(ctx context.Context, creds Credentials, id string) (
	result remoteproto.AccountApprovalDetails, err error,
) {
	err = c.deviceJSON(ctx, creds, http.MethodGet, "/v1/approvals/"+url.PathEscape(id), nil, &result)
	return result, err
}

func (c *Client) DecideApproval(ctx context.Context, creds Credentials, id, jws string) (
	result remoteproto.ApprovalDelivery, err error,
) {
	input := remoteproto.ApprovalDecided{DecisionJWS: jws}
	err = c.deviceJSON(ctx, creds, http.MethodPost, "/v1/approvals/"+url.PathEscape(id)+"/decision", input, &result)
	return result, err
}

func (c *Client) Checkout(ctx context.Context, creds Credentials, interval string) (string, error) {
	input := remoteproto.CheckoutRequest{Interval: interval}
	return c.billingURL(ctx, creds, "/v1/billing/checkout", input)
}

func (c *Client) BillingPortal(ctx context.Context, creds Credentials) (string, error) {
	return c.billingURL(ctx, creds, "/v1/billing/portal", nil)
}

func (c *Client) billingURL(ctx context.Context, creds Credentials, path string, body any) (string, error) {
	var result remoteproto.BillingURL
	if err := c.deviceJSON(ctx, creds, http.MethodPost, path, body, &result); err != nil {
		return "", err
	}
	link, err := url.Parse(result.URL)
	if err != nil || link.Scheme != "https" || link.Hostname() == "" || link.User != nil {
		return "", unavailable()
	}
	return result.URL, nil
}

func (c *Client) OpenAccountEvents(ctx context.Context, creds Credentials) (io.ReadCloser, error) {
	response, err := c.open(
		ctx,
		http.MethodGet,
		"/v1/account/events",
		nil,
		creds.AccessToken,
		creds.Key,
		true,
		"text/event-stream",
	)
	if err != nil {
		return nil, err
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "text/event-stream" {
		_ = response.Body.Close()
		return nil, unavailable()
	}
	return response.Body, nil
}

func (c *Client) deviceJSON(ctx context.Context, creds Credentials, method, path string, body, output any) error {
	return c.json(ctx, method, path, body, output, creds.AccessToken, creds.Key, true)
}

func (c *Client) PatchAccountEmail(
	ctx context.Context, creds Credentials, patch remoteproto.AccountEmailPatch,
) (result remoteproto.AccountSummary, err error) {
	err = c.deviceJSON(ctx, creds, http.MethodPatch, "/v1/account", patch, &result)
	return result, err
}

func (c *Client) DeleteAccount(
	ctx context.Context, creds Credentials, request remoteproto.AccountDeleteRequest,
) (result remoteproto.AccountDeleteResponse, err error) {
	err = c.deviceJSON(ctx, creds, http.MethodDelete, "/v1/account", request, &result)
	return result, err
}
