package core

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network"
)

func validateParticipationChannel(path string, channel string) error {
	trimmed := strings.TrimSpace(channel)
	if trimmed == "" {
		return nil
	}
	if err := network.ValidateChannel(trimmed); err != nil {
		return NewTaskValidationError(fmt.Errorf("%s: %w", path, err))
	}
	return nil
}
