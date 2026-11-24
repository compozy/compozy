package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
)

type accountCommandClient interface {
	ReadAccount(context.Context) (contract.AccountStatusResponse, error)
	OpenAccountSignIn(context.Context) (io.ReadCloser, error)
	StartAccountLogin(context.Context, contract.AccountLoginRequest) (contract.AccountLoginResponse, error)
	CancelAccountLogin(context.Context, string) error
	LogoutAccount(context.Context, bool) (contract.AccountLogoutResponse, error)
	CheckoutAccount(context.Context, string) (contract.AccountBillingResponse, error)
	UpdateAccountConsent(context.Context, bool) (contract.AccountStatusResponse, error)
	ListAccountTokens(context.Context) (remoteproto.AccountTokenList, error)
	CreateAccountToken(context.Context, string) (remoteproto.AccountTokenCreated, error)
	RevokeAccountToken(context.Context, string) error
	ReadAccountForDelete(context.Context) (contract.AccountStatusResponse, error)
	DeleteAccount(context.Context, string) (contract.AccountDeleteResponse, error)
}

var _ accountCommandClient = (*daemonClient)(nil)

func accountClientFromDeps(deps commandDeps) (accountCommandClient, error) {
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
	local, ok := client.(accountCommandClient)
	if !ok {
		return nil, errors.New("daemon client does not support account commands")
	}
	return local, nil
}

func (c *daemonClient) ReadAccount(ctx context.Context) (status contract.AccountStatusResponse, err error) {
	err = c.doJSON(ctx, http.MethodGet, contract.AccountPath, nil, nil, &status)
	return status, err
}

func (c *daemonClient) OpenAccountSignIn(ctx context.Context) (io.ReadCloser, error) {
	response, err := c.doRequestWithCredentialsAndClient(ctx, http.MethodGet, contract.AccountSignInEventsPath,
		nil, nil, "", agentidentity.Credentials{}, c.streamHTTPClient())
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.Join(readAPIError(response), response.Body.Close())
	}
	return response.Body, nil
}

func (c *daemonClient) StartAccountLogin(
	ctx context.Context, input contract.AccountLoginRequest,
) (result contract.AccountLoginResponse, err error) {
	err = c.doJSON(ctx, http.MethodPost, contract.AccountLoginPath, nil, input, &result)
	return result, err
}

func (c *daemonClient) CancelAccountLogin(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, contract.AccountLoginPath, url.Values{"attempt_id": {id}}, nil, nil)
}

func (c *daemonClient) LogoutAccount(
	ctx context.Context,
	unlink bool,
) (result contract.AccountLogoutResponse, err error) {
	err = c.doJSON(
		ctx,
		http.MethodPost,
		contract.AccountLogoutPath,
		nil,
		contract.AccountLogoutRequest{Unlink: unlink},
		&result,
	)
	return result, err
}

func (c *daemonClient) UpdateAccountConsent(
	ctx context.Context, news bool,
) (result contract.AccountStatusResponse, err error) {
	err = c.doJSON(ctx, http.MethodPatch, contract.AccountPath, nil,
		contract.AccountEmailPatchRequest{MarketingOptIn: &news, Source: "cli"}, &result)
	return result, err
}

func (c *daemonClient) ListAccountTokens(ctx context.Context) (result remoteproto.AccountTokenList, err error) {
	err = c.doJSON(ctx, http.MethodGet, contract.AccountTokensPath, nil, nil, &result)
	return result, err
}

func (c *daemonClient) CreateAccountToken(
	ctx context.Context,
	name string,
) (result remoteproto.AccountTokenCreated, err error) {
	err = c.doJSON(
		ctx,
		http.MethodPost,
		contract.AccountTokensPath,
		nil,
		remoteproto.AccountTokenCreate{Name: name},
		&result,
	)
	return result, err
}

func (c *daemonClient) RevokeAccountToken(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, contract.AccountTokensPath+"/"+url.PathEscape(id), nil, nil, nil)
}

func (c *daemonClient) ReadAccountForDelete(ctx context.Context) (result contract.AccountStatusResponse, err error) {
	err = c.doJSON(ctx, http.MethodGet, contract.AccountPath, url.Values{"refresh": {"true"}}, nil, &result)
	return result, err
}

func (c *daemonClient) DeleteAccount(
	ctx context.Context, email string,
) (result contract.AccountDeleteResponse, err error) {
	err = c.doJSON(
		ctx,
		http.MethodDelete,
		contract.AccountPath,
		nil,
		contract.AccountDeleteRequest{ConfirmEmail: email},
		&result,
	)
	return result, err
}

func (c *daemonClient) CheckoutAccount(
	ctx context.Context,
	interval string,
) (result contract.AccountBillingResponse, err error) {
	err = c.doJSON(
		ctx,
		http.MethodPost,
		contract.AccountCheckoutPath,
		nil,
		contract.AccountCheckoutRequest{Interval: interval},
		&result,
	)
	return result, err
}
