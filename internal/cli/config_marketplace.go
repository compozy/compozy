package cli

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

const (
	marketplaceCatalogBaseURLPath = "marketplace.catalog.base_url"
	marketplaceCatalogTTLPath     = "marketplace.catalog.ttl"
	marketplaceCatalogTimeoutPath = "marketplace.catalog.timeout"
)

func marketplaceConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		marketplaceCatalogBaseURLPath: configSetString,
		marketplaceCatalogTTLPath:     configSetDuration,
		marketplaceCatalogTimeoutPath: configSetDuration,
	}
}

func isMarketplaceSourceEnabledPath(path []string) bool {
	return len(path) == 4 && path[0] == "marketplace" && path[1] == "plugin_sources" && path[3] == configEnabledKey
}

func runMarketplaceSourceConfigSet(
	cmd *cobra.Command, deps commandDeps, target compozyconfig.WriteTarget, path []string, value any,
) error {
	if target.Scope() != compozyconfig.WriteScopeUser {
		return fmt.Errorf("cli: marketplace sources are global; use --scope user")
	}
	enabled, ok := value.(bool)
	if !ok {
		return fmt.Errorf("cli: marketplace source enabled must be a boolean")
	}
	client, err := marketplaceSourcesClient(deps)
	if err != nil {
		return err
	}
	response, err := client.UpdateMarketplaceSource(
		cmd.Context(),
		path[2],
		contract.UpdateMarketplaceSourceRequest{Enabled: &enabled},
	)
	if err != nil {
		return fmt.Errorf("cli: set marketplace source enabled: %w", err)
	}
	record := configSetRecordForLocalWrite(
		path,
		response.Source.Enabled,
		target,
		false,
		classifyConfigSetLifecycle(path),
	)
	record.Applied = true
	return writeCommandOutput(cmd, configSetBundle(record))
}
