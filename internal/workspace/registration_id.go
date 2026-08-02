package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

func generateID(prefix string) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("workspace: read random id: %w", err)
	}

	encoded := hex.EncodeToString(random[:])
	if strings.TrimSpace(prefix) == "" {
		return encoded, nil
	}
	return prefix + "_" + encoded, nil
}

func (r *Resolver) nextRegistrationID() (string, error) {
	id, err := r.idGenerator("ws")
	if err != nil {
		return "", fmt.Errorf("workspace: generate registration id: %w", err)
	}
	return id, nil
}
