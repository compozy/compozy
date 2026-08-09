package cli

import (
	"context"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
)

type networkAgentCredentialsKey struct{}

func withNetworkAgentCredentials(ctx context.Context, credentials agentidentity.Credentials) context.Context {
	credentials.SessionID = strings.TrimSpace(credentials.SessionID)
	credentials.AgentName = strings.TrimSpace(credentials.AgentName)
	if credentials.SessionID == "" && credentials.AgentName == "" {
		return ctx
	}
	return context.WithValue(ctx, networkAgentCredentialsKey{}, credentials)
}

func networkAgentCredentialsFromContext(ctx context.Context) agentidentity.Credentials {
	if ctx == nil {
		return agentidentity.Credentials{}
	}
	credentials, ok := ctx.Value(networkAgentCredentialsKey{}).(agentidentity.Credentials)
	if !ok {
		return agentidentity.Credentials{}
	}
	return credentials
}

func (c *daemonClient) doNetworkJSON(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	responseBody any,
) error {
	return c.doAgentJSON(
		ctx,
		method,
		path,
		query,
		requestBody,
		networkAgentCredentialsFromContext(ctx),
		responseBody,
	)
}
