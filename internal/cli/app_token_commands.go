package cli

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	appTokenOriginFlag       = "origin"
	appTokenStateFlag        = "state"
	appTokenClearOriginsFlag = "clear-origins"
)

func newAppTokenCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app-token",
		Short: "Manage app access tokens",
		Long: "Manage tokens for browser and non-browser apps through the local daemon.\n\n" +
			"Experimental in 0.4: names and shapes may change in 0.5 without a deprecation window. " +
			"Pin the exact version.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			_, err := resolveOutputFormat(cmd)
			return withCommandExitCode(2, err)
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return withCommandExitCode(2, err)
	})
	cmd.AddCommand(
		newAppTokenCreateCommand(deps),
		newAppTokenListCommand(deps),
		newAppTokenUpdateCommand(deps),
		newAppTokenRevokeCommand(deps),
	)
	return cmd
}

func appTokenCommandArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return withCommandExitCode(2, validate(cmd, args))
	}
}

func newAppTokenCreateCommand(deps commandDeps) *cobra.Command {
	var origins []string
	cmd := &cobra.Command{
		Use:   cliNamedCreateUse,
		Short: "Create an app token and show its secret once",
		Args:  appTokenCommandArgs(exactOneNonBlankArg()),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := appTokenClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.CreateAppToken(cmd.Context(), contract.CreateAppTokenRequest{
				Name: strings.TrimSpace(args[0]), Origins: trimNonEmptyStrings(origins),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, appTokenIssuedOutput(result))
		},
	}
	cmd.Flags().StringArrayVar(&origins, appTokenOriginFlag, nil, "Allowed browser origin (repeatable)")
	return cmd
}

func newAppTokenListCommand(deps commandDeps) *cobra.Command {
	var state string
	cmd := &cobra.Command{
		Use:   cliOutputListKey,
		Short: "List app tokens without their secrets",
		Args:  appTokenCommandArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch state {
			case "active", gatewayRevokedState, "all":
			default:
				return newCLIUsageError("--state must be active, revoked, or all")
			}
			client, err := appTokenClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.ListAppTokens(cmd.Context(), state)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, appTokensOutput(result, deps.now))
		},
	}
	cmd.Flags().StringVar(&state, appTokenStateFlag, "active", "Token state: active, revoked, or all")
	return cmd
}

func newAppTokenUpdateCommand(deps commandDeps) *cobra.Command {
	var name string
	var origins []string
	var clearOrigins bool
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an app token name or allowed browser origins",
		Args:  appTokenCommandArgs(exactOneNonBlankArg()),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clearOrigins && cmd.Flags().Changed(appTokenOriginFlag) {
				return newCLIUsageError("--clear-origins cannot be combined with --origin")
			}
			request := contract.UpdateAppTokenRequest{}
			if cmd.Flags().Changed(automationNameKey) {
				request.Name = new(strings.TrimSpace(name))
			}
			if clearOrigins {
				request.Origins = new([]string{})
			} else if cmd.Flags().Changed(appTokenOriginFlag) {
				request.Origins = new(trimNonEmptyStrings(origins))
			}
			client, err := appTokenClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.UpdateAppToken(cmd.Context(), strings.TrimSpace(args[0]), request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, appTokenUpdatedOutput(result))
		},
	}
	cmd.Flags().StringVar(&name, automationNameKey, "", "New app token name")
	cmd.Flags().StringArrayVar(&origins, appTokenOriginFlag, nil, "Replace allowed browser origins (repeatable)")
	cmd.Flags().BoolVar(&clearOrigins, appTokenClearOriginsFlag, false, "Remove all allowed browser origins")
	return cmd
}

func newAppTokenRevokeCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an app token and close its live streams",
		Args:  appTokenCommandArgs(exactOneNonBlankArg()),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := appTokenClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.RevokeAppToken(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, appTokenRevokedOutput(result))
		},
	}
}
