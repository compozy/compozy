package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
)

type googleKeyCache struct {
	mu     sync.Mutex
	keys   map[string]*rsa.PublicKey
	loaded time.Time
}

type googleClaims struct {
	Issuer          string         `json:"iss"`
	Audience        jsontext.Value `json:"aud"`
	AuthorizedParty string         `json:"azp"`
	Subject         string         `json:"sub"`
	Nonce           string         `json:"nonce"`
	Email           string         `json:"email"`
	EmailVerified   bool           `json:"email_verified"`
	Name            string         `json:"name"`
	IssuedAt        int64          `json:"iat"`
	ExpiresAt       int64          `json:"exp"`
	AuthTime        int64          `json:"auth_time"`
}

func (o *OAuth) googleIdentity(
	ctx context.Context, token string, a store.OAuthAttempt, now time.Time,
) (store.VerifiedIdentity, error) {
	claims, err := o.verifyGoogleToken(ctx, token, now)
	if err != nil {
		return store.VerifiedIdentity{}, err
	}
	if !equalHash(hashSecret(claims.Nonce), a.NonceHash) {
		return store.VerifiedIdentity{}, errProvider
	}
	if a.FreshSignIn && (claims.AuthTime < a.CreatedAt.Unix() || claims.AuthTime > claims.IssuedAt) {
		return store.VerifiedIdentity{}, errFreshAuth
	}
	if !claims.EmailVerified {
		return store.VerifiedIdentity{}, errEmailUnverified
	}
	return identity(Google, claims.Subject, claims.Email, claims.Name)
}

func (o *OAuth) verifyGoogleToken(ctx context.Context, token string, now time.Time) (googleClaims, error) {
	input, encodedSignature, ok := strings.CutLast(token, ".")
	protected, payload, split := strings.Cut(input, ".")
	if !ok || !split || len(token) > 16<<10 || strings.ContainsAny(token, "\r\n") {
		return googleClaims{}, errProvider
	}
	encoding := base64.RawURLEncoding.Strict()
	header, err := encoding.DecodeString(protected)
	if err != nil {
		return googleClaims{}, errProvider
	}
	var jws struct {
		Algorithm string   `json:"alg"`
		KeyID     string   `json:"kid"`
		Critical  []string `json:"crit"`
		Unencoded *bool    `json:"b64"`
	}
	if err := json.Unmarshal(header, &jws); err != nil || jws.Algorithm != "RS256" ||
		jws.KeyID == "" || len(jws.KeyID) > 128 || len(jws.Critical) > 0 || jws.Unencoded != nil {
		return googleClaims{}, errProvider
	}
	signature, err := encoding.DecodeString(encodedSignature)
	if err != nil {
		return googleClaims{}, errProvider
	}
	key := o.googleKey(ctx, jws.KeyID, now)
	digest := sha256.Sum256([]byte(input))
	if key == nil || rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return googleClaims{}, errProvider
	}
	body, err := encoding.DecodeString(payload)
	var claims googleClaims
	if err != nil || json.Unmarshal(body, &claims) != nil || !o.validGoogleClaims(claims, now) {
		return googleClaims{}, errProvider
	}
	return claims, nil
}

func (o *OAuth) validGoogleClaims(c googleClaims, now time.Time) bool {
	if (c.Issuer != googleIssuer && c.Issuer != "accounts.google.com") || c.ExpiresAt <= now.Unix() ||
		c.IssuedAt <= 0 || c.IssuedAt > now.Add(time.Minute).Unix() || c.IssuedAt >= c.ExpiresAt {
		return false
	}
	if c.AuthorizedParty != "" && c.AuthorizedParty != o.google.ID {
		return false
	}
	var audience string
	if json.Unmarshal(c.Audience, &audience) == nil {
		return audience == o.google.ID
	}
	var audiences []string
	return json.Unmarshal(c.Audience, &audiences) == nil && slices.Contains(audiences, o.google.ID) &&
		(len(audiences) == 1 || c.AuthorizedParty == o.google.ID)
}

type googleJWK struct {
	Type, Algorithm, Use, KeyID, Modulus, Exponent string
}

func (o *OAuth) googleKey(ctx context.Context, kid string, now time.Time) *rsa.PublicKey {
	cache := &o.googleKeys
	cache.mu.Lock()
	defer cache.mu.Unlock()
	const ttl, refreshFloor = time.Hour, 10 * time.Second
	age := now.Sub(cache.loaded)
	if cache.keys[kid] != nil && age < ttl || !cache.loaded.IsZero() && age < refreshFloor {
		return cache.keys[kid]
	}
	cache.loaded = now
	var document struct {
		Keys []struct {
			Type      string `json:"kty"`
			Algorithm string `json:"alg"`
			Use       string `json:"use"`
			KeyID     string `json:"kid"`
			Modulus   string `json:"n"`
			Exponent  string `json:"e"`
		} `json:"keys"`
	}
	if err := o.request(ctx, o.endpoints.GoogleJWKS, "", nil, &document); err != nil {
		cache.keys = nil
		return nil
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, raw := range document.Keys {
		k := googleJWK{raw.Type, raw.Algorithm, raw.Use, raw.KeyID, raw.Modulus, raw.Exponent}
		key := k.publicKey()
		if key != nil {
			if keys[k.KeyID] != nil {
				cache.keys = nil
				return nil
			}
			keys[k.KeyID] = key
		}
	}
	cache.keys = keys
	return keys[kid]
}

func (k googleJWK) publicKey() *rsa.PublicKey {
	if k.Type != "RSA" || k.Algorithm != "RS256" || k.Use != "sig" || k.KeyID == "" {
		return nil
	}
	encoding := base64.RawURLEncoding.Strict()
	modulus, err := encoding.DecodeString(k.Modulus)
	if err != nil || len(modulus) < 256 || len(modulus) > 1024 || modulus[0] == 0 {
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
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(e)}
}
