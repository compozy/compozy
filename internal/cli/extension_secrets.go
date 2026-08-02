package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const extensionSecretValueStdinFlag = "value-stdin"

type extensionSecretUnsetRecord struct {
	EnvName string `json:"env_name"`
	Status  string `json:"status"`
}

func newExtensionSecretsCommand(deps commandDeps) *cobra.Command {
	command := &cobra.Command{Use: "secrets", Short: "Manage write-only extension environment bindings"}
	command.AddCommand(newExtensionSecretsSetCommand(deps))
	command.AddCommand(newExtensionSecretsBindCommand(deps))
	command.AddCommand(newExtensionSecretsListCommand(deps))
	command.AddCommand(newExtensionSecretsUnsetCommand(deps))
	return command
}

func newExtensionSecretsSetCommand(deps commandDeps) *cobra.Command {
	var envName string
	var workspaceRef string
	var valueStdin bool
	command := &cobra.Command{
		Use:   "set <name>",
		Short: "Set one declared extension secret from hidden input",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName = strings.TrimSpace(envName)
			if envName == "" {
				return errors.New("cli: --env is required")
			}
			value, err := readExtensionSecretValue(cmd, deps, envName, valueStdin)
			if err != nil {
				return err
			}
			client, err := requireExtensionSecretsClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			payload, err := client.SetExtensionSecrets(
				cmd.Context(), args[0], workspaceRef,
				contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
					envName: {Value: &value},
				}},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSecretsBundle(payload))
		},
	}
	command.Flags().StringVar(&envName, "env", "", "Declared environment variable name")
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	command.Flags().BoolVar(&valueStdin, extensionSecretValueStdinFlag, false, "Read the secret value from stdin")
	return command
}

func newExtensionSecretsBindCommand(deps commandDeps) *cobra.Command {
	var envName string
	var vaultRef string
	var workspaceRef string
	command := &cobra.Command{
		Use:   "bind <name>",
		Short: "Bind one declared environment name to an existing extension Vault ref",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName = strings.TrimSpace(envName)
			vaultRef = strings.TrimSpace(vaultRef)
			if envName == "" || vaultRef == "" {
				return errors.New("cli: --env and --vault-ref are required")
			}
			client, err := requireExtensionSecretsClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			payload, err := client.SetExtensionSecrets(
				cmd.Context(), args[0], workspaceRef,
				contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
					envName: {VaultRef: &vaultRef},
				}},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSecretsBundle(payload))
		},
	}
	command.Flags().StringVar(&envName, "env", "", "Declared environment variable name")
	command.Flags().StringVar(&vaultRef, "vault-ref", "", "Existing vault:extensions/... reference")
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	return command
}

func newExtensionSecretsListCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	command := &cobra.Command{
		Use:   "list <name>",
		Short: "List declared and bound environment names without values",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireExtensionSecretsClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			payload, err := client.ListExtensionSecrets(cmd.Context(), args[0], workspaceRef)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSecretsBundle(payload))
		},
	}
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	return command
}

func newExtensionSecretsUnsetCommand(deps commandDeps) *cobra.Command {
	var envName string
	var workspaceRef string
	command := &cobra.Command{
		Use:   "unset <name>",
		Short: "Remove one extension environment binding",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName = strings.TrimSpace(envName)
			if envName == "" {
				return errors.New("cli: --env is required")
			}
			client, err := requireExtensionSecretsClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			if err := client.DeleteExtensionSecret(cmd.Context(), args[0], workspaceRef, envName); err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSecretUnsetBundle(extensionSecretUnsetRecord{
				EnvName: envName, Status: "unbound",
			}))
		},
	}
	command.Flags().StringVar(&envName, "env", "", "Bound environment variable name")
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	return command
}

func requireExtensionSecretsClient(
	ctx context.Context,
	deps commandDeps,
) (extensionSecretsClient, error) {
	base, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return nil, err
	}
	client, ok := base.(extensionSecretsClient)
	if !ok {
		return nil, errors.New("cli: extension secrets transport is unavailable")
	}
	return client, nil
}

func readExtensionSecretValue(
	cmd *cobra.Command,
	deps commandDeps,
	envName string,
	valueStdin bool,
) (string, error) {
	if valueStdin {
		return readVaultSecretStdin(cmd)
	}
	input := cmd.InOrStdin()
	if deps.inputIsTerminal == nil || !deps.inputIsTerminal(input) {
		return "", errors.New("cli: --value-stdin is required when stdin is not a terminal")
	}
	file, ok := input.(*os.File)
	if !ok {
		return "", errors.New("cli: terminal extension secret input must be an operating-system file")
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s: ", envName); err != nil {
		return "", fmt.Errorf("cli: write extension secret prompt: %w", err)
	}
	value, readErr := term.ReadPassword(int(file.Fd()))
	_, newlineErr := fmt.Fprintln(cmd.ErrOrStderr())
	if readErr != nil {
		return "", errors.Join(
			fmt.Errorf("cli: read hidden extension secret %q: %w", envName, readErr),
			newlineErr,
		)
	}
	if newlineErr != nil {
		return "", fmt.Errorf("cli: write hidden-input newline: %w", newlineErr)
	}
	result := strings.TrimRight(string(value), "\r\n")
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("cli: extension secret %q requires a non-blank value", envName)
	}
	return result, nil
}
