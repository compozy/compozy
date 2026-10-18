package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

const appTokensPath = "/api/app-tokens"

type appTokenClientAPI interface {
	CreateAppToken(context.Context, contract.CreateAppTokenRequest) (contract.AppTokenIssuedResponse, error)
	ListAppTokens(context.Context, string) (contract.AppTokensResponse, error)
	UpdateAppToken(context.Context, string, contract.UpdateAppTokenRequest) (contract.AppTokenResponse, error)
	RevokeAppToken(context.Context, string) (contract.AppTokenRevokeResponse, error)
}

var _ appTokenClientAPI = (*daemonClient)(nil)

func (c *daemonClient) CreateAppToken(
	ctx context.Context,
	request contract.CreateAppTokenRequest,
) (contract.AppTokenIssuedResponse, error) {
	var response contract.AppTokenIssuedResponse
	if err := c.doJSON(ctx, http.MethodPost, appTokensPath, nil, request, &response); err != nil {
		return contract.AppTokenIssuedResponse{}, appTokenRequestError(err)
	}
	return response, nil
}

func (c *daemonClient) ListAppTokens(ctx context.Context, state string) (contract.AppTokensResponse, error) {
	var response contract.AppTokensResponse
	query := url.Values{appTokenStateFlag: []string{state}}
	if err := c.doJSON(ctx, http.MethodGet, appTokensPath, query, nil, &response); err != nil {
		return contract.AppTokensResponse{}, appTokenRequestError(err)
	}
	return response, nil
}

func (c *daemonClient) UpdateAppToken(
	ctx context.Context,
	id string,
	request contract.UpdateAppTokenRequest,
) (contract.AppTokenResponse, error) {
	path := appTokensPath + "/" + url.PathEscape(strings.TrimSpace(id))
	var response contract.AppTokenResponse
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return contract.AppTokenResponse{}, appTokenRequestError(err)
	}
	return response, nil
}

func (c *daemonClient) RevokeAppToken(ctx context.Context, id string) (contract.AppTokenRevokeResponse, error) {
	path := appTokensPath + "/" + url.PathEscape(strings.TrimSpace(id))
	var response contract.AppTokenRevokeResponse
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil, &response); err != nil {
		return contract.AppTokenRevokeResponse{}, appTokenRequestError(err)
	}
	return response, nil
}

func appTokenRequestError(err error) error {
	if apiErr, ok := errors.AsType[*daemonAPIError](err); ok && apiErr.payload.Code != "" {
		return fmt.Errorf("%w (%s)", err, apiErr.payload.Code)
	}
	return err
}

func appTokenClientFromDeps(deps commandDeps) (appTokenClientAPI, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	appClient, ok := client.(appTokenClientAPI)
	if !ok {
		return nil, errors.New("cli: daemon client does not support app token management")
	}
	return appClient, nil
}
