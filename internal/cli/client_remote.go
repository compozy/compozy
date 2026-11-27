package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

type remoteCommandClient interface {
	accountCommandClient
	RemoteStatus(context.Context) (contract.RemoteStatusResponse, error)
	PatchRemoteStatus(context.Context, contract.RemotePatchRequest) (contract.RemoteStatusResponse, error)
	LinkRemote(context.Context, contract.RemoteLinkRequest) (contract.RemoteLinkResponse, error)
	UnlinkRemote(context.Context) error
	RemoteDevices(context.Context) (contract.RemoteDevicesResponse, error)
	PatchRemoteDevice(context.Context, string, contract.RemoteDevicePatchRequest) (contract.RemoteDeviceResponse, error)
	RevokeRemoteDevice(context.Context, string) error
}

var _ remoteCommandClient = (*daemonClient)(nil)

func remoteClientFromDeps(deps commandDeps) (remoteCommandClient, error) {
	client, err := accountClientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	remote, ok := client.(remoteCommandClient)
	if !ok {
		return nil, errors.New("daemon client does not support remote commands")
	}
	return remote, nil
}

func (c *daemonClient) RemoteStatus(ctx context.Context) (result contract.RemoteStatusResponse, err error) {
	err = c.doJSON(ctx, http.MethodGet, contract.RemotePath, nil, nil, &result)
	return result, err
}

func (c *daemonClient) PatchRemoteStatus(
	ctx context.Context, input contract.RemotePatchRequest,
) (result contract.RemoteStatusResponse, err error) {
	err = c.doJSON(ctx, http.MethodPatch, contract.RemotePath, nil, input, &result)
	return result, err
}

func (c *daemonClient) LinkRemote(
	ctx context.Context, input contract.RemoteLinkRequest,
) (result contract.RemoteLinkResponse, err error) {
	err = c.doJSON(ctx, http.MethodPost, contract.RemoteLinkPath, nil, input, &result)
	return result, err
}

func (c *daemonClient) UnlinkRemote(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodDelete, contract.RemoteLinkPath, nil, nil, nil)
}

func (c *daemonClient) RemoteDevices(ctx context.Context) (result contract.RemoteDevicesResponse, err error) {
	err = c.doJSON(ctx, http.MethodGet, contract.RemoteDevicesPath, nil, nil, &result)
	return result, err
}

func (c *daemonClient) PatchRemoteDevice(
	ctx context.Context, id string, input contract.RemoteDevicePatchRequest,
) (result contract.RemoteDeviceResponse, err error) {
	err = c.doJSON(ctx, http.MethodPatch, contract.RemoteDevicesPath+"/"+url.PathEscape(id), nil, input, &result)
	return result, err
}

func (c *daemonClient) RevokeRemoteDevice(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, contract.RemoteDevicesPath+"/"+url.PathEscape(id), nil, nil, nil)
}
