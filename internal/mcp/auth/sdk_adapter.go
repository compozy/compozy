package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func (s *Service) beginHostedOAuth(
	ctx context.Context,
	cfg ServerConfig,
	redirectURL string,
) (LoginState, error) {
	resourceURL := firstNonEmpty(cfg.ResourceURL, cfg.RemoteURL)
	if err := validateAbsoluteHTTPURL("resource URL", resourceURL); err != nil {
		return LoginState{}, err
	}
	if err := validateAbsoluteHTTPURL("redirect URL", redirectURL); err != nil {
		return LoginState{}, err
	}
	prm, err := s.protectedResourceMetadata(ctx, cfg, resourceURL)
	if err != nil {
		return LoginState{}, err
	}
	issuer, err := hostedAuthorizationServer(cfg, prm.AuthorizationServers)
	if err != nil {
		return LoginState{}, err
	}
	asm, err := s.authServerMetadata(ctx, cfg, issuer)
	if err != nil {
		return LoginState{}, err
	}
	metadata := metadataFromAuthServer(asm)
	loginScopes := prm.loginScopes(cfg.Scopes)
	client, err := s.resolveHostedRegistration(
		ctx,
		cfg,
		resourceURL,
		redirectURL,
		asm,
		loginScopes,
	)
	if err != nil {
		return LoginState{}, err
	}
	if !supportsS256(metadata.CodeChallengeMethodsSupported) {
		return LoginState{}, errors.New("mcp auth: OAuth server must support S256 PKCE")
	}
	pkce, err := newPKCEPair(s.random)
	if err != nil {
		return LoginState{}, err
	}
	state, err := newState(s.random)
	if err != nil {
		return LoginState{}, err
	}
	effectiveCfg := cfg
	effectiveCfg.ClientID = client.id
	effectiveCfg.ClientSecret = client.secret
	effectiveCfg.TokenEndpointAuthMethod = client.authMethod
	authorizationCfg := effectiveCfg
	authorizationCfg.Scopes = client.scopes
	authorizationURL, scopes, err := authorizationURL(
		metadata,
		authorizationCfg,
		redirectURL,
		state,
		pkce,
		client.allowOfflineAccess,
	)
	if err != nil {
		return LoginState{}, err
	}
	return LoginState{
		requestedScopes:  scopes,
		Target:           cfg.Target.Normalize(),
		RedirectURL:      strings.TrimSpace(redirectURL),
		State:            state,
		Verifier:         pkce.Verifier,
		AuthorizationURL: authorizationURL,
		Metadata:         metadata,
		Config:           effectiveCfg,
		ResourceURL:      resourceURL,
	}, nil
}

func (s *Service) hostedRefreshConfig(
	ctx context.Context,
	cfg ServerConfig,
) (ServerConfig, Metadata, error) {
	resourceURL := firstNonEmpty(cfg.ResourceURL, cfg.RemoteURL)
	prm, err := s.protectedResourceMetadata(ctx, cfg, resourceURL)
	if err != nil {
		return ServerConfig{}, Metadata{}, err
	}
	issuer, err := hostedAuthorizationServer(cfg, prm.AuthorizationServers)
	if err != nil {
		return ServerConfig{}, Metadata{}, err
	}
	asm, err := s.authServerMetadata(ctx, cfg, issuer)
	if err != nil {
		return ServerConfig{}, Metadata{}, err
	}
	registration, err := s.loadTokenRegistration(ctx, cfg.Target)
	if err != nil {
		return ServerConfig{}, Metadata{}, fmt.Errorf("mcp auth: load registration for refresh: %w", err)
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return ServerConfig{}, Metadata{}, err
	}
	if !s.dynamicRegistrationReusable(
		registration,
		cfg,
		fingerprint,
		resourceURL,
		registration.RedirectURL,
		registration.Issuer,
	) {
		return ServerConfig{}, Metadata{}, errRefreshRegistrationInvalid
	}
	if !issuersEqual(registration.Issuer, asm.Issuer) {
		return ServerConfig{}, metadataFromAuthServer(asm), fmt.Errorf(
			"mcp auth: stored registration is no longer valid for the discovered issuer: %w",
			ErrTokenBindingInvalid,
		)
	}
	resolved := cfg
	resolved.ResourceURL = resourceURL
	resolved.ClientID = registration.ClientID
	resolved.TokenEndpointAuthMethod = registration.TokenEndpointAuthMethod
	if resolved.TokenEndpointAuthMethod != tokenEndpointAuthMethodNone {
		if s.secretResolver == nil {
			return ServerConfig{}, Metadata{}, errRefreshRegistrationInvalid
		}
		secret, err := s.resolveRegistrationSecret(ctx, registration.ClientSecretRef)
		if registrationSecretUnavailable(err) || (err == nil && strings.TrimSpace(secret) == "") {
			return ServerConfig{}, Metadata{}, errRefreshRegistrationInvalid
		}
		if err != nil {
			return ServerConfig{}, Metadata{}, err
		}
		resolved.ClientSecret = secret
	}
	return resolved, metadataFromAuthServer(asm), nil
}

