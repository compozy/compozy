package subagentid

import (
	"crypto/sha256"
	"encoding/hex"
)

// Derive returns the shared identity for a parent's delegation or provider tool call.
func Derive(parentSessionID, key string) string {
	sum := sha256.Sum256([]byte(parentSessionID + key))
	return "sub-" + hex.EncodeToString(sum[:8])
}
