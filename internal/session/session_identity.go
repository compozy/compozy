package session

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid"
)

func newULIDGenerator(prefix string) IDGenerator {
	return func() (string, error) {
		id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), rand.Reader)
		if err != nil {
			return "", fmt.Errorf("session: generate ULID: %w", err)
		}
		trimmedPrefix := strings.TrimSpace(prefix)
		if trimmedPrefix == "" {
			return id.String(), nil
		}
		return trimmedPrefix + "_" + id.String(), nil
	}
}
