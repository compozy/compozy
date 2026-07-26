package cli

import (
	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"
)

func prepareConfigMutationTarget(target aghconfig.WriteTarget, path []string) error {
	if err := aghconfig.ValidateConfigWriteScope(target.Scope(), path); err != nil {
		return fmt.Errorf("cli: %w", err)
	}
	return ensureWriteTargetParent(target)
}
