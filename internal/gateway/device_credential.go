package gateway

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/compozy/internal/remotehost"
)

const (
	// #nosec G101 -- this is a public protocol prefix, not credential material.
	deviceCredentialPrefix = "cpz_gwd_"
	pairingArtifactPrefix  = "cpz_gwp_"

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

func generateDeviceCredential(entropy io.Reader) (string, error) {
	return remotehost.GenerateOpaqueSecret(entropy, deviceCredentialPrefix)
}

// GenerateDeviceCredential creates a client-owned gateway credential.
func GenerateDeviceCredential() (string, error) {
	return generateDeviceCredential(rand.Reader)
}

func generatePairingArtifact(entropy io.Reader) (string, error) {
	return remotehost.GenerateOpaqueSecret(entropy, pairingArtifactPrefix)
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

// IsDeviceCredential reports whether a value has the canonical gateway device credential shape.
func IsDeviceCredential(value string) bool {
	return remotehost.ValidOpaqueSecret(strings.TrimSpace(value), deviceCredentialPrefix)
}

func defaultEntropy() io.Reader {
	return rand.Reader
}
