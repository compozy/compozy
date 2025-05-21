package common

import (
	"fmt"

	"github.com/segmentio/ksuid"
)

type ID string

func (c ID) String() string {
	return string(c)
}

func NewID() (ID, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate new ID: %w", err)
	}
	return ID(id.String()), nil
}
