package remoteproto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"strings"
)

const (
	algEdDSA       = "EdDSA"
	algES256       = "ES256"
	curveEd        = "Ed25519"
	curveEC        = "P-256"
	keyOKP         = "OKP"
	keyEC          = "EC"
	useSignature   = "sig"
	coordinateSize = 32
	p256YStart     = 1 + coordinateSize
)

var (
	base64URL = base64.RawURLEncoding.Strict()
	errJWK    = errors.New("invalid public JWK")
)

type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y,omitempty"`
	D   string `json:"d,omitempty"`
	Kid string `json:"kid,omitempty"`
	Alg string `json:"alg,omitempty"`
	Use string `json:"use,omitempty"`
}

type JWKS struct {
	Keys    []JWK    `json:"keys"`
	Retired []string `json:"retired,omitempty"`
}

func NewPublicJWK(key crypto.PublicKey) (JWK, error) {
	switch public := key.(type) {
	case *ecdsa.PublicKey:
		if public == nil || public.Curve != elliptic.P256() {
			return JWK{}, errJWK
		}
		point, err := public.Bytes()
		if err != nil {
			return JWK{}, errJWK
		}
		return JWK{
			Kty: keyEC, Crv: curveEC,
			X: base64URL.EncodeToString(point[1:p256YStart]),
			Y: base64URL.EncodeToString(point[p256YStart:]),
		}, nil
	case ed25519.PublicKey:
		if len(public) != ed25519.PublicKeySize {
			return JWK{}, errJWK
		}
		return JWK{Kty: keyOKP, Crv: curveEd, X: base64URL.EncodeToString(public)}, nil
	default:
		return JWK{}, errJWK
	}
}

func (j JWK) PublicKey() (crypto.PublicKey, error) {
	if j.D != "" || (j.Use != "" && j.Use != useSignature) {
		return nil, errJWK
	}
	switch j.Kty {
	case keyOKP:
		if j.Crv != curveEd || j.Y != "" || (j.Alg != "" && j.Alg != algEdDSA) {
			return nil, errJWK
		}
		key, err := decodeCoordinate(j.X)
		return ed25519.PublicKey(key), err
	case keyEC:
		return j.ecKey()
	default:
		return nil, errJWK
	}
}

func (j JWK) ecKey() (*ecdsa.PublicKey, error) {
	if j.Crv != curveEC || (j.Alg != "" && j.Alg != algES256) || j.D != "" || (j.Use != "" && j.Use != useSignature) {
		return nil, errJWK
	}
	x, err := decodeCoordinate(j.X)
	if err != nil {
		return nil, err
	}
	y, err := decodeCoordinate(j.Y)
	if err != nil {
		return nil, err
	}
	var point [1 + 2*coordinateSize]byte
	point[0] = 0x04 // SEC 1 uncompressed point prefix.
	copy(point[1:p256YStart], x)
	copy(point[p256YStart:], y)
	key, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point[:])
	if err != nil {
		return nil, errJWK
	}
	return key, nil
}

// Thumbprint hashes only the required public members in RFC 7638 order.
func (j JWK) Thumbprint() (string, error) {
	if _, err := j.PublicKey(); err != nil {
		return "", err
	}
	public := struct {
		Crv string `json:"crv"`
		Kty string `json:"kty"`
		X   string `json:"x"`
		Y   string `json:"y,omitempty"`
	}{Crv: j.Crv, Kty: j.Kty, X: j.X, Y: j.Y}
	data, err := json.Marshal(public)
	if err != nil {
		return "", err
	}
	return hashBase64(data), nil
}

func decodeCoordinate(value string) ([]byte, error) {
	if len(value) != base64URL.EncodedLen(coordinateSize) {
		return nil, errJWK
	}
	key, err := decodeBase64URL(value)
	if err != nil || len(key) != coordinateSize {
		return nil, errJWK
	}
	return key, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	// The base64 decoder accepts CR/LF even in strict mode.
	if strings.ContainsAny(value, "\r\n") {
		return nil, errors.New("invalid base64url")
	}
	return base64URL.DecodeString(value)
}

func hashBase64(value []byte) string {
	hash := sha256.Sum256(value)
	return base64URL.EncodeToString(hash[:])
}
