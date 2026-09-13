package cli

import (
	"errors"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newMarketplaceSourcesCommand(deps commandDeps) *cobra.Command {
	command := &cobra.Command{
		Use:   "sources",
		Short: "Manage plugin marketplace sources",
		Long:  "Manage global plugin marketplace sources.\n\nStability: experimental through v0.6.0.",
	}
	command.AddCommand(
		newMarketplaceSourcesListCommand(deps),
		newMarketplaceSourcesAddCommand(deps),
		newMarketplaceSourcesRemoveCommand(deps),
		newMarketplaceSourcesRefreshCommand(deps),
	)
	return command
}

func newMarketplaceSourcesListCommand(deps commandDeps) *cobra.Command {
	list := &cobra.Command{
		Use:   "list",
		Short: "List configured sources and diagnostics",
		Long:  "List configured sources and diagnostics.\n\nStability: experimental through v0.6.0.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := marketplaceSourcesClient(deps)
			if err != nil {
				return err
			}
			response, err := client.ListMarketplaceSources(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceSourcesBundle(response, response.Sources))
		},
	}
	return list
}

func newMarketplaceSourcesAddCommand(deps commandDeps) *cobra.Command {
	var name string
	add := &cobra.Command{
		Use:   "add <ref>",
		Short: "Add a GitHub repository, HTTPS git source, or absolute folder",
		Long:  "Validate and add a plugin marketplace.\n\nStability: experimental through v0.6.0.",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := marketplaceSourcesClient(deps)
			if err != nil {
				return err
			}
			response, err := client.AddMarketplaceSource(
				cmd.Context(),
				contract.AddMarketplaceSourceRequest{Ref: args[0], Name: name},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				marketplaceSourcesBundle(response, []contract.MarketplaceSourcePayload{response.Source}),
			)
		},
	}
	add.Flags().StringVar(&name, "name", "", "Choose a source name")
	return add
}

func newMarketplaceSourcesRemoveCommand(deps commandDeps) *cobra.Command {
	remove := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom source and keep installed extensions",
		Long:  "Remove a custom source and keep installed extensions.\n\nStability: experimental through v0.6.0.",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := marketplaceSourcesClient(deps)
			if err != nil {
				return err
			}
			if err := client.RemoveMarketplaceSource(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceSourcesBundle(map[string]string{"removed": args[0]}, nil))
		},
	}
	return remove
}

func newMarketplaceSourcesRefreshCommand(deps commandDeps) *cobra.Command {
	refresh := &cobra.Command{
		Use:   "refresh <name>",
		Short: "Refresh one configured source",
		Long:  "Refresh one configured source.\n\nStability: experimental through v0.6.0.",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := marketplaceSourcesClient(deps)
			if err != nil {
				return err
			}
			response, err := client.RefreshMarketplaceSource(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := writeCommandOutput(
				cmd,
				marketplaceSourcesBundle(response, []contract.MarketplaceSourcePayload{response.Source}),
			); err != nil {
				return err
			}
			if response.Source.State == "degraded" {
				return withCommandExitCode(1, errors.New("marketplace source could not refresh"))
			}
			return nil
		},
	}
	return refresh
}

func marketplaceSourcesClient(deps commandDeps) (MarketplaceSourcesClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	sources, ok := client.(MarketplaceSourcesClient)
	if !ok {
		return nil, errors.New("cli: daemon client does not support marketplace sources")
	}
	return sources, nil
}

func marketplaceSourcesBundle(value any, sources []contract.MarketplaceSourcePayload) outputBundle {
	row := func(source contract.MarketplaceSourcePayload) []string {
		return []string{source.Name, source.Kind, source.State, strconv.Itoa(source.Plugins), source.Source}
	}
	bundle := listBundle(value, sources, "Marketplace Sources (experimental)",
		[]string{"Name", "Kind", "State", "Plugins", "Source"}, "marketplace_sources",
		[]string{"name", "kind", "state", "plugins", "source"}, row, row)
	bundle.json = func(cmd *cobra.Command) error { return writeJSONWithoutWorkspaceResolution(cmd, value) }
	return bundle
}
