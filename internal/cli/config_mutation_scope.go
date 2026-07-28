package cli

import (
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func prepareConfigMutationTarget(target compozyconfig.WriteTarget, path []string) error {
	if err := compozyconfig.ValidateConfigWriteScope(target.Scope(), path); err != nil {
		return fmt.Errorf("cli: %w", err)
	}
	return ensureWriteTargetParent(target)
}
