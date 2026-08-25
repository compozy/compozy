package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const outputRefPrefix = "sha256:"

// OutputRefForPayload returns the content-addressed ref for a persisted output.
func OutputRefForPayload(payload json.RawMessage) string {
	sum := sha256.Sum256(payload)
	return outputRefPrefix + hex.EncodeToString(sum[:])
}

// OutputRefLooksContentAddressed reports whether a ref has the canonical digest form.
func OutputRefLooksContentAddressed(ref string) bool {
	digest := strings.TrimPrefix(strings.TrimSpace(ref), outputRefPrefix)
	if digest == ref || len(digest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
