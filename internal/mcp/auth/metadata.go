package auth

import (
	"context"
	"errors"
	"slices"
	"strings"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func (m Metadata) supportsOfflineAccess() bool {
	return slices.Contains(m.ScopesSupported, offlineAccessScope) &&
		(len(m.grantTypesSupported) == 0 || slices.Contains(m.grantTypesSupported, serviceRefreshTokenKey))
}

type discoveredProtectedResource struct {
	*oauthex.ProtectedResourceMetadata
	challengeScopes []string
}

func (r discoveredProtectedResource) loginScopes(configured []string, metadata Metadata) []string {
	scopes := trimStrings(configured)
	if len(scopes) != 0 || !metadata.supportsOfflineAccess() {
		return scopes
	}
	if len(r.challengeScopes) != 0 {
		return r.challengeScopes
	}
	return resourceOAuthScopes(r.ScopesSupported)
}

func resourceOAuthScopes(scopes []string) []string {
	return slices.DeleteFunc(trimStrings(scopes), func(scope string) bool { return scope == offlineAccessScope })
}

func (s *Service) authServerMetadata(
	ctx context.Context,
	cfg ServerConfig,
	issuer string,
) (*oauthex.AuthServerMeta, error) {
	metadata, err := sdkauth.GetAuthServerMetadata(
		ctx,
		strings.TrimSpace(issuer),
		s.sdkHTTPClientFor(cfg),
	)
	if err != nil {
		return nil, errors.New("mcp auth: authorization server metadata discovery failed")
	}
	if metadata == nil {
		return nil, errors.New("mcp auth: authorization server metadata is required")
	}
	if cfg.registrationStrategy() == RegistrationAuto && strings.TrimSpace(cfg.IssuerURL) != "" &&
		!issuersEqual(cfg.IssuerURL, metadata.Issuer) {
		return nil, errors.New("mcp auth: configured issuer does not match authorization server metadata")
	}
	return metadata, nil
}

// hostedAuthorizationServer selects the pinned issuer only from resource-advertised servers.
func hostedAuthorizationServer(cfg ServerConfig, issuers []string) (string, error) {
	if len(issuers) == 0 {
		return "", errors.New("mcp auth: protected resource metadata has no authorization servers")
	}
	if strings.TrimSpace(cfg.IssuerURL) == "" {
		return strings.TrimSpace(issuers[0]), nil
	}
	for _, issuer := range issuers {
		if issuersEqual(cfg.IssuerURL, issuer) {
			return strings.TrimSpace(issuer), nil
		}
	}
	return "", errors.New("mcp auth: configured issuer is not advertised by the protected resource")
}
