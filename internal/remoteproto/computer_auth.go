package remoteproto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"time"
)

const (
	ComputerProofHeader = "X-Compozy-Computer-Proof"
	computerProofType   = "compozy-computer-proof+jwt"
)

var ErrComputerProof = errors.New("computer_proof_invalid")

type ComputerProof struct {
	Method   string `json:"htm"`
	URL      string `json:"htu"`
	BodyHash string `json:"body_sha256"`
	IssuedAt int64  `json:"iat"`
	JTI      string `json:"jti"`
}

func SignComputerProof(key ed25519.PrivateKey, method, url string, body []byte, now time.Time) (string, error) {
	if len(key) != ed25519.PrivateKeySize {
		return "", ErrComputerProof
	}
	proof := ComputerProof{
		Method: method, URL: url, BodyHash: hashBase64(body), IssuedAt: now.Unix(), JTI: rand.Text(),
	}
	input, err := signingInput(protectedHeader{Alg: algEdDSA, Typ: computerProofType}, proof)
	if err != nil {
		return "", err
	}
	return input + "." + base64URL.EncodeToString(ed25519.Sign(key, []byte(input))), nil
}

func VerifyComputerProof(token string, key JWK, method, url string, body []byte, now time.Time) (ComputerProof, error) {
	public, err := key.PublicKey()
	edKey, ok := public.(ed25519.PublicKey)
	if err != nil || !ok || len(token) > maxProofSize {
		return ComputerProof{}, ErrComputerProof
	}
	jws, err := parseCompact(token)
	if err != nil || jws.header.Alg != algEdDSA || jws.header.Typ != computerProofType ||
		jws.header.JWK != nil || !ed25519.Verify(edKey, []byte(jws.input), jws.signature) {
		return ComputerProof{}, ErrComputerProof
	}
	var proof ComputerProof
	if err := json.Unmarshal(jws.body, &proof); err != nil ||
		proof.Method != method || proof.URL != url || proof.BodyHash != hashBase64(body) ||
		proof.JTI == "" || len(proof.JTI) > 128 ||
		proof.IssuedAt < now.Add(-DPoPLeeway).Unix() || proof.IssuedAt > now.Add(DPoPLeeway).Unix() {
		return ComputerProof{}, ErrComputerProof
	}
	return proof, nil
}

func computerChallengePayload(issuer string, id ComputerID, nonce string) []byte {
	return []byte("compozy-control-v1\n" + issuer + "\n" + string(id) + "\n" + nonce)
}

func SignComputerChallenge(key ed25519.PrivateKey, issuer string, id ComputerID, nonce string) string {
	if len(key) != ed25519.PrivateKeySize {
		return ""
	}
	return base64URL.EncodeToString(ed25519.Sign(key, computerChallengePayload(issuer, id, nonce)))
}

func VerifyComputerChallenge(key JWK, issuer string, id ComputerID, nonce, signature string) bool {
	public, err := key.PublicKey()
	edKey, ok := public.(ed25519.PublicKey)
	if err != nil || !ok || len(signature) != base64URL.EncodedLen(ed25519.SignatureSize) {
		return false
	}
	decoded, err := decodeBase64URL(signature)
	return err == nil && ed25519.Verify(edKey, computerChallengePayload(issuer, id, nonce), decoded)
}
