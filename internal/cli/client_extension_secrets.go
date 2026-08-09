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

func (c *daemonClient) ListExtensionSecrets(
	ctx context.Context,
	name string,
	workspaceRef string,
) (ExtensionSecretsRecord, error) {
	var response ExtensionSecretsRecord
	err := c.doJSON(
		ctx,
		http.MethodGet,
		extensionPath(name, "secrets"),
		extensionSecretsQuery(workspaceRef),
		nil,
		&response,
	)
	return response, err
}

func (c *daemonClient) SetExtensionSecrets(
	ctx context.Context,
	name string,
	workspaceRef string,
	request SetExtensionSecretsRequest,
) (ExtensionSecretsRecord, error) {
	var response ExtensionSecretsRecord
	err := c.doJSON(
		ctx,
		http.MethodPut,
		extensionPath(name, "secrets"),
		extensionSecretsQuery(workspaceRef),
		request,
		&response,
	)
	return response, err
}

func (c *daemonClient) DeleteExtensionSecret(
	ctx context.Context,
	name string,
	workspaceRef string,
	envName string,
) error {
	return c.doJSON(
		ctx,
		http.MethodDelete,
		extensionPath(name, "secrets")+"/"+url.PathEscape(strings.TrimSpace(envName)),
		extensionSecretsQuery(workspaceRef),
		nil,
		nil,
	)
}

func extensionSecretsQuery(workspaceRef string) url.Values {
	query := make(url.Values)
	if workspace := strings.TrimSpace(workspaceRef); workspace != "" {
		query.Set(extensionWorkspaceQueryKey, workspace)
	}
	return query
}
