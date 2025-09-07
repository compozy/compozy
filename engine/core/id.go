package core

import (
	"fmt"

	"github.com/segmentio/ksuid"
)

type ID string

// String returns the string representation of the ID.
func (id ID) String() string {
	return string(id)
}

// IsZero reports whether the ID is the zero value ("")
func (id ID) IsZero() bool {
	return id == ""
}

func NewID() (ID, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate new ID: %w", err)
	}
	return ID(id.String()), nil
}

func MustNewID() ID {
	id, err := NewID()
	if err != nil {
		panic(err)
	}
	return id
}

func ParseID(s string) (ID, error) {
	if s == "" {
		return "", fmt.Errorf("empty ID")
	}
	// Validate it's a valid KSUID
	if _, err := ksuid.Parse(s); err != nil {
		return "", fmt.Errorf("invalid ID format: %w", err)
	}
	return ID(s), nil
}
