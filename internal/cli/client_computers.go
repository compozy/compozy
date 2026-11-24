package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
)

type computerListClient interface {
	ListComputers(context.Context) (contract.ComputerListResponse, error)
}

type computerCommandClient interface {
	computerListClient
	AvailableComputers(context.Context) (contract.AvailableComputersResponse, error)
	AddAccountComputer(context.Context, remoteproto.ComputerID) (contract.Computer, error)
	GetComputer(context.Context, remoteproto.ComputerID) (contract.Computer, error)
	PatchComputer(context.Context, remoteproto.ComputerID, contract.PatchComputerRequest) (contract.Computer, error)
	RemoveComputer(context.Context, remoteproto.ComputerID, bool, bool) (contract.RemoveComputerResponse, error)
	RemoveComputerRoute(context.Context, remoteproto.ComputerID, string) error
}

var _ computerCommandClient = (*daemonClient)(nil)

func computerClientFromDeps(deps commandDeps) (computerCommandClient, error) {
	home, err := deps.resolveHome()
	if err != nil {
		return nil, err
	}
	target, err := resolveLocalClientTarget(deps, home)
	if err != nil {
		return nil, err
	}
	client, err := deps.newClient(target)
	if err != nil {
		return nil, err
	}
	computers, ok := client.(computerCommandClient)
	if !ok {
		return nil, errors.New("daemon client does not support computer commands")
	}
	return computers, nil
}

func (c *daemonClient) ListComputers(ctx context.Context) (result contract.ComputerListResponse, err error) {
	err = c.doJSON(ctx, http.MethodGet, contract.ComputersPath, nil, nil, &result)
	return result, err
}

func (c *daemonClient) AvailableComputers(ctx context.Context) (result contract.AvailableComputersResponse, err error) {
	err = c.doJSON(ctx, http.MethodGet, contract.ComputersPath+"/available", nil, nil, &result)
	return result, err
}

func (c *daemonClient) AddAccountComputer(
	ctx context.Context, id remoteproto.ComputerID,
) (result contract.Computer, err error) {
	input := contract.AddComputerRequest{Kind: "account", ComputerID: id}
	err = c.doJSON(ctx, http.MethodPost, contract.ComputersPath, nil, input, &result)
	return result, err
}

func computerResourcePath(id remoteproto.ComputerID) string {
	return contract.ComputersPath + "/" + url.PathEscape(string(id))
}

func (c *daemonClient) GetComputer(
	ctx context.Context,
	id remoteproto.ComputerID,
) (result contract.Computer, err error) {
	err = c.doJSON(ctx, http.MethodGet, computerResourcePath(id), nil, nil, &result)
	return result, err
}

func (c *daemonClient) PatchComputer(
	ctx context.Context, id remoteproto.ComputerID, input contract.PatchComputerRequest,
) (result contract.Computer, err error) {
	err = c.doJSON(ctx, http.MethodPatch, computerResourcePath(id), nil, input, &result)
	return result, err
}

func (c *daemonClient) RemoveComputer(
	ctx context.Context, id remoteproto.ComputerID, unlink, uninstall bool,
) (result contract.RemoveComputerResponse, err error) {
	query := url.Values{"unlink": {strconv.FormatBool(unlink)}, "uninstall": {strconv.FormatBool(uninstall)}}
	err = c.doJSON(ctx, http.MethodDelete, computerResourcePath(id), query, nil, &result)
	return result, err
}

func (c *daemonClient) RemoveComputerRoute(ctx context.Context, id remoteproto.ComputerID, route string) error {
	return c.doJSON(ctx, http.MethodDelete, computerResourcePath(id)+"/routes/"+url.PathEscape(route), nil, nil, nil)
}
