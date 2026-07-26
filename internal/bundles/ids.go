package bundles

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func stableID(prefix string, parts ...string) string {
	trimmedParts := make([]string, 0, len(parts))
	size := 0
	for idx, part := range parts {
		trimmed := strings.TrimSpace(part)
		trimmedParts = append(trimmedParts, trimmed)
		if idx > 0 {
			size++
		}
		size += len(trimmed)
	}
	payload := make([]byte, 0, size)
	for idx, part := range trimmedParts {
		if idx > 0 {
			payload = append(payload, '\n')
		}
		payload = append(payload, part...)
	}
	sum := sha256.Sum256(payload)
	var encoded [16]byte
	hex.Encode(encoded[:], sum[:8])
	return prefix + "_" + string(encoded[:])
}
