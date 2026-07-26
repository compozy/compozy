package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	// LoopOutputInlineLimitBytes is the largest loop node result kept inline in task_runs.
	LoopOutputInlineLimitBytes = 16 * 1024
	outputRefSHA256Prefix      = "sha256:"
)

// OutputPayloadRequiresRef reports whether a loop node result must be externalized.
func OutputPayloadRequiresRef(payload json.RawMessage) bool {
	return len(payload) > LoopOutputInlineLimitBytes
}

// OutputRefForPayload returns the content-addressed ref for a persisted loop output payload.
func OutputRefForPayload(payload json.RawMessage) string {
	sum := sha256.Sum256(payload)
	return outputRefSHA256Prefix + hex.EncodeToString(sum[:])
}

// OutputRefLooksContentAddressed reports whether a ref uses AGH's loop output hash form.
func OutputRefLooksContentAddressed(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmed, outputRefSHA256Prefix) {
		return false
	}
	digest := strings.TrimPrefix(trimmed, outputRefSHA256Prefix)
	if len(digest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
