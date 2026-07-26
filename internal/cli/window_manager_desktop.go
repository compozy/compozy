package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/spf13/cobra"
)

const desktopCommandKey = "desktop"

func newDesktopCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   desktopCommandKey,
		Short: "Manage virtual desktops in a workspace",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newDesktopListCommand(deps))
	cmd.AddCommand(newDesktopCreateCommand(deps))
	cmd.AddCommand(newDesktopUpdateCommand(deps))
	cmd.AddCommand(newDesktopReorderCommand(deps))
	cmd.AddCommand(newDesktopSwitchCommand(deps))
	cmd.AddCommand(newDesktopDeleteCommand(deps))
	cmd.AddCommand(newDesktopClientsCommand(deps))
	return cmd
}

func newDesktopListCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:     windowManagerListKey,
		Short:   "List persistent virtual desktops",
		Example: "  agh desktop list --workspace ws_1234 -o json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			snapshot, err := client.GetWindowManagerSnapshot(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerDesktopListBundle(snapshot.Desktops))
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	return cmd
}

func newDesktopCreateCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var desktopID, name, purpose, focusOwner, afterID string
	cmd := &cobra.Command{
		Use:   windowManagerCreateKey,
		Short: "Create a persistent virtual desktop",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			focusOwnerID, err := optionalWindowManagerID[windowmanager.WindowID](cmd, "focus-owner", focusOwner)
			if err != nil {
				return err
			}
			afterDesktopID, err := optionalWindowManagerID[windowmanager.DesktopID](cmd, "after", afterID)
			if err != nil {
				return err
			}
			desktopPurpose := windowmanager.DesktopPurpose(strings.TrimSpace(purpose))
			if desktopPurpose != windowmanager.DesktopPurposeStandard &&
				desktopPurpose != windowmanager.DesktopPurposeFocus {
				return fmt.Errorf(
					"cli: --purpose must be %q or %q",
					windowmanager.DesktopPurposeStandard,
					windowmanager.DesktopPurposeFocus,
				)
			}
			if desktopPurpose == windowmanager.DesktopPurposeFocus && focusOwnerID == nil {
				return errors.New("cli: --focus-owner is required when --purpose=focus")
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandDesktopCreate,
				contract.WindowManagerCreateDesktopPayload{
					DesktopID: windowmanager.DesktopID(strings.TrimSpace(desktopID)),
					Name:      strings.TrimSpace(name), Purpose: desktopPurpose,
					FocusOwner: focusOwnerID, AfterID: afterDesktopID,
				},
			)
			if err != nil {
				return err
			}
			result, err := executeWindowManagerCommand(cmd, deps, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&desktopID, "id", "", "Stable desktop ID; generated when omitted")
	cmd.Flags().StringVar(&name, "name", "", "Desktop name")
	cmd.Flags().
		StringVar(&purpose, "purpose", string(windowmanager.DesktopPurposeStandard), "Desktop purpose: standard or focus")
	cmd.Flags().StringVar(&focusOwner, "focus-owner", "", "Window that owns a focus desktop")
	cmd.Flags().StringVar(&afterID, "after", "", "Insert after this desktop ID")
	return cmd
}

func newDesktopUpdateCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var desktopID, name string
	cmd := &cobra.Command{
		Use:   windowManagerUpdateKey,
		Short: "Rename a virtual desktop",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			desktopID, err := requiredWindowManagerFlag(desktopID, "id")
			if err != nil {
				return err
			}
			name, err := requiredWindowManagerFlag(name, "name")
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandDesktopUpdate,
				contract.WindowManagerUpdateDesktopPayload{
					DesktopID: windowmanager.DesktopID(desktopID), Name: name,
				},
			)
			if err != nil {
				return err
			}
			result, err := executeWindowManagerCommand(cmd, deps, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&desktopID, "id", "", "Desktop ID")
	cmd.Flags().StringVar(&name, "name", "", "New desktop name")
	return cmd
}

func newDesktopReorderCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var desktopID string
	var order int
	cmd := &cobra.Command{
		Use:   "reorder",
		Short: "Move a desktop to an ordered position",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			desktopID, err := requiredWindowManagerFlag(desktopID, "id")
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("order") {
				return errors.New("cli: --order is required")
			}
			if order < 0 {
				return errors.New("cli: --order must be nonnegative")
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandDesktopReorder,
				contract.WindowManagerReorderDesktopPayload{
					DesktopID: windowmanager.DesktopID(desktopID), Order: order,
				},
			)
			if err != nil {
				return err
			}
			result, err := executeWindowManagerCommand(cmd, deps, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&desktopID, "id", "", "Desktop ID")
	cmd.Flags().IntVar(&order, "order", 0, "Zero-based destination order")
	return cmd
}

func newDesktopSwitchCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var desktopID string
	cmd := &cobra.Command{
		Use:   windowManagerSwitchKey,
		Short: "Switch one connected client to a desktop",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requiredWindowManagerClient(flags); err != nil {
				return err
			}
			desktopID, err := requiredWindowManagerFlag(desktopID, "id")
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandDesktopSwitch,
				contract.WindowManagerSwitchDesktopPayload{
					DesktopID: windowmanager.DesktopID(desktopID),
				},
			)
			if err != nil {
				return err
			}
			result, err := executeWindowManagerCommand(cmd, deps, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&desktopID, "id", "", "Desktop ID")
	return cmd
}

func newDesktopDeleteCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var desktopID, destination string
	cmd := &cobra.Command{
		Use:   windowManagerDeleteKey,
		Short: "Delete a desktop with an explicit transfer destination when needed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			desktopID, err := requiredWindowManagerFlag(desktopID, "id")
			if err != nil {
				return err
			}
			destinationID, err := optionalWindowManagerID[windowmanager.DesktopID](cmd, "destination", destination)
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandDesktopDelete,
				contract.WindowManagerDeleteDesktopPayload{
					DesktopID: windowmanager.DesktopID(desktopID), DestinationID: destinationID,
				},
			)
			if err != nil {
				return err
			}
			result, err := executeWindowManagerCommand(cmd, deps, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&desktopID, "id", "", "Desktop ID")
	cmd.Flags().StringVar(&destination, "destination", "", "Desktop that receives transferred windows")
	return cmd
}

func newDesktopClientsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "clients", Short: "Manage connected window-manager clients", Args: cobra.NoArgs}
	cmd.AddCommand(newDesktopClientsListCommand(deps))
	cmd.AddCommand(newDesktopClientsRegisterCommand(deps))
	cmd.AddCommand(newDesktopClientsUnregisterCommand(deps))
	return cmd
}

func newDesktopClientsListCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use: windowManagerListKey, Short: "List connected client-local desktop views", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ListWindowManagerClients(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerClientListBundle(response.Clients))
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	return cmd
}

func newDesktopClientsRegisterCommand(deps commandDeps) *cobra.Command {
	var workspace, clientID, activeDesktop string
	cmd := &cobra.Command{
		Use: "register", Short: "Register or refresh a client-local desktop view", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			resolvedClientID, err := optionalWindowManagerID[windowmanager.ClientID](
				cmd,
				windowManagerClientFlag,
				clientID,
			)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			registration := contract.WindowManagerClientRegistration{
				WorkspaceID:     windowmanager.WorkspaceID(workspace),
				ActiveDesktopID: windowmanager.DesktopID(strings.TrimSpace(activeDesktop)),
			}
			if resolvedClientID != nil {
				registration.ClientID = *resolvedClientID
			}
			view, err := client.RegisterWindowManagerClient(
				cmd.Context(),
				workspace,
				registration,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				windowManagerJSONBundle("window_manager_client", "Window manager client", view),
			)
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	cmd.Flags().StringVar(&clientID, windowManagerClientFlag, "", "Stable client ID; generated when omitted")
	cmd.Flags().StringVar(&activeDesktop, "desktop", "", "Initial active desktop ID")
	return cmd
}

func newDesktopClientsUnregisterCommand(deps commandDeps) *cobra.Command {
	var workspace, clientID string
	cmd := &cobra.Command{
		Use: "unregister", Short: "Remove a connected client-local view", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			clientID, err := requiredWindowManagerFlag(clientID, windowManagerClientFlag)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.UnregisterWindowManagerClient(cmd.Context(), workspace, clientID); err != nil {
				return err
			}
			payload := struct {
				ClientID string `json:"client_id"`
				Removed  bool   `json:"removed"`
			}{ClientID: clientID, Removed: true}
			return writeCommandOutput(
				cmd,
				windowManagerJSONBundle("window_manager_client", "Window manager client", payload),
			)
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	cmd.Flags().StringVar(&clientID, windowManagerClientFlag, "", "Connected client ID")
	return cmd
}
