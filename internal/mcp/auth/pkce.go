package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	pkceS256Value = "S256"
)

const (
	pkceVerifierBytes = 48
	stateBytes        = 32
)

// PKCEPair holds the generated verifier and S256 code challenge. The verifier
// is secret and must not be logged.
type PKCEPair struct {
	Verifier  string
	Challenge string
	Method    string
}

func newPKCEPair(random io.Reader) (PKCEPair, error) {
	if random == nil {
		random = rand.Reader
	}
	verifier, err := randomURLToken(random, pkceVerifierBytes)
	if err != nil {
		return PKCEPair{}, fmt.Errorf("mcp auth: generate PKCE verifier: %w", err)
	}
	sum := sha256.Sum256([]byte(verifier))
	return PKCEPair{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		Method:    pkceS256Value,
	}, nil
}

func newState(random io.Reader) (string, error) {
	if random == nil {
		random = rand.Reader
	}
	state, err := randomURLToken(random, stateBytes)
	if err != nil {
		return "", fmt.Errorf("mcp auth: generate oauth state: %w", err)
	}
	return state, nil
}

func randomURLToken(random io.Reader, bytesLen int) (string, error) {
	if bytesLen <= 0 {
		return "", errors.New("mcp auth: token byte length must be positive")
	}
	buf := make([]byte, bytesLen)
	if _, err := io.ReadFull(random, buf); err != nil {
		return "", fmt.Errorf("mcp auth: generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validateVerifier(verifier string) error {
	length := len(strings.TrimSpace(verifier))
	if length < 43 || length > 128 {
		return errors.New("mcp auth: PKCE verifier length must be between 43 and 128 characters")
	}
	for _, r := range verifier {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '.' || r == '_' || r == '~':
		default:
			return errors.New("mcp auth: PKCE verifier contains invalid characters")
		}
	}
	return nil
}
