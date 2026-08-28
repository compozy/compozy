// Package idgen creates permanent terminal identifiers from cryptographic entropy.
package idgen

import (
	"encoding/hex"
	"fmt"
	"io"
)

const entropyBytes = 16

// New returns a prefixed permanent identifier with 128 bits of caller-supplied entropy.
func New(entropy io.Reader, prefix string) (string, error) {
	if entropy == nil {
		return "", fmt.Errorf("terminal id: entropy reader is required")
	}
	raw := make([]byte, entropyBytes)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("terminal id: read entropy: %w", err)
	}
	return prefix + hex.EncodeToString(raw), nil
}
