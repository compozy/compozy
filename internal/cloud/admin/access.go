// SPDX-License-Identifier: FSL-1.1-MIT

package admin

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	assertionHeader   = "Cf-Access-Jwt-Assertion"
	keyLifetime       = 15 * time.Minute
	keyMinimumRefresh = 5 * time.Second
)

var teamDomain = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.cloudflareaccess\.com$`)

type AccessConfig struct {
	TeamDomain string
	Audience   string
	Allowlist  []string
	Transport  http.RoundTripper
}

type accessVerifier struct {
	issuer, audience string
	allowlist        map[string]struct{}
	client           *http.Client

	mu                sync.Mutex
	keys              map[string]*rsa.PublicKey
	loaded, attempted time.Time
}

func newAccessVerifier(cfg *AccessConfig) (*accessVerifier, error) {
	if cfg == nil {
		return nil, nil
	}
	domain, audience := strings.ToLower(strings.TrimSpace(cfg.TeamDomain)), strings.TrimSpace(cfg.Audience)
	if !teamDomain.MatchString(domain) || audience == "" || len(cfg.Allowlist) == 0 {
		return nil, errors.New("configure a Cloudflare Access team domain, application audience and operator allowlist")
	}
	allowlist := make(map[string]struct{}, len(cfg.Allowlist))
	for _, item := range cfg.Allowlist {
		address := strings.ToLower(strings.TrimSpace(item))
		parsed, err := mail.ParseAddress(address)
		if err != nil || parsed.Name != "" || parsed.Address != address {
			return nil, errors.New("invalid Cloudflare Access allowlist email")
		}
		allowlist[address] = struct{}{}
	}
	return &accessVerifier{
		issuer: "https://" + domain, audience: audience, allowlist: allowlist,
		client: &http.Client{Timeout: 3 * time.Second, Transport: cfg.Transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}, nil
}

type accessClaims struct {
	Issuer    string   `json:"iss"`
	Audience  []string `json:"aud"`
	Email     string   `json:"email"`
	Subject   string   `json:"sub"`
	Type      string   `json:"type"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
}

func (v *accessVerifier) operator(req *http.Request, now time.Time) (string, string) {
	if v == nil {
		return "", "access_unconfigured"
	}
	assertions := req.Header.Values(assertionHeader)
	if len(assertions) == 0 {
		return "", "assertion_missing"
	}
	if len(assertions) != 1 {
		return "", "assertion_invalid"
	}
	claims, ok := v.verify(req.Context(), assertions[0], now)
	if !ok {
		return "", "assertion_invalid"
	}
	address := strings.ToLower(strings.TrimSpace(claims.Email))
	if _, allowed := v.allowlist[address]; !allowed {
		return "", "email_not_allowed"
	}
	return address, ""
}

func (v *accessVerifier) verify(ctx context.Context, token string, now time.Time) (accessClaims, bool) {
	var claims accessClaims
	if len(token) > 16<<10 || strings.ContainsAny(token, " \r\n\t,") {
		return claims, false
	}
	input, rawSignature, ok := strings.CutLast(token, ".")
	protected, payload, split := strings.Cut(input, ".")
	if !ok || !split {
		return claims, false
	}
	encoding := base64.RawURLEncoding.Strict()
	header, err := encoding.DecodeString(protected)
	if err != nil {
		return claims, false
	}
	var jws struct {
		Algorithm string   `json:"alg"`
		KeyID     string   `json:"kid"`
		Critical  []string `json:"crit"`
		Unencoded *bool    `json:"b64"`
	}
	if json.Unmarshal(header, &jws) != nil || jws.Algorithm != "RS256" ||
		jws.KeyID == "" || len(jws.KeyID) > 128 || len(jws.Critical) != 0 || jws.Unencoded != nil {
		return claims, false
	}
	signature, err := encoding.DecodeString(rawSignature)
	if err != nil {
		return claims, false
	}
	body, err := encoding.DecodeString(payload)
	if err != nil || json.Unmarshal(body, &claims) != nil || !v.validClaims(claims, now) {
		return claims, false
	}
	key := v.key(ctx, jws.KeyID, now)
	digest := sha256.Sum256([]byte(input))
	return claims, key != nil && rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) == nil
}

func (v *accessVerifier) validClaims(claims accessClaims, now time.Time) bool {
	return claims.Type == "app" && claims.Subject != "" && claims.Email != "" && claims.Issuer == v.issuer &&
		slices.Contains(claims.Audience, v.audience) && claims.ExpiresAt > now.Unix() &&
		claims.IssuedAt > 0 && claims.IssuedAt <= now.Unix() && claims.IssuedAt < claims.ExpiresAt &&
		claims.NotBefore <= now.Unix()
}

func (v *accessVerifier) key(ctx context.Context, id string, now time.Time) *rsa.PublicKey {
	v.mu.Lock()
	defer v.mu.Unlock()
	fresh := now.Sub(v.loaded) < keyLifetime
	if key := v.keys[id]; key != nil && fresh {
		return key
	}
	if !v.attempted.IsZero() && now.Sub(v.attempted) < keyMinimumRefresh {
		return nil
	}
	v.attempted = now
	keys, err := v.fetchKeys(ctx)
	if err != nil {
		return nil
	}
	v.keys, v.loaded = keys, now
	return keys[id]
}

type accessJWK struct {
	ID        string `json:"kid"`
	Type      string `json:"kty"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

func (v *accessVerifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.issuer+"/cdn-cgi/access/certs", http.NoBody)
	if err != nil {
		return nil, err
	}
	// #nosec G704 -- The origin is constrained to the configured Cloudflare Access team domain.
	res, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("could not load Cloudflare Access keys")
	}
	var document struct {
		Keys []accessJWK `json:"keys"`
	}
	if err := json.UnmarshalRead(io.LimitReader(res.Body, 64<<10), &document); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, jwk := range document.Keys {
		key := jwk.publicKey()
		if key == nil {
			continue
		}
		if keys[jwk.ID] != nil {
			return nil, errors.New("duplicate Cloudflare Access key ID")
		}
		keys[jwk.ID] = key
	}
	return keys, nil
}

func (k accessJWK) publicKey() *rsa.PublicKey {
	if k.Type != "RSA" || (k.Use != "" && k.Use != "sig") ||
		(k.Algorithm != "" && k.Algorithm != "RS256") || k.ID == "" {
		return nil
	}
	encoding := base64.RawURLEncoding.Strict()
	modulus, err := encoding.DecodeString(k.Modulus)
	if err != nil || len(modulus) < 256 || len(modulus) > 1024 || modulus[0] == 0 {
		return nil
	}
	n := new(big.Int).SetBytes(modulus)
	if n.BitLen() < 2048 {
		return nil
	}
	exponent, err := encoding.DecodeString(k.Exponent)
	if err != nil || len(exponent) == 0 || len(exponent) > 4 {
		return nil
	}
	e := new(big.Int).SetBytes(exponent).Int64()
	if e < 3 || e > 1<<31-1 || e%2 == 0 {
		return nil
	}
	return &rsa.PublicKey{N: n, E: int(e)}
}
