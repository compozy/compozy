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
	var layoutErr *registrypkg.ClientSpecificPluginLayoutError
	if errors.As(err, &layoutErr) {
		return &AgentPluginClientLayoutError{
			Root:   strings.TrimSpace(slug),
			Layout: strings.TrimSpace(layoutErr.Layout),
		}
	}
	return err
}
