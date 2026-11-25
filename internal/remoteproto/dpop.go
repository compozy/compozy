package remoteproto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/asn1"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	dpopType           = "dpop+jwt"
	DPoPLeeway         = 60 * time.Second
	DPoPReplayWindow   = 120 * time.Second
	defaultReplayLimit = 65_536
	maxProofSize       = 8 << 10
	httpScheme         = "http"
)

var (
	ErrDPoPInvalid  = errors.New("dpop_invalid")
	ErrDPoPReplayed = errors.New("dpop_replayed")
)

type DPoPProof struct {
	Method    string     `json:"htm"`
	URL       string     `json:"htu"`
	IssuedAt  int64      `json:"iat"`
	JTI       string     `json:"jti"`
	TokenHash *string    `json:"ath,omitempty"`
	Audience  ComputerID `json:"aud,omitempty"`
}

type DPoPBinding struct {
	Method         string
	URL            string
	JKT            string
	Credential     string
	CredentialHash string
	Audience       ComputerID
}

func BuildDPoP(signer crypto.Signer, method, target, credential string, now time.Time) (string, error) {
	return buildDPoP(signer, method, target, credential, "", now)
}

// BuildTunnelDPoP includes the receiving computer in a proof that the host may delegate to Cloud.
func BuildTunnelDPoP(
	signer crypto.Signer, computer ComputerID, method, path, credential string, now time.Time,
) (string, error) {
	return buildDPoP(signer, method, "https://"+TunnelHost+path, credential, computer, now)
}

func buildDPoP(
	signer crypto.Signer, method, target, credential string, audience ComputerID, now time.Time,
) (string, error) {
	if signer == nil || method == "" {
		return "", ErrDPoPInvalid
	}
	target, err := proofURL(target)
	if err != nil {
		return "", err
	}
	key, err := NewPublicJWK(signer.Public())
	if err != nil || key.Kty != keyEC {
		return "", ErrDPoPInvalid
	}
	proof := DPoPProof{Method: method, URL: target, IssuedAt: now.Unix(), JTI: rand.Text(), Audience: audience}
	if credential != "" {
		proof.TokenHash = new(hashBase64([]byte(credential)))
	}
	return signDeviceJWS(signer, key, dpopType, proof)
}

func signDeviceJWS(signer crypto.Signer, key JWK, typ string, claims any) (string, error) {
	input, err := signingInput(protectedHeader{Alg: algES256, Typ: typ, JWK: &key}, claims)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(input))
	raw, err := signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("sign device proof: %w", err)
	}
	var sig struct{ R, S *big.Int }
	tail, err := asn1.Unmarshal(raw, &sig)
	if err != nil || len(tail) != 0 || !validSignaturePart(sig.R) || !validSignaturePart(sig.S) {
		return "", ErrDPoPInvalid
	}
	var encoded [64]byte
	sig.R.FillBytes(encoded[:32])
	sig.S.FillBytes(encoded[32:])
	return input + "." + base64URL.EncodeToString(encoded[:]), nil
}

func validSignaturePart(value *big.Int) bool {
	return value != nil && value.Sign() > 0 && value.BitLen() <= 256
}

type DPoPVerifier struct {
	mu        sync.Mutex
	seen      map[string]time.Time
	nextSweep time.Time
	maxSeen   int
}

func NewDPoPVerifier() *DPoPVerifier {
	return &DPoPVerifier{seen: make(map[string]time.Time), maxSeen: defaultReplayLimit}
}

func (v *DPoPVerifier) VerifyDPoP(token string, binding DPoPBinding, now time.Time) (DPoPProof, error) {
	proof, err := CheckDPoP(token, binding, now)
	if err != nil {
		return DPoPProof{}, err
	}
	if err := v.consume(proof.JTI, now); err != nil {
		return DPoPProof{}, err
	}
	return proof, nil
}

