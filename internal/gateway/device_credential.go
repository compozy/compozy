package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
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

	pairingManualCodeDigits  = 6
	pairingManualCodeModulus = 1_000_000
	pairingManualCodeDraws   = 32
)

// pairingManualCodeSeparators accepts hand-typed grouping without weakening the code space.
var pairingManualCodeSeparators = strings.NewReplacer(
	" ", "", "\t", "", "\r", "", "\n", "",
	"-", "", "\u2010", "", "\u2011", "", "\u2012", "", "\u2013", "",
	"\u2014", "", "\u2015", "", "\u2212", "",
)

var dummyCredentialHash = sha256.Sum256([]byte("compozy gateway unknown credential"))

func generateDeviceCredential(entropy io.Reader) (string, error) {
	return generateOpaqueSecret(entropy, deviceCredentialPrefix)
}

// GenerateDeviceCredential creates a client-owned gateway credential.
func GenerateDeviceCredential() (string, error) {
	return generateDeviceCredential(rand.Reader)
}

func generatePairingArtifact(entropy io.Reader) (string, error) {
	return generateOpaqueSecret(entropy, pairingArtifactPrefix)
}

// generatePairingManualCode returns a uniformly random 6-digit code from rejection sampling.
func generatePairingManualCode(entropy io.Reader) (string, error) {
	if entropy == nil {
		return "", errors.New("gateway: secret entropy reader is required")
	}
	// Reject the short tail so every code in 0..999999 is equally likely.
	limit := uint64(1<<32) - (uint64(1<<32) % pairingManualCodeModulus)
	draw := make([]byte, 4)
	for range pairingManualCodeDraws {
		if _, err := io.ReadFull(entropy, draw); err != nil {
			return "", fmt.Errorf("gateway: generate pairing manual code: %w", err)
		}
		value := uint64(binary.BigEndian.Uint32(draw))
		if value >= limit {
			continue
		}
		return fmt.Sprintf("%0*d", pairingManualCodeDigits, value%pairingManualCodeModulus), nil
	}
	return "", errors.New("gateway: pairing manual code entropy rejected every draw")
}

// NormalizePairingManualCode strips the grouping separators people add when retyping a code.
func NormalizePairingManualCode(value string) string {
	return pairingManualCodeSeparators.Replace(strings.TrimSpace(value))
}

// IsPairingManualCode reports whether value is a 6-digit pairing code after normalization.
func IsPairingManualCode(value string) bool {
	return validPairingManualCode(NormalizePairingManualCode(value))
}

func validPairingManualCode(value string) bool {
	if len(value) != pairingManualCodeDigits {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
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

// IsDeviceCredential reports whether a value has the canonical gateway device credential shape.
func IsDeviceCredential(value string) bool {
	return validOpaqueSecret(strings.TrimSpace(value), deviceCredentialPrefix)
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
