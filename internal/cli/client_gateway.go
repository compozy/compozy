package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type gatewayClientAPI interface {
	GetGatewayStatus(context.Context) (contract.GatewayStatusPayload, error)
	GetGatewayAudit(context.Context) (contract.GatewayAuditPayload, error)
	SetGatewaySurface(context.Context, contract.GatewaySurfaceRequest) (contract.GatewayStatusPayload, error)
	EnableGatewayProvider(
		context.Context,
		string,
		contract.GatewayProviderEnableRequest,
	) (contract.GatewayStatusPayload, error)
	DisableGatewayProvider(context.Context, string, string) (contract.GatewayStatusPayload, error)
	MintGatewayPairing(context.Context) (contract.GatewayPairingArtifactPayload, error)
	RedeemGatewayPairing(
		context.Context,
		contract.GatewayPairingRedeemRequest,
	) (contract.GatewayIssuedCredentialPayload, error)
	ListGatewayDevices(context.Context) ([]contract.GatewayDevicePayload, error)
	RenameGatewayDevice(context.Context, string, string) (contract.GatewayDevicePayload, error)
	RevokeGatewayDevice(context.Context, string) (contract.GatewayRevokePayload, error)
}

var _ gatewayClientAPI = (*daemonClient)(nil)

func (c *daemonClient) GetGatewayStatus(ctx context.Context) (contract.GatewayStatusPayload, error) {
	var response contract.GatewayStatusPayload
	if err := c.doJSON(ctx, http.MethodGet, "/api/gateway/status", nil, nil, &response); err != nil {
		return contract.GatewayStatusPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) GetGatewayAudit(ctx context.Context) (contract.GatewayAuditPayload, error) {
	var response contract.GatewayAuditPayload
	if err := c.doJSON(ctx, http.MethodGet, "/api/gateway/audit", nil, nil, &response); err != nil {
		return contract.GatewayAuditPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) SetGatewaySurface(
	ctx context.Context,
	request contract.GatewaySurfaceRequest,
) (contract.GatewayStatusPayload, error) {
	var response contract.GatewayStatusPayload
	if err := c.doJSON(ctx, http.MethodPost, "/api/gateway/surfaces", nil, request, &response); err != nil {
		return contract.GatewayStatusPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) EnableGatewayProvider(
	ctx context.Context,
	name string,
	request contract.GatewayProviderEnableRequest,
) (contract.GatewayStatusPayload, error) {
	path := "/api/gateway/providers/" + url.PathEscape(strings.TrimSpace(name)) + "/enable"
	var response contract.GatewayStatusPayload
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return contract.GatewayStatusPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) DisableGatewayProvider(
	ctx context.Context,
	name string,
	tier string,
) (contract.GatewayStatusPayload, error) {
	path := "/api/gateway/providers/" + url.PathEscape(strings.TrimSpace(name)) + "/disable"
	query := url.Values{gatewayTierKey: []string{strings.TrimSpace(tier)}}
	var response contract.GatewayStatusPayload
	if err := c.doJSON(ctx, http.MethodPost, path, query, nil, &response); err != nil {
		return contract.GatewayStatusPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) MintGatewayPairing(
	ctx context.Context,
) (contract.GatewayPairingArtifactPayload, error) {
	var response contract.GatewayPairingArtifactPayload
	if err := c.doJSON(ctx, http.MethodPost, "/api/gateway/pairings", nil, nil, &response); err != nil {
		return contract.GatewayPairingArtifactPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) RedeemGatewayPairing(
	ctx context.Context,
	request contract.GatewayPairingRedeemRequest,
) (contract.GatewayIssuedCredentialPayload, error) {
	var response contract.GatewayIssuedCredentialPayload
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/gateway/pairings/redeem",
		nil,
		request,
		&response,
	); err != nil {
		return contract.GatewayIssuedCredentialPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) ListGatewayDevices(ctx context.Context) ([]contract.GatewayDevicePayload, error) {
	var response contract.GatewayDevicesResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/gateway/devices", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Devices, nil
}

func (c *daemonClient) RenameGatewayDevice(
	ctx context.Context,
	id string,
	name string,
) (contract.GatewayDevicePayload, error) {
	path := "/api/gateway/devices/" + url.PathEscape(strings.TrimSpace(id))
	var response contract.GatewayDevicePayload
	if err := c.doJSON(
		ctx,
		http.MethodPatch,
		path,
		nil,
		contract.GatewayDeviceRenameRequest{Name: strings.TrimSpace(name)},
		&response,
	); err != nil {
		return contract.GatewayDevicePayload{}, err
	}
	return response, nil
}

func (c *daemonClient) RevokeGatewayDevice(
	ctx context.Context,
	id string,
) (contract.GatewayRevokePayload, error) {
	path := "/api/gateway/devices/" + url.PathEscape(strings.TrimSpace(id))
	var response contract.GatewayRevokePayload
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil, &response); err != nil {
		return contract.GatewayRevokePayload{}, err
	}
	return response, nil
}

func gatewayClientFromDeps(deps commandDeps) (gatewayClientAPI, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	gatewayClient, ok := client.(gatewayClientAPI)
	if !ok {
		return nil, errors.New("cli: daemon client does not support gateway management")
	}
	return gatewayClient, nil
}
