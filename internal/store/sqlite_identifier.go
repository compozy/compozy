package store

import (
	"errors"
	"fmt"
	"strings"
)

// NormalizeSQLiteIdentifier validates a SQLite identifier for use in helper queries.
func NormalizeSQLiteIdentifier(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.New("store: sqlite identifier is required")
	}

	for idx, r := range name {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case idx > 0 && r >= '0' && r <= '9':
		default:
			return "", fmt.Errorf("store: invalid sqlite identifier %q", value)
		}
	}

	return name, nil
}
