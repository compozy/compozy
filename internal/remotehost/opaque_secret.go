package remotehost

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const secretEntropyBytes = 32

var dummyCredentialHash = sha256.Sum256([]byte("compozy unknown credential"))

func GenerateOpaqueSecret(entropy io.Reader, prefix string) (string, error) {
	if entropy == nil {
		return "", errors.New("remotehost: secret entropy reader is required")
	}
	secret := make([]byte, secretEntropyBytes)
	if _, err := io.ReadFull(entropy, secret); err != nil {
		return "", fmt.Errorf("remotehost: generate secret entropy: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(secret), nil
}

func HashCredential(credential string) string {
	digest := sha256.Sum256([]byte(credential))
	return hex.EncodeToString(digest[:])
}

func ValidOpaqueSecret(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil && len(decoded) == secretEntropyBytes
}

func ConstantTimeCredentialMatch(credential, storedHash string) bool {
	computed := sha256.Sum256([]byte(credential))
	expected := dummyCredentialHash[:]
	if decoded, err := hex.DecodeString(storedHash); err == nil && len(decoded) == sha256.Size {
		expected = decoded
	}
	return subtle.ConstantTimeCompare(computed[:], expected) == 1
}