func (s *Service) hostedRegistrationRefreshable(
	cfg ServerConfig,
	registration ClientRegistration,
	fingerprint string,
) bool {
	return s.dynamicRegistrationReusable(
		registration, cfg, fingerprint,
		firstNonEmpty(cfg.ResourceURL, cfg.RemoteURL), registration.RedirectURL, registration.Issuer,
	) && (registration.TokenEndpointAuthMethod == tokenEndpointAuthMethodNone || s.secretResolver != nil)
}

func (s *Service) protectedResourceMetadata(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
) (discoveredProtectedResource, error) {
	candidates := protectedResourceMetadataURLs(cfg.PRMURL, resourceURL)
	probeURL := resourceURL
	if cfg.pluginHeaderOAuth {
		probeURL = cfg.RemoteURL
	}
	advertisedURL, challengeScopes := s.probeProtectedResource(ctx, cfg, probeURL, http.MethodGet)
	if advertisedURL == "" && cfg.pluginHeaderOAuth {
		var postScopes []string
		advertisedURL, postScopes = s.probeProtectedResource(ctx, cfg, probeURL, http.MethodPost)
		if len(challengeScopes) == 0 {
			challengeScopes = postScopes
		}
	}
	if advertisedURL != "" {
		candidates = append([]protectedResourceMetadataCandidate{{
			URL: advertisedURL, ResourceURL: resourceURL,
		}}, candidates...)
	}
	for _, candidate := range candidates {
		prm, err := oauthex.GetProtectedResourceMetadata(
			ctx,
			candidate.URL,
			candidate.ResourceURL,
			s.sdkHTTPClientFor(cfg),
		)
		if err == nil && prm != nil {
			return discoveredProtectedResource{ProtectedResourceMetadata: prm, challengeScopes: challengeScopes}, nil
		}
	}
	return discoveredProtectedResource{}, errors.New("mcp auth: protected resource metadata discovery failed")
}

func (s *Service) probeProtectedResource(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
	method string,
) (string, []string) {
	var body io.Reader = http.NoBody
	if method == http.MethodPost {
		body = strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"initialize",` +
				`"params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"compozy","version":"1"}}}`,
		)
	}
	req, err := http.NewRequestWithContext(ctx, method, resourceURL, body)
	if err != nil {
		return "", nil
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	resp, err := s.clientFor(cfg).Do(req)
	if err != nil {
		return "", nil
	}
	defer func() {
		if drainErr := drainAndCloseResponseBody(resp.Body); drainErr != nil {
			slog.DebugContext(
				ctx,
				"mcp auth: drain protected resource probe response body",
				"error",
				drainErr,
			)
		}
	}()
	advertisedURL := advertisedProtectedResourceMetadataURL(resp.Header, resourceURL, cfg.pluginHeaderOAuth)
	if resp.StatusCode != http.StatusUnauthorized || resp.Request == nil ||
		validateAdvertisedMetadataURL(resp.Request.URL.String(), resourceURL, true) != nil {
		return advertisedURL, nil
	}
	challenges, err := oauthex.ParseWWWAuthenticate(resp.Header.Values("WWW-Authenticate"))
	if err != nil {
		return advertisedURL, nil
	}
	for _, challenge := range challenges {
		if challenge.Scheme == "bearer" {
			if scopes := resourceOAuthScopes(strings.Fields(challenge.Params["scope"])); len(scopes) > 0 {
				return advertisedURL, scopes
			}
		}
	}
	return advertisedURL, nil
}

