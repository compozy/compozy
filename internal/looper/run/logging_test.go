package run

import (
	"bytes"
	"strings"
	"testing"
)

func TestShouldSuppressCodexRolloutStderrLine(t *testing.T) {
	t.Parallel()

	noiseLine := "\x1b[2m2026-02-11T22:55:19.818397Z\x1b[0m \x1b[31mERROR\x1b[0m " +
		"\x1b[2mcodex_core::rollout::list\x1b[0m\x1b[2m:\x1b[0m state db missing rollout path " +
		"for thread 019c4084-4858-7df3-84e1-b0873437aa64"
	if !shouldSuppressCodexRolloutStderrLine(noiseLine) {
		t.Fatalf("expected known rollout noise line to be suppressed")
	}

	realError := "2026-02-11T22:55:19.818397Z ERROR codex_core::network: request failed: EOF"
	if shouldSuppressCodexRolloutStderrLine(realError) {
		t.Fatalf("expected real codex error to be kept")
	}
}

func TestLineFilterWriterSuppressesOnlyKnownCodexNoise(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	writer := newLineFilterWriter(&out, nil, shouldSuppressCodexRolloutStderrLine)

	chunk1 := "2026-02-11T22:55:19.818397Z ERROR codex_core::rollout::list: state db missing rollout "
	chunk2 := "path for thread 019c4084-4858-7df3-84e1-b0873437aa64\nREAL ERROR: failed to open file\n"
	if _, err := writer.Write([]byte(chunk1)); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if _, err := writer.Write([]byte(chunk2)); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	got := out.String()
	if strings.Contains(got, "state db missing rollout path for thread") {
		t.Fatalf("expected rollout noise to be filtered, got %q", got)
	}
	if !strings.Contains(got, "REAL ERROR: failed to open file") {
		t.Fatalf("expected real stderr line to remain, got %q", got)
	}
}
