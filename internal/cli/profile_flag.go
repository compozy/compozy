package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	profileFlagName                 = "profile"
	profileEnvName                  = "COMPOZY_PROFILE"
	profileSelectionUnsupportedCode = "profile_selection_unsupported"
)

func configureRootProfileFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().String(profileFlagName, "", "Act as a profile for this command")
}

func configureProfileIndependentCompletionCommands(root *cobra.Command) {
	if root == nil {
		return
	}
	root.InitDefaultCompletionCmd()
	completion, _, err := root.Find([]string{completionCommandKey})
	if err != nil || completion == nil || completion == root {
		return
	}
	hideProfileFlag(completion, false)
	for _, command := range completion.Commands() {
		configureCompletionOutput(command)
		configureProfileIndependentFlag(command, "shell completion generation does not use a profile")
	}
}

func configureCompletionOutput(command *cobra.Command) {
	if command == nil {
		return
	}
	shell := command.Name()
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		noDescriptions, err := cmd.Flags().GetBool("no-descriptions")
		if err != nil {
			return fmt.Errorf("cli: read completion description flag: %w", err)
		}
		output := cmd.OutOrStdout()
		switch shell {
		case shellBashKey:
			return cmd.Root().GenBashCompletionV2(output, !noDescriptions)
		case "zsh":
			if noDescriptions {
				return cmd.Root().GenZshCompletionNoDesc(output)
			}
			return cmd.Root().GenZshCompletion(output)
		case "fish":
			return cmd.Root().GenFishCompletion(output, !noDescriptions)
		case "powershell":
			if noDescriptions {
				return cmd.Root().GenPowerShellCompletion(output)
			}
			return cmd.Root().GenPowerShellCompletionWithDesc(output)
		default:
			return fmt.Errorf("cli: unsupported completion shell %q", shell)
		}
	}
}

func commandProfileFlag(cmd *cobra.Command) (string, error) {
	if cmd == nil {
		return "", nil
	}
	return cmd.Flags().GetString(profileFlagName)
}

func rejectMachineProfileFlag(cmd *cobra.Command) error {
	if cmd == nil || !cmd.Flags().Changed(profileFlagName) {
		return nil
	}
	return newProfileSelectionError(
		profileSelectionUnsupportedCode,
		"this machine-wide command does not use a profile",
		"remove --profile and retry",
	)
}

func configureMachineProfileFlag(cmd *cobra.Command, persistent bool) {
	hideProfileFlag(cmd, persistent)
}

func configureProfileIndependentFlag(cmd *cobra.Command, reason string) {
	if cmd == nil {
		return
	}
	hideProfileFlag(cmd, false)
	previous := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(profileFlagName) {
			return newProfileSelectionError(
				profileSelectionUnsupportedCode,
				reason,
				"remove --profile and retry",
			)
		}
		if previous != nil {
			return previous(cmd, args)
		}
		return nil
	}
}

func hideProfileFlag(cmd *cobra.Command, persistent bool) {
	if cmd == nil {
		return
	}
	flags := cmd.Flags()
	if persistent {
		flags = cmd.PersistentFlags()
	}
	flags.String(profileFlagName, "", "")
	if err := flags.MarkHidden(profileFlagName); err != nil {
		panic("cli: hide machine profile flag: " + err.Error())
	}
}
