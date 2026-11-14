package auth

import (
	"context"
	"net/http"
)

// WithNetworkAuthorization rechecks source consent immediately before every OAuth request,
// including discovery, registration, refresh, revocation and redirected requests.
func WithNetworkAuthorization(check func(context.Context, ServerConfig) error) ServiceOption {
	return func(s *Service) { s.authorizeNetwork = check }
}

type authorizedTransport struct {
	base   http.RoundTripper
	check  func(context.Context, ServerConfig) error
	config ServerConfig
}

func (t authorizedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.check(req.Context(), t.config); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func (s *Service) authorizedHTTPClient(cfg ServerConfig, base *http.Client) *http.Client {
	if s.authorizeNetwork == nil {
		return base
	}
	clone := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = authorizedTransport{base: transport, check: s.authorizeNetwork, config: cfg}
	return &clone
}

// ConsentScope identifies the active lens separately from the OAuth credential owner.
type ConsentScope struct {
	ProfileID   string
	ProfileName string
	WorkspaceID string
	ResourceID  string
}

type consentScopeKey struct{}

func WithConsentScope(ctx context.Context, scope ConsentScope) context.Context {
	return context.WithValue(ctx, consentScopeKey{}, scope)
}

func ConsentScopeFromContext(ctx context.Context) ConsentScope {
	if scope, ok := ctx.Value(consentScopeKey{}).(ConsentScope); ok {
		return scope
	}
	return ConsentScope{}
}