// AdvertisedProtectedResourceMetadataURL reads an origin-bound RFC 9728 Bearer challenge.
func AdvertisedProtectedResourceMetadataURL(headers http.Header, resourceURL string) string {
	return advertisedProtectedResourceMetadataURL(headers, resourceURL, true)
}

func advertisedProtectedResourceMetadataURL(headers http.Header, resourceURL string, requireOrigin bool) string {
	challenges, err := oauthex.ParseWWWAuthenticate(headers.Values("WWW-Authenticate"))
	if err != nil {
		return ""
	}
	for _, challenge := range challenges {
		raw := strings.TrimSpace(challenge.Params["resource_metadata"])
		if requireOrigin && challenge.Scheme != "bearer" {
			continue
		}
		if raw != "" && validateAdvertisedMetadataURL(raw, resourceURL, requireOrigin) == nil {
			return raw
		}
	}
	return ""
}

func validateAdvertisedMetadataURL(metadataURL, resourceURL string, requireOrigin bool) error {
	if err := validateAbsoluteHTTPURL("advertised protected resource metadata URL", metadataURL); err != nil {
		return err
	}
	if err := validateAbsoluteHTTPURL("resource URL", resourceURL); err != nil {
		return err
	}
	metadata, err := url.Parse(strings.TrimSpace(metadataURL))
	if err != nil {
		return fmt.Errorf("mcp auth: parse advertised protected resource metadata URL: %w", err)
	}
	resource, err := url.Parse(strings.TrimSpace(resourceURL))
	if err != nil {
		return fmt.Errorf("mcp auth: parse resource URL: %w", err)
	}
	if !strings.EqualFold(metadata.Host, resource.Host) ||
		(requireOrigin && !strings.EqualFold(metadata.Scheme, resource.Scheme)) {
		return errors.New("mcp auth: advertised protected resource metadata URL does not match the resource")
	}
	return nil
}

type protectedResourceMetadataCandidate struct {
	URL         string
	ResourceURL string
}

func protectedResourceMetadataURLs(
	explicitURL, resourceURL string,
) []protectedResourceMetadataCandidate {
	urls := make([]protectedResourceMetadataCandidate, 0, 4)
	if raw := strings.TrimSpace(explicitURL); raw != "" {
		urls = append(urls, protectedResourceMetadataCandidate{URL: raw, ResourceURL: resourceURL})
	}
	resource, err := url.Parse(resourceURL)
	if err != nil {
		return urls
	}
	atEndpoint := resource.Clone()
	atEndpoint.Path = "/.well-known/oauth-protected-resource/" + strings.TrimLeft(
		resource.Path,
		"/",
	)
	atEndpoint.RawQuery = ""
	atEndpoint.Fragment = ""
	urls = append(
		urls,
		protectedResourceMetadataCandidate{URL: atEndpoint.String(), ResourceURL: resourceURL},
	)
	rootResource := url.URL{Scheme: resource.Scheme, Host: resource.Host}
	atRoot := rootResource
	atRoot.Path = "/.well-known/oauth-protected-resource"
	rootMetadataURL := atRoot.String()
	// RFC 3986 §6.2.3 equates the empty and slash paths at the origin root.
	urls = append(
		urls,
		protectedResourceMetadataCandidate{
			URL:         rootMetadataURL,
			ResourceURL: rootResource.String(),
		},
	)
	rootResource.Path = "/"
	urls = append(urls, protectedResourceMetadataCandidate{URL: rootMetadataURL, ResourceURL: rootResource.String()})
	return urls
}

func metadataFromAuthServer(asm *oauthex.AuthServerMeta) Metadata {
	return Metadata{
		Issuer:                        strings.TrimSpace(asm.Issuer),
		AuthorizationEndpoint:         strings.TrimSpace(asm.AuthorizationEndpoint),
		TokenEndpoint:                 strings.TrimSpace(asm.TokenEndpoint),
		RevocationEndpoint:            strings.TrimSpace(asm.RevocationEndpoint),
		CodeChallengeMethodsSupported: append([]string(nil), asm.CodeChallengeMethodsSupported...),
		ScopesSupported:               append([]string(nil), asm.ScopesSupported...),
		grantTypesSupported:           slices.Clone(asm.GrantTypesSupported),
		IssuerParameterSupported:      asm.AuthorizationResponseIssParameterSupported,
	}
}

