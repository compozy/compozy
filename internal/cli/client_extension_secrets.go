package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type extensionSecretsClient interface {
	ListExtensionSecrets(context.Context, string, string) (ExtensionSecretsRecord, error)
	SetExtensionSecrets(context.Context, string, string, SetExtensionSecretsRequest) (ExtensionSecretsRecord, error)
	DeleteExtensionSecret(context.Context, string, string, string) error
}

func (c *unixSocketClient) ListExtensionSecrets(
	ctx context.Context,
	name string,
	workspaceRef string,
) (ExtensionSecretsRecord, error) {
	var response ExtensionSecretsRecord
	err := c.doJSON(
		ctx,
		http.MethodGet,
		extensionSecretsPath(name),
		extensionSecretsQuery(workspaceRef),
		nil,
		&response,
	)
	return response, err
}

func (c *unixSocketClient) SetExtensionSecrets(
	ctx context.Context,
	name string,
	workspaceRef string,
	request SetExtensionSecretsRequest,
) (ExtensionSecretsRecord, error) {
	var response ExtensionSecretsRecord
	err := c.doJSON(
		ctx,
		http.MethodPut,
		extensionSecretsPath(name),
		extensionSecretsQuery(workspaceRef),
		request,
		&response,
	)
	return response, err
}

func (c *unixSocketClient) DeleteExtensionSecret(
	ctx context.Context,
	name string,
	workspaceRef string,
	envName string,
) error {
	return c.doJSON(
		ctx,
		http.MethodDelete,
		extensionSecretsPath(name)+"/"+url.PathEscape(strings.TrimSpace(envName)),
		extensionSecretsQuery(workspaceRef),
		nil,
		nil,
	)
}

func extensionSecretsPath(name string) string {
	return "/api/extensions/" + url.PathEscape(strings.TrimSpace(name)) + "/secrets"
}

func extensionSecretsQuery(workspaceRef string) url.Values {
	query := make(url.Values)
	if workspace := strings.TrimSpace(workspaceRef); workspace != "" {
		query.Set(workspaceFlagName, workspace)
	}
	return query
}
