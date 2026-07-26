package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/compozy/agh/internal/bridgesdk"
)

func verifyGChatWebhookBearer(ctx context.Context, req *http.Request, body []byte, cfg *resolvedInstanceConfig) error {
	if cfg == nil {
		return errors.New("gchat: config is required")
	}
	switch detectGChatWebhookShape(body) {
	case gchatModePubSub:
		if !modeUsesPubSubIngress(cfg.mode) {
			return nil
		}
		return verifyPubSubBearerToken(ctx, req, cfg)
	case gchatModeDirect:
		if !modeUsesDirectIngress(cfg.mode) {
			return nil
		}
		return verifyDirectBearerToken(ctx, req, cfg)
	default:
		return errors.New("gchat: unrecognized webhook payload shape")
	}
}

func verifyDirectBearerToken(ctx context.Context, req *http.Request, cfg *resolvedInstanceConfig) error {
	if cfg == nil {
		return errors.New("gchat: config is required")
	}
	tokenString, err := bearerToken(req)
	if err != nil {
		return err
	}
	keys, err := fetchGoogleX509Keys(ctx, cfg.directCertsURL)
	if err != nil {
		return err
	}
	claims := &googleDirectClaims{}
	parsed, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("gchat: unsupported signing method %q", token.Method.Alg())
		}
		return keyByTokenHeader(token.Header, keys)
	}, jwt.WithAudience(strings.TrimSpace(cfg.projectNumber)), jwt.WithLeeway(5*time.Minute))
	if err != nil {
		return fmt.Errorf("gchat: invalid direct bearer token: %w", err)
	}
	if !parsed.Valid {
		return errors.New("gchat: invalid direct bearer token")
	}
	if !issuerMatches(claims.Issuer, strings.TrimSpace(cfg.directIssuer)) {
		return fmt.Errorf("gchat: direct bearer issuer %q did not match %q", claims.Issuer, cfg.directIssuer)
	}
	return nil
}

func verifyPubSubBearerToken(ctx context.Context, req *http.Request, cfg *resolvedInstanceConfig) error {
	if cfg == nil {
		return errors.New("gchat: config is required")
	}
	tokenString, err := bearerToken(req)
	if err != nil {
		return err
	}
	keys, err := fetchGoogleX509Keys(ctx, cfg.pubsubCertsURL)
	if err != nil {
		return err
	}
	claims := &googleOIDCClaims{}
	parsed, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("gchat: unsupported signing method %q", token.Method.Alg())
		}
		return keyByTokenHeader(token.Header, keys)
	}, jwt.WithAudience(strings.TrimSpace(cfg.pubsubAudience)), jwt.WithLeeway(5*time.Minute))
	if err != nil {
		return fmt.Errorf("gchat: invalid pubsub bearer token: %w", err)
	}
	if !parsed.Valid {
		return errors.New("gchat: invalid pubsub bearer token")
	}
	if !issuerMatches(
		claims.Issuer,
		strings.TrimSpace(cfg.pubsubIssuer),
		"accounts.google.com",
		"https://accounts.google.com",
	) {
		return fmt.Errorf("gchat: pubsub bearer issuer %q did not match expected Google issuer", claims.Issuer)
	}
	if !strings.EqualFold(strings.TrimSpace(claims.Email), strings.TrimSpace(cfg.pubsubServiceAccountEmail)) {
		return fmt.Errorf(
			"gchat: pubsub bearer email %q did not match expected service account %q",
			claims.Email,
			cfg.pubsubServiceAccountEmail,
		)
	}
	if !claims.EmailVerified {
		return fmt.Errorf("gchat: pubsub bearer email %q is not verified", strings.TrimSpace(claims.Email))
	}
	return nil
}

func fetchGoogleX509Keys(ctx context.Context, certsURL string) (map[string]*rsa.PublicKey, error) {
	if strings.TrimSpace(certsURL) == "" {
		return nil, errors.New("gchat: certs url is required")
	}
	return defaultGoogleX509KeyCache.fetch(ctx, certsURL)
}

