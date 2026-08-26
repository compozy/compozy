package extensionpkg

import (
	"errors"
	"strings"

	registrypkg "github.com/compozy/compozy/internal/registry"
)

func mapMarketplaceRegistryError(slug string, err error) error {
	if err == nil {
		return nil
	}
	if layoutErr, ok := errors.AsType[*registrypkg.ClientSpecificPluginLayoutError](err); ok {
		return &AgentPluginClientLayoutError{
			Root:   strings.TrimSpace(slug),
			Layout: strings.TrimSpace(layoutErr.Layout),
		}
	}
	return err
}
