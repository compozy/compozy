package config

import (
	"fmt"
	"strings"
)

// ValidateConfigWriteScope rejects paths that the selected runtime scope cannot consume.
func ValidateConfigWriteScope(scope WriteScope, path []string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	clean, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}
	if scope == WriteScopeWorkspace && clean[0] == toolSurfaceMarketplaceKey {
		return fmt.Errorf(
			"config: path %q is global-only and cannot be written at workspace scope",
			strings.Join(clean, "."),
		)
	}
	return nil
}
