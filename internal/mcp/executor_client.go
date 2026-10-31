package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/mcp/securehttp"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	"github.com/compozy/compozy/internal/version"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

type managedMCPClient struct {
	session *mcpsdk.ClientSession
	cleanup func() error
	// release returns a pooled session instead of closing it.
	release func(error)
}

const maxAuthorizationChallengeDrainBytes = int64(64 << 10)

func (e *CallExecutor) openClient(
	ctx context.Context,
	resolved ResolvedServer,
	authorizationHeader string,
) (*managedMCPClient, error) {
	server := resolved.Server
	transport := server.EffectiveTransport()
	switch transport {
	case compozyconfig.MCPServerTransportStdio:
		if e.stdioPool != nil {
			return e.openPooledStdioClient(ctx, resolved)
		}
		launch, err := e.newMCPStdioLaunch(ctx, server, resolved.Target)
		if err != nil {
			return nil, err
		}
		session, err := e.connectClient(ctx, &mcpsdk.CommandTransport{Command: launch.command})
		if err != nil {
			return nil, errors.Join(err, launch.cleanup())
		}
		return &managedMCPClient{session: session, cleanup: launch.cleanup}, nil
	case compozyconfig.MCPServerTransportHTTP:
		httpClient, err := e.httpClientWithAuthorization(ctx, resolved, authorizationHeader)
		if err != nil {
			return nil, err
		}
		session, err := e.connectClient(ctx, &mcpsdk.StreamableClientTransport{
			Endpoint:             strings.TrimSpace(server.URL),
			HTTPClient:           httpClient,
			DisableStandaloneSSE: true,
			MaxRetries:           -1,
		})
		if err != nil {
			return nil, err
		}
		return &managedMCPClient{session: session}, nil
	default:
		return nil, toolspkg.NewValidationError(
			"mcp_server.transport",
			toolspkg.ReasonMCPUnreachable,
			"unsupported mcp transport",
		)
	}
}

func (e *CallExecutor) connectClient(ctx context.Context, transport mcpsdk.Transport) (*mcpsdk.ClientSession, error) {
	return connectMCPClient(ctx, transport)
}

func connectMCPClient(ctx context.Context, transport mcpsdk.Transport) (*mcpsdk.ClientSession, error) {
	client := mcpsdk.NewClient(
		&mcpsdk.Implementation{Name: "compozy", Version: version.Current().Version},
		&mcpsdk.ClientOptions{
			Capabilities:   &mcpsdk.ClientCapabilities{},
			MultiRoundTrip: &mcpsdk.MultiRoundTripOptions{Disabled: true},
		},
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, normalizeMCPError("", err)
	}
	if result := session.InitializeResult(); result == nil || !supportedProtocolVersion(result.ProtocolVersion) {
		version := ""
		if result != nil {
			version = result.ProtocolVersion
		}
		return nil, errors.Join(
			&UnsupportedProtocolVersionError{Version: version},
			closeMCPClient(&managedMCPClient{session: session}),
		)
	}
	return session, nil
}

func (e *CallExecutor) openPooledStdioClient(ctx context.Context, resolved ResolvedServer) (*managedMCPClient, error) {
	spec, err := e.mcpStdioSpec(ctx, resolved.Server, resolved.Target)
	if err != nil {
		return nil, err
	}
	lease, err := e.stdioPool.acquire(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &managedMCPClient{session: lease.session.session, release: lease.release}, nil
}

func closeMCPClient(client *managedMCPClient) error {
	return releaseMCPClient(client, nil)
}

func joinMCPClientCleanup(err error, client *managedMCPClient) error {
	return errors.Join(err, releaseMCPClient(client, err))
}

func releaseMCPClient(client *managedMCPClient, callErr error) error {
	if client == nil {
		return nil
	}
	if client.release != nil {
		client.release(callErr)
		return nil
	}
	var err error
	if client.session != nil {
		if closeErr := client.session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("mcp: close client session: %w", closeErr))
		}
	}
	if client.cleanup != nil {
		if cleanupErr := client.cleanup(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("mcp: clean up client resources: %w", cleanupErr))
		}
	}
	return err
}

func (e *CallExecutor) httpClientWithAuthorization(
	ctx context.Context,
	resolved ResolvedServer,
	authorizationHeader string,
) (*http.Client, error) {
	client := e.httpClientForServer(resolved)
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	headers, err := e.resolveMCPRequestHeaders(ctx, resolved.Server)
	if err != nil {
		return nil, err
	}
	configureMCPRedirectPolicy(client, resolved.Server.Auth.Enabled(), authorizationHeader != "" || len(headers) > 0)
	client.Transport = authorizationRoundTripper{
		next: transport, header: authorizationHeader, headers: headers,
	}
	return client, nil
}

