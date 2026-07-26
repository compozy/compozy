package cli

import (
	"errors"

	"strings"

	"github.com/spf13/cobra"
)

func newMemoryProviderCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cliProviderKey,
		Short: "Operate Memory v2 providers",
	}
	cmd.AddCommand(newMemoryProviderListCommand(deps))
	cmd.AddCommand(newMemoryProviderEnableCommand(deps))
	cmd.AddCommand(newMemoryProviderDisableCommand(deps))
	return cmd
}

func newMemoryProviderListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   memoryListKey,
		Short: "List registered Memory v2 providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ListMemoryProviders(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryProviderListBundle(response))
		},
	}
}

func newMemoryProviderEnableCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   cliUseEnableName,
		Short: "Enable and select one Memory v2 provider",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			response, err := client.EnableMemoryProvider(
				cmd.Context(),
				name,
				MemoryProviderLifecycleRequest{},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Provider", response))
		},
	}
}

func newMemoryProviderDisableCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   cliUseDisableName,
		Short: "Disable one Memory v2 provider",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			response, err := client.DisableMemoryProvider(
				cmd.Context(),
				name,
				MemoryProviderLifecycleRequest{},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Provider", response))
		},
	}
}

func newMemoryAdhocCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   memoryAdhocKey,
		Short: "Inspect ad-hoc Memory v2 notes",
	}
	cmd.AddCommand(newMemoryAdhocListCommand())
	cmd.AddCommand(
		newUnsupportedMemoryCommand(
			"show <slug>",
			"memory.unsupported: adhoc show is not registered in Slice 1 API",
			exactOneNonBlankArg(),
		),
	)
	return cmd
}

func newMemoryAdhocListCommand() *cobra.Command {
	var flags memorySelectorFlags
	cmd := newUnsupportedMemoryCommand(
		memoryListKey,
		"memory.unsupported: adhoc list is not registered in Slice 1 API",
		cobra.NoArgs,
	)
	addMemorySelectorFlags(cmd, &flags)
	return cmd
}

func newMemoryDailyUnsupportedSelectorCommand(
	use string,
	message string,
	args cobra.PositionalArgs,
) *cobra.Command {
	var flags memorySelectorFlags
	cmd := newUnsupportedMemoryCommand(use, message, args)
	addMemorySelectorFlags(cmd, &flags)
	return cmd
}

func newMemoryDailyRetentionCommand(use string, message string) *cobra.Command {
	var olderThan string
	var dryRun bool
	cmd := newUnsupportedMemoryCommand(use, message, cobra.NoArgs)
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Retention age threshold")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show retention work without applying it")
	return cmd
}

func newUnsupportedMemoryCommand(use string, message string, args cobra.PositionalArgs) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: "Reserved Memory v2 command",
		Args:  args,
		RunE: func(*cobra.Command, []string) error {
			return errors.New(message)
		},
	}
}
