package run

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/xpty"
)

const (
	composerLineBreak = byte(0x0a)
	composerSubmit    = byte(0x0d)
)

// sendComposerInput injects a short composer message into the PTY.
// Callers should pass the prompt-file reference, not full task contents.
func sendComposerInput(pty xpty.Pty, message string) error {
	if pty == nil {
		return errors.New("pty is nil")
	}

	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(message)
	lines := strings.Split(normalized, "\n")

	for index, line := range lines {
		if line != "" {
			if err := writeComposerChunk(pty, []byte(line)); err != nil {
				return fmt.Errorf("write composer text: %w", err)
			}
		}

		if index < len(lines)-1 {
			if err := writeComposerChunk(pty, []byte{composerLineBreak}); err != nil {
				return fmt.Errorf("write composer newline: %w", err)
			}
		}
	}

	if err := writeComposerChunk(pty, []byte{composerSubmit}); err != nil {
		return fmt.Errorf("submit composer input: %w", err)
	}

	return nil
}

func writeComposerChunk(pty xpty.Pty, chunk []byte) error {
	written, err := pty.Write(chunk)
	if err != nil {
		return err
	}
	if written != len(chunk) {
		return io.ErrShortWrite
	}
	return nil
}
