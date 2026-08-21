package windowmanager

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const attachmentTokenBytes = 32

func newAttachmentToken() (string, [32]byte, error) {
	raw := make([]byte, attachmentTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", [32]byte{}, fmt.Errorf("read secure randomness: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, sha256.Sum256([]byte(token)), nil
}

// AuthorizeClient validates one self-originated attachment token against its registered client.
func (m *Manager) AuthorizeClient(
	ctx context.Context,
	workspaceID WorkspaceID,
	clientID ClientID,
	token string,
) error {
	if ctx == nil {
		return errors.New("window manager client authorization context is required")
	}
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(token)
	if clientID == "" || trimmed == "" {
		return ErrClientUnauthorized
	}
	candidate := sha256.Sum256([]byte(trimmed))
	m.mu.Lock()
	wanted, exists := m.clientTokens[workspaceID][clientID]
	m.mu.Unlock()
	if !exists || subtle.ConstantTimeCompare(candidate[:], wanted[:]) != 1 {
		return ErrClientUnauthorized
	}
	return nil
}
