package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func allowTeamsDirectMessage(cfg resolvedInstanceConfig, user teamsUserIdentity, direct bool) bool {
	if !direct {
		return true
	}
	switch cfg.dmPolicy.Normalize() {
	case "", bridgepkg.BridgeDMPolicyOpen:
		return true
	case bridgepkg.BridgeDMPolicyAllowlist:
		return teamsIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	case bridgepkg.BridgeDMPolicyPairing:
		if teamsIdentityAllowed(cfg.pairedUserIDs, cfg.pairedUsernames, user) {
			return true
		}
		return teamsIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	default:
		return false
	}
}

func teamsIdentityAllowed(
	ids map[string]struct{},
	usernames map[string]struct{},
	user teamsUserIdentity,
) bool {
	if len(ids) == 0 && len(usernames) == 0 {
		return false
	}
	if _, ok := ids[normalizeTeamsID(user.ID)]; ok {
		return true
	}
	if _, ok := usernames[normalizeTeamsUsername(firstNonEmpty(user.Username, user.DisplayName))]; ok {
		return true
	}
	return false
}

func parseTeamsBearerToken(header string) (string, error) {
	authz := strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return "", errors.New("teams: bearer authorization is required")
	}
	tokenString := strings.TrimSpace(authz[len("Bearer "):])
	if tokenString == "" {
		return "", errors.New("teams: bearer token is required")
	}
	return tokenString, nil
}

func decodeTeamsAuthorizationProbe(body []byte) (struct {
	ServiceURL string `json:"serviceUrl,omitempty"`
	ChannelID  string `json:"channelId,omitempty"`
}, error) {
	probe := struct {
		ServiceURL string `json:"serviceUrl,omitempty"`
		ChannelID  string `json:"channelId,omitempty"`
	}{}
	if err := json.Unmarshal(body, &probe); err != nil {
		return probe, errors.New("teams: webhook payload is not valid json")
	}
	return probe, nil
}

func teamsAuthorizationServiceURL(probeServiceURL string, defaultServiceURL string) (string, error) {
	serviceURL := normalizeURL(probeServiceURL)
	if serviceURL == "" {
		serviceURL = defaultServiceURL
	}
	if serviceURL == "" {
		return "", errors.New("teams: serviceUrl is required for token validation")
	}
	return serviceURL, nil
}

func verifyTeamsAuthorization(
	ctx context.Context,
	req *http.Request,
	body []byte,
	cfg resolvedInstanceConfig,
) error {
	if req == nil {
		return errors.New("teams: webhook request is required")
	}
	tokenString, err := parseTeamsBearerToken(req.Header.Get("Authorization"))
	if err != nil {
		return err
	}
	probe, err := decodeTeamsAuthorizationProbe(body)
	if err != nil {
		return err
	}
	serviceURL, err := teamsAuthorizationServiceURL(probe.ServiceURL, cfg.serviceURL)
	if err != nil {
		return err
	}

	metadata, err := fetchTeamsOpenIDMetadata(ctx, cfg.openIDMetadataURL)
	if err != nil {
		return err
	}
	jwks, err := fetchTeamsJWKS(ctx, metadata.JWKSURI)
	if err != nil {
		return err
	}
	claims := &teamsAuthClaims{}
	parsed, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("teams: unsupported signing method %q", token.Method.Alg())
			}
			keyID := firstNonEmpty(
				stringHeader(token.Header, "kid"),
				stringHeader(token.Header, "x5t"),
			)
			if keyID == "" {
				return nil, errors.New("teams: token header missing key id")
			}
			jwk, err := jwks.keyByID(keyID)
			if err != nil {
				if refreshed, refreshErr := refreshTeamsJWKS(ctx, metadata.JWKSURI); refreshErr == nil {
					jwks = refreshed
					jwk, err = jwks.keyByID(keyID)
				}
			}
			if err != nil {
				return nil, err
			}
			if err := jwk.validateEndorsement(
				firstNonEmpty(strings.TrimSpace(probe.ChannelID), "msteams"),
			); err != nil {
				return nil, err
			}
			return jwk.publicKey()
		},
		jwt.WithAudience(strings.TrimSpace(cfg.appID)),
		jwt.WithIssuer(
			firstNonEmpty(strings.TrimSpace(metadata.Issuer), "https://api.botframework.com"),
		),
		jwt.WithLeeway(5*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("teams: invalid bearer token: %w", err)
	}
	if !parsed.Valid {
		return errors.New("teams: invalid bearer token")
	}
	return validateTeamsAuthorizationClaims(claims, serviceURL)
}