func configureMCPRedirectPolicy(client *http.Client, authEnabled bool, carriesCredentials bool) {
	previous := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if authEnabled {
			return http.ErrUseLastResponse
		}
		if carriesCredentials && len(via) > 0 && !sameHTTPOrigin(via[0].URL, request.URL) {
			return http.ErrUseLastResponse
		}
		if previous != nil {
			return previous(request, via)
		}
		if len(via) >= 10 {
			return errors.New("mcp: stopped after 10 redirects")
		}
		return nil
	}
}

func sameHTTPOrigin(left *url.URL, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

func (e *CallExecutor) resolveMCPRequestHeaders(
	ctx context.Context,
	server compozyconfig.MCPServer,
) (http.Header, error) {
	headers := make(http.Header, len(server.Headers)+len(server.SecretHeaders))
	for name, value := range server.Headers {
		headers.Set(name, value)
	}
	names := slices.Sorted(maps.Keys(server.SecretHeaders))
	for _, name := range names {
		value, err := e.resolveSecretRef(ctx, server.SecretHeaders[name])
		if err != nil {
			return nil, fmt.Errorf(
				"mcp: resolve secret header %s for server %q: %w",
				name,
				server.Name,
				err,
			)
		}
		headers.Set(name, value)
	}
	return headers, nil
}

func (e *CallExecutor) httpClientForServer(resolved ResolvedServer) *http.Client {
	if strings.TrimSpace(resolved.Server.CatalogEntry) != "" {
		return securehttp.NewClient(securehttp.WithTimeout(e.timeout)).HTTPClient()
	}
	return e.httpClientForCall()
}

type authorizationRoundTripper struct {
	next    http.RoundTripper
	header  string
	headers http.Header
}

func (t authorizationRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	for name, values := range t.headers {
		cloned.Header.Del(name)
		for _, value := range values {
			cloned.Header.Add(name, value)
		}
	}
	if t.header != "" {
		cloned.Header.Set("Authorization", t.header)
	}
	response, err := t.next.RoundTrip(cloned)
	if err != nil {
		return nil, err
	}
	if authErr := authorizationChallengeError(response); authErr != nil {
		return nil, errors.Join(authErr, drainAndCloseAuthorizationChallenge(response.Body))
	}
	return response, nil
}

func drainAndCloseAuthorizationChallenge(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	var cleanupErr error
	if _, err := io.Copy(io.Discard, io.LimitReader(body, maxAuthorizationChallengeDrainBytes)); err != nil {
		cleanupErr = fmt.Errorf("mcp: drain authorization challenge response body: %w", err)
	}
	if err := body.Close(); err != nil {
		cleanupErr = errors.Join(
			cleanupErr,
			fmt.Errorf("mcp: close authorization challenge response body: %w", err),
		)
	}
	return cleanupErr
}

func authorizationChallengeError(response *http.Response) error {
	if response == nil {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return ErrAuthorizationRequired
	}
	return insufficientScopeError(response)
}

func insufficientScopeError(response *http.Response) error {
	if response == nil || response.StatusCode != http.StatusForbidden {
		return nil
	}
	challenges, err := oauthex.ParseWWWAuthenticate(response.Header.Values("WWW-Authenticate"))
	if err != nil {
		return nil
	}
	for _, challenge := range challenges {
		if challenge.Scheme != "bearer" || challenge.Params["error"] != "insufficient_scope" {
			continue
		}
		return &InsufficientScopeError{Scopes: trimStrings(strings.Fields(challenge.Params["scope"]))}
	}
	return nil
}

func (e *CallExecutor) httpClientForCall() *http.Client {
	cloned := *e.httpClient
	if cloned.Timeout <= 0 || cloned.Timeout > e.timeout {
		cloned.Timeout = e.timeout
	}
	return &cloned
}

func (e *CallExecutor) resolveSecretRef(ctx context.Context, ref string) (string, error) {
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		return "", nil
	}
	if e != nil && e.secretResolver != nil {
		value, err := e.secretResolver.ResolveRef(ctx, normalized)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			registerMCPRequestSecret(ctx, value)
		}
		return value, nil
	}
	if vault.IsEnvRef(normalized) {
		envName, err := vault.EnvNameFromRef(normalized)
		if err != nil {
			return "", err
		}
		if e != nil && e.lookupSecret != nil {
			value := e.lookupSecret(envName)
			if strings.TrimSpace(value) == "" {
				return "", fmt.Errorf("%w: env:%s", vault.ErrMissingSecret, envName)
			}
			registerMCPRequestSecret(ctx, value)
			return value, nil
		}
	}
	return "", fmt.Errorf("%w: %s", vault.ErrUnsupportedSecretRef, normalized)
}
