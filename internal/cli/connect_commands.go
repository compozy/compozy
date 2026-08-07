package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

func newConnectCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Manage CLI connection profiles",
		Long:  "Manage CLI connection profiles.\n\n" + gatewayOperationMatrix,
	}
	cmd.AddCommand(newConnectAddCommand(deps))
	cmd.AddCommand(newConnectListCommand(deps))
	cmd.AddCommand(newConnectExportCommand(deps))
	cmd.AddCommand(newConnectImportCommand(deps))
	cmd.AddCommand(newConnectUseCommand(deps))
	cmd.AddCommand(newConnectRemoveCommand(deps))
	cmd.AddCommand(newConnectSSHCommand(deps))
	return cmd
}

func newConnectAddCommand(deps commandDeps) *cobra.Command {
	var pairing string
	var options pairRedeemOptions
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a paired HTTPS connection profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = strings.TrimSpace(args[0])
			return redeemGatewayProfile(cmd, deps, strings.TrimSpace(pairing), options)
		},
	}
	cmd.Flags().StringVar(&pairing, "pairing", "", "Short-lived pairing artifact")
	cmd.Flags().StringVar(&options.address, "address", "", "Gateway HTTPS address")
	cmd.Flags().StringVar(&options.defaultWorkspace, "default-workspace", "", "Default remote workspace")
	cmd.Flags().BoolVar(&options.overwrite, "overwrite", false, "Replace an existing profile with this name")
	cmd.Flags().BoolVar(&options.use, "use", false, "Make the new profile active")
	mustMarkFlagRequired(cmd, "pairing")
	mustMarkFlagRequired(cmd, "address")
	return cmd
}

func newConnectListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   bridgeListKey,
		Short: "List connection profiles without reading credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (returnErr error) {
			homePaths, err := deps.resolveHome()
			if err != nil {
				return err
			}
			lock, gatewayConfig, err := acquireRecoveredGatewayProfileState(cmd.Context(), homePaths, deps)
			if err != nil {
				return err
			}
			defer func() {
				returnErr = errors.Join(returnErr, lock.Release())
			}()
			records := make([]gatewayProfileRecord, 0, len(gatewayConfig.Connections))
			for index := range gatewayConfig.Connections {
				records = append(
					records,
					gatewayProfileFromConfig(gatewayConfig.Connections[index], gatewayConfig.ActiveConnection),
				)
			}
			return writeCommandOutput(cmd, gatewayProfilesOutput(records))
		},
	}
}

func newConnectUseCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name|local>",
		Short: "Choose the default CLI target",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) (returnErr error) {
			name := strings.TrimSpace(args[0])
			homePaths, err := deps.resolveHome()
			if err != nil {
				return err
			}
			lock, gatewayConfig, err := acquireRecoveredGatewayProfileState(cmd.Context(), homePaths, deps)
			if err != nil {
				return err
			}
			defer func() {
				returnErr = errors.Join(returnErr, lock.Release())
			}()
			if name == clientTargetLocal {
				if err := deps.setActiveGatewayProfile(homePaths, ""); err != nil {
					return err
				}
				return writeCommandOutput(cmd, gatewayProfileOutput(gatewayProfileMutationRecord{
					Profile: gatewayProfileRecord{
						Name:   clientTargetLocal,
						Scheme: "unix",
						Active: true,
					},
					Status: memoryActiveKey,
				}))
			}
			profile, exists := findGatewayProfile(gatewayConfig.Connections, name)
			if !exists {
				return errors.New("cli: connection profile was not found")
			}
			if profile.Scheme == gatewaySchemeSSH {
				return errors.New("cli: SSH profiles are activated by `compozy connect ssh`")
			}
			if _, err := deps.readGatewayCredential(homePaths.GatewayCredentialsDir, name); err != nil {
				return err
			}
			if err := deps.setActiveGatewayProfile(homePaths, name); err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayProfileOutput(gatewayProfileMutationRecord{
				Profile: gatewayProfileFromConfig(profile, name), Status: memoryActiveKey,
			}))
		},
	}
}

func newConnectRemoveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   cliRemoveNameUse,
		Short: "Remove profile metadata and its client-local credential",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) (returnErr error) {
			name := strings.TrimSpace(args[0])
			homePaths, err := deps.resolveHome()
			if err != nil {
				return err
			}
			lock, gatewayConfig, err := acquireRecoveredGatewayProfileState(cmd.Context(), homePaths, deps)
			if err != nil {
				return err
			}
			defer func() {
				returnErr = errors.Join(returnErr, lock.Release())
			}()
			profile, exists := findGatewayProfile(gatewayConfig.Connections, name)
			if !exists {
				return errors.New("cli: connection profile was not found")
			}
			if err := removeGatewayProfileWithLock(homePaths, gatewayConfig, name, deps, lock); err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayProfileOutput(gatewayProfileMutationRecord{
				Profile: gatewayProfileFromConfig(profile, ""), Status: lifecycleRemovedKey,
			}))
		},
	}
}