func validateTeamsAuthorizationClaims(claims *teamsAuthClaims, serviceURL string) error {
	if normalizeURL(claims.ServiceURL) != serviceURL {
		return fmt.Errorf(
			"teams: token serviceUrl %q did not match activity serviceUrl %q",
			claims.ServiceURL,
			serviceURL,
		)
	}
	return nil
}

func fetchTeamsOpenIDMetadata(
	ctx context.Context,
	metadataURL string,
) (*teamsOpenIDMetadata, error) {
	if strings.TrimSpace(metadataURL) == "" {
		return nil, errors.New("teams: openid metadata url is required")
	}
	endpoint, err := validatedTeamsCredentialedURL(metadataURL, "openid metadata url")
	if err != nil {
		return nil, err
	}
	if cached := cachedTeamsOpenIDMetadata(endpoint); cached != nil {
		return cached, nil
	}
	metadata, err := fetchTeamsOpenIDMetadataUncached(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	storeTeamsOpenIDMetadata(endpoint, metadata)
	return cloneTeamsOpenIDMetadata(metadata), nil
}

func fetchTeamsOpenIDMetadataUncached(
	ctx context.Context,
	endpoint string,
) (metadataResult *teamsOpenIDMetadata, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := teamsAuthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, bridgesdk.DrainAndCloseHTTPResponseBody(resp.Body))
	}()
	if resp.StatusCode != http.StatusOK {
		body, readErr := bridgesdk.ReadHTTPResponseBody(resp.Body)
		return nil, errors.Join(
			classifyTeamsHTTPError(resp.StatusCode, resp.Header.Get("Retry-After"), body),
			readErr,
		)
	}
	var metadata teamsOpenIDMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}
	if strings.TrimSpace(metadata.JWKSURI) == "" {
		return nil, errors.New("teams: openid metadata jwks_uri is required")
	}
	if !validTeamsCredentialedURL(metadata.JWKSURI) {
		return nil, fmt.Errorf("teams: openid metadata jwks_uri %q is invalid", metadata.JWKSURI)
	}
	return &metadata, nil
}

func fetchTeamsJWKS(ctx context.Context, jwksURL string) (*teamsJWKS, error) {
	endpoint, err := validatedTeamsCredentialedURL(jwksURL, "jwks url")
	if err != nil {
		return nil, err
	}
	if cached := cachedTeamsJWKS(endpoint); cached != nil {
		return cached, nil
	}
	keys, err := fetchTeamsJWKSUncached(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	storeTeamsJWKS(endpoint, keys)
	return cloneTeamsJWKS(keys), nil
}

func refreshTeamsJWKS(ctx context.Context, jwksURL string) (*teamsJWKS, error) {
	endpoint, err := validatedTeamsCredentialedURL(jwksURL, "jwks url")
	if err != nil {
		return nil, err
	}
	keys, err := fetchTeamsJWKSUncached(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	storeTeamsJWKS(endpoint, keys)
	return cloneTeamsJWKS(keys), nil
}

func fetchTeamsJWKSUncached(ctx context.Context, endpoint string) (keysResult *teamsJWKS, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := teamsAuthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, bridgesdk.DrainAndCloseHTTPResponseBody(resp.Body))
	}()
	if resp.StatusCode != http.StatusOK {
		body, readErr := bridgesdk.ReadHTTPResponseBody(resp.Body)
		return nil, errors.Join(
			classifyTeamsHTTPError(resp.StatusCode, resp.Header.Get("Retry-After"), body),
			readErr,
		)
	}
	var keys teamsJWKS
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, err
	}
	if len(keys.Keys) == 0 {
		return nil, errors.New("teams: jwks document omitted signing keys")
	}
	return &keys, nil
}

