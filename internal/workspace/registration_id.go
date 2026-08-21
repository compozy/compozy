package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var registrationIDPattern = regexp.MustCompile(`^ws_[0-9a-f]{16}$`)

// IsRegistrationID reports whether value is a public workspace registration id.
func IsRegistrationID(value string) bool {
	return registrationIDPattern.MatchString(strings.TrimSpace(value))
}

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
