package remoteproto

import (
	"context"
	"crypto/ed25519"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	grantType      = "JWT"
	leaseType      = "compozy-lease+jwt"
	snapshotType   = "compozy-security-snapshot+jwt"
	gatePolicyType = "compozy-gate-policy+jwt"
	maxCompactSize = 1 << 20
)

var errSignature = errors.New("invalid compact JWS signature")

type protectedHeader struct {
	Alg  string   `json:"alg"`
	Typ  string   `json:"typ"`
	Kid  string   `json:"kid,omitempty"`
	JWK  *JWK     `json:"jwk,omitempty"`
	Crit []string `json:"crit,omitempty"`
	B64  *bool    `json:"b64,omitempty"`
}

type compactJWS struct {
	header    protectedHeader
	input     string
	body      []byte
	signature []byte
}

func parseCompact(token string) (compactJWS, error) {
	if len(token) > maxCompactSize {
		return compactJWS{}, errSignature
	}
	input, encodedSignature, ok := strings.CutLast(token, ".")
	if !ok {
		return compactJWS{}, errSignature
	}
	encodedHeader, encodedBody, ok := strings.Cut(input, ".")
	if !ok {
		return compactJWS{}, errSignature
	}
	headerBytes, err := decodeBase64URL(encodedHeader)
	if err != nil {
		return compactJWS{}, errSignature
	}
	var header protectedHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil || len(header.Crit) > 0 || header.B64 != nil {
		return compactJWS{}, errSignature
	}
	body, err := decodeBase64URL(encodedBody)
	if err != nil {
		return compactJWS{}, errSignature
	}
	signature, err := decodeBase64URL(encodedSignature)
	if err != nil {
		return compactJWS{}, errSignature
	}
	return compactJWS{header: header, input: input, body: body, signature: signature}, nil
}

func signingInput(header protectedHeader, claims any) (string, error) {
	protected, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("encode protected header: %w", err)
	}
	data, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode signed claims: %w", err)
	}
	return base64URL.EncodeToString(protected) + "." + base64URL.EncodeToString(data), nil
}

type Signer struct {
	kid string
	key ed25519.PrivateKey
}

func NewSigner(kid string, key ed25519.PrivateKey) (*Signer, error) {
	if kid == "" || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("signing requires a key ID and an Ed25519 private key")
	}
	return &Signer{kid: kid, key: slices.Clone(key)}, nil
}

func (s *Signer) PublicJWK() JWK {
	public := s.key[ed25519.SeedSize:]
	return JWK{
		Kty: keyOKP, Crv: curveEd, X: base64URL.EncodeToString(public),
		Kid: s.kid, Alg: algEdDSA, Use: useSignature,
	}
}

func (s *Signer) sign(typ string, claims any) (string, error) {
	input, err := signingInput(protectedHeader{Alg: algEdDSA, Typ: typ, Kid: s.kid}, claims)
	if err != nil {
		return "", err
	}
	return input + "." + base64URL.EncodeToString(ed25519.Sign(s.key, []byte(input))), nil
}

type JWKSFetcher func(context.Context) (JWKS, error)

type verificationKeys map[string]ed25519.PublicKey

type JWSVerifier struct {
	issuer  string
	keys    atomic.Pointer[verificationKeys]
	refresh JWKSFetcher
	mu      sync.Mutex
}

var _ Verifier = (*JWSVerifier)(nil)

func NewJWSVerifier(issuer string, initial JWKS, refresh JWKSFetcher) (*JWSVerifier, error) {
	v := &JWSVerifier{issuer: issuer, refresh: refresh}
	if v.issuer == "" {
		v.issuer = CloudIssuer
	}
	if len(initial.Keys) == 0 && refresh != nil {
		v.keys.Store(new(verificationKeys))
		return v, nil
	}
	if err := v.ReplaceKeys(initial); err != nil {
		return nil, err
	}
	return v, nil
}

// ReplaceKeys atomically swaps the complete, trusted JWKS, including retired keys.
func (v *JWSVerifier) ReplaceKeys(jwks JWKS) error {
	keys, err := indexVerificationKeys(jwks)
	if err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys.Store(&keys)
	return nil
}

func indexVerificationKeys(jwks JWKS) (verificationKeys, error) {
	keys := make(verificationKeys, len(jwks.Keys))
	for _, jwk := range jwks.Keys {
		public, err := jwk.PublicKey()
		key, ok := public.(ed25519.PublicKey)
		if err != nil || !ok || jwk.Kid == "" || keys[jwk.Kid] != nil {
			return nil, errors.New("JWKS must contain Ed25519 verification keys with distinct key IDs")
		}
		keys[jwk.Kid] = key
	}
	if len(keys) == 0 {
		return nil, errors.New("JWKS is empty")
	}
	return keys, nil
}

func (v *JWSVerifier) key(ctx context.Context, kid string) ed25519.PublicKey {
	initial := v.keys.Load()
	if key := (*initial)[kid]; key != nil || v.refresh == nil {
		return key
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	current := v.keys.Load()
	if current != initial {
		return (*current)[kid]
	}
	jwks, err := v.refresh(ctx)
	if err != nil {
		return nil
	}
	keys, err := indexVerificationKeys(jwks)
	if err != nil {
		return nil
	}
	v.keys.Store(&keys)
	return keys[kid]
}

func (v *JWSVerifier) verify(ctx context.Context, token, typ string, claims any) error {
	jws, err := parseCompact(token)
	if err != nil || jws.header.Alg != algEdDSA || jws.header.Typ != typ || jws.header.Kid == "" ||
		jws.header.JWK != nil {
		return errSignature
	}
	key := v.key(ctx, jws.header.Kid)
	if key == nil || !ed25519.Verify(key, []byte(jws.input), jws.signature) {
		return errSignature
	}
	if err := json.Unmarshal(jws.body, claims); err != nil {
		return errSignature
	}
	return nil
}
