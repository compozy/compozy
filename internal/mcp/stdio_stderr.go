package mcp

import (
	"strings"
	"sync"
)

const (
	defaultStdioStderrBufferBytes = 16 << 10
	stdioStderrTailBytes          = 2 << 10
	stdioStderrRedactedValue      = "[REDACTED]"
)

// stdioStderrRing keeps the bounded, most recent stderr bytes of one stdio MCP child.
// Secrets resolved for the launch are replaced on read, so the raw buffer never leaves the type.
type stdioStderrRing struct {
	mu      sync.Mutex
	limit   int
	buf     []byte
	secrets []string
}

func newStdioStderrRing(limit int, secrets []string) *stdioStderrRing {
	if limit <= 0 {
		limit = defaultStdioStderrBufferBytes
	}
	kept := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if strings.TrimSpace(secret) != "" {
			kept = append(kept, secret)
		}
	}
	return &stdioStderrRing{limit: limit, secrets: kept}
}

// Write never fails so a chatty child can't break its own stdio transport.
func (r *stdioStderrRing) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(p) >= r.limit {
		r.buf = append(r.buf[:0], p[len(p)-r.limit:]...)
		return len(p), nil
	}
	if overflow := len(r.buf) + len(p) - r.limit; overflow > 0 {
		r.buf = append(r.buf[:0], r.buf[overflow:]...)
	}
	r.buf = append(r.buf, p...)
	return len(p), nil
}

// Tail returns the redacted trailing stderr lines, bounded for diagnostics.
func (r *stdioStderrRing) Tail() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	text := string(r.buf)
	secrets := r.secrets
	r.mu.Unlock()
	for _, secret := range secrets {
		text = strings.ReplaceAll(text, secret, stdioStderrRedactedValue)
	}
	text = strings.TrimSpace(strings.ToValidUTF8(text, ""))
	if len(text) > stdioStderrTailBytes {
		text = text[len(text)-stdioStderrTailBytes:]
		if newline := strings.IndexByte(text, '\n'); newline >= 0 && newline < len(text)-1 {
			text = text[newline+1:]
		}
		text = strings.ToValidUTF8(text, "")
	}
	return redactMCPDiagnosticString(text)
}