func cachedTeamsOpenIDMetadata(endpoint string) *teamsOpenIDMetadata {
	teamsAuthCache.mu.Lock()
	defer teamsAuthCache.mu.Unlock()
	entry, ok := teamsAuthCache.metadata[endpoint]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(teamsAuthCache.metadata, endpoint)
		}
		return nil
	}
	return cloneTeamsOpenIDMetadata(&entry.metadata)
}

func storeTeamsOpenIDMetadata(endpoint string, metadata *teamsOpenIDMetadata) {
	if metadata == nil {
		return
	}
	teamsAuthCache.mu.Lock()
	defer teamsAuthCache.mu.Unlock()
	teamsAuthCache.metadata[endpoint] = teamsOpenIDMetadataCacheEntry{
		metadata:  *cloneTeamsOpenIDMetadata(metadata),
		expiresAt: time.Now().Add(teamsAuthCacheTTL),
	}
}

func cachedTeamsJWKS(endpoint string) *teamsJWKS {
	teamsAuthCache.mu.Lock()
	defer teamsAuthCache.mu.Unlock()
	entry, ok := teamsAuthCache.jwks[endpoint]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(teamsAuthCache.jwks, endpoint)
		}
		return nil
	}
	return cloneTeamsJWKS(&entry.jwks)
}

func storeTeamsJWKS(endpoint string, keys *teamsJWKS) {
	if keys == nil {
		return
	}
	teamsAuthCache.mu.Lock()
	defer teamsAuthCache.mu.Unlock()
	teamsAuthCache.jwks[endpoint] = teamsJWKSCacheEntry{
		jwks:      *cloneTeamsJWKS(keys),
		expiresAt: time.Now().Add(teamsAuthCacheTTL),
	}
}

func cloneTeamsOpenIDMetadata(metadata *teamsOpenIDMetadata) *teamsOpenIDMetadata {
	if metadata == nil {
		return nil
	}
	clone := *metadata
	return &clone
}

func cloneTeamsJWKS(keys *teamsJWKS) *teamsJWKS {
	if keys == nil {
		return nil
	}
	clone := teamsJWKS{Keys: make([]teamsJWK, len(keys.Keys))}
	copy(clone.Keys, keys.Keys)
	for idx := range clone.Keys {
		clone.Keys[idx].Endorsements = append([]string(nil), keys.Keys[idx].Endorsements...)
	}
	return &clone
}

func (k teamsJWKS) keyByID(keyID string) (*teamsJWK, error) {
	for idx := range k.Keys {
		if strings.TrimSpace(k.Keys[idx].Kid) == keyID ||
			strings.TrimSpace(k.Keys[idx].X5T) == keyID {
			return &k.Keys[idx], nil
		}
	}
	return nil, fmt.Errorf("teams: jwk %q not found", keyID)
}

func (k teamsJWK) validateEndorsement(channelID string) error {
	if len(k.Endorsements) == 0 {
		return nil
	}
	for _, endorsement := range k.Endorsements {
		if strings.EqualFold(strings.TrimSpace(endorsement), strings.TrimSpace(channelID)) {
			return nil
		}
	}
	return fmt.Errorf("teams: jwk is not endorsed for channel %q", channelID)
}

func (k teamsJWK) publicKey() (*rsa.PublicKey, error) {
	if strings.TrimSpace(k.Kty) != "" && !strings.EqualFold(strings.TrimSpace(k.Kty), "RSA") {
		return nil, fmt.Errorf("teams: unsupported jwk kty %q", k.Kty)
	}
	modulusBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(k.N))
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(k.E))
	if err != nil {
		return nil, err
	}
	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, errors.New("teams: jwk exponent is invalid")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}