func newGoogleX509KeyCache(client *http.Client, fallbackTTL time.Duration, now func() time.Time) *googleX509KeyCache {
	if client == nil {
		client = &http.Client{Timeout: gchatCertFetchTimeout}
	}
	if fallbackTTL <= 0 {
		fallbackTTL = gchatCertCacheFallbackTTL
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &googleX509KeyCache{
		client:      client,
		fallbackTTL: fallbackTTL,
		now:         now,
		entries:     make(map[string]googleX509KeyCacheEntry),
	}
}

func (c *googleX509KeyCache) fetch(ctx context.Context, certsURL string) (map[string]*rsa.PublicKey, error) {
	trimmedURL := strings.TrimSpace(certsURL)
	if trimmedURL == "" {
		return nil, errors.New("gchat: certs url is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if entry, ok := c.entries[trimmedURL]; ok && len(entry.keys) > 0 && now.Before(entry.expiresAt) {
		return entry.keys, nil
	}

	keys, expiresAt, err := c.fetchRemote(ctx, trimmedURL)
	if err != nil {
		if entry, ok := c.entries[trimmedURL]; ok && len(entry.keys) > 0 {
			return entry.keys, nil
		}
		return nil, err
	}
	c.entries[trimmedURL] = googleX509KeyCacheEntry{
		keys:      keys,
		expiresAt: expiresAt,
	}
	return keys, nil
}

func (c *googleX509KeyCache) fetchRemote(
	ctx context.Context,
	certsURL string,
) (keys map[string]*rsa.PublicKey, expiresAt time.Time, err error) {
	endpoint, err := newValidatedGChatURL(certsURL)
	if err != nil {
		return nil, time.Time{}, err
	}
	req, err := newGChatRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, time.Time{}, err
	}
	//nolint:gosec // G704: provider URLs are Google-host allowlisted; env overrides are explicit operator test inputs.
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer func() {
		err = errors.Join(err, bridgesdk.DrainAndCloseHTTPResponseBody(resp.Body))
	}()
	if resp.StatusCode != http.StatusOK {
		body, readErr := bridgesdk.ReadHTTPResponseBody(resp.Body)
		return nil, time.Time{}, errors.Join(
			classifyGChatHTTPError(resp.StatusCode, resp.Header.Get("Retry-After"), body),
			readErr,
		)
	}
	certs := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&certs); err != nil {
		return nil, time.Time{}, err
	}
	if len(certs) == 0 {
		return nil, time.Time{}, errors.New("gchat: x509 cert document omitted keys")
	}
	keysByID := make(map[string]*rsa.PublicKey, len(certs))
	for keyID, pemCert := range certs {
		block, _ := pem.Decode([]byte(strings.TrimSpace(pemCert)))
		if block == nil {
			return nil, time.Time{}, fmt.Errorf("gchat: decode x509 cert %q: missing pem block", keyID)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("gchat: parse x509 cert %q: %w", keyID, err)
		}
		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, time.Time{}, fmt.Errorf("gchat: x509 cert %q did not contain an rsa public key", keyID)
		}
		keysByID[strings.TrimSpace(keyID)] = publicKey
	}
	return keysByID, cacheExpiryFromHeaders(c.now(), resp.Header, c.fallbackTTL), nil
}

func resolveAllowedGoogleURLOverride(
	envOverride string,
	providerOverride string,
	fallback string,
	fieldName string,
	allowedHosts ...string,
) (string, error) {
	if trimmedEnv := strings.TrimSpace(envOverride); trimmedEnv != "" {
		normalized := normalizeURL(trimmedEnv)
		return normalized, validateGChatEndpointURL(normalized)
	}
	if strings.TrimSpace(providerOverride) == "" {
		return normalizeURL(fallback), nil
	}
	normalized := normalizeURL(providerOverride)
	parsed, err := url.Parse(normalized)
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Hostname()) == "" ||
		!strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("gchat: %s must use an allowed Google https host", fieldName)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	for _, allowedHost := range allowedHosts {
		if host == strings.ToLower(strings.TrimSpace(allowedHost)) {
			return normalized, nil
		}
	}
	return "", fmt.Errorf("gchat: %s host %q is not allowed", fieldName, parsed.Hostname())
}

func cacheExpiryFromHeaders(now time.Time, header http.Header, fallback time.Duration) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if fallback <= 0 {
		fallback = gchatCertCacheFallbackTTL
	}
	for directive := range strings.SplitSeq(header.Get("Cache-Control"), ",") {
		part := strings.TrimSpace(directive)
		lower := strings.ToLower(part)
		if !strings.HasPrefix(lower, "max-age=") {
			continue
		}
		seconds, err := strconv.Atoi(strings.TrimSpace(lower[len("max-age="):]))
		if err == nil && seconds > 0 {
			return now.Add(time.Duration(seconds) * time.Second)
		}
	}
	return now.Add(fallback)
}

func bearerToken(req *http.Request) (string, error) {
	if req == nil {
		return "", errors.New("gchat: webhook request is required")
	}
	authz := strings.TrimSpace(req.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return "", errors.New("gchat: bearer authorization is required")
	}
	token := strings.TrimSpace(authz[len("Bearer "):])
	if token == "" {
		return "", errors.New("gchat: bearer token is required")
	}
	return token, nil
}

func keyByTokenHeader(header map[string]any, keys map[string]*rsa.PublicKey) (*rsa.PublicKey, error) {
	keyID := firstNonEmpty(stringHeader(header, "kid"), stringHeader(header, "x5t"))
	if keyID == "" {
		return nil, errors.New("gchat: token header missing key id")
	}
	key, ok := keys[keyID]
	if !ok {
		return nil, fmt.Errorf("gchat: signing key %q not found", keyID)
	}
	return key, nil
}
