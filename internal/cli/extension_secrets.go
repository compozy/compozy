package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/mcppolicy"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	extensionSecretValueStdinFlag   = "value-stdin"
	extensionSecretRemoteHeaderFlag = "remote-header"
)

type extensionSecretReader struct {
	input        io.Reader
	output       io.Writer
	isTerminal   func(io.Reader) bool
	readPassword func(int) ([]byte, error)
}

func newExtensionSecretReader(
	input io.Reader,
	output io.Writer,
	isTerminal func(io.Reader) bool,
) *extensionSecretReader {
	return &extensionSecretReader{
		input: input, output: output, isTerminal: isTerminal, readPassword: term.ReadPassword,
	}
}

func (r *extensionSecretReader) Read(envName string) (string, error) {
	if r.isTerminal == nil || !r.isTerminal(r.input) {
		return "", errors.New("cli: --value-stdin is required when stdin is not a terminal")
	}
	file, ok := r.input.(*os.File)
	if !ok {
		return "", errors.New("cli: terminal extension secret input must be an operating-system file")
	}
	if _, err := fmt.Fprintf(r.output, "%s: ", envName); err != nil {
		return "", fmt.Errorf("cli: write extension secret prompt: %w", err)
	}
	value, readErr := r.readPassword(int(file.Fd()))
	_, newlineErr := fmt.Fprintln(r.output)
	if newlineErr != nil {
		newlineErr = fmt.Errorf("cli: write hidden-input newline: %w", newlineErr)
	}
	if readErr != nil {
		return "", errors.Join(
			fmt.Errorf("cli: read hidden extension secret %q: %w", envName, readErr),
			newlineErr,
		)
	}
	if newlineErr != nil {
		return "", newlineErr
	}
	result := strings.TrimRight(string(value), "\r\n")
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("cli: extension secret %q requires a non-blank value", envName)
	}
	return result, nil
}

type extensionSecretUnsetRecord struct {
	EnvName string `json:"env_name"`
	Status  string `json:"status"`
}

func newExtensionSecretsCommand(deps commandDeps) *cobra.Command {
	command := &cobra.Command{Use: extensionSecretsKey, Short: "Manage write-only extension environment bindings"}
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
				contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{{
					EnvName: envName, Value: &value,
				}}},
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
	configureProfileMutationCommand(command, deps)
	return command
}

func newExtensionSecretsBindCommand(deps commandDeps) *cobra.Command {
	var envName string
	var vaultRef string
	var workspaceRef string
	var remoteHeader string
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
			mcpServer, headerName, err := parseExtensionRemoteHeader(remoteHeader)
			if err != nil {
				return err
			}
			client, err := requireExtensionSecretsClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			payload, err := client.SetExtensionSecrets(
				cmd.Context(), args[0], workspaceRef,
				contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{{
					EnvName: envName, VaultRef: &vaultRef, MCPServer: mcpServer, HeaderName: headerName,
				}}},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSecretsBundle(payload))
		},
	}
	command.Flags().StringVar(&envName, "env", "", "Declared environment variable name")
	command.Flags().StringVar(&vaultRef, "vault-ref", "", "Existing vault:extensions/... reference")
	command.Flags().StringVar(
		&remoteHeader,
		extensionSecretRemoteHeaderFlag,
		"",
		"Bind to one remote MCP server header as <server>:<header>",
	)
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	configureProfileMutationCommand(command, deps)
	return command
}

func parseExtensionRemoteHeader(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", nil
	}
	server, header, found := strings.Cut(trimmed, ":")
	server = strings.TrimSpace(server)
	header = strings.TrimSpace(header)
	if !found {
		return "", "", errors.New("cli: --remote-header must be <server>:<header>")
	}
	if server == "" {
		return "", "", errors.New("cli: --remote-header server is required")
	}
	if header == "" {
		return "", "", errors.New("cli: --remote-header header is required")
	}
	if err := compozyconfig.ValidateMCPServerName(server); err != nil {
		return "", "", fmt.Errorf("cli: --remote-header server: %w", err)
	}
	if err := mcppolicy.ValidateHeaders(
		nil,
		map[string]string{header: "secret-reference"},
		mcppolicy.SourceOperatorSecret,
		false,
	); err != nil {
		return "", "", fmt.Errorf("cli: --remote-header header: %w", err)
	}
	return server, header, nil
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
	configureSingleProfileReadCommand(command, deps)
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
	configureProfileMutationCommand(command, deps)
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
	return newExtensionSecretReader(cmd.InOrStdin(), cmd.ErrOrStderr(), deps.inputIsTerminal).Read(envName)
}
