package terminal

import (
	"encoding/hex"
	"fmt"
	"io"
)

func newTerminalID(entropy io.Reader) (ID, error) {
	raw := make([]byte, 6)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("terminal: generate id: %w", err)
	}
	return ID("term-" + hex.EncodeToString(raw)), nil
}

func newMarkerNonce(entropy io.Reader) (string, error) {
	raw := make([]byte, 16)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("terminal: generate marker nonce: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
