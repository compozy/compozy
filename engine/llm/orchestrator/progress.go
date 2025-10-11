package orchestrator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
)

func buildIterationFingerprint(calls []llmadapter.ToolCall, results []llmadapter.ToolResult) string {
	var b bytes.Buffer
	for _, call := range calls {
		b.WriteString(call.Name)
		b.WriteByte('|')
		if len(call.Arguments) > 0 {
			b.WriteString(stableJSONFingerprint(call.Arguments))
		}
		b.WriteByte(';')
	}
	for _, result := range results {
		b.WriteString(result.Name)
		b.WriteByte('|')
		if len(result.JSONContent) > 0 {
			b.WriteString(stableJSONFingerprint(result.JSONContent))
		} else {
			normalised := normalizeFingerprintText(result.Content)
			b.WriteString(stableJSONFingerprint([]byte(normalised)))
		}
		b.WriteByte(';')
	}
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:])
}

// normalizeFingerprintText collapses consecutive whitespace into single spaces
// and trims leading/trailing whitespace for stable fingerprinting.
func normalizeFingerprintText(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
