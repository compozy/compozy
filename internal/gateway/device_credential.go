package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// #nosec G101 -- this is a public protocol prefix, not credential material.
	deviceCredentialPrefix = "cpz_gwd_"
	pairingArtifactPrefix  = "cpz_gwp_"
	streamTicketPrefix     = "cpz_gwt_"
	secretEntropyBytes     = 32
)

var dummyCredentialHash = sha256.Sum256([]byte("compozy gateway unknown credential"))

func generateDeviceCredential(entropy io.Reader) (string, error) {
	return generateOpaqueSecret(entropy, deviceCredentialPrefix)
}

func generatePairingArtifact(entropy io.Reader) (string, error) {
	return generateOpaqueSecret(entropy, pairingArtifactPrefix)
}

func generateStreamTicket(entropy io.Reader) (string, error) {
	return generateOpaqueSecret(entropy, streamTicketPrefix)
}

func generateOpaqueSecret(entropy io.Reader, prefix string) (string, error) {
	if entropy == nil {
		return "", errors.New("gateway: secret entropy reader is required")
	}
	secret := make([]byte, secretEntropyBytes)
	if _, err := io.ReadFull(entropy, secret); err != nil {
		return "", fmt.Errorf("gateway: generate secret entropy: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(secret), nil
}

func hashCredential(credential string) string {
	digest := sha256.Sum256([]byte(credential))
	return hex.EncodeToString(digest[:])
}

func validOpaqueSecret(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil && len(decoded) == secretEntropyBytes
}

func constantTimeCredentialMatch(credential, storedHash string) bool {
	computed := sha256.Sum256([]byte(credential))
	expected := dummyCredentialHash[:]
	if decoded, err := hex.DecodeString(storedHash); err == nil && len(decoded) == sha256.Size {
		expected = decoded
	}
	return subtle.ConstantTimeCompare(computed[:], expected) == 1
}

func defaultEntropy() io.Reader {
	return rand.Reader
}
