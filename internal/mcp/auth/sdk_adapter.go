package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
	if len(prm.AuthorizationServers) == 0 {
		return LoginState{}, errors.New(
			"mcp auth: protected resource metadata has no authorization servers",
		)
	}
	issuer := strings.TrimSpace(prm.AuthorizationServers[0])
	asm, err := s.authServerMetadata(ctx, cfg, issuer)
	if err != nil {
		return LoginState{}, err
	}
	clientID, clientSecret, tokenEndpointAuthMethod, err := s.resolveHostedRegistration(
		ctx,
		cfg,
		resourceURL,
		redirectURL,
		asm,
	)
	if err != nil {
		return LoginState{}, err
	}
	metadata := metadataFromAuthServer(asm)
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
	effectiveCfg.ClientID = clientID
	effectiveCfg.ClientSecret = clientSecret
	effectiveCfg.TokenEndpointAuthMethod = tokenEndpointAuthMethod
	authorizationURL, err := authorizationURL(
		metadata.AuthorizationEndpoint,
		effectiveCfg,
		redirectURL,
		state,
		pkce,
	)
	if err != nil {
		return LoginState{}, err
	}
	return LoginState{
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
	if len(prm.AuthorizationServers) == 0 {
		return ServerConfig{}, Metadata{}, errors.New(
			"mcp auth: protected resource metadata has no authorization servers",
		)
	}
	asm, err := s.authServerMetadata(ctx, cfg, prm.AuthorizationServers[0])
	if err != nil {
		return ServerConfig{}, Metadata{}, err
	}
	redirectURL := strings.TrimSpace(s.defaultRedirectURL)
	if redirectURL == "" {
		return ServerConfig{}, Metadata{}, errors.New(
			"mcp auth: redirect URL is required to resolve client registration",
		)
	}
	clientID, clientSecret, tokenEndpointAuthMethod, err := s.resolveHostedRegistration(
		ctx,
		cfg,
		resourceURL,
		redirectURL,
		asm,
	)
	if err != nil {
		return ServerConfig{}, Metadata{}, err
	}
	resolved := cfg
	resolved.ResourceURL = resourceURL
	resolved.ClientID = clientID
	resolved.ClientSecret = clientSecret
	resolved.TokenEndpointAuthMethod = tokenEndpointAuthMethod
	return resolved, metadataFromAuthServer(asm), nil
}

func (s *Service) protectedResourceMetadata(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
) (*oauthex.ProtectedResourceMetadata, error) {
	candidates := protectedResourceMetadataURLs(cfg.PRMURL, resourceURL)
	if advertisedURL := s.protectedResourceMetadataURLFromProbe(ctx, cfg, resourceURL); advertisedURL != "" {
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
			return prm, nil
		}
	}
	return nil, errors.New("mcp auth: protected resource metadata discovery failed")
}

func (s *Service) protectedResourceMetadataURLFromProbe(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
) (metadataURL string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, http.NoBody)
	if err != nil {
		return ""
	}
	resp, err := s.clientFor(cfg).Do(req)
	if err != nil {
		return ""
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
	challenges, err := oauthex.ParseWWWAuthenticate(resp.Header.Values("WWW-Authenticate"))
	if err == nil {
		for _, challenge := range challenges {
			raw := strings.TrimSpace(challenge.Params["resource_metadata"])
			if raw == "" {
				continue
			}
			if err := validateAdvertisedMetadataURL(raw, resourceURL); err != nil {
				continue
			}
			metadataURL = raw
			break
		}
	}
	return metadataURL
}

func validateAdvertisedMetadataURL(metadataURL, resourceURL string) error {
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
	if !strings.EqualFold(metadata.Host, resource.Host) {
		return errors.New("mcp auth: advertised protected resource metadata host does not match resource URL")
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
	urls := make([]protectedResourceMetadataCandidate, 0, 3)
	if raw := strings.TrimSpace(explicitURL); raw != "" {
		urls = append(urls, protectedResourceMetadataCandidate{URL: raw, ResourceURL: resourceURL})
	}
	resource, err := url.Parse(resourceURL)
	if err != nil {
		return urls
	}
	atEndpoint := *resource
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
	atRoot := *resource
	atRoot.Path = "/.well-known/oauth-protected-resource"
	atRoot.RawQuery = ""
	atRoot.Fragment = ""
	rootResource := *resource
	rootResource.Path = ""
	rootResource.RawQuery = ""
	rootResource.Fragment = ""
	urls = append(
		urls,
		protectedResourceMetadataCandidate{
			URL:         atRoot.String(),
			ResourceURL: rootResource.String(),
		},
	)
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
		IssuerParameterSupported:      asm.AuthorizationResponseIssParameterSupported,
	}
}

func (s *Service) resolveHostedRegistration(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
	redirectURL string,
	asm *oauthex.AuthServerMeta,
) (string, string, string, error) {
	if cfg.registrationStrategy() == RegistrationPreRegistered {
		if !issuersEqual(cfg.IssuerURL, asm.Issuer) {
			return "", "", "", errors.New(
				"mcp auth: pre-registered issuer does not match authorization server",
			)
		}
		method, err := effectiveTokenEndpointAuthMethod(cfg)
		if err != nil {
			return "", "", "", err
		}
		return strings.TrimSpace(cfg.ClientID), strings.TrimSpace(cfg.ClientSecret), method, nil
	}
	if asm.ClientIDMetadataDocumentSupported {
		clientMetadataURL := strings.TrimSpace(s.clientMetadataURL)
		if clientMetadataURL == "" {
			clientMetadataURL = compozyconfig.DefaultMCPClientMetadataURL
		}
		if err := validateClientMetadataURL(clientMetadataURL); err == nil {
			if validateErr := s.validateCIMD(ctx, cfg, clientMetadataURL, redirectURL); validateErr == nil {
				if persistErr := s.persistCIMDRegistration(
					ctx,
					cfg,
					resourceURL,
					redirectURL,
					asm.Issuer,
					clientMetadataURL,
				); persistErr != nil {
					return "", "", "", persistErr
				}
				return clientMetadataURL, "", tokenEndpointAuthMethodNone, nil
			}
		}
	}
	return s.dynamicClientRegistration(ctx, cfg, resourceURL, redirectURL, asm)
}

func (s *Service) validateCIMD(
	ctx context.Context,
	cfg ServerConfig,
	metadataURL string,
	redirectURL string,
) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("mcp auth: build CIMD request: %w", err)
	}
	resp, err := s.clientFor(cfg).Do(req)
	if err != nil {
		return fmt.Errorf("mcp auth: fetch CIMD: %w", err)
	}
	defer func() {
		err = errors.Join(err, drainAndCloseResponseBody(resp.Body))
	}()
	if resp.StatusCode != http.StatusOK {
		return errors.New("mcp auth: CIMD unavailable")
	}
	var document struct {
		ClientID     string   `json:"client_id"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return fmt.Errorf("mcp auth: decode CIMD: %w", err)
	}
	configuredID, err := canonicalCIMDURL(metadataURL)
	if err != nil {
		return err
	}
	documentID, err := canonicalCIMDURL(document.ClientID)
	if err != nil || documentID != configuredID {
		return errors.New("mcp auth: CIMD client_id does not match configured URL")
	}
	for _, candidate := range document.RedirectURIs {
		if strings.TrimSpace(candidate) == strings.TrimSpace(redirectURL) {
			return nil
		}
	}
	return errors.New("mcp auth: CIMD does not permit the active redirect URL")
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
