package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// NewID returns a random identifier with an optional prefix.
func NewID(prefix string) (string, error) {
	return newID(prefix, rand.Reader)
}

func newID(prefix string, entropy io.Reader) (string, error) {
	if entropy == nil {
		return "", errors.New("store: ID entropy reader is required")
	}
	var random [8]byte
	if _, err := io.ReadFull(entropy, random[:]); err != nil {
		return "", fmt.Errorf("store: generate ID entropy: %w", err)
	}

	if strings.TrimSpace(prefix) == "" {
		return hex.EncodeToString(random[:]), nil
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(random[:])), nil
}
