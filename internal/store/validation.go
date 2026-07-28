package store

import (
	"fmt"
	"strings"
)

func requireField(value string, label string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("store: %s is required", label)
	}
	return nil
}

func requirePositiveLimit(limit int, label string) error {
	if limit < 0 {
		return fmt.Errorf("store: invalid %s %d", label, limit)
	}
	return nil
}
