package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
)

const computerUninstallFailed = "failed"

func newComputerCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "computer", Short: "Manage computers used by this controller"}
	add := &cobra.Command{Use: "add", Short: "Add a computer to this controller"}
	add.AddCommand(&cobra.Command{
		Use: "account <id>", Args: cobra.ExactArgs(1), Short: "Add a computer already linked to your account",
		RunE: func(command *cobra.Command, args []string) error {
			client, err := computerClientFromDeps(deps)
			if err != nil {
				return err
			}
			item, err := client.AddAccountComputer(command.Context(), remoteproto.ComputerID(args[0]))
			if err != nil {
				return err
			}
			return writeComputerStatus(command, item)
		},
	})
	add.AddCommand(newComputerAddSSHCommand(deps))
	cmd.AddCommand(
		add, newComputerListCommand(deps), newAvailableComputersCommand(deps),
		newComputerStatusCommand(deps), newComputerToggleCommand(deps, "enable", true),
		newComputerToggleCommand(deps, "disable", false), newComputerRemoveCommand(deps),
		newComputerRoutesCommand(deps), newComputerUpdateCommand(deps), newComputerSetupCommand(deps),
	)
	return cmd
}

func newComputerListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{Use: cliListVerb, Short: "List computers added to this controller", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := computerClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.ListComputers(cmd.Context())
			if err != nil {
				return err
			}
			rows := result.Computers
			return writeCommandOutput(cmd, listBundle(
				rows, rows, "Computers", []string{cliNameHeader, "ID", cliStatusHeader, "ROUTE", "PLAN"}, "computers",
				[]string{automationNameKey, "id", automationStatusKey, "route", "plan_state"}, computerRow, computerRow,
			))
		},
	}
}

func newAvailableComputersCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{Use: "available", Short: "List account computers available to add", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := computerClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.AvailableComputers(cmd.Context())
			if err != nil {
				return err
			}
			row := func(item contract.AvailableComputer) []string {
				status := "offline"
				if item.Online {
					status = "online"
				}
				return []string{item.Name, string(item.ID), status}
			}
			return writeCommandOutput(cmd, listBundle(
				result.Computers,
				result.Computers,
				"Available computers",
				[]string{cliNameHeader, "ID", cliStatusHeader},
				"computers",
				[]string{automationNameKey, "id", automationStatusKey},
				row,
				row,
			))
		},
	}
}

func computerRow(item contract.Computer) []string {
	route := "—"
	if item.RouteInUse != nil {
		route = string(item.RouteInUse.Kind)
	}
	return []string{item.Name, string(item.ID), item.Status, route, item.PlanState}
}

func newComputerStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status <name|id>",
		Short: "Show a computer's connection and routes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, item, err := computerCommandTarget(cmd, deps, args[0])
			if err != nil {
				return err
			}
			result, err := client.GetComputer(cmd.Context(), item.ID)
			if err != nil {
				return err
			}
			return writeComputerStatus(cmd, result)
		},
	}
}

func newComputerToggleCommand(deps commandDeps, name string, enabled bool) *cobra.Command {
	return &cobra.Command{Use: name + " <name|id>", Short: "Change whether this controller connects to a computer",
		Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			client, item, err := computerCommandTarget(cmd, deps, args[0])
			if err != nil {
				return err
			}
			result, err := client.PatchComputer(
				cmd.Context(),
				item.ID,
				contract.PatchComputerRequest{Enabled: &enabled},
			)
			if err != nil {
				return err
			}
			return writeComputerStatus(cmd, result)
		},
	}
}

func newComputerRemoveCommand(deps commandDeps) *cobra.Command {
	var unlink, uninstall bool
	cmd := &cobra.Command{
		Use:   "remove <name|id>",
		Short: "Remove a computer from this controller",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, item, err := computerCommandTarget(cmd, deps, args[0])
			if err != nil {
				return err
			}
			result, err := client.RemoveComputer(cmd.Context(), item.ID, unlink, uninstall)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
				message := fmt.Sprintf("Removed %s. Agents already running there keep running.", item.Name)
				if result.Uninstall == computerUninstallFailed {
					message += fmt.Sprintf(
						"\nCouldn't uninstall from %s; run compozy service uninstall there.",
						item.Name,
					)
				}
				return message, nil
			}})
		},
	}
	cmd.Flags().BoolVar(&unlink, "unlink", false, "Also unlink the computer from the account")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Also uninstall the managed SSH service")
	return cmd
}

func newComputerRoutesCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "routes <name|id> [remove <kind:address>]", Short: "List or remove a computer's learned routes",
		Args: cobra.RangeArgs(1, 3), RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 && (len(args) != 3 || args[1] != "remove") {
				return &cliUsageError{message: "Use computer routes <name|id> [remove <kind:address>]."}
			}
			client, item, err := computerCommandTarget(cmd, deps, args[0])
			if err != nil {
				return err
			}
			if len(args) == 3 {
				return removeComputerRoute(cmd, client, item, args[2])
			}
			row := func(route contract.ComputerRoute) []string {
				return []string{string(route.Kind), route.Address, fmt.Sprint(route.InUse)}
			}
			return writeCommandOutput(cmd, listBundle(
				item.Routes, item.Routes, "Routes", []string{"KIND", "ADDRESS", "IN USE"},
				"routes", []string{cliKindKey, "address", "in_use"}, row, row,
			))
		},
	}
}

func removeComputerRoute(
	cmd *cobra.Command,
	client computerCommandClient,
	item contract.Computer,
	selection string,
) error {
	kind, address, ok := strings.Cut(selection, ":")
	index := slices.IndexFunc(item.Routes, func(route contract.ComputerRoute) bool {
		return route.ID == selection || (ok && string(route.Kind) == kind && route.Address == address)
	})
	if index < 0 {
		return &cliUsageError{message: "No route matches that ID or kind and address."}
	}
	if err := client.RemoveComputerRoute(cmd.Context(), item.ID, item.Routes[index].ID); err != nil {
		return err
	}
	return writeCommandOutput(cmd, outputBundle{
		jsonValue: map[string]bool{"removed": true}, human: func() (string, error) { return "Removed route.", nil },
	})
}

func computerCommandTarget(
	cmd *cobra.Command, deps commandDeps, selection string,
) (computerCommandClient, contract.Computer, error) {
	client, err := computerClientFromDeps(deps)
	if err != nil {
		return nil, contract.Computer{}, err
	}
	item, err := resolveComputer(cmd.Context(), client, selection)
	return client, item, err
}

func writeComputerStatus(cmd *cobra.Command, item contract.Computer) error {
	return writeCommandOutput(cmd, outputBundle{jsonValue: item, human: func() (string, error) {
		row := computerRow(item)
		text := fmt.Sprintf("%s (%s)  %s  %s", row[0], row[1], row[2], row[3])
		if item.Diagnosis != nil {
			text += "\n" + item.Diagnosis.Message
		}
		return text, nil
	}})
}