type hostedOAuthClient struct {
	id, secret, authMethod string
	scopes                 []string
	allowOfflineAccess     bool
}

func (s *Service) resolveHostedRegistration(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
	redirectURL string,
	asm *oauthex.AuthServerMeta,
	loginScopes hostedLoginScopes,
) (hostedOAuthClient, error) {
	if asm.ClientIDMetadataDocumentSupported {
		clientMetadataURL := strings.TrimSpace(s.clientMetadataURL)
		if clientMetadataURL == "" {
			clientMetadataURL = compozyconfig.DefaultMCPClientMetadataURL
		}
		if err := validateClientMetadataURL(clientMetadataURL); err == nil {
			if refreshCapable, validateErr := s.validateCIMD(
				ctx,
				cfg,
				clientMetadataURL,
				redirectURL,
			); validateErr == nil {
				if persistErr := s.persistCIMDRegistration(
					ctx,
					cfg,
					resourceURL,
					redirectURL,
					asm.Issuer,
					clientMetadataURL,
				); persistErr != nil {
					return hostedOAuthClient{}, persistErr
				}
				return hostedOAuthClient{
					id: clientMetadataURL, authMethod: tokenEndpointAuthMethodNone,
					scopes: loginScopes.values, allowOfflineAccess: refreshCapable,
				}, nil
			}
		}
	}
	return s.dynamicClientRegistration(ctx, cfg, resourceURL, redirectURL, asm, loginScopes)
}

func (s *Service) validateCIMD(
	ctx context.Context,
	cfg ServerConfig,
	metadataURL string,
	redirectURL string,
) (refreshCapable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("mcp auth: build CIMD request: %w", err)
	}
	resp, err := s.clientFor(cfg).Do(req)
	if err != nil {
		return false, fmt.Errorf("mcp auth: fetch CIMD: %w", err)
	}
	defer func() {
		err = errors.Join(err, drainAndCloseResponseBody(resp.Body))
	}()
	if resp.StatusCode != http.StatusOK {
		return false, errors.New("mcp auth: CIMD unavailable")
	}
	var document struct {
		ClientID     string   `json:"client_id"`
		RedirectURIs []string `json:"redirect_uris"`
		GrantTypes   []string `json:"grant_types"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return false, fmt.Errorf("mcp auth: decode CIMD: %w", err)
	}
	configuredID, err := canonicalCIMDURL(metadataURL)
	if err != nil {
		return false, err
	}
	documentID, err := canonicalCIMDURL(document.ClientID)
	if err != nil || documentID != configuredID {
		return false, errors.New("mcp auth: CIMD client_id does not match configured URL")
	}
	for _, candidate := range document.RedirectURIs {
		if strings.TrimSpace(candidate) == strings.TrimSpace(redirectURL) {
			return slices.Contains(document.GrantTypes, serviceRefreshTokenKey), nil
		}
	}
	return false, errors.New("mcp auth: CIMD does not permit the active redirect URL")
}

func (s *Service) persistCIMDRegistration(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
	redirectURL string,
	issuer string,
	clientID string,
) error {
	if s.registrations == nil {
		return errors.New("mcp auth: registration store is required for CIMD")
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return err
	}
	_, err = s.registrations.SaveMCPAuthRegistration(ctx, ClientRegistration{
		Target:                cfg.Target.Normalize(),
		DefinitionFingerprint: fingerprint,
		ResourceURL:           resourceURL,
		Issuer:                strings.TrimSpace(issuer),
		ClientID:              clientID,
		RedirectURL:           redirectURL,
		Scopes:                trimStrings(cfg.Scopes),
		UpdatedAt:             s.now().UTC(),
	}, RegistrationSecrets{})
	if err != nil {
		return fmt.Errorf("mcp auth: persist CIMD registration: %w", err)
	}
	return nil
}
