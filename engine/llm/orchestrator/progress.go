package orchestrator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

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
			b.WriteString(stableJSONFingerprint([]byte(result.Content)))
		}
		b.WriteByte(';')
	}
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:])
}

func (s *loopState) detectNoProgress(threshold int, fingerprint string) bool {
	if fingerprint == s.lastFingerprint {
		s.noProgressCount++
		return s.noProgressCount >= threshold
	}
	s.noProgressCount = 0
	s.lastFingerprint = fingerprint
	return false
}
