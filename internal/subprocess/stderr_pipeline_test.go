package subprocess

import (
	"bytes"
	"strings"
	"testing"
)

func TestStderrPipeline(t *testing.T) {
	t.Parallel()

	t.Run("Should discard an oversized record without exposing a secret across the size boundary", func(t *testing.T) {
		t.Parallel()

		const secret = "boundary-secret-material"
		var capture bytes.Buffer
		var sink bytes.Buffer
		pipeline := newStderrPipeline(&capture, &sink, func(value string) string {
			return strings.ReplaceAll(value, secret, "[REDACTED]")
		})

		secretPrefix := secret[:8]
		first := strings.Repeat("x", maxPendingStderrBytes-len(secretPrefix)) + secretPrefix
		if _, err := pipeline.Write([]byte(first)); err != nil {
			t.Fatalf("Write(first) error = %v", err)
		}
		second := secret[len(secretPrefix):] + "\nnext=" + secret + "\n"
		if _, err := pipeline.Write([]byte(second)); err != nil {
			t.Fatalf("Write(second) error = %v", err)
		}
		if err := pipeline.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		for surface, got := range map[string]string{
			"capture": capture.String(),
			"sink":    sink.String(),
		} {
			if strings.Contains(got, secret) {
				t.Fatalf("%s leaked %q: %q", surface, secret, got)
			}
			want := oversizedStderrRecord + "next=[REDACTED]\n"
			if got != want {
				t.Fatalf("%s = %q, want %q", surface, got, want)
			}
		}
	})
}
