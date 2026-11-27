package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

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

type computerSetupClient interface {
	computerCommandClient
	StartSSHComputer(context.Context, contract.AddComputerRequest) (contract.ComputerSetupStarted, error)
	StartComputerUpdate(context.Context, remoteproto.ComputerID) (contract.ComputerUpdateStarted, error)
	GetComputerSetup(context.Context, string, time.Duration) (contract.ComputerSetupStatus, error)
	AnswerComputerSetup(context.Context, string, contract.ComputerSetupAnswer) error
	CancelComputerSetup(context.Context, string) (contract.ComputerSetupCanceled, error)
}

var _ computerSetupClient = (*daemonClient)(nil)

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
	input := contract.AddComputerRequest{Kind: contract.ComputerAddKindAccount, ComputerID: id}
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

func (c *daemonClient) StartSSHComputer(
	ctx context.Context, input contract.AddComputerRequest,
) (result contract.ComputerSetupStarted, err error) {
	err = c.doJSON(ctx, http.MethodPost, contract.ComputersPath, nil, input, &result)
	return result, err
}

func (c *daemonClient) StartComputerUpdate(
	ctx context.Context, id remoteproto.ComputerID,
) (result contract.ComputerUpdateStarted, err error) {
	err = c.doJSON(ctx, http.MethodPost, computerResourcePath(id)+"/update", nil, nil, &result)
	return result, err
}

func (c *daemonClient) GetComputerSetup(
	ctx context.Context, id string, wait time.Duration,
) (result contract.ComputerSetupStatus, err error) {
	ctx, cancel := context.WithTimeout(ctx, wait+10*time.Second)
	defer cancel()
	query := url.Values{"wait_ms": {strconv.FormatInt(wait.Milliseconds(), 10)}}
	err = c.doJSONWithClient(ctx, http.MethodGet, computerSetupPath(id), query, nil, &result, c.streamHTTPClient())
	return result, err
}

func (c *daemonClient) AnswerComputerSetup(ctx context.Context, id string, input contract.ComputerSetupAnswer) error {
	return c.doJSON(ctx, http.MethodPost, computerSetupPath(id)+"/answer", nil, input, nil)
}

func (c *daemonClient) CancelComputerSetup(
	ctx context.Context, id string,
) (result contract.ComputerSetupCanceled, err error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	err = c.doJSONWithClient(ctx, http.MethodDelete, computerSetupPath(id), nil, nil, &result, c.streamHTTPClient())
	return result, err
}

func computerSetupPath(id string) string {
	return contract.ComputerSetupsPath + "/" + url.PathEscape(id)
}
