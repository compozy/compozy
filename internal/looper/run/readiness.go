package run

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

var (
	readinessPollInterval = 200 * time.Millisecond
	readinessTimeout      = 15 * time.Second
)

// waitForReady waits for the Claude composer to appear, but falls back after
// the timeout so prompt delivery can continue.
func waitForReady(ctx context.Context, emu *vt.SafeEmulator) error {
	if emu == nil {
		return nil
	}

	ticker := time.NewTicker(readinessPollInterval)
	defer ticker.Stop()

	timeout := time.NewTimer(readinessTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			if detectComposerReady(screenSnapshot(emu)) {
				return nil
			}
		case <-timeout.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func screenSnapshot(emu *vt.SafeEmulator) string {
	if emu == nil {
		return ""
	}
	return ansi.Strip(emu.Render())
}

func detectComposerReady(screen string) bool {
	lines := strings.Split(screen, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			continue
		}

		return strings.HasPrefix(trimmed, ">") ||
			strings.Contains(trimmed, "What can I help") ||
			strings.Contains(trimmed, "Type your")
	}

	return false
}