// CheckDPoP verifies a proof when the consumer owns a separate replay fence.
func CheckDPoP(token string, binding DPoPBinding, now time.Time) (DPoPProof, error) {
	if len(token) > maxProofSize || binding.JKT == "" || binding.Method == "" {
		return DPoPProof{}, ErrDPoPInvalid
	}
	jws, err := parseCompact(token)
	if err != nil {
		return DPoPProof{}, ErrDPoPInvalid
	}
	return verifyDPoP(jws, binding, now)
}

// BrowserDPoPKey validates a proof before the presented key has been authorized.
func BrowserDPoPKey(token, method, target string, now time.Time) (string, error) {
	if len(token) > maxProofSize || method == "" {
		return "", ErrDPoPInvalid
	}
	jws, err := parseCompact(token)
	if err != nil || jws.header.JWK == nil {
		return "", ErrDPoPInvalid
	}
	jkt, err := jws.header.JWK.Thumbprint()
	if err != nil {
		return "", ErrDPoPInvalid
	}
	if _, err := verifyDPoP(jws, DPoPBinding{Method: method, URL: target, JKT: jkt}, now); err != nil {
		return "", err
	}
	return jkt, nil
}

func verifyDPoP(jws compactJWS, binding DPoPBinding, now time.Time) (DPoPProof, error) {
	if verifyDeviceSignature(jws, dpopType, binding.JKT) {
		var proof DPoPProof
		if err := json.Unmarshal(jws.body, &proof); err == nil && proof.matches(binding, now) {
			return proof, nil
		}
	}
	return DPoPProof{}, ErrDPoPInvalid
}

func verifyDeviceSignature(jws compactJWS, typ, jkt string) bool {
	header := jws.header
	if header.Alg != algES256 || header.Typ != typ || header.JWK == nil ||
		header.JWK.Kty != keyEC || len(jws.signature) != 64 {
		return false
	}
	key, err := header.JWK.ecKey()
	if err != nil {
		return false
	}
	thumbprint, err := header.JWK.Thumbprint()
	if err != nil || subtle.ConstantTimeCompare([]byte(jkt), []byte(thumbprint)) != 1 {
		return false
	}
	digest := sha256.Sum256([]byte(jws.input))
	r := new(big.Int).SetBytes(jws.signature[:32])
	s := new(big.Int).SetBytes(jws.signature[32:])
	return ecdsa.Verify(key, digest[:], r, s)
}

func (p DPoPProof) matches(binding DPoPBinding, now time.Time) bool {
	if p.Method != binding.Method || p.JTI == "" || len(p.JTI) > 128 || p.IssuedAt <= 0 ||
		p.IssuedAt < now.Add(-DPoPLeeway).Unix() || p.IssuedAt > now.Add(DPoPLeeway).Unix() {
		return false
	}
	target, err := proofURL(binding.URL)
	if err != nil || p.URL != target || (binding.Audience != "" && p.Audience != binding.Audience) {
		return false
	}
	want := binding.CredentialHash
	if binding.Credential != "" {
		want = hashBase64([]byte(binding.Credential))
	}
	if want == "" {
		return p.TokenHash == nil
	}
	return p.TokenHash != nil &&
		subtle.ConstantTimeCompare([]byte(*p.TokenHash), []byte(want)) == 1
}

func (v *DPoPVerifier) consume(jti string, now time.Time) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !now.Before(v.nextSweep) {
		maps.DeleteFunc(v.seen, func(_ string, expiry time.Time) bool { return now.After(expiry) })
		v.nextSweep = now.Add(10 * time.Second)
	}
	if expiry, seen := v.seen[jti]; seen {
		if !now.After(expiry) {
			return ErrDPoPReplayed
		}
	} else if len(v.seen) >= v.maxSeen {
		return ErrDPoPInvalid
	}
	v.seen[jti] = now.Add(DPoPReplayWindow)
	return nil
}

func proofURL(target string) (string, error) {
	u, err := url.Parse(target)
	if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != httpScheme) {
		return "", ErrDPoPInvalid
	}
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == httpScheme && u.Port() == "80") {
		u.Host, _, _ = strings.CutLast(u.Host, ":")
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.RawQuery, u.ForceQuery = "", false
	u.Fragment, u.RawFragment = "", ""
	return u.String(), nil
}
