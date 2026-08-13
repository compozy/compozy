package workspace

import (
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func validateOptionalDefaultAgent(defaultAgent string) error {
	if strings.TrimSpace(defaultAgent) == "" {
		return nil
	}
	if err := compozyconfig.ValidateAgentName(defaultAgent); err != nil {
		return fmt.Errorf(
			"%w: %w",
			ErrWorkspaceValidation,
			compozyconfig.ValidationError{Path: "workspace.default_agent", Message: err.Error()},
		)
	}
	return nil
}
