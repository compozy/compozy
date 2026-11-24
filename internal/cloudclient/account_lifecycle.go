package cloudclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/remoteproto"
)

const accountTokensPath = "/v1/account/tokens"

func (c *Client) ExchangeAccountToken(
	ctx context.Context, token, installationID string,
) (result remoteproto.AccountTokenIdentity, err error) {
	err = c.json(ctx, http.MethodPost, remoteproto.AccountTokenExchangePath,
		remoteproto.AccountTokenExchange{InstallationID: installationID}, &result, token, nil, false)
	return result, err
}

func (c *Client) ListAccountTokens(
	ctx context.Context, creds Credentials,
) (result remoteproto.AccountTokenList, err error) {
	err = c.deviceJSON(ctx, creds, http.MethodGet, accountTokensPath, nil, &result)
	return result, err
}

func (c *Client) CreateAccountToken(
	ctx context.Context, creds Credentials, name string,
) (result remoteproto.AccountTokenCreated, err error) {
	err = c.deviceJSON(
		ctx,
		creds,
		http.MethodPost,
		accountTokensPath,
		remoteproto.AccountTokenCreate{Name: name},
		&result,
	)
	return result, err
}

func (c *Client) RevokeAccountToken(ctx context.Context, creds Credentials, id string) error {
	return c.deviceJSON(ctx, creds, http.MethodDelete, accountTokensPath+"/"+url.PathEscape(id), nil, nil)
}
